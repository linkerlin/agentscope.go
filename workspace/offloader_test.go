package workspace

import (
	"context"
	"testing"
)

func TestMemoryOffloader(t *testing.T) {
	o := NewMemoryOffloader()
	ref, err := o.OffloadContext(context.Background(), "sess1", []byte(`{"hello":1}`))
	if err != nil {
		t.Fatal(err)
	}
	data, ok := o.Get(ref)
	if !ok || len(data) == 0 {
		t.Fatal("expected stored data")
	}
}
