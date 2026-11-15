package local

import "testing"

func TestLikePattern(t *testing.T) {
	for in, want := range map[string]string{
		"hog-test.jpg":     "hog-test.jpg",
		"hog-test*.jpg":    "hog-test%.jpg",
		"*_adv_*":          `%\_adv\_%`,
		"hog-test?.jpg":    "hog-test_.jpg",
		"100%":             `100\%`,
		`back\slash`:       `back\\slash`,
		"*":                "%",
		"":                 "",
		"mixed_*?%_\\done": `mixed\_%_\%\_\\done`,
	} {
		if got := likePattern(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}
