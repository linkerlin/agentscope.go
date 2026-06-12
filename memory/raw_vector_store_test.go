package memory

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/memory/vector"
)

func TestRawVectorStoreStub(t *testing.T) {
	s := NewRawVectorStore(nil)

	err := s.Insert(context.Background(), nil)
	if err != nil {
		t.Fatal("Insert returned error:", err)
	}

	nodes, err := s.Search(context.Background(), "", vector.RetrieveOptions{})
	if err != nil {
		t.Fatal("Search returned error:", err)
	}
	if nodes != nil {
		t.Fatal("Search should return nil for stub")
	}

	node, err := s.Get(context.Background(), "test")
	if err != nil {
		t.Fatal("Get returned error:", err)
	}
	if node != nil {
		t.Fatal("Get should return nil for stub")
	}
}
