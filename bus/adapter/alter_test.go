package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/belobragin/werewolf/bus/internal/pdata"
	model "github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/google/uuid"
)

func TestPostAlterObject(t *testing.T) {
	db := &stubDb{imageName: "hog-test.jpg"}
	broker := &stubBroker{}
	l := &ApiClientRealization{Db: db, Broker: broker}
	id := uuid.New()
	algo := "biterate_targeted"
	post := func(in *model.InputAlter) (any, error) { return l.PostAlterObject(context.Background(), id, in) }

	t.Run("published message and reply", func(t *testing.T) {
		out, err := post(&model.InputAlter{Algorythm: &algo, WithFilter: true, AlterGoal: "tiger_shark"})
		if err != nil {
			t.Fatal(err)
		}
		reply := out.(model.AlterPostReply)
		if reply.Code != 200 || reply.ImageName != "hog-test.jpg" || reply.SaveFilter != "1" {
			t.Errorf("reply: %+v", reply)
		}
		if len(broker.published) != 1 {
			t.Fatalf("published %d messages", len(broker.published))
		}
		var got model.MLMessageRequest
		if err := json.Unmarshal(broker.published[0], &got); err != nil {
			t.Fatal(err)
		}
		if got.ImageID != id || got.Algorythm != algo || got.ImageName != "hog-test.jpg" || got.AlterGoal != "tiger_shark" || got.SaveFilter != "1" {
			t.Errorf("message: %s", broker.published[0])
		}
	})
	t.Run("no filter keeps save_filter empty", func(t *testing.T) {
		// aker takes any non-empty string as true
		broker.published = nil
		if _, err := post(&model.InputAlter{Algorythm: &algo, AlterGoal: "tiger_shark"}); err != nil {
			t.Fatal(err)
		}
		var got model.MLMessageRequest
		if err := json.Unmarshal(broker.published[0], &got); err != nil || got.SaveFilter != "" {
			t.Errorf("message %s, %v", broker.published[0], err)
		}
	})
	t.Run("unknown image is a 404", func(t *testing.T) {
		db.imageNameErr = pdata.ErrNoObjectId
		defer func() { db.imageNameErr = nil }()
		broker.published = nil
		if _, err := post(&model.InputAlter{Algorythm: &algo}); !errors.Is(err, pdata.ErrNoObjectId) || len(broker.published) != 0 {
			t.Errorf("got %v, published %d", err, len(broker.published))
		}
	})
	t.Run("invalid algorythm", func(t *testing.T) {
		bad := "nope"
		if _, err := post(&model.InputAlter{Algorythm: &bad}); !errors.Is(err, pdata.ErrInvalidAlgo) {
			t.Errorf("got %v", err)
		}
		if _, err := post(&model.InputAlter{}); !errors.Is(err, pdata.ErrInvalidAlgo) {
			t.Errorf("nil algorythm: got %v", err)
		}
	})
	t.Run("publish failure is a 502", func(t *testing.T) {
		broker.err = errors.New("nats is down")
		defer func() { broker.err = nil }()
		_, err := post(&model.InputAlter{Algorythm: &algo})
		if !errors.Is(err, pdata.ErrPublish) || pdata.GetCodeFromErr(err) != 502 {
			t.Errorf("got %v", err)
		}
	})
}

// TestGetAdversarialObject uses the database (skipped without it); everything is rolled back.
func TestGetAdversarialObject(t *testing.T) {
	d := newTxDb(t)
	tx := d.Db.DB
	l := &ApiClientRealization{Db: d}

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	img, adv1, adv2, flt := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	must(tx.Exec(`INSERT INTO wfn.image (image_uuid,image_name,mime_type,image_size,image_height,image_width) VALUES (?,?,?,?,?,?)`,
		img, "tmpget.jpg", "image/jpeg", "1Kb", 1, 1).Error)
	must(tx.Exec(`INSERT INTO wfn.adversarial_object (adversarial_object_uuid,adversarial_object_name,image_uuid,algorythm,alter_goal)
		VALUES (?,?,?,'biterate_targeted','tiger_shark'),(?,?,?,'emulator',NULL)`, adv1, "tmpget_adv1.jpg", img, adv2, "tmpget_adv2.jpg", img).Error)
	must(tx.Exec(`INSERT INTO wfn.filter_object (filter_object_uuid,filter_object_name,adversarial_object_uuid) VALUES (?,?,?)`,
		flt, "tmpget_filter.jpg", adv1).Error)

	out, err := l.GetAdversarialObject(context.Background(), img)
	must(err)
	reply := out.(model.AlterGetReply)
	if reply.Code != 200 || len(reply.AltergetAllreply) != 2 {
		t.Fatalf("reply: %+v", reply)
	}
	for _, o := range reply.AltergetAllreply {
		switch o.AdversarialObjectID {
		case adv1:
			if o.FilterObjectID != flt || o.FilterName != "tmpget_filter.jpg" || o.AlterGoal != "tiger_shark" {
				t.Errorf("filter or goal is wrong: %+v", o)
			}
		case adv2:
			if o.FilterObjectID != uuid.Nil || o.AlterGoal != "" {
				t.Errorf("unexpected filter: %+v", o)
			}
		default:
			t.Errorf("unexpected object: %+v", o)
		}
	}
	if _, err := l.GetAdversarialObject(context.Background(), uuid.New()); !errors.Is(err, pdata.ErrNoObjectId) {
		t.Errorf("unknown id: %v", err)
	}
}
