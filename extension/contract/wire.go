package contract

import "time"

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

//nolint:unused // used by the intent handlers added in later tasks
func tsPtr(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := ts(*t)
	return &s
}

// dimsOrEmpty and stringsOrEmpty keep a nil map or slice from reaching the
// client as null, which a page iterating it would crash on.
//
//nolint:unused // used by the intent handlers added in later tasks
func dimsOrEmpty(m map[string]float64) map[string]float64 {
	if m == nil {
		return map[string]float64{}
	}
	return m
}

//nolint:unused // used by the intent handlers added in later tasks
func stringsOrEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
