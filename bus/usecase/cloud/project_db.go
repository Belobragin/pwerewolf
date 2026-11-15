package cloud

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/belobragin/bvv_utils/goutils/awsutil"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/database"
	"github.com/belobragin/werewolf/bus/pkg/filter"
	"github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/belobragin/werewolf/bus/pkg/model/diagnose_model"
	"github.com/belobragin/werewolf/bus/pkg/model/list_model"
	"github.com/belobragin/werewolf/bus/pkg/model/picture_model"
	stt "github.com/belobragin/werewolf/bus/pkg/storage"
	"github.com/google/uuid"
)

// the stack exports the table name under this key
const ddbTableNameOutput = "WerewolfDdbTableName"

var _ database.ProjectDbaseI = (*DbRealizationCloud)(nil)

// DbRealizationCloud keeps the data of the images, adversarial objects and filters in the one dynamodb table
// of the stack. See ddb_item.go for the layout of the rows.
type DbRealizationCloud struct {
	st store
}

// stackConfigI is the part of the cloud config that tells the stack with the table.
type stackConfigI interface {
	GetStackName() string
}

// BuildDynamoDbS resolves the table name from the outputs of the cfn stack.
func BuildDynamoDbS(conf stackConfigI) (*DbRealizationCloud, error) {
	ctx, cancel := context.WithTimeout(context.Background(), initTimeout)
	defer cancel()

	cfg, err := awscfg.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	table, err := awsutil.GetStackOutput(ctx, cfg, conf.GetStackName(), ddbTableNameOutput)
	if err != nil {
		return nil, err
	}
	return newCloudDb(&ddbStore{api: dynamodb.NewFromConfig(cfg), table: table}), nil
}

func newCloudDb(st store) *DbRealizationCloud {
	return &DbRealizationCloud{st: st}
}

func (d *DbRealizationCloud) GetVersion() string {
	return ""
}

func (d *DbRealizationCloud) CloseDb() error {
	return nil
}

// object returns the entity row of the uuid (nil if there is none) and the prediction rows it owns, by rank.
func (d *DbRealizationCloud) object(ctx context.Context, id uuid.UUID) (*item, []item, error) {
	rows, err := d.st.byUUID(ctx, id.String())
	if err != nil {
		return nil, nil, err
	}
	var (
		entity *item
		preds  []item
	)
	for i := range rows {
		switch {
		case rows[i].PredictionRank != nil:
			preds = append(preds, rows[i])
		case rows[i].kind() != "":
			e := rows[i]
			entity = &e
		}
	}
	sort.Slice(preds, func(i, j int) bool { return *preds[i].PredictionRank < *preds[j].PredictionRank })
	return entity, preds, nil
}

func (d *DbRealizationCloud) commit(ctx context.Context, w *writeSet) error {
	if err := d.st.write(ctx, w); err != nil {
		if errors.Is(err, errConflict) {
			return pdata.ErrExistObjectName
		}
		return err
	}
	return nil
}

// adversarialsOfImage are the adversarial objects of the image, sorted by created_at.
func (d *DbRealizationCloud) adversarialsOfImage(ctx context.Context, imageID string) ([]item, error) {
	rows, err := d.st.scanEntities(ctx, scanFilter{ImageUUID: imageID})
	if err != nil {
		return nil, err
	}
	advs := slices.DeleteFunc(rows, func(it item) bool { return it.kind() != stt.KindAdversarial })
	sortByCreated(advs)
	return advs, nil
}

// filtersOf are the filters of the adversarial objects, sorted by created_at.
func (d *DbRealizationCloud) filtersOf(ctx context.Context, advIDs []string) ([]item, error) {
	if len(advIDs) == 0 {
		return nil, nil
	}
	flts, err := d.st.scanEntities(ctx, scanFilter{AdversarialUUIDs: advIDs})
	if err != nil {
		return nil, err
	}
	sortByCreated(flts)
	return flts, nil
}

func adversarialIDs(advs []item) []string {
	ids := make([]string, len(advs))
	for i, a := range advs {
		ids[i] = a.AdversarialObjectUUID
	}
	return ids
}

// planDeleteAdversarials puts into w the rows of the adversarial objects: themselves, their predictions and their filters.
func (d *DbRealizationCloud) planDeleteAdversarials(ctx context.Context, w *writeSet, advIDs []string) error {
	for _, id := range advIDs {
		rows, err := d.st.byUUID(ctx, id)
		if err != nil {
			return err
		}
		for _, r := range rows {
			w.del(r.key())
		}
	}
	flts, err := d.filtersOf(ctx, advIDs)
	if err != nil {
		return err
	}
	for _, f := range flts {
		w.del(f.key())
	}
	return nil
}

func (d *DbRealizationCloud) DbPostDiagnoseObject(ctx context.Context, modelData *diagnose_model.DiagnosePostReply) error {
	if modelData == nil {
		return errors.New("diagnose post data is required")
	}
	predict := modelData.ResnetReply.Predict
	if len(predict) == 0 {
		return diagnose_model.ErrEmptyPrediction
	}

	img, preds, err := d.object(ctx, modelData.ImageID)
	if err != nil {
		return err
	}
	if img == nil || img.kind() != stt.KindImage {
		return pdata.ErrNoObjectId
	}
	w := newWriteSet()
	planPredictions(w, *img, preds, predict, nowString())
	return d.commit(ctx, w)
}

func (d *DbRealizationCloud) DbGetDiagnoseObject(ctx context.Context, objectId uuid.UUID) (*diagnose_model.DiagnoseGetReply, error) {
	img, preds, err := d.object(ctx, objectId)
	if err != nil {
		return nil, err
	}
	// only the image rows are searched, so the uuid of an adversarial object or of a filter is unknown
	if img == nil || img.kind() != stt.KindImage {
		return nil, pdata.ErrNoObjectId
	}
	if len(preds) == 0 {
		return nil, pdata.ErrNoResultInDb
	}

	output := &diagnose_model.DiagnoseGetReply{ImageID: objectId, ImageName: img.ImageName}
	createdAt, updatedAt := preds[0].CreatedAt, preds[0].UpdatedAt
	output.DbResultReply.Predict = make([]map[string]diagnose_model.ResnetPredictionData, 0, len(preds))
	for _, p := range preds {
		output.DbResultReply.Predict = append(output.DbResultReply.Predict, p.DiagnoseResult)
		createdAt, updatedAt = min(createdAt, p.CreatedAt), max(updatedAt, p.UpdatedAt)
	}
	output.DbResultReply.CreatedAt = createdAt
	output.DbResultReply.UpdatedAt = updatedAt
	return output, nil
}

func (d *DbRealizationCloud) DbUploadImage(ctx context.Context, modelData *picture_model.UploadReply) error {
	if modelData == nil {
		return errors.New("upload picture data is required")
	}

	existing, _, err := d.object(ctx, modelData.ImageID)
	if err != nil {
		return err
	}
	if existing != nil {
		return pdata.ErrExistObjectID
	}

	// the put fails when the name is taken by another image
	w := newWriteSet()
	w.put(newImageItem(modelData, nowString()), true)
	return d.commit(ctx, w)
}

// get data from database about downloaded object
func (d *DbRealizationCloud) DbDownloadImage(ctx context.Context, objectId uuid.UUID) (*picture_model.DownloadObjectReply, error) {
	ent, _, err := d.object(ctx, objectId)
	if err != nil {
		return nil, err
	}
	if ent == nil {
		return nil, pdata.ErrNoObjectId
	}

	reply := &picture_model.DownloadObjectReply{ObjectID: objectId, ObjectName: ent.ImageObjectName, CreatedAt: ent.CreatedAt}
	switch ent.kind() {
	case stt.KindImage:
		reply.IfImage = true
	case stt.KindAdversarial:
		reply.IfAdversarial = true
	case stt.KindFilter:
		reply.IfFilter = true
	}
	return reply, nil
}

// DbGetImageUuidByName returns image_uuid of the object with the given name; kind is "image", "adversarial" or "filter".
func (d *DbRealizationCloud) DbGetImageUuidByName(ctx context.Context, imageName string, kind string) (uuid.UUID, error) {
	k := stt.ObjectKind(kind)
	if k != stt.KindImage && k != stt.KindAdversarial && k != stt.KindFilter {
		return uuid.Nil, pdata.ErrInvalidObjectKind
	}

	rows, err := d.st.byName(ctx, imageName, true)
	if err != nil {
		return uuid.Nil, err
	}
	i := slices.IndexFunc(rows, func(it item) bool { return it.kind() == k })
	if i < 0 {
		return uuid.Nil, pdata.ErrNoObjectName
	}
	row := rows[i]

	imageID := row.ImageUUID
	if k == stt.KindFilter {
		// the filter has no image_uuid of its own, so it is resolved through its adversarial object.
		advID, err := uuid.Parse(row.AdversarialUUID)
		if err != nil {
			return uuid.Nil, err
		}
		adv, _, err := d.object(ctx, advID)
		if err != nil {
			return uuid.Nil, err
		}
		if adv == nil || adv.kind() != stt.KindAdversarial {
			return uuid.Nil, pdata.ErrNoObjectName
		}
		imageID = adv.ImageUUID
	}
	return uuid.Parse(imageID)
}

func (d *DbRealizationCloud) DbGetImageName(ctx context.Context, imageID uuid.UUID) (string, error) {
	img, _, err := d.object(ctx, imageID)
	if err != nil {
		return "", err
	}
	if img == nil || img.kind() != stt.KindImage {
		return "", pdata.ErrNoObjectId
	}
	return img.ImageName, nil
}

func objectKind(it item) (picture_model.ObjectKind, error) {
	id, err := uuid.Parse(it.ImageObjectUUID)
	if err != nil {
		return picture_model.ObjectKind{}, err
	}
	return picture_model.ObjectKind{ID: id, Name: it.ImageObjectName, Kind: it.kind(), CreatedAt: it.CreatedAt}, nil
}

// outputs the list of interconnected objects
func (d *DbRealizationCloud) DbListCascadeObjects(ctx context.Context, objectId uuid.UUID) ([]picture_model.ObjectKind, error) {
	ent, _, err := d.object(ctx, objectId)
	if err != nil {
		return nil, err
	}
	if ent == nil {
		return nil, pdata.ErrNoObjectId
	}

	// the object itself, then what is removed with it: adversarial objects, then filters
	related := []item{*ent}
	var advIDs []string
	switch ent.kind() {
	case stt.KindImage:
		advs, err := d.adversarialsOfImage(ctx, ent.ImageUUID)
		if err != nil {
			return nil, err
		}
		related = append(related, advs...)
		advIDs = adversarialIDs(advs)
	case stt.KindAdversarial:
		advIDs = []string{ent.AdversarialObjectUUID}
	}
	flts, err := d.filtersOf(ctx, advIDs)
	if err != nil {
		return nil, err
	}
	related = append(related, flts...)

	out := make([]picture_model.ObjectKind, 0, len(related))
	for _, it := range related {
		o, err := objectKind(it)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

// DbDeleteImage deletes the object with everything that goes with it: the adversarial objects of an image,
// the filters of an adversarial object and the predictions.
func (d *DbRealizationCloud) DbDeleteImage(ctx context.Context, objectId uuid.UUID) error {
	ent, preds, err := d.object(ctx, objectId)
	if err != nil {
		return err
	}
	if ent == nil {
		return pdata.ErrNoObjectId
	}

	w := newWriteSet()
	w.del(ent.key())
	for _, p := range preds {
		w.del(p.key())
	}
	switch ent.kind() {
	case stt.KindImage:
		advs, err := d.adversarialsOfImage(ctx, ent.ImageUUID)
		if err != nil {
			return err
		}
		err = d.planDeleteAdversarials(ctx, w, adversarialIDs(advs))
		if err != nil {
			return err
		}
	case stt.KindAdversarial:
		if err = d.planDeleteAdversarials(ctx, w, []string{ent.AdversarialObjectUUID}); err != nil {
			return err
		}
	}
	return d.commit(ctx, w)
}

// DbAlterObject saves the ML result: the new adversarial object, its filter if there is one, and the predictions.
// The predictions are saved for predictionsImageID, an image or an adversarial object (the new one is accepted);
// they are skipped when the result has none.
// A repeated result for the same image and algorythm replaces the old one; the old filter goes with the old adversarial object.
func (d *DbRealizationCloud) DbAlterObject(ctx context.Context, modelData *alter_model.AlterPostToDb, predictionsImageID uuid.UUID) error {
	if modelData == nil {
		return errors.New("alter post data is required")
	}
	if len(modelData.Predict) > 0 && predictionsImageID == uuid.Nil {
		return errors.New("image uuid for the predictions is required")
	}

	img, _, err := d.object(ctx, modelData.ImageID)
	if err != nil {
		return err
	}
	if img == nil || img.kind() != stt.KindImage {
		return pdata.ErrNoObjectId
	}

	// the deletes go first: a put of the same key replaces its delete
	advs, err := d.adversarialsOfImage(ctx, img.ImageUUID)
	if err != nil {
		return err
	}
	var wiped []string
	for _, a := range advs {
		if a.Algorythm == modelData.Algorythm {
			wiped = append(wiped, a.AdversarialObjectUUID)
		}
	}
	w := newWriteSet()
	if err = d.planDeleteAdversarials(ctx, w, wiped); err != nil {
		return err
	}

	now := nowString()
	adv := newAdversarialItem(modelData, now)
	w.put(adv, true)
	if modelData.HasFilter() {
		w.put(newFilterItem(modelData, now), true)
	}

	if len(modelData.Predict) > 0 {
		owner, existing := adv, []item(nil)
		if predictionsImageID != modelData.AdversarialObjectID {
			o, preds, err := d.object(ctx, predictionsImageID)
			if err != nil {
				return err
			}
			if o == nil || (o.kind() != stt.KindImage && o.kind() != stt.KindAdversarial) || slices.Contains(wiped, o.ImageObjectUUID) {
				return pdata.ErrNoObjectId
			}
			owner, existing = *o, preds
		}
		planPredictions(w, owner, existing, modelData.Predict, now)
	}
	return d.commit(ctx, w)
}

func (d *DbRealizationCloud) DbGetAdvDataImageId(ctx context.Context, objectId uuid.UUID) (*alter_model.AlterGetReply, error) {
	advs, err := d.adversarialsOfImage(ctx, objectId.String())
	if err != nil {
		return nil, err
	}
	if len(advs) == 0 {
		return nil, pdata.ErrNoObjectId
	}
	// filters are optional, so an adversarial object without one is not an error.
	flts, err := d.filtersOf(ctx, adversarialIDs(advs))
	if err != nil {
		return nil, err
	}
	filterOf := make(map[string]item, len(flts))
	for _, f := range flts {
		filterOf[f.AdversarialUUID] = f
	}

	output := &alter_model.AlterGetReply{AltergetAllreply: make([]alter_model.AlterGetOneReply, 0, len(advs))}
	for _, a := range advs {
		advID, err := uuid.Parse(a.AdversarialObjectUUID)
		if err != nil {
			return nil, err
		}
		one := alter_model.AlterGetOneReply{
			ImageID:               objectId,
			AdversarialObjectID:   advID,
			AdversarialObjectName: a.AdversarialObjectName,
			Algorythm:             a.Algorythm,
			AlterGoal:             a.AlterGoal,
			CreatedAt:             a.CreatedAt,
		}
		if f, ok := filterOf[a.AdversarialObjectUUID]; ok {
			if one.FilterObjectID, err = uuid.Parse(f.FilterObjectUUID); err != nil {
				return nil, err
			}
			one.FilterName = f.FilterObjectName
		}
		output.AltergetAllreply = append(output.AltergetAllreply, one)
	}
	return output, nil
}

// wildcardMatch: * is any text, ? is any single character; the other characters match themselves.
func wildcardMatch(pattern, s string) bool {
	p, t := []rune(pattern), []rune(s)
	pi, ti, star, mark := 0, 0, -1, 0
	for ti < len(t) {
		switch {
		case pi < len(p) && p[pi] != '*' && (p[pi] == '?' || p[pi] == t[ti]):
			pi++
			ti++
		case pi < len(p) && p[pi] == '*':
			star, mark = pi, ti
			pi++
		case star >= 0:
			pi = star + 1
			mark++
			ti = mark
		default:
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func idPtr(s string, err *error) *uuid.UUID {
	id, e := uuid.Parse(s)
	if e != nil {
		*err = e
		return nil
	}
	return &id
}

// DbListImage lists the objects the filter asks for, names accept the wildcards * and ?. The kind of the listed objects
// follows the most specific field that is set:
//   - only image fields (or none): every matching image, once, without its adversarial objects;
//   - an adversarial field (id, name, algorythm, alter_goal): the matching adversarial objects, each with its image and filter;
//   - a filter field (id, name): the matching filters, each with its adversarial object and image.
//
// The order is by created_at of the image, its uuid, created_at of the adversarial object and of the filter.
func (d *DbRealizationCloud) DbListImage(ctx context.Context, f filter.ImageFilter) (*list_model.ListImageReply, error) {
	isSet := func(value *string) bool { return value != nil && *value != "" }
	idOk := func(want *uuid.UUID, got string) bool { return want == nil || want.String() == got }
	nameOk := func(want *string, got string) bool { return !isSet(want) || wildcardMatch(*want, got) }

	ents, err := d.st.scanEntities(ctx, scanFilter{})
	if err != nil {
		return nil, err
	}
	images := map[string]item{}
	advs := map[string]item{}
	filterOf := map[string]item{}
	var imageList, advList, fltList []item
	for _, e := range ents {
		switch e.kind() {
		case stt.KindImage:
			images[e.ImageUUID] = e
			imageList = append(imageList, e)
		case stt.KindAdversarial:
			advs[e.AdversarialObjectUUID] = e
			advList = append(advList, e)
		case stt.KindFilter:
			filterOf[e.AdversarialUUID] = e
			fltList = append(fltList, e)
		}
	}
	imageOk := func(i item) bool { return idOk(f.ImageID, i.ImageUUID) && nameOk(f.ImageName, i.ImageName) }
	advOk := func(a item) bool {
		return idOk(f.AdversarialID, a.AdversarialObjectUUID) && nameOk(f.AdversarialName, a.AdversarialObjectName) &&
			nameOk(f.AlterGoal, a.AlterGoal) && (!isSet(f.Algorythm) || a.Algorythm == *f.Algorythm)
	}
	fltOk := func(x item) bool {
		return idOk(f.FilterID, x.FilterObjectUUID) && nameOk(f.FilterName, x.FilterObjectName)
	}

	type listed struct {
		row  list_model.OneImageReply
		sort [4]string
	}
	var rows []listed
	var perr error
	add := func(img item, adv, flt *item) {
		r := list_model.OneImageReply{ImageID: idPtr(img.ImageUUID, &perr), ImageName: strPtr(img.ImageName)}
		s := [4]string{img.CreatedAt, img.ImageUUID}
		if adv != nil {
			r.AdversarialID = idPtr(adv.AdversarialObjectUUID, &perr)
			r.AdversarialName = strPtr(adv.AdversarialObjectName)
			r.Algorythm = strPtr(adv.Algorythm)
			r.AlterGoal = strPtr(adv.AlterGoal)
			s[2] = adv.CreatedAt
		}
		if flt != nil {
			r.FilterID = idPtr(flt.FilterObjectUUID, &perr)
			r.FilterName = strPtr(flt.FilterObjectName)
			s[3] = flt.CreatedAt
		}
		rows = append(rows, listed{row: r, sort: s})
	}

	switch {
	case f.FilterID != nil || isSet(f.FilterName):
		for _, x := range fltList {
			adv, okA := advs[x.AdversarialUUID]
			if !okA {
				continue
			}
			img, okI := images[adv.ImageUUID]
			if okI && imageOk(img) && advOk(adv) && fltOk(x) {
				add(img, &adv, &x)
			}
		}
	case f.AdversarialID != nil || isSet(f.AdversarialName) || isSet(f.AlterGoal) || isSet(f.Algorythm):
		for _, a := range advList {
			img, okI := images[a.ImageUUID]
			if !okI || !imageOk(img) || !advOk(a) {
				continue
			}
			if x, ok := filterOf[a.AdversarialObjectUUID]; ok {
				add(img, &a, &x)
			} else {
				add(img, &a, nil)
			}
		}
	default:
		for _, i := range imageList {
			if imageOk(i) {
				add(i, nil, nil)
			}
		}
	}
	if perr != nil {
		return nil, perr
	}

	sort.SliceStable(rows, func(i, j int) bool { return slices.Compare(rows[i].sort[:], rows[j].sort[:]) < 0 })
	output := &list_model.ListImageReply{AllImage: make([]list_model.OneImageReply, 0, len(rows))}
	for _, r := range rows {
		output.AllImage = append(output.AllImage, r.row)
	}
	return output, nil
}
