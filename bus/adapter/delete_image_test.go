package adapter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/model/picture_model"
	"github.com/google/uuid"
)

func TestDeleteImageCascade(t *testing.T) {
	d := newTxDb(t)
	st, proj := newTestStorage(t, "4Mb")
	l := &ApiClientRealization{Db: d, Storage: st}
	tx := d.Db.DB

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	img, adv1, adv2, flt := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	must(tx.Exec(`INSERT INTO wfn.image (image_uuid,image_name,mime_type,image_size,image_height,image_width) VALUES (?,?,?,?,?,?)`,
		img, "tmpdel.jpg", "image/jpeg", "1Kb", 1, 1).Error)
	must(tx.Exec(`INSERT INTO wfn.adversarial_object (adversarial_object_uuid,adversarial_object_name,image_uuid,algorythm)
		VALUES (?,?,?,'biterate_targeted'),(?,?,?,'emulator')`, adv1, "tmpdel_adv1.jpg", img, adv2, "tmpdel_adv2.jpg", img).Error)
	must(tx.Exec(`INSERT INTO wfn.filter_object (filter_object_uuid,filter_object_name,adversarial_object_uuid) VALUES (?,?,?)`,
		flt, "tmpdel_filter.jpg", adv1).Error)
	// the file of adv2 is missing on purpose: a delete must not need it
	files := []string{"image/tmpdel.jpg", "adversarial/tmpdel_adv1.jpg", "filter/tmpdel_filter.jpg"}
	writeFiles := func() {
		for _, f := range files {
			must(os.WriteFile(filepath.Join(proj, "store", f), []byte("x"), 0o644))
		}
	}
	writeFiles()

	rows := func(table, column string, id uuid.UUID) int64 {
		var n int64
		must(tx.Table(table).Where(column+" = ?", id).Count(&n).Error)
		return n
	}
	del := func(id uuid.UUID) (picture_model.DeleteObjectReply, error) {
		out, err := l.DeleteImage(context.Background(), id)
		if err != nil {
			return picture_model.DeleteObjectReply{}, err
		}
		return out.(picture_model.DeleteObjectReply), nil
	}

	t.Run("a filter alone", func(t *testing.T) {
		reply, err := del(flt)
		must(err)
		if len(reply.AllObject) != 1 || reply.AllObject[0].Kind != "filter" {
			t.Fatalf("reply: %+v", reply)
		}
		if rows("wfn.filter_object", "filter_object_uuid", flt) != 0 || rows("wfn.adversarial_object", "adversarial_object_uuid", adv1) != 1 {
			t.Error("wrong rows deleted")
		}
		if _, err := os.Stat(filepath.Join(proj, "store/filter/tmpdel_filter.jpg")); err == nil {
			t.Error("filter file is kept")
		}
		// put the filter back for the next cases
		must(tx.Exec(`INSERT INTO wfn.filter_object (filter_object_uuid,filter_object_name,adversarial_object_uuid) VALUES (?,?,?)`,
			flt, "tmpdel_filter.jpg", adv1).Error)
		writeFiles()
	})
	t.Run("an adversarial object takes its filter", func(t *testing.T) {
		reply, err := del(adv1)
		must(err)
		if len(reply.AllObject) != 2 || reply.AllObject[0].Kind != "adversarial" || reply.AllObject[1].Kind != "filter" {
			t.Fatalf("reply: %+v", reply)
		}
		if rows("wfn.filter_object", "filter_object_uuid", flt) != 0 {
			t.Error("filter row is not cascaded")
		}
	})
	t.Run("an image takes everything", func(t *testing.T) {
		reply, err := del(img)
		must(err)
		if len(reply.AllObject) != 2 || reply.AllObject[0].Kind != "image" || reply.AllObject[1].ID != adv2 || reply.Code != 200 {
			t.Fatalf("reply: %+v", reply)
		}
		if rows("wfn.image", "image_uuid", img) != 0 || rows("wfn.adversarial_object", "image_uuid", img) != 0 {
			t.Error("rows are left")
		}
		for _, f := range files {
			if _, err := os.Stat(filepath.Join(proj, "store", f)); err == nil {
				t.Errorf("%s is kept", f)
			}
		}
	})
	t.Run("a repeated delete is a 404", func(t *testing.T) {
		if _, err := del(img); !errors.Is(err, pdata.ErrNoObjectId) {
			t.Errorf("got %v", err)
		}
	})
}
