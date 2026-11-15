package diagnose_model

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestToResnetReply(t *testing.T) {
	// the reply of the resnet server: [class_label, class_name, class_id, probability] under "n<rank>"
	raw := `{"image":"hog-test.jpg","folder":"image","predictions":{
		"n0":["n02396427","wild_boar","342","96.27"],"n1":["n02395406","hog","341","3.61"],"n2":["n02397096","warthog","343","0.1"]}}`
	var server ResnetServerReply
	if err := json.Unmarshal([]byte(raw), &server); err != nil {
		t.Fatal(err)
	}
	reply, err := server.ToResnetReply()
	if err != nil {
		t.Fatal(err)
	}
	if reply.ImageName != "hog-test.jpg" || len(reply.Predict) != 3 {
		t.Fatalf("reply: %+v", reply)
	}
	// ordered by rank, whatever the map order is
	for rank, key := range []string{"n0", "n1", "n2"} {
		if _, ok := reply.Predict[rank][key]; !ok {
			t.Errorf("rank %d holds %v, want %s", rank, reply.Predict[rank], key)
		}
	}
	if first := reply.Predict[0]["n0"]; first.ClassName != "wild_boar" || first.ClassID != 342 || first.ClassLabel != "n02396427" || first.Probability != 96.27 {
		t.Errorf("first prediction: %+v", first)
	}
}

func TestToResnetReplyInvalid(t *testing.T) {
	for name, c := range map[string]struct {
		predictions map[string][]string
		want        error
	}{
		"empty":             {map[string][]string{}, ErrEmptyPrediction},
		"key is not a rank": {map[string][]string{"x": {"a", "b", "1", "1"}}, ErrInvalidPrediction},
		"rank out of range": {map[string][]string{"n5": {"a", "b", "1", "1"}}, ErrInvalidPrediction},
		"duplicate rank":    {map[string][]string{"n1": {"a", "b", "1", "1"}, "1": {"a", "b", "1", "1"}}, ErrInvalidPrediction},
		"short entry":       {map[string][]string{"n0": {"a", "b"}}, ErrInvalidPrediction},
		"class id":          {map[string][]string{"n0": {"a", "b", "x", "1"}}, ErrInvalidPrediction},
		"probability":       {map[string][]string{"n0": {"a", "b", "1", "x"}}, ErrInvalidPrediction},
	} {
		t.Run(name, func(t *testing.T) {
			server := ResnetServerReply{Predictions: c.predictions}
			if _, err := server.ToResnetReply(); !errors.Is(err, c.want) {
				t.Errorf("got %v, want %v", err, c.want)
			}
		})
	}
}
