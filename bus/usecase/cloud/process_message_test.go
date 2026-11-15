package cloud

import (
	"context"
	"errors"
	"testing"

	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/database"
	"github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/google/uuid"
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

type stubMsg struct{ data []byte }

func (s stubMsg) GetData() []byte    { return s.data }
func (s stubMsg) GetKey() string     { return "" }
func (s stubMsg) GetSubject() string { return "" }

func TestNewProcessMessage(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	db := &stubAlterDb{}
	handle := NewProcessMessage(db, zap.New(core))
	img := uuid.NewString()
	send := func(payload string) error { return handle(stubMsg{[]byte(payload)}) }
	last := func() observer.LoggedEntry {
		all := logs.All()
		return all[len(all)-1]
	}
	good := `{"order_id":"` + img + `","success_adversarial":true,"success_filter":"1","adversarial_name":"/a/adversarial/x_adv.jpg",
		"filter_name":"/a/filter/x_f.jpg","timestamp":"t","error":"","algorythm":"fgsm","alter_goal":"g",
		"predictions":{"n0":["a","A","1","50.5"],"n1":["b","B","2","20"]}}`

	t.Run("a good reply is saved with its predictions for the adversarial object", func(t *testing.T) {
		if err := send(good); err != nil {
			t.Fatal(err)
		}
		if e := last(); e.Level != zapcore.InfoLevel || e.Message != "ML reply is saved" {
			t.Fatalf("log: %+v", e)
		}
		if db.got == nil || db.owner != db.got.AdversarialObjectID || !db.got.HasFilter() ||
			len(db.got.Predict) != 2 || db.got.ImageID.String() != img {
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
		t.Run(name+" is logged, not saved and dropped from the queue", func(t *testing.T) {
			db.got = nil
			if err := send(c.payload); err != nil {
				t.Errorf("the message must be dropped: %v", err)
			}
			if e := last(); e.Level != zapcore.ErrorLevel || e.Message != c.message || db.got != nil {
				t.Errorf("log: %+v, saved %v", e, db.got)
			}
		})
	}

	t.Run("an unknown image is dropped from the queue", func(t *testing.T) {
		db.err = pdata.ErrNoObjectId
		defer func() { db.err = nil }()
		if err := send(good); err != nil {
			t.Errorf("the message must be dropped: %v", err)
		}
	})

	t.Run("a database error keeps the message in the queue", func(t *testing.T) {
		db.err = errors.New("db is down")
		defer func() { db.err = nil }()
		if err := send(good); !errors.Is(err, db.err) {
			t.Errorf("the message must be redelivered: %v", err)
		}
		if e := last(); e.Level != zapcore.ErrorLevel || e.Message != "ML reply is not saved to the database" {
			t.Errorf("log: %+v", e)
		}
	})
}
