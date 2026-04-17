package state

import (
	"os"
	"path/filepath"
	"testing"
)

type testState struct {
	V string `json:"v"`
}

func (t testState) StateType() string { return "test" }

func TestJSONStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSONStore(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save("k1", testState{V: "hello"}); err != nil {
		t.Fatal(err)
	}
	if !s.Exists("k1") {
		t.Fatal("expected exists")
	}
	var got testState
	if err := s.Get("k1", &got); err != nil {
		t.Fatal(err)
	}
	if got.V != "hello" {
		t.Fatalf("got %q", got.V)
	}
	keys := s.ListKeys()
	if len(keys) != 1 || keys[0] != "k1" {
		t.Fatalf("keys %#v", keys)
	}
	if err := s.Delete("k1"); err != nil {
		t.Fatal(err)
	}
	if s.Exists("k1") {
		t.Fatal("expected deleted")
	}
}


func TestJSONStore_NewError(t *testing.T) {
	f, err := os.CreateTemp("", "state-block-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()
	_, err = NewJSONStore(f.Name())
	if err == nil {
		t.Fatal("expected error when basePath is a file")
	}
}

func TestJSONStore_SaveErrors(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewJSONStore(dir)
	if err := s.Save("", testState{V: "x"}); err == nil {
		t.Fatal("expected error for empty key")
	}
	if err := s.Save("k", nil); err == nil {
		t.Fatal("expected error for nil value")
	}
}

func TestJSONStore_GetErrors(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewJSONStore(dir)
	if err := s.Get("", &testState{}); err == nil {
		t.Fatal("expected error for empty key")
	}
	if err := s.Get("k", nil); err == nil {
		t.Fatal("expected error for nil dest")
	}
	if err := s.Get("missing", &testState{}); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestJSONStore_ExistsEmptyKey(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewJSONStore(dir)
	if s.Exists("") {
		t.Fatal("expected false for empty key")
	}
}

func TestJSONStore_DeleteEmptyKey(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewJSONStore(dir)
	if err := s.Delete(""); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestJSONStore_ListKeysWithSubdir(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewJSONStore(dir)
	_ = s.Save("a", testState{V: "1"})
	_ = os.Mkdir(filepath.Join(dir, "subdir"), 0o755)
	keys := s.ListKeys()
	if len(keys) != 1 || keys[0] != "a" {
		t.Fatalf("expected [a], got %v", keys)
	}
}

func TestJSONStore_ListKeysBadDir(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewJSONStore(dir)
	// corrupt basePath to trigger ReadDir error
	s.basePath = filepath.Join(dir, "nonexistent")
	keys := s.ListKeys()
	if keys != nil {
		t.Fatalf("expected nil, got %v", keys)
	}
}
