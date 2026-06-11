package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPadRight(t *testing.T) {
	cases := []struct {
		s    string
		w    int
		want string
	}{
		{"ab", 5, "ab   "},
		{"ab", 2, "ab"},
		{"abcde", 3, "abcde"},
		{"x", 0, "x"},
		{"", 3, "   "},
	}
	for _, c := range cases {
		if got := padRight(c.s, c.w); got != c.want {
			t.Errorf("padRight(%q,%d)=%q want %q", c.s, c.w, got, c.want)
		}
	}
}

func TestIPLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"10.6.0.3", "10.6.0.20", true},
		{"10.6.0.20", "10.6.0.3", false},
		{"10.6.1.2", "10.6.0.20", false},
		{"10.6.0.7", "10.6.1.2", true},
		{"10.6.0.5", "10.6.0.5", false},
		{"10.6.0.1", "---", true},
		{"---", "10.6.0.1", false},
		{"---", "???", true},
	}
	for _, c := range cases {
		if got := ipLess(c.a, c.b); got != c.want {
			t.Errorf("ipLess(%q,%q)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestBoolStr(t *testing.T) {
	if got := boolStr(true); got != "true" {
		t.Errorf("boolStr(true)=%q", got)
	}
	if got := boolStr(false); got != "false" {
		t.Errorf("boolStr(false)=%q", got)
	}
}

func TestEmitJSONPretty(t *testing.T) {
	var buf bytes.Buffer
	type s struct {
		A int    `json:"a"`
		B string `json:"b"`
	}
	in := s{A: 1, B: "x"}
	if err := emitJSON(&buf, in); err != nil {
		t.Fatalf("emitJSON: %v", err)
	}
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("output should end in newline: %q", out)
	}
	if !strings.Contains(out, "\n  \"a\": 1") {
		t.Errorf("expected 2-space indent, got %q", out)
	}
	var back s
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back != in {
		t.Errorf("round-trip mismatch: %+v != %+v", back, in)
	}
}

func TestEmitJSONNilAndEmptySlice(t *testing.T) {
	var buf bytes.Buffer
	if err := emitJSON(&buf, ([]int)(nil)); err != nil {
		t.Fatalf("emitJSON nil slice: %v", err)
	}
	if got := buf.String(); got != "null\n" {
		t.Errorf("nil slice => %q want %q", got, "null\n")
	}
	buf.Reset()
	if err := emitJSON(&buf, []int{}); err != nil {
		t.Fatalf("emitJSON empty slice: %v", err)
	}
	if got := buf.String(); got != "[]\n" {
		t.Errorf("empty slice => %q want %q", got, "[]\n")
	}
}

func TestTableAlignmentAndSeparator(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"ID", "NAME"}
	rows := [][]string{
		{"1", "alice"},
		{"22", "bob"},
	}
	table(&buf, headers, rows)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected header+sep+2 rows, got %d lines: %q", len(lines), buf.String())
	}
	if lines[0] != "ID  NAME " {
		t.Errorf("header=%q", lines[0])
	}
	if lines[1] != "--  -----" {
		t.Errorf("separator=%q", lines[1])
	}
	if lines[2] != "1   alice" {
		t.Errorf("row0=%q", lines[2])
	}
	if lines[3] != "22  bob  " {
		t.Errorf("row1=%q", lines[3])
	}
	for _, r := range rows {
		for _, cell := range r {
			if !strings.Contains(buf.String(), cell) {
				t.Errorf("missing cell %q", cell)
			}
		}
	}
}

func TestTableCellWiderThanHeaderWidensColumn(t *testing.T) {
	var buf bytes.Buffer
	table(&buf, []string{"H"}, [][]string{{"wide-cell"}})
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if lines[0] != "H        " {
		t.Errorf("header=%q", lines[0])
	}
	if lines[1] != "---------" {
		t.Errorf("sep=%q (len %d)", lines[1], len(lines[1]))
	}
	if lines[2] != "wide-cell" {
		t.Errorf("row=%q", lines[2])
	}
}

func TestTableEmptyRows(t *testing.T) {
	var buf bytes.Buffer
	table(&buf, []string{"A", "B"}, nil)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected just header+separator, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "A  B" {
		t.Errorf("header=%q", lines[0])
	}
	if lines[1] != "-  -" {
		t.Errorf("sep=%q", lines[1])
	}
}
