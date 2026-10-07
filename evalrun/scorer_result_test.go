package evalrun

import "testing"

func TestScorerResultErrored(t *testing.T) {
	for _, c := range []struct {
		r    ScorerResult
		want bool
	}{
		{ScorerResult{ScorerName: "judge", Reason: ScorerErrorPrefix + "model unavailable"}, true},
		{ScorerResult{ScorerName: "judge", Reason: "off persona"}, false},
		{ScorerResult{ScorerName: "judge", Passed: true}, false},
	} {
		if got := c.r.Errored(); got != c.want {
			t.Errorf("%+v: Errored() = %v, want %v", c.r, got, c.want)
		}
	}
}
