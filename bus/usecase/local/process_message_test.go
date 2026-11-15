package local

import (
	"context"
	"errors"
	"testing"

	"github.com/belobragin/werewolf/bus/pkg/database"
	"github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type stubAlterDb struct {
	database.ProjectDbaseI
	got   *alter_model.AlterPostToDb
	owner uuid.UUID
	err   error
}

func (s *stubAlterDb) DbAlterObject(_ context.Context, data *alter_model.AlterPostToDb, owner uuid.UUID) error {
	s.got, s.owner = data, owner
	return s.err
}

func TestNewProcessMessage(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	db := &stubAlterDb{}
	handle := NewProcessMessage(db, zap.New(core))
	img := uuid.NewString()
	send := func(payload string) { handle(&nats.Msg{Data: []byte(payload)}) }
	last := func() observer.LoggedEntry {
		all := logs.All()
		return all[len(all)-1]
	}
	// aker sends success_filter as the string "1" when the filter was asked for
	good := `{"order_id":"` + img + `","success_adversarial":true,"success_filter":"1","adversarial_name":"/a/adversarial/x_adv.jpg",
		"filter_name":"/a/filter/x_f.jpg","timestamp":"t","error":"","algorythm":"fgsm","alter_goal":"g",
		"predictions":{"n0":["a","A","1","50.5"],"n1":["b","B","2","20"]}}`

	t.Run("a good reply is saved for the adversarial object", func(t *testing.T) {
		send(good)
		if e := last(); e.Level != zapcore.InfoLevel || e.Message != "ML reply is saved" {
			t.Fatalf("log: %+v", e)
		}
		if db.got == nil || db.owner != db.got.AdversarialObjectID || !db.got.HasFilter() ||
			db.got.AdversarialName != "x_adv.jpg" || db.got.FilterName != "x_f.jpg" || len(db.got.Predict) != 2 || db.got.ImageID.String() != img {
			t.Errorf("saved: %+v, owner %v", db.got, db.owner)
		}
	})

	for name, c := range map[string]struct{ payload, message string }{
		"not json": {`not json`, "ML reply unmarshal failed"},
		"ML failure": {`{"order_id":"` + img + `","success_adversarial":false,"success_filter":false,"error":"boom"}`,
			"ML service failed to process the order"},
		"not a uuid order": {`{"order_id":"5678234","success_adversarial":true,"success_filter":false,"adversarial_name":"/a/b.jpg","error":"","algorythm":"fgsm","predictions":{}}`,
			"ML reply is invalid"},
	} {
		t.Run(name+" is logged and not saved", func(t *testing.T) {
			db.got = nil
			send(c.payload)
			if e := last(); e.Level != zapcore.ErrorLevel || e.Message != c.message || db.got != nil {
				t.Errorf("log: %+v, saved %v", e, db.got)
			}
		})
	}
	t.Run("a database error is logged", func(t *testing.T) {
		db.err = errors.New("db is down")
		defer func() { db.err = nil }()
		send(good)
		if e := last(); e.Level != zapcore.ErrorLevel || e.Message != "ML reply is not saved to the database" {
			t.Errorf("log: %+v", e)
		}
	})
}
