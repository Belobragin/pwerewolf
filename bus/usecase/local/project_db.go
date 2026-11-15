package local

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	dbase "github.com/belobragin/bvv_utils/goutils/database"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/filter"
	"github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/belobragin/werewolf/bus/pkg/model/diagnose_model"
	"github.com/belobragin/werewolf/bus/pkg/model/list_model"
	"github.com/belobragin/werewolf/bus/pkg/model/picture_model"
	stt "github.com/belobragin/werewolf/bus/pkg/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DbRealizationLocal struct {
	Db *dbase.ProjectPsqlGormDb
}

func (d *DbRealizationLocal) GetVersion() string {
	return d.Db.GetVersion()
}

func (d *DbRealizationLocal) CloseDb() error {
	return d.Db.CloseDb()
}

// builds structire with gorm database object. Musthave data in environment variables, relevant to database configure
func BuildGormDbS() (*DbRealizationLocal, error) {
	var err []error
	db := new(DbRealizationLocal)
	db.Db = new(dbase.ProjectPsqlGormDb)
	if e := db.Db.ConfigureDb(); e != nil {
		err = append(err, e)
	}
	if len(err) != 0 {
		return nil, errors.Join(err...)
	}
	return db, nil
}

func (d *DbRealizationLocal) getDB() *gorm.DB {
	if d.Db == nil {
		return nil
	}
	return d.Db.DB
}

func (d *DbRealizationLocal) DbPostDiagnoseObject(ctx context.Context, modelData *diagnose_model.DiagnosePostReply) error {
	if modelData == nil {
		return errors.New("diagnose post data is required")
	}
	predict := modelData.ResnetReply.Predict
	if len(predict) == 0 {
		return diagnose_model.ErrEmptyPrediction
	}

	return d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return upsertDiagnoseResult(tx, diagnoseOwnerImage, modelData.ImageID, predict)
	})
}

// the column of wfn.diagnose_result that holds the owner of the predictions; exactly one of them is set in a row
const (
	diagnoseOwnerImage       = "image_uuid"
	diagnoseOwnerAdversarial = "adversarial_object_uuid"
)

// diagnoseOwnerColumn finds out whether the uuid is an image or an adversarial object.
func diagnoseOwnerColumn(tx *gorm.DB, id uuid.UUID) (string, error) {
	for _, o := range []struct{ table, column string }{
		{"wfn.image", diagnoseOwnerImage},
		{"wfn.adversarial_object", diagnoseOwnerAdversarial},
	} {
		var count int64
		if err := tx.Table(o.table).Where(o.column+" = ?", id).Count(&count).Error; err != nil {
			return "", err
		}
		if count > 0 {
			return o.column, nil
		}
	}
	return "", pdata.ErrNoObjectId
}

// upsertDiagnoseResult must be called with the *gorm.DB of an already-open transaction and a non-empty predict.
// ownerColumn is diagnoseOwnerImage or diagnoseOwnerAdversarial.
func upsertDiagnoseResult(tx *gorm.DB, ownerColumn string, ownerID uuid.UUID, predict []map[string]diagnose_model.ResnetPredictionData) error {
	rows := make([]map[string]any, 0, len(predict))
	for rank, prediction := range predict {
		result, err := json.Marshal(prediction)
		if err != nil {
			return err
		}
		rows = append(rows, map[string]any{
			ownerColumn:       ownerID,
			"prediction_rank": rank,
			"result":          gorm.Expr("?::jsonb", string(result)),
		})
	}

	// a repeated post replaces the previous predictions of the owner, keeping created_at of surviving ranks.
	// The unique indexes are partial, so the conflict target must repeat their predicate.
	if err := tx.Clauses(clause.OnConflict{
		Columns:     []clause.Column{{Name: ownerColumn}, {Name: "prediction_rank"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: ownerColumn + " IS NOT NULL"}}},
		DoUpdates: clause.Assignments(map[string]any{
			"result":     gorm.Expr("excluded.result"),
			"updated_at": gorm.Expr("now()"),
		}),
	}).Table("wfn.diagnose_result").Create(&rows).Error; err != nil {
		return err
	}
	return tx.Exec("DELETE FROM wfn.diagnose_result WHERE "+ownerColumn+" = ? AND prediction_rank >= ?",
		ownerID, len(predict)).Error
}

func (d *DbRealizationLocal) DbGetDiagnoseObject(ctx context.Context, objectId uuid.UUID) (*diagnose_model.DiagnoseGetReply, error) {
	type imageRow struct {
		ImageID   uuid.UUID `gorm:"column:image_uuid"`
		ImageName string    `gorm:"column:image_name"`
	}
	type resultRow struct {
		Result    json.RawMessage `gorm:"column:result"`
		CreatedAt time.Time       `gorm:"column:created_at"`
		UpdatedAt time.Time       `gorm:"column:updated_at"`
	}

	db := d.getDB().WithContext(ctx)
	image := imageRow{}
	if err := db.Table("wfn.image").
		Select("image_uuid, image_name").
		Where("image_uuid = ?", objectId).
		Take(&image).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pdata.ErrNoObjectId
		}
		return nil, err
	}

	var results []resultRow
	if err := db.Table("wfn.diagnose_result").
		Select("result, created_at, updated_at").
		Where("image_uuid = ?", objectId).
		Order("prediction_rank").
		Find(&results).Error; err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, pdata.ErrNoResultInDb
	}

	output := &diagnose_model.DiagnoseGetReply{
		ImageID:   image.ImageID,
		ImageName: image.ImageName,
	}
	createdAt, updatedAt := results[0].CreatedAt, results[0].UpdatedAt
	output.DbResultReply.Predict = make([]map[string]diagnose_model.ResnetPredictionData, 0, len(results))
	for _, r := range results {
		prediction := map[string]diagnose_model.ResnetPredictionData{}
		if err := json.Unmarshal(r.Result, &prediction); err != nil {
			return nil, err
		}
		output.DbResultReply.Predict = append(output.DbResultReply.Predict, prediction)
		if r.CreatedAt.Before(createdAt) {
			createdAt = r.CreatedAt
		}
		if r.UpdatedAt.After(updatedAt) {
			updatedAt = r.UpdatedAt
		}
	}
	output.DbResultReply.CreatedAt = createdAt.Format(time.RFC3339Nano)
	output.DbResultReply.UpdatedAt = updatedAt.Format(time.RFC3339Nano)

	return output, nil
}

func (d *DbRealizationLocal) DbUploadImage(ctx context.Context, modelData *picture_model.UploadReply) error {
	if modelData == nil {
		return errors.New("upload picture data is required")
	}

	return d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64

		if err := tx.Table("wfn.image").Where("image_uuid = ?", modelData.ImageID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return pdata.ErrExistObjectID
		}

		if err := tx.Table("wfn.image").Where("image_name = ?", modelData.ImageName).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return pdata.ErrExistObjectName
		}

		row := map[string]any{
			"image_uuid":   modelData.ImageID,
			"image_name":   modelData.ImageName,
			"mime_type":    modelData.MimeType,
			"image_size":   modelData.ImageSize,
			"image_height": modelData.ImageHeight,
			"image_width":  modelData.ImageWidth,
		}
		// an empty description stays NULL in the table
		if modelData.ImageDescription != "" {
			row["image_description"] = modelData.ImageDescription
		}
		return tx.Table("wfn.image").Create(row).Error
	})
}

// get data from database  about downloaded object
func (d *DbRealizationLocal) DbDownloadImage(ctx context.Context, objectId uuid.UUID) (*picture_model.DownloadObjectReply, error) {
	type imageRow struct {
		ImageName string    `gorm:"column:image_name"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	type adversarialRow struct {
		AdversarialObjectName string    `gorm:"column:adversarial_object_name"`
		CreatedAt             time.Time `gorm:"column:created_at"`
	}
	type filterRow struct {
		FilterObjectName string    `gorm:"column:filter_object_name"`
		CreatedAt        time.Time `gorm:"column:created_at"`
	}

	db := d.getDB().WithContext(ctx)

	image := imageRow{}
	err := db.Table("wfn.image").
		Select("image_name, created_at").
		Where("image_uuid = ?", objectId).
		Take(&image).Error
	if err == nil {
		return &picture_model.DownloadObjectReply{
			ObjectID:   objectId,
			IfImage:    true,
			ObjectName: image.ImageName,
			CreatedAt:  image.CreatedAt.Format(time.RFC3339Nano),
		}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	adversarial := adversarialRow{}
	err = db.Table("wfn.adversarial_object").
		Select("adversarial_object_name, created_at").
		Where("adversarial_object_uuid = ?", objectId).
		Take(&adversarial).Error
	if err == nil {
		return &picture_model.DownloadObjectReply{
			ObjectID:      objectId,
			IfAdversarial: true,
			ObjectName:    adversarial.AdversarialObjectName,
			CreatedAt:     adversarial.CreatedAt.Format(time.RFC3339Nano),
		}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	filterObject := filterRow{}
	if err := db.Table("wfn.filter_object").
		Select("filter_object_name, created_at").
		Where("filter_object_uuid = ?", objectId).
		Take(&filterObject).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pdata.ErrNoObjectId
		}
		return nil, err
	}

	return &picture_model.DownloadObjectReply{
		ObjectID:   objectId,
		IfFilter:   true,
		ObjectName: filterObject.FilterObjectName,
		CreatedAt:  filterObject.CreatedAt.Format(time.RFC3339Nano),
	}, nil
}

// DbGetImageUuidByName returns image_uuid of the object with the given name; kind is "image", "adversarial" or "filter".
func (d *DbRealizationLocal) DbGetImageUuidByName(ctx context.Context, imageName string, kind string) (uuid.UUID, error) {
	db := d.getDB().WithContext(ctx)

	var query *gorm.DB
	switch kind {
	case "image":
		query = db.Table("wfn.image").
			Select("image_uuid").
			Where("image_name = ?", imageName)
	case "adversarial":
		query = db.Table("wfn.adversarial_object").
			Select("image_uuid").
			Where("adversarial_object_name = ?", imageName)
	case "filter":
		// filter_object has no image_uuid of its own, so it is resolved through its adversarial object.
		query = db.Table("wfn.filter_object AS f").
			Select("a.image_uuid").
			Joins("JOIN wfn.adversarial_object AS a ON a.adversarial_object_uuid = f.adversarial_object_uuid").
			Where("f.filter_object_name = ?", imageName)
	default:
		return uuid.Nil, pdata.ErrInvalidObjectKind
	}

	row := struct {
		ImageID uuid.UUID `gorm:"column:image_uuid"`
	}{}
	if err := query.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, pdata.ErrNoObjectName
		}
		return uuid.Nil, err
	}
	return row.ImageID, nil
}

func (d *DbRealizationLocal) DbGetImageName(ctx context.Context, imageID uuid.UUID) (string, error) {
	row := struct {
		ImageName string `gorm:"column:image_name"`
	}{}
	if err := d.getDB().WithContext(ctx).Table("wfn.image").
		Select("image_name").
		Where("image_uuid = ?", imageID).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", pdata.ErrNoObjectId
		}
		return "", err
	}
	return row.ImageName, nil
}

// outputs the list of interconnected objects
func (d *DbRealizationLocal) DbListCascadeObjects(ctx context.Context, objectId uuid.UUID) ([]picture_model.ObjectKind, error) {
	type objectRow struct {
		ID        uuid.UUID `gorm:"column:id"`
		Name      string    `gorm:"column:name"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	db := d.getDB().WithContext(ctx)

	find := func(kind stt.ObjectKind, query string) ([]picture_model.ObjectKind, error) {
		var rows []objectRow
		if err := db.Raw(query, objectId).Scan(&rows).Error; err != nil {
			return nil, err
		}
		out := make([]picture_model.ObjectKind, 0, len(rows))
		for _, r := range rows {
			out = append(out, picture_model.ObjectKind{
				ID: r.ID, Name: r.Name, Kind: kind, CreatedAt: r.CreatedAt.Format(time.RFC3339Nano),
			})
		}
		return out, nil
	}

	// the first step of each chain finds the object itself, the next steps find what is removed with it
	type step struct {
		kind  stt.ObjectKind
		query string
	}
	chains := [][]step{
		{ // image
			{stt.KindImage, `SELECT image_uuid AS id, image_name AS name, created_at FROM wfn.image WHERE image_uuid = ?`},
			{stt.KindAdversarial, `SELECT adversarial_object_uuid AS id, adversarial_object_name AS name, created_at
				FROM wfn.adversarial_object WHERE image_uuid = ? ORDER BY created_at`},
			{stt.KindFilter, `SELECT f.filter_object_uuid AS id, f.filter_object_name AS name, f.created_at
				FROM wfn.filter_object f JOIN wfn.adversarial_object a ON a.adversarial_object_uuid = f.adversarial_object_uuid
				WHERE a.image_uuid = ? ORDER BY f.created_at`},
		},
		{ // adversarial object
			{stt.KindAdversarial, `SELECT adversarial_object_uuid AS id, adversarial_object_name AS name, created_at
				FROM wfn.adversarial_object WHERE adversarial_object_uuid = ?`},
			{stt.KindFilter, `SELECT filter_object_uuid AS id, filter_object_name AS name, created_at
				FROM wfn.filter_object WHERE adversarial_object_uuid = ?`},
		},
		{ // filter
			{stt.KindFilter, `SELECT filter_object_uuid AS id, filter_object_name AS name, created_at
				FROM wfn.filter_object WHERE filter_object_uuid = ?`},
		},
	}
	for _, chain := range chains {
		all, err := find(chain[0].kind, chain[0].query)
		if err != nil {
			return nil, err
		}
		if len(all) == 0 {
			continue
		}
		for _, s := range chain[1:] {
			more, err := find(s.kind, s.query)
			if err != nil {
				return nil, err
			}
			all = append(all, more...)
		}
		return all, nil
	}
	return nil, pdata.ErrNoObjectId
}

// DbDeleteImage deletes the record; the foreign keys cascade to the dependent adversarial objects and filters.
func (d *DbRealizationLocal) DbDeleteImage(ctx context.Context, objectId uuid.UUID) error {
	db := d.getDB().WithContext(ctx)
	for _, q := range []string{
		"DELETE FROM wfn.image WHERE image_uuid = ?",
		"DELETE FROM wfn.adversarial_object WHERE adversarial_object_uuid = ?",
		"DELETE FROM wfn.filter_object WHERE filter_object_uuid = ?",
	} {
		res := db.Exec(q, objectId)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return nil
		}
	}
	return pdata.ErrNoObjectId
}

// dbCheckUniqueImageAlgo must be called with the *gorm.DB of an already-open transaction.
func (d *DbRealizationLocal) dbCheckUniqueImageAlgo(tx *gorm.DB, modelData *alter_model.AlterCheckUniqueImageAlgo) (bool, error) {
	if modelData == nil {
		return false, errors.New("check unique image algo data is required")
	}

	var count int64
	if err := tx.Table("wfn.adversarial_object").
		Where("image_uuid = ? AND algorythm = ?", modelData.ImageID, modelData.Algorythm).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count == 0 {
		return false, nil
	}

	if !modelData.WipeExistent {
		return true, pdata.ErrRepeatImageAlgo
	}

	if err := tx.Exec("DELETE FROM wfn.adversarial_object WHERE image_uuid = ? AND algorythm = ?", modelData.ImageID, modelData.Algorythm).Error; err != nil {
		return false, err
	}

	return true, nil
}

// DbAlterObject saves the ML result: the new adversarial object, its filter if there is one, and the predictions.
// The predictions are saved for predictionsImageID, an image or an adversarial object (the new one is inserted first,
// so its uuid is accepted); they are skipped when the result has none.
// A repeated result for the same image and algorythm replaces the old one; the old filter goes with the old adversarial object.
func (d *DbRealizationLocal) DbAlterObject(ctx context.Context, modelData *alter_model.AlterPostToDb, predictionsImageID uuid.UUID) error {
	if modelData == nil {
		return errors.New("alter post data is required")
	}
	if len(modelData.Predict) > 0 && predictionsImageID == uuid.Nil {
		return errors.New("image uuid for the predictions is required")
	}

	return d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Table("wfn.image").Where("image_uuid = ?", modelData.ImageID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return pdata.ErrNoObjectId
		}

		if _, err := d.dbCheckUniqueImageAlgo(tx, &alter_model.AlterCheckUniqueImageAlgo{
			ImageID:      modelData.ImageID,
			Algorythm:    modelData.Algorythm,
			WipeExistent: true,
		}); err != nil {
			return err
		}

		row := map[string]any{
			"adversarial_object_uuid": modelData.AdversarialObjectID,
			"adversarial_object_name": modelData.AdversarialName,
			"image_uuid":              modelData.ImageID,
			"with_filter_object":      modelData.HasFilter(),
			"algorythm":               modelData.Algorythm,
		}
		// an empty goal stays NULL in the table
		if modelData.AlterGoal != "" {
			row["alter_goal"] = modelData.AlterGoal
		}
		if err := tx.Table("wfn.adversarial_object").Create(row).Error; err != nil {
			return err
		}

		if modelData.HasFilter() {
			if err := tx.Table("wfn.filter_object").Create(map[string]any{
				"filter_object_uuid":      modelData.FilterObjectID,
				"filter_object_name":      modelData.FilterName,
				"adversarial_object_uuid": modelData.AdversarialObjectID,
			}).Error; err != nil {
				return err
			}
		}

		if len(modelData.Predict) == 0 {
			return nil
		}
		ownerColumn, err := diagnoseOwnerColumn(tx, predictionsImageID)
		if err != nil {
			return err
		}
		return upsertDiagnoseResult(tx, ownerColumn, predictionsImageID, modelData.Predict)
	})
}

func (d *DbRealizationLocal) DbGetAdvDataImageId(ctx context.Context, objectId uuid.UUID) (*alter_model.AlterGetReply, error) {
	type advRow struct {
		AdversarialObjectID   uuid.UUID `gorm:"column:adversarial_object_uuid"`
		AdversarialObjectName string    `gorm:"column:adversarial_object_name"`
		Algorythm             string    `gorm:"column:algorythm"`
		AlterGoal             string    `gorm:"column:alter_goal"`
		CreatedAt             time.Time `gorm:"column:created_at"`
	}
	type filterRow struct {
		FilterObjectID   uuid.UUID `gorm:"column:filter_object_uuid"`
		FilterObjectName string    `gorm:"column:filter_object_name"`
	}

	db := d.getDB().WithContext(ctx)

	var advRows []advRow
	if err := db.Table("wfn.adversarial_object").
		Select("adversarial_object_uuid, adversarial_object_name, algorythm, alter_goal, created_at").
		Where("image_uuid = ?", objectId).
		Find(&advRows).Error; err != nil {
		return nil, err
	}
	if len(advRows) == 0 {
		return nil, pdata.ErrNoObjectId
	}

	output := &alter_model.AlterGetReply{
		AltergetAllreply: make([]alter_model.AlterGetOneReply, 0, len(advRows)),
	}
	for _, adv := range advRows {
		item := alter_model.AlterGetOneReply{
			ImageID:               objectId,
			AdversarialObjectID:   adv.AdversarialObjectID,
			AdversarialObjectName: adv.AdversarialObjectName,
			Algorythm:             adv.Algorythm,
			AlterGoal:             adv.AlterGoal,
			CreatedAt:             adv.CreatedAt.Format(time.RFC3339Nano),
		}

		// filters are optional, so a missing row for this adversarial object is not an error.
		fRow := filterRow{}
		err := db.Table("wfn.filter_object").
			Select("filter_object_uuid, filter_object_name").
			Where("adversarial_object_uuid = ?", adv.AdversarialObjectID).
			Take(&fRow).Error
		if err == nil {
			item.FilterObjectID = fRow.FilterObjectID
			item.FilterName = fRow.FilterObjectName
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		output.AltergetAllreply = append(output.AltergetAllreply, item)
	}

	return output, nil
}

// DbListImage lists the objects the filter asks for, names accept the wildcards * and ?. The kind of the listed objects
// follows the most specific field that is set:
//   - only image fields (or none): every matching image, once, without its adversarial objects;
//   - an adversarial field (id, name, algorythm, alter_goal): the matching adversarial objects, each with its image and filter;
//   - a filter field (id, name): the matching filters, each with its adversarial object and image.
func (d *DbRealizationLocal) DbListImage(ctx context.Context, f filter.ImageFilter) (*list_model.ListImageReply, error) {
	isSet := func(value *string) bool { return value != nil && *value != "" }

	const allColumns = `i.image_uuid AS image_id, i.image_name AS image_name,
		a.adversarial_object_uuid AS adversarial_id, a.adversarial_object_name AS adversarial_name,
		a.algorythm::text AS algorythm, a.alter_goal AS alter_goal,
		f.filter_object_uuid AS filter_id, f.filter_object_name AS filter_name`
	db := d.getDB().WithContext(ctx)
	var query *gorm.DB
	switch {
	case f.FilterID != nil || isSet(f.FilterName):
		query = db.Table("wfn.filter_object AS f").Select(allColumns).
			Joins("JOIN wfn.adversarial_object AS a ON a.adversarial_object_uuid = f.adversarial_object_uuid").
			Joins("JOIN wfn.image AS i ON i.image_uuid = a.image_uuid").
			Order("i.created_at, i.image_uuid, a.created_at, f.created_at")
	case f.AdversarialID != nil || isSet(f.AdversarialName) || isSet(f.AlterGoal) || isSet(f.Algorythm):
		query = db.Table("wfn.adversarial_object AS a").Select(allColumns).
			Joins("JOIN wfn.image AS i ON i.image_uuid = a.image_uuid").
			Joins("LEFT JOIN wfn.filter_object AS f ON f.adversarial_object_uuid = a.adversarial_object_uuid").
			Order("i.created_at, i.image_uuid, a.created_at")
	default:
		query = db.Table("wfn.image AS i").Select("i.image_uuid AS image_id, i.image_name AS image_name").
			Order("i.created_at, i.image_uuid")
	}

	equal := func(column string, value *uuid.UUID) {
		if value != nil {
			query = query.Where(column+" = ?", *value)
		}
	}
	like := func(column string, value *string) {
		// an empty value means "not set", otherwise it would match nothing
		if isSet(value) {
			query = query.Where(column+" LIKE ?", likePattern(*value))
		}
	}
	equal("i.image_uuid", f.ImageID)
	equal("a.adversarial_object_uuid", f.AdversarialID)
	equal("f.filter_object_uuid", f.FilterID)
	like("i.image_name", f.ImageName)
	like("a.adversarial_object_name", f.AdversarialName)
	like("f.filter_object_name", f.FilterName)
	like("a.alter_goal", f.AlterGoal)
	if isSet(f.Algorythm) {
		query = query.Where("a.algorythm::text = ?", *f.Algorythm)
	}

	output := &list_model.ListImageReply{AllImage: []list_model.OneImageReply{}}
	if err := query.Scan(&output.AllImage).Error; err != nil {
		return nil, err
	}
	return output, nil
}
