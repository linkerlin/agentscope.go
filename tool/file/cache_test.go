package file

import (
	"testing"
	"time"
)

func TestReadCache(t *testing.T) {
	c := NewReadCache(2, 1)
	now := time.Now()
	c.Put("/a", now, []byte("aaa"))
	c.Put("/b", now, []byte("bbb"))
	if _, ok := c.Get("/a", now); !ok {
		t.Fatal("expected cache hit for /a")
	}
	c.Put("/c", now, []byte("cccccc"))
	// /b was LRU after /a was touched, so /b should be evicted first.
	if _, ok := c.Get("/b", now); ok {
		t.Fatal("expected /b evicted")
	}
	if _, ok := c.Get("/a", now); !ok {
		t.Fatal("expected /a retained")
	}
}
