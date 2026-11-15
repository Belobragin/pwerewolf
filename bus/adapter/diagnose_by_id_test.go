package adapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/model/diagnose_model"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// stubResnet points the api client at the test server.
type stubResnet struct{ url string }

func (s stubResnet) GetPredictApiUrl() string        { return s.url }
func (s stubResnet) GetTransport() http.RoundTripper { return nil }

// TestPostDiagnoseObjectByID uses the database (skipped without it); everything is rolled back.
func TestPostDiagnoseObjectByID(t *testing.T) {
	d := newTxDb(t)
	tx := d.Db.DB

	// the resnet server: remembers the query of the last call
	var calls []string
	resnet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Query().Get("image_name")+"|"+r.URL.Query().Get("folder"))
		w.Write([]byte(`{"image":"x","folder":"x","predictions":{"n0":["n01","boar","342","90.5"],"n1":["n02","hog","341","9.5"]}}`))
	}))
	defer resnet.Close()
	core, logs := observer.New(zapcore.InfoLevel)
	l := &ApiClientRealization{Db: d, HttpClient: http.DefaultClient, Resnet: stubResnet{url: resnet.URL}, Zapstruct: zap.New(core)}

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	img, adv, flt := uuid.New(), uuid.New(), uuid.New()
	must(tx.Exec(`INSERT INTO wfn.image (image_uuid,image_name,mime_type,image_size,image_height,image_width) VALUES (?,?,?,?,?,?)`,
		img, "tmpbyid.jpg", "image/jpeg", "1Kb", 1, 1).Error)
	must(tx.Exec(`INSERT INTO wfn.adversarial_object (adversarial_object_uuid,adversarial_object_name,image_uuid,algorythm)
		VALUES (?,?,?,'emulator')`, adv, "tmpbyid_adversarial_emulator.jpg", img).Error)
	must(tx.Exec(`INSERT INTO wfn.filter_object (filter_object_uuid,filter_object_name,adversarial_object_uuid) VALUES (?,?,?)`,
		flt, "tmpbyid_filter_emulator.jpg", adv).Error)

	t.Run("an image is diagnosed in the image folder", func(t *testing.T) {
		calls = nil
		if _, err := l.PostDiagnoseObjectByID(context.Background(), img); err != nil {
			t.Fatal(err)
		}
		if len(calls) != 1 || calls[0] != "tmpbyid.jpg|image" {
			t.Errorf("resnet calls: %v", calls)
		}
		got, err := d.DbGetDiagnoseObject(context.Background(), img)
		if err != nil || len(got.DbResultReply.Predict) != 2 {
			t.Errorf("saved result: %+v, %v", got, err)
		}
	})
	t.Run("the predictions are logged in the server terminal", func(t *testing.T) {
		entries := logs.FilterMessage("image is diagnosed").All()
		if len(entries) == 0 {
			t.Fatal("nothing is logged")
		}
		fields := entries[len(entries)-1].ContextMap()
		predictions, ok := fields["predictions"].([]map[string]diagnose_model.ResnetPredictionData)
		if fields["image_name"] != "tmpbyid.jpg" || fields["folder"] != "image" || !ok || len(predictions) != 2 || predictions[0]["n0"].ClassName != "boar" {
			t.Errorf("logged fields: %v", fields)
		}
	})
	t.Run("the reply is the saved diagnose result with code 201", func(t *testing.T) {
		out, err := l.PostDiagnoseObjectByID(context.Background(), img)
		if err != nil {
			t.Fatal(err)
		}
		reply, ok := out.(diagnose_model.DiagnosePostReply)
		if !ok || reply.Code != http.StatusCreated || reply.ImageID != img || len(reply.ResnetReply.Predict) != 2 {
			t.Errorf("reply: %+v", out)
		}
	})
	// only the image table is searched, so the uuid of an adversarial object or of a filter is unknown
	for name, id := range map[string]uuid.UUID{"an adversarial object": adv, "a filter": flt, "an unknown uuid": uuid.New()} {
		t.Run(name+" is a 404", func(t *testing.T) {
			calls = nil
			_, err := l.PostDiagnoseObjectByID(context.Background(), id)
			if !errors.Is(err, pdata.ErrNoObjectId) || pdata.GetCodeFromErr(err) != http.StatusNotFound || len(calls) != 0 {
				t.Errorf("got %v, resnet calls %v", err, calls)
			}
		})
	}
}
