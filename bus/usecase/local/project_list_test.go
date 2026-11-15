package local

import (
	"context"
	"testing"

	"github.com/belobragin/werewolf/bus/pkg/filter"
	"github.com/belobragin/werewolf/bus/pkg/model/list_model"
	"github.com/google/uuid"
)

// TestDbListImage uses the database (skipped without it); everything is rolled back.
func TestDbListImage(t *testing.T) {
	d := newTxDb(t)
	tx := d.Db.DB
	ctx := context.Background()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	// hog:  two adversarial objects, the fgsm one has a filter
	// hog2: no adversarial objects
	// a_b and axb: names that differ only in the character which is a wildcard in LIKE
	hog, hog2, ab, axb := insertImage(t, tx, "tmplist-hog-test.jpg"), insertImage(t, tx, "tmplist-hog-test2.jpg"),
		insertImage(t, tx, "tmplist-a_b.jpg"), insertImage(t, tx, "tmplist-axb.jpg")
	advFgsm, advEmu, flt := uuid.New(), uuid.New(), uuid.New()
	must(tx.Exec(`INSERT INTO wfn.adversarial_object (adversarial_object_uuid,adversarial_object_name,image_uuid,algorythm,alter_goal)
		VALUES (?,?,?,'fgsm','tiger_shark'),(?,?,?,'emulator',NULL)`,
		advFgsm, "tmplist-hog-test_adv_fgsm.jpg", hog, advEmu, "tmplist-hog-test_adv_emu.jpg", hog).Error)
	must(tx.Exec(`INSERT INTO wfn.filter_object (filter_object_uuid,filter_object_name,adversarial_object_uuid) VALUES (?,?,?)`,
		flt, "tmplist-hog-test_filter_fgsm.jpg", advFgsm).Error)

	str := func(s string) *string { return &s }
	list := func(f filter.ImageFilter) []list_model.OneImageReply {
		t.Helper()
		reply, err := d.DbListImage(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		if reply.AllImage == nil {
			t.Fatal("the list is nil, an empty list is expected")
		}
		return reply.AllImage
	}

	cases := []struct {
		name   string
		filter filter.ImageFilter
		rows   int
	}{
		{"every image, each once", filter.ImageFilter{ImageName: str("tmplist-*")}, 4},
		{"star in the middle", filter.ImageFilter{ImageName: str("tmplist-hog*.jpg")}, 2},
		{"exact name", filter.ImageFilter{ImageName: str("tmplist-hog-test.jpg")}, 1},
		{"question mark is one character", filter.ImageFilter{ImageName: str("tmplist-hog-test?.jpg")}, 1},
		{"underscore matches itself", filter.ImageFilter{ImageName: str("tmplist-a_b.jpg")}, 1},
		{"percent matches itself", filter.ImageFilter{ImageName: str("tmplist-%")}, 0},
		{"algorythm lists adversarial objects", filter.ImageFilter{ImageName: str("tmplist-*"), Algorythm: str("fgsm")}, 1},
		{"algorythm and image name", filter.ImageFilter{ImageName: str("tmplist-hog*"), Algorythm: str("emulator")}, 1},
		{"alter goal", filter.ImageFilter{ImageName: str("tmplist-*"), AlterGoal: str("tiger*")}, 1},
		{"adversarial name", filter.ImageFilter{AdversarialName: str("tmplist-*_adv_emu*")}, 1},
		{"every adversarial object of an image", filter.ImageFilter{AdversarialName: str("tmplist-hog-test_adv*")}, 2},
		{"filter name", filter.ImageFilter{FilterName: str("tmplist-*filter*")}, 1},
		{"an empty value is not a filter", filter.ImageFilter{ImageName: str("tmplist-*"), AlterGoal: str("")}, 4},
		{"no match", filter.ImageFilter{ImageName: str("tmplist-nothing*")}, 0},
		{"image uuid", filter.ImageFilter{ImageID: &hog}, 1},
		{"adversarial uuid", filter.ImageFilter{AdversarialID: &advEmu}, 1},
		{"filter uuid", filter.ImageFilter{FilterID: &flt}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := list(c.filter); len(got) != c.rows {
				t.Errorf("got %d rows, want %d: %+v", len(got), c.rows, got)
			}
		})
	}
	_, _ = ab, axb

	t.Run("listing images returns only images, even those with adversarial objects", func(t *testing.T) {
		rows := list(filter.ImageFilter{ImageName: str("tmplist-hog-test.jpg")})
		if len(rows) != 1 || rows[0].ImageID == nil || *rows[0].ImageID != hog || rows[0].ImageName == nil ||
			rows[0].AdversarialID != nil || rows[0].AdversarialName != nil || rows[0].Algorythm != nil ||
			rows[0].AlterGoal != nil || rows[0].FilterID != nil || rows[0].FilterName != nil {
			t.Errorf("rows: %+v", rows)
		}
	})
	t.Run("an adversarial row carries its image and filter", func(t *testing.T) {
		rows := list(filter.ImageFilter{AdversarialName: str("tmplist-hog-test_adv*")})
		withFilter := 0
		for _, r := range rows {
			if r.ImageID == nil || *r.ImageID != hog || r.AdversarialID == nil {
				t.Errorf("row: %+v", r)
			}
			if r.FilterID != nil {
				withFilter++
			}
		}
		if withFilter != 1 {
			t.Errorf("%d rows have a filter, want 1", withFilter)
		}
	})
	t.Run("a row carries the whole chain", func(t *testing.T) {
		rows := list(filter.ImageFilter{FilterID: &flt})
		r := rows[0]
		if r.ImageID == nil || *r.ImageID != hog || r.ImageName == nil || *r.ImageName != "tmplist-hog-test.jpg" ||
			r.AdversarialID == nil || *r.AdversarialID != advFgsm || r.AdversarialName == nil || *r.AdversarialName != "tmplist-hog-test_adv_fgsm.jpg" ||
			r.Algorythm == nil || *r.Algorythm != "fgsm" || r.AlterGoal == nil || *r.AlterGoal != "tiger_shark" ||
			r.FilterID == nil || *r.FilterID != flt || r.FilterName == nil || *r.FilterName != "tmplist-hog-test_filter_fgsm.jpg" {
			t.Errorf("row: %+v", r)
		}
	})
	t.Run("missing parts of the chain are nil", func(t *testing.T) {
		rows := list(filter.ImageFilter{ImageID: &hog2})
		if len(rows) != 1 || rows[0].AdversarialID != nil || rows[0].FilterID != nil || rows[0].Algorythm != nil || rows[0].AlterGoal != nil {
			t.Errorf("rows: %+v", rows)
		}
		rows = list(filter.ImageFilter{AdversarialID: &advEmu})
		if len(rows) != 1 || rows[0].FilterID != nil || rows[0].AlterGoal != nil {
			t.Errorf("adversarial without a filter: %+v", rows)
		}
	})
}
