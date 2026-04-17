package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePath(t *testing.T) {
	dir := t.TempDir()
	// relative inside baseDir
	p, err := validatePath("foo.txt", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(p) {
		t.Fatal("expected absolute path")
	}

	// traversal
	_, err = validatePath("../outside.txt", dir)
	if err == nil {
		t.Fatal("expected traversal error")
	}

	// empty
	_, err = validatePath("", dir)
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestParseRanges(t *testing.T) {
	cases := []struct {
		in        string
		expStart  int
		expEnd    int
		expOk     bool
	}{
		{"1,5", 1, 5, true},
		{"[1,5]", 1, 5, true},
		{" 1 , 5 ", 1, 5, true},
		{"1-5", 0, 0, false},
		{"abc", 0, 0, false},
	}
	for _, c := range cases {
		s, e, ok := parseRanges(c.in)
		if ok != c.expOk || s != c.expStart || e != c.expEnd {
			t.Fatalf("parseRanges(%q) = %d,%d,%v, want %d,%d,%v", c.in, s, e, ok, c.expStart, c.expEnd, c.expOk)
		}
	}
}

func TestViewTextFile(t *testing.T) {
	f, err := os.CreateTemp("", "view*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("line1\nline2\nline3\n")
	f.Close()

	out, err := viewTextFile(f.Name(), 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if out != "2: line2\n3: line3\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}
