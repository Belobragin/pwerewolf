package alter_model

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/google/uuid"
)

func TestLooseBool(t *testing.T) {
	for raw, want := range map[string]bool{
		`true`: true, `"1"`: true, `"true"`: true, `"True"`: true,
		`false`: false, `"0"`: false, `"false"`: false, `""`: false, `null`: false,
	} {
		var got LooseBool
		if err := json.Unmarshal([]byte(raw), &got); err != nil || bool(got) != want {
			t.Errorf("%s: got %v, %v; want %v", raw, got, err, want)
		}
	}
	var b LooseBool
	if err := json.Unmarshal([]byte(`"maybe"`), &b); err == nil {
		t.Error(`"maybe" is accepted`)
	}
}

func replyJSON(order, filter, extra string) string {
	return `{"order_id":"` + order + `","success_adversarial":true,"success_filter":` + filter + `,
		"adversarial_name":"/aker-data/adversarial/hog_adv_fgsm.jpg",` + extra + `"timestamp":"2026-10-20T14:03:45.270005","error":"",
		"algorythm":"fgsm","alter_goal":"tiger_shark",
		"predictions":{"n0":["n01","Komodo","48","52.38"],"n1":["n02","peacock","84","7.82"],"n2":["n03","armadillo","363","7.71"]}}`
}

func TestToAlterPostToDb(t *testing.T) {
	img := uuid.New()
	parse := func(raw string) (*AlterPostToDb, error) {
		var m MLMessageReply
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return m.ToAlterPostToDb()
	}

	t.Run("with filter", func(t *testing.T) {
		got, err := parse(replyJSON(img.String(), `"1"`, `"filter_name":"/aker-data/filter/hog_filter_fgsm.jpg",`))
		if err != nil {
			t.Fatal(err)
		}
		if got.ImageID != img || got.Algorythm != "fgsm" || got.AlterGoal != "tiger_shark" || got.AdversarialName != "hog_adv_fgsm.jpg" ||
			got.FilterName != "hog_filter_fgsm.jpg" || !got.HasFilter() || len(got.Predict) != 3 {
			t.Errorf("got %+v", got)
		}
		if got.AdversarialObjectID == uuid.Nil || got.FilterObjectID == uuid.Nil || got.AdversarialObjectID == got.FilterObjectID {
			t.Error("new uuids are not made")
		}
		if got.Predict[0]["n0"].ClassName != "Komodo" || got.Predict[0]["n0"].ClassID != 48 || got.Predict[2]["n2"].Probability != 7.71 {
			t.Errorf("predictions: %+v", got.Predict)
		}
	})
	t.Run("without filter", func(t *testing.T) {
		got, err := parse(replyJSON(img.String(), `false`, ``))
		if err != nil || got.HasFilter() || got.FilterName != "" {
			t.Errorf("got %+v, %v", got, err)
		}
	})
	t.Run("every call makes new uuids", func(t *testing.T) {
		a, _ := parse(replyJSON(img.String(), `false`, ``))
		b, _ := parse(replyJSON(img.String(), `false`, ``))
		if a.AdversarialObjectID == b.AdversarialObjectID {
			t.Error("the adversarial uuid repeats")
		}
	})

	failures := map[string]struct {
		raw  string
		want error
	}{
		"ML failure":                {`{"order_id":"` + img.String() + `","success_adversarial":false,"success_filter":false,"error":"boom"}`, ErrMLFailed},
		"error text":                {`{"order_id":"` + img.String() + `","success_adversarial":true,"success_filter":false,"error":"boom","adversarial_name":"a.jpg","algorythm":"fgsm"}`, ErrMLFailed},
		"order id is not a uuid":    {replyJSON("5678234", `false`, ``), ErrInvalidMLReply},
		"order id has a stray char": {replyJSON("5"+img.String(), `false`, ``), ErrInvalidMLReply},
		"filter without a name":     {replyJSON(img.String(), `"1"`, ``), ErrInvalidMLReply},
		"no adversarial name":       {`{"order_id":"` + img.String() + `","success_adversarial":true,"success_filter":false,"error":"","algorythm":"fgsm"}`, ErrInvalidMLReply},
		"broken prediction":         {`{"order_id":"` + img.String() + `","success_adversarial":true,"success_filter":false,"error":"","adversarial_name":"a.jpg","algorythm":"fgsm","predictions":{"n0":["x"]}}`, ErrInvalidMLReply},
	}
	for name, c := range failures {
		t.Run(name, func(t *testing.T) {
			if _, err := parse(c.raw); !errors.Is(err, c.want) {
				t.Errorf("got %v, want %v", err, c.want)
			}
		})
	}
	t.Run("no predictions is allowed", func(t *testing.T) {
		got, err := parse(`{"order_id":"` + img.String() + `","success_adversarial":true,"success_filter":false,"error":"","adversarial_name":"a.jpg","algorythm":"fgsm","predictions":{}}`)
		if err != nil || len(got.Predict) != 0 {
			t.Errorf("got %+v, %v", got, err)
		}
	})
}

func TestInputAlterValidation(t *testing.T) {
	for name, c := range map[string]struct {
		raw  string
		want error
	}{
		"fgsm":                    {`{"algorythm":"fgsm"}`, nil},
		"targeted with a goal":    {`{"algorythm":"biterate_targeted","alter_goal":"tiger_shark"}`, nil},
		"targeted without a goal": {`{"algorythm":"biterate_targeted"}`, pdata.ErrNilAlterGoal},
		"no algorythm":            {`{"with_filter":true}`, pdata.ErrNilAlgo},
		"unknown algorythm":       {`{"algorythm":"nope"}`, pdata.ErrInvalidAlgo},
	} {
		t.Run(name, func(t *testing.T) {
			var in InputAlter
			err := json.Unmarshal([]byte(c.raw), &in)
			if c.want == nil && err != nil || c.want != nil && !errors.Is(err, c.want) {
				t.Errorf("got %v, want %v", err, c.want)
			}
		})
	}
}
