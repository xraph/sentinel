package evalrun

import (
	"encoding/json"
	"reflect"
)

// Run.Config keys a run's settings are recorded under. They are flat on
// purpose: mongo decodes nested maps as bson.D, which serialises as a list.
const (
	SettingPassThreshold       = "pass_threshold"
	SettingRegressionThreshold = "regression_threshold"
	SettingConcurrency         = "concurrency"
	SettingTarget              = "target"
	SettingScorers             = "scorers"
	SettingModel               = "model"
	SettingPromptVersionID     = "prompt_version_id"
)

// Settings are what a run was scored with, recorded when it started. A
// pointer or slice is nil, and a string empty, when the run predates
// recording: that means "not recorded", which is not the same as today's
// configuration and must never be shown as if it were.
type Settings struct {
	PassThreshold       *float64
	RegressionThreshold *float64
	Concurrency         *int
	Target              string
	Scorers             []string
	Model               string
	PromptVersionID     string
}

// Config returns the settings as Run.Config entries.
func (s Settings) Config() map[string]any {
	m := map[string]any{}
	if s.PassThreshold != nil {
		m[SettingPassThreshold] = *s.PassThreshold
	}
	if s.RegressionThreshold != nil {
		m[SettingRegressionThreshold] = *s.RegressionThreshold
	}
	if s.Concurrency != nil {
		m[SettingConcurrency] = *s.Concurrency
	}
	if s.Target != "" {
		m[SettingTarget] = s.Target
	}
	if s.Scorers != nil {
		m[SettingScorers] = append([]string(nil), s.Scorers...)
	}
	if s.Model != "" {
		m[SettingModel] = s.Model
	}
	m[SettingPromptVersionID] = s.PromptVersionID
	return m
}

// SettingsFrom reads settings back from Run.Config, tolerating the number
// and array types each backend decodes into.
func SettingsFrom(config map[string]any) Settings {
	var s Settings
	if v, ok := toFloat(config[SettingPassThreshold]); ok {
		s.PassThreshold = &v
	}
	if v, ok := toFloat(config[SettingRegressionThreshold]); ok {
		s.RegressionThreshold = &v
	}
	if v, ok := toFloat(config[SettingConcurrency]); ok {
		n := int(v)
		s.Concurrency = &n
	}
	s.Target, _ = config[SettingTarget].(string)
	s.Scorers = toStrings(config[SettingScorers])
	s.Model, _ = config[SettingModel].(string)
	s.PromptVersionID, _ = config[SettingPromptVersionID].(string)
	return s
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// toStrings accepts any slice kind, named or not, whose elements are
// strings: []string, []any, and mongo's bson.A.
func toStrings(v any) []string {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	out := make([]string, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		if s, ok := rv.Index(i).Interface().(string); ok {
			out = append(out, s)
		}
	}
	return out
}
