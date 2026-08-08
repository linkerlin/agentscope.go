package hub

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	mcpserver "github.com/linkerlin/agentscope.go/toolkit/mcp"
)

func TestPage_Basic(t *testing.T) {
	all := []int{1, 2, 3, 4, 5, 6, 7}
	page1, next := Page(all, 0, 3)
	if len(page1) != 3 || next != 3 {
		t.Fatalf("page1: %v next=%d", page1, next)
	}
	page2, next := Page(all, 3, 3)
	if len(page2) != 3 || next != 6 {
		t.Fatalf("page2: %v next=%d", page2, next)
	}
	page3, next := Page(all, 6, 3)
	if len(page3) != 1 || next != -1 {
		t.Fatalf("page3: %v next=%d", page3, next)
	}
}

func TestPage_EdgeCases(t *testing.T) {
	if p, next := Page([]int{1}, 5, 10); len(p) != 0 || next != -1 {
		t.Fatalf("cursor past end: %v %d", p, next)
	}
	if p, next := Page([]int{1, 2}, 0, 0); len(p) != 2 || next != -1 {
		t.Fatalf("zero limit: %v %d", p, next)
	}
	var empty []int
	if p, _ := Page(empty, 0, 10); p != nil {
		t.Fatal("nil input should give nil page")
	}
}

func TestFilterCards_Query(t *testing.T) {
	cards := []MCPCard{
		{Card: Card{ID: "a", Name: "filesystem", Description: "local fs access"}},
		{Card: Card{ID: "b", Name: "github", Description: "github api"}},
	}
	got := FilterCards(cards, "github", func(c MCPCard) string { return c.Name }, func(c MCPCard) string { return c.Description })
	if len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("filtered: %+v", got)
	}
	if len(FilterCards(cards, "", func(c MCPCard) string { return c.Name }, func(c MCPCard) string { return c.Description })) != 2 {
		t.Fatal("empty query returns all")
	}
}

func TestSafeJoin(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"a/b.txt", true},
		{"a/../b.txt", true}, // cleans to b.txt
		{"../escape.txt", false},
		{"/abs/path", false},
		{"..", false},
		{"", false},
		{"..\\win-escape", false}, // windows traversal
	}
	for _, c := range cases {
		_, ok := safeJoin("/dest", c.name)
		if ok != c.want {
			t.Errorf("safeJoin(%q) = %v want %v", c.name, ok, c.want)
		}
	}
}

func makeZip(t *testing.T, entries map[string]string, evil bool) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for name, content := range entries {
		if evil {
			name = "../evil_" + name
		}
		w, _ := zw.Create(name)
		w.Write([]byte(content))
	}
	zw.Close()
	return buf.Bytes()
}

func makeTarGz(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range entries {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))})
		tw.Write([]byte(content))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestExtractArchive_Zip(t *testing.T) {
	dest := t.TempDir()
	z := makeZip(t, map[string]string{"skills/a.md": "# A", "skills/b.md": "# B"}, false)
	if err := extractArchive(z, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "skills", "a.md")); err != nil {
		t.Fatal("a.md not extracted")
	}
}

func TestExtractArchive_ZipSlip(t *testing.T) {
	dest := t.TempDir()
	z := makeZip(t, map[string]string{"a.md": "# A"}, true) // ../evil_a.md
	if err := extractArchive(z, dest); err == nil {
		t.Fatal("expected zip-slip error")
	}
}

func TestExtractArchive_TarGz(t *testing.T) {
	dest := t.TempDir()
	gz := makeTarGz(t, map[string]string{"skills/c.md": "# C"})
	if err := extractArchive(gz, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "skills", "c.md")); err != nil {
		t.Fatal("c.md not extracted")
	}
}

func TestExtractArchive_TarSlip(t *testing.T) {
	dest := t.TempDir()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	tw.WriteHeader(&tar.Header{Name: "../escape.md", Mode: 0o644, Size: 1})
	tw.Write([]byte("x"))
	tw.Close()
	if err := extractArchive(buf.Bytes(), dest); err == nil {
		t.Fatal("expected tar-slip error")
	}
}

func TestInstallSkill_HTTPDownload(t *testing.T) {
	z := makeZip(t, map[string]string{"skill/SKILL.md": "# Demo skill"}, false)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(z)
	}))
	defer srv.Close()

	dest := t.TempDir()
	card := SkillCard{Card: Card{ID: "s1", Name: "demo"}, ArchiveURL: srv.URL + "/skill.zip"}
	if err := InstallSkill(context.Background(), card, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "skill", "SKILL.md")); err != nil {
		t.Fatal("SKILL.md not extracted")
	}
}

func TestInstallSkill_Errors(t *testing.T) {
	// no archive url
	if err := InstallSkill(context.Background(), SkillCard{Card: Card{ID: "x"}}, t.TempDir()); err == nil {
		t.Fatal("expected error for missing archive url")
	}
	// 404
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	if err := InstallSkill(context.Background(), SkillCard{ArchiveURL: srv.URL + "/nope"}, t.TempDir()); err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestInstallMCPs_Resilient(t *testing.T) {
	f := false
	cards := []MCPCard{
		{Card: Card{ID: "off", Name: "disabled"}, Spec: mcpserver.ServerSpec{Name: "off", Enabled: &f}},
		{Card: Card{ID: "ghost", Name: "missing-binary"}, Spec: mcpserver.ServerSpec{Name: "ghost", Command: "no-such-binary-xyz"}},
	}
	mgr, results := InstallMCPs(context.Background(), cards)
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// both skipped gracefully, no error propagation
	if results[0].Err == nil || results[1].Err == nil {
		t.Fatal("both should report skip reasons")
	}
}
