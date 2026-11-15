package adapter

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/filter"
	"github.com/belobragin/werewolf/bus/pkg/model/list_model"
	"github.com/google/uuid"
)

type notImageFilter struct{}

func (notImageFilter) ParseQueries(url.Values) error                    { return nil }
func (notImageFilter) DistillateQuery() (map[string]interface{}, error) { return nil, nil }

// TestListImage uses the database (skipped without it); everything is rolled back.
func TestListImage(t *testing.T) {
	d := newTxDb(t)
	tx := d.Db.DB
	l := &ApiClientRealization{Db: d}

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	img, adv, flt := uuid.New(), uuid.New(), uuid.New()
	must(tx.Exec(`INSERT INTO wfn.image (image_uuid,image_name,mime_type,image_size,image_height,image_width) VALUES (?,?,?,?,?,?)`,
		img, "tmpadlist-hog.jpg", "image/jpeg", "1Kb", 1, 1).Error)
	must(tx.Exec(`INSERT INTO wfn.adversarial_object (adversarial_object_uuid,adversarial_object_name,image_uuid,algorythm)
		VALUES (?,?,?,'fgsm')`, adv, "tmpadlist-hog_adv.jpg", img).Error)
	must(tx.Exec(`INSERT INTO wfn.filter_object (filter_object_uuid,filter_object_name,adversarial_object_uuid) VALUES (?,?,?)`,
		flt, "tmpadlist-hog_filter.jpg", adv).Error)

	list := func(f *filter.ImageFilter) (list_model.ListImageReply, error) {
		out, err := l.ListImage(context.Background(), f)
		if err != nil {
			return list_model.ListImageReply{}, err
		}
		return out.(list_model.ListImageReply), nil
	}
	parse := func(values url.Values) *filter.ImageFilter {
		t.Helper()
		f, _ := filter.NewImageFilter()
		must(f.ParseQueries(values))
		return f.(*filter.ImageFilter)
	}

	t.Run("wildcards from the query", func(t *testing.T) {
		reply, err := list(parse(url.Values{"image_name": {"tmpadlist-*.jpg"}, "algorythm": {"fgsm"}}))
		if err != nil || reply.Code != 200 || len(reply.AllImage) != 1 || *reply.AllImage[0].FilterID != flt {
			t.Errorf("got %+v, %v", reply, err)
		}
	})
	t.Run("an unknown algorythm is dropped, not an error", func(t *testing.T) {
		reply, err := list(parse(url.Values{"image_name": {"tmpadlist-*"}, "algorythm": {"nope"}}))
		if err != nil || len(reply.AllImage) != 1 {
			t.Errorf("got %+v, %v", reply, err)
		}
	})
	t.Run("nothing found is an empty list", func(t *testing.T) {
		reply, err := list(parse(url.Values{"image_name": {"tmpadlist-nothing*"}}))
		if err != nil || reply.Code != 200 || reply.AllImage == nil || len(reply.AllImage) != 0 {
			t.Errorf("got %+v, %v", reply, err)
		}
	})
	t.Run("a filter of another type", func(t *testing.T) {
		if _, err := l.ListImage(context.Background(), notImageFilter{}); !errors.Is(err, pdata.ErrInvalidListFilter) || pdata.GetCodeFromErr(err) != 400 {
			t.Errorf("got %v", err)
		}
	})
}
