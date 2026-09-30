package engine_test

import (
	"errors"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
)

func TestImportCases(t *testing.T) {
	e := newEngine(t)
	cases := []struct {
		format, data string
		want         int64
	}{
		{"json", `[{"name":"a","input":"x"},{"name":"b","input":"y","tags":["t"]}]`, 2},
		{"csv", "name,input,expected,tags\na,x,,t1;t2\n", 1},
		{"jsonl", "{\"name\":\"a\",\"input\":\"x\"}\n{\"name\":\"b\",\"input\":\"y\"}\n", 2},
	}
	for _, c := range cases {
		s := seedSuite(t, e, "p")
		n, err := e.ImportCases(bg(), s.ID, c.format, []byte(c.data))
		if err != nil || n != c.want {
			t.Fatalf("%s: n=%d err=%v", c.format, n, err)
		}
		stored, _ := e.ListCases(bg(), s.ID)
		if int64(len(stored)) != c.want {
			t.Fatalf("%s: imported %d, stored %d", c.format, n, len(stored))
		}
	}
}

func TestImportRefusals(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "p")
	if _, err := e.ImportCases(bg(), s.ID, "yaml", []byte("x")); !errors.Is(err, sentinel.ErrUnsupportedFormat) {
		t.Errorf("yaml: %v", err)
	}
	if _, err := e.ImportCases(bg(), s.ID, "json", []byte("[]")); !errors.Is(err, sentinel.ErrEmptyInput) {
		t.Errorf("empty: %v", err)
	}
	if _, err := e.ImportCases(bg(), s.ID, "json", []byte("{nope")); err == nil {
		t.Error("malformed json must fail")
	}
	if _, err := e.ImportCases(bg(), id.NewSuiteID(), "json", []byte(`[{"name":"a","input":"x"}]`)); !errors.Is(err, sentinel.ErrSuiteNotFound) {
		t.Errorf("missing suite: %v", err)
	}
	if stored, _ := e.ListCases(bg(), s.ID); len(stored) != 0 {
		t.Fatalf("refused imports must write nothing: %d", len(stored))
	}
}
