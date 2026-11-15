package filter

import (
	"net/url"
	"testing"

	"github.com/google/uuid"
)

func TestImageFilterParseQueries(t *testing.T) {
	id := uuid.New()
	f, err := NewImageFilter()
	if err != nil {
		t.Fatal(err)
	}
	err = f.ParseQueries(url.Values{
		"image_name":       {"hog-test*.jpg"},
		"algorythm":        {"fgsm"},
		"image_uuid":       {id.String()},
		"alter_goal":       {"tiger_shark"},
		"filter_id":        {id.String()},
		"adversarial_name": {"*_adv_*"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := f.(*ImageFilter)
	if got.ImageName == nil || *got.ImageName != "hog-test*.jpg" || got.Algorythm == nil || *got.Algorythm != "fgsm" ||
		got.ImageID == nil || *got.ImageID != id || got.AlterGoal == nil || *got.AlterGoal != "tiger_shark" ||
		got.FilterID == nil || *got.FilterID != id || got.AdversarialName == nil || *got.AdversarialName != "*_adv_*" {
		t.Errorf("parsed: %+v", got)
	}
	if got.AdversarialID != nil || got.FilterName != nil {
		t.Errorf("unset fields are not nil: %+v", got)
	}
}

func TestImageFilterParseQueriesInvalid(t *testing.T) {
	for name, values := range map[string]url.Values{
		"bad uuid": {"image_uuid": {"not-a-uuid"}},
	} {
		t.Run(name, func(t *testing.T) {
			f, _ := NewImageFilter()
			if err := f.ParseQueries(values); err == nil {
				t.Error("the invalid query is accepted")
			}
		})
	}
}

func TestImageFilterDistillateQuery(t *testing.T) {
	bad, good := "nope", "fgsm"
	f := &ImageFilter{Algorythm: &bad}
	if _, err := f.DistillateQuery(); err != nil || f.Algorythm != nil {
		t.Errorf("an unknown algorythm stays: %v, %v", f.Algorythm, err)
	}
	f = &ImageFilter{Algorythm: &good}
	if _, err := f.DistillateQuery(); err != nil || f.Algorythm == nil || *f.Algorythm != "fgsm" {
		t.Errorf("a known algorythm is dropped: %v, %v", f.Algorythm, err)
	}
}
