package state

import (
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
