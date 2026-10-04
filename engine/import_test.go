package engine_test

import (
	"errors"
	"strings"
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
	} else if !strings.Contains(err.Error(), "the data holds no cases") {
		t.Errorf("an empty import must say why: %v", err)
	}
	if _, err := e.ImportCases(bg(), s.ID, "json", []byte("{nope")); !errors.Is(err, sentinel.ErrInvalidInput) {
		t.Errorf("malformed json must be ErrInvalidInput, so the dashboard answers BAD_REQUEST: %v", err)
	}
	if _, err := e.ImportCases(bg(), id.NewSuiteID(), "json", []byte(`[{"name":"a","input":"x"}]`)); !errors.Is(err, sentinel.ErrSuiteNotFound) {
		t.Errorf("missing suite: %v", err)
	}
	if stored, _ := e.ListCases(bg(), s.ID); len(stored) != 0 {
		t.Fatalf("refused imports must write nothing: %d", len(stored))
	}
}

// A row without a name or an input would be stored as a case that every
// run answers with an error, so the whole import is refused and nothing is
// written. The message names the row.
func TestImportRefusesBlankRows(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "p")
	for name, c := range map[string]struct{ format, data, row string }{
		"json no name":     {"json", `[{"name":"a","input":"x"},{"input":"y"}]`, "row 2"},
		"json no input":    {"json", `[{"name":"a","input":"x"},{"name":"b","input":"  "}]`, "row 2"},
		"jsonl no input":   {"jsonl", "{\"name\":\"a\",\"input\":\"x\"}\n{\"name\":\"b\"}\n", "row 2"},
		"csv no name":      {"csv", "name,input\na,x\n,y\n", "row 2"},
		"csv no input col": {"csv", "name\na\n", "row 1"},
	} {
		_, err := e.ImportCases(bg(), s.ID, c.format, []byte(c.data))
		if !errors.Is(err, sentinel.ErrInvalidInput) {
			t.Errorf("%s: want ErrInvalidInput, got %v", name, err)
			continue
		}
		if !strings.Contains(err.Error(), c.row) {
			t.Errorf("%s: the message must name %q: %v", name, c.row, err)
		}
	}
	if stored, _ := e.ListCases(bg(), s.ID); len(stored) != 0 {
		t.Fatalf("a refused import must write nothing, found %d cases", len(stored))
	}
}
