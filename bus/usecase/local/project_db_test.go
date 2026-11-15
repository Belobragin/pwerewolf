package local

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/belobragin/werewolf/bus/pkg/model/diagnose_model"
	"github.com/belobragin/werewolf/bus/pkg/model/picture_model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// The tests of this file use the database and are skipped without it; every one is rolled back.

func insertImage(t *testing.T, tx *gorm.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	err := tx.Exec(`INSERT INTO wfn.image (image_uuid,image_name,mime_type,image_size,image_height,image_width) VALUES (?,?,?,?,?,?)`,
		id, name, "image/jpeg", "1Kb", 1, 1).Error
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func countRows(t *testing.T, tx *gorm.DB, table, column string, id uuid.UUID) int64 {
	t.Helper()
	var n int64
	if err := tx.Table(table).Where(column+" = ?", id).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDiagnoseResultRoundTrip(t *testing.T) {
	d := newTxDb(t)
	ctx := context.Background()
	img := insertImage(t, d.Db.DB, "tmpdiag.jpg")

	three := []map[string]diagnose_model.ResnetPredictionData{{"n0": prediction("a")}, {"n1": prediction("b")}, {"n2": prediction("c")}}
	post := func(predict []map[string]diagnose_model.ResnetPredictionData) error {
		return d.DbPostDiagnoseObject(ctx, &diagnose_model.DiagnosePostReply{ImageID: img, ResnetReply: diagnose_model.ResnetReply{Predict: predict}})
	}

	if err := post(three); err != nil {
		t.Fatal(err)
	}
	got, err := d.DbGetDiagnoseObject(ctx, img)
	if err != nil {
		t.Fatal(err)
	}
	if got.ImageName != "tmpdiag.jpg" || len(got.DbResultReply.Predict) != 3 || got.DbResultReply.Predict[1]["n1"].ClassName != "b" {
		t.Errorf("get: %+v", got)
	}

	// a repeated post replaces the ranks and drops the surplus ones
	if err := post(three[:2]); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, d.Db.DB, "wfn.diagnose_result", "image_uuid", img); n != 2 {
		t.Errorf("rows after a repeated post: %d", n)
	}

	if err := post(nil); !errors.Is(err, diagnose_model.ErrEmptyPrediction) {
		t.Errorf("empty predictions: %v", err)
	}
	if _, err := d.DbGetDiagnoseObject(ctx, uuid.New()); !errors.Is(err, pdata.ErrNoObjectId) {
		t.Errorf("unknown image: %v", err)
	}
	other := insertImage(t, d.Db.DB, "tmpdiag2.jpg")
	if _, err := d.DbGetDiagnoseObject(ctx, other); !errors.Is(err, pdata.ErrNoResultInDb) {
		t.Errorf("image without a result: %v", err)
	}
}

func TestUploadImageDescription(t *testing.T) {
	d := newTxDb(t)
	ctx := context.Background()
	for name, description := range map[string]string{"tmp-with.jpg": "a boar in the woods", "tmp-without.jpg": ""} {
		reply := &picture_model.UploadReply{ImageID: uuid.New(), ImageName: name, ImageDescription: description,
			MimeType: "image/jpeg", ImageSize: "1Kb", ImageHeight: 1, ImageWidth: 1}
		if err := d.DbUploadImage(ctx, reply); err != nil {
			t.Fatal(err)
		}
		var stored *string
		if err := d.Db.DB.Raw("SELECT image_description FROM wfn.image WHERE image_name = ?", name).Scan(&stored).Error; err != nil {
			t.Fatal(err)
		}
		if (description == "") != (stored == nil) || (stored != nil && *stored != description) {
			t.Errorf("%s: stored %v, want %q (empty stays NULL)", name, stored, description)
		}
	}
	dup := &picture_model.UploadReply{ImageID: uuid.New(), ImageName: "tmp-with.jpg", MimeType: "image/jpeg", ImageSize: "1Kb", ImageHeight: 1, ImageWidth: 1}
	if err := d.DbUploadImage(ctx, dup); !errors.Is(err, pdata.ErrExistObjectName) {
		t.Errorf("duplicate name: %v", err)
	}
}

func TestDbGetImageNameAndUuid(t *testing.T) {
	d := newTxDb(t)
	ctx := context.Background()
	img := insertImage(t, d.Db.DB, "tmpname.jpg")

	if name, err := d.DbGetImageName(ctx, img); err != nil || name != "tmpname.jpg" {
		t.Errorf("name: %q, %v", name, err)
	}
	if _, err := d.DbGetImageName(ctx, uuid.New()); !errors.Is(err, pdata.ErrNoObjectId) {
		t.Errorf("unknown uuid: %v", err)
	}
	if id, err := d.DbGetImageUuidByName(ctx, "tmpname.jpg", "image"); err != nil || id != img {
		t.Errorf("uuid by name: %v, %v", id, err)
	}
	if _, err := d.DbGetImageUuidByName(ctx, "nope.jpg", "image"); !errors.Is(err, pdata.ErrNoObjectName) {
		t.Errorf("unknown name: %v", err)
	}
	if _, err := d.DbGetImageUuidByName(ctx, "tmpname.jpg", "bogus"); !errors.Is(err, pdata.ErrInvalidObjectKind) {
		t.Errorf("bogus kind: %v", err)
	}
}

func TestDbAlterObject(t *testing.T) {
	d := newTxDb(t)
	tx := d.Db.DB
	ctx := context.Background()
	img := insertImage(t, tx, "tmpalter.jpg")

	three := []map[string]diagnose_model.ResnetPredictionData{{"n0": prediction("a")}, {"n1": prediction("b")}, {"n2": prediction("c")}}
	newData := func(algorythm string, withFilter bool, predict []map[string]diagnose_model.ResnetPredictionData) *alter_model.AlterPostToDb {
		data := &alter_model.AlterPostToDb{ImageID: img, AdversarialObjectID: uuid.New(), AdversarialName: "tmpalter_adv.jpg",
			Algorythm: algorythm, Predict: predict}
		if withFilter {
			data.FilterObjectID, data.FilterName = uuid.New(), "tmpalter_filter.jpg"
		}
		return data
	}

	t.Run("object, filter and predictions of the adversarial object", func(t *testing.T) {
		first := newData("fgsm", true, three)
		first.AlterGoal = "tiger_shark"
		if err := d.DbAlterObject(ctx, first, first.AdversarialObjectID); err != nil {
			t.Fatal(err)
		}
		adv, err := d.DbGetAdvDataImageId(ctx, img)
		if err != nil || len(adv.AltergetAllreply) != 1 || adv.AltergetAllreply[0].AlterGoal != "tiger_shark" ||
			adv.AltergetAllreply[0].FilterName != "tmpalter_filter.jpg" {
			t.Fatalf("the goal is not saved or read back: %+v, %v", adv, err)
		}
		if countRows(t, tx, "wfn.adversarial_object", "adversarial_object_uuid", first.AdversarialObjectID) != 1 ||
			countRows(t, tx, "wfn.filter_object", "filter_object_uuid", first.FilterObjectID) != 1 ||
			countRows(t, tx, "wfn.diagnose_result", "adversarial_object_uuid", first.AdversarialObjectID) != 3 {
			t.Fatal("rows are not saved")
		}
		if countRows(t, tx, "wfn.diagnose_result", "image_uuid", img) != 0 {
			t.Error("the predictions went to the prime image")
		}

		// a repeated result for the same image and algorythm replaces the old one together with its filter and predictions
		second := newData("fgsm", false, three[:2])
		if err := d.DbAlterObject(ctx, second, second.AdversarialObjectID); err != nil {
			t.Fatal(err)
		}
		if countRows(t, tx, "wfn.adversarial_object", "adversarial_object_uuid", first.AdversarialObjectID) != 0 ||
			countRows(t, tx, "wfn.filter_object", "filter_object_uuid", first.FilterObjectID) != 0 ||
			countRows(t, tx, "wfn.diagnose_result", "adversarial_object_uuid", first.AdversarialObjectID) != 0 {
			t.Error("the old result is kept")
		}
		if countRows(t, tx, "wfn.diagnose_result", "adversarial_object_uuid", second.AdversarialObjectID) != 2 {
			t.Error("the new predictions are not saved")
		}
		// an empty goal stays NULL, and is read back as an empty string
		var goal *string
		if err := tx.Raw("SELECT alter_goal FROM wfn.adversarial_object WHERE adversarial_object_uuid = ?", second.AdversarialObjectID).Scan(&goal).Error; err != nil || goal != nil {
			t.Errorf("stored goal: %v, %v", goal, err)
		}
		if adv, err := d.DbGetAdvDataImageId(ctx, img); err != nil || len(adv.AltergetAllreply) != 1 || adv.AltergetAllreply[0].AlterGoal != "" {
			t.Errorf("read back: %+v, %v", adv, err)
		}

		// deleting the adversarial object takes its predictions
		if err := d.DbDeleteImage(ctx, second.AdversarialObjectID); err != nil {
			t.Fatal(err)
		}
		if countRows(t, tx, "wfn.diagnose_result", "adversarial_object_uuid", second.AdversarialObjectID) != 0 {
			t.Error("the predictions survive the adversarial object")
		}
	})
	t.Run("predictions for an image", func(t *testing.T) {
		data := newData("emulator", false, three)
		if err := d.DbAlterObject(ctx, data, img); err != nil {
			t.Fatal(err)
		}
		if countRows(t, tx, "wfn.diagnose_result", "image_uuid", img) != 3 {
			t.Error("the predictions are not saved for the image")
		}
	})
	t.Run("no predictions needs no owner", func(t *testing.T) {
		data := newData("biterate_untargeted", false, nil)
		if err := d.DbAlterObject(ctx, data, uuid.Nil); err != nil {
			t.Error(err)
		}
	})
	t.Run("invalid owners", func(t *testing.T) {
		data := newData("biterate_targeted", false, three)
		if err := d.DbAlterObject(ctx, data, uuid.New()); !errors.Is(err, pdata.ErrNoObjectId) {
			t.Errorf("unknown owner: %v", err)
		}
		if err := d.DbAlterObject(ctx, data, uuid.Nil); err == nil {
			t.Error("a nil owner is accepted")
		}
	})
	t.Run("unknown prime image", func(t *testing.T) {
		data := newData("fgsm", false, three)
		data.ImageID = uuid.New()
		if err := d.DbAlterObject(ctx, data, data.AdversarialObjectID); !errors.Is(err, pdata.ErrNoObjectId) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("the reader of the image result ignores adversarial rows", func(t *testing.T) {
		got, err := d.DbGetDiagnoseObject(ctx, img)
		if err != nil || len(got.DbResultReply.Predict) != 3 {
			t.Errorf("got %v, %v", got, err)
		}
	})
}

// TestSchemaConstraints checks the constraints of the schema the migrations make, on the current database.
func TestSchemaConstraints(t *testing.T) {
	d := newTxDb(t)
	tx := d.Db.DB
	img := insertImage(t, tx, "tmpschema.jpg")
	// a failed statement aborts the whole transaction, so the ones that must fail run in a savepoint
	rejected := func(sql string, args ...any) bool {
		t.Helper()
		if err := tx.SavePoint("sp").Error; err != nil {
			t.Fatal(err)
		}
		err := tx.Exec(sql, args...).Error
		if rollbackErr := tx.RollbackTo("sp").Error; rollbackErr != nil {
			t.Fatal(rollbackErr)
		}
		return err != nil
	}

	t.Run("a prediction needs exactly one owner", func(t *testing.T) {
		if !rejected(`INSERT INTO wfn.diagnose_result (prediction_rank) VALUES (0)`) {
			t.Error("a row without an owner is accepted")
		}
		adv := uuid.New()
		if err := tx.Exec(`INSERT INTO wfn.adversarial_object (adversarial_object_uuid,adversarial_object_name,image_uuid,algorythm)
			VALUES (?,?,?,'emulator')`, adv, "tmpschema_adv.jpg", img).Error; err != nil {
			t.Fatal(err)
		}
		if !rejected(`INSERT INTO wfn.diagnose_result (image_uuid, adversarial_object_uuid, prediction_rank) VALUES (?,?,0)`, img, adv) {
			t.Error("a row with two owners is accepted")
		}
	})
	t.Run("a rank is unique per owner", func(t *testing.T) {
		if err := tx.Exec(`INSERT INTO wfn.diagnose_result (image_uuid, prediction_rank) VALUES (?,0)`, img).Error; err != nil {
			t.Fatal(err)
		}
		if !rejected(`INSERT INTO wfn.diagnose_result (image_uuid, prediction_rank) VALUES (?,0)`, img) {
			t.Error("the same rank is accepted twice for an image")
		}
	})
	t.Run("alter_goal is limited to 100 characters", func(t *testing.T) {
		if !rejected(`INSERT INTO wfn.adversarial_object (adversarial_object_uuid,adversarial_object_name,image_uuid,algorythm,alter_goal)
			VALUES (?,?,?,'biterate_targeted',?)`, uuid.New(), "tmpschema_goal.jpg", img, strings.Repeat("x", 101)) {
			t.Error("a goal longer than 100 characters is accepted")
		}
	})
	t.Run("the algorythm enum has fgsm", func(t *testing.T) {
		err := tx.Exec(`INSERT INTO wfn.adversarial_object (adversarial_object_uuid,adversarial_object_name,image_uuid,algorythm)
			VALUES (?,?,?,'fgsm')`, uuid.New(), "tmpschema_fgsm.jpg", img).Error
		if err != nil {
			t.Error(err)
		}
	})
}
