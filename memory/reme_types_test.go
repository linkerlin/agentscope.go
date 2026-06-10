package memory

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestSimpleTokenCounter(t *testing.T) {
	c := NewSimpleTokenCounter()
	n, err := c.Count("abcd")
	if err != nil || n != 1 {
		t.Fatalf("got %d err=%v", n, err)
	}
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hello").Build(),
	}
	n2, err := c.CountMessages(msgs)
	if err != nil || n2 < 1 {
		t.Fatalf("messages tokens %d", n2)
	}
}

func TestEmbeddingContent(t *testing.T) {
	t.Run("no when_to_use returns content", func(t *testing.T) {
		n := &MemoryNode{Content: "hello world", WhenToUse: ""}
		if got := n.EmbeddingContent(); got != "hello world" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("when_to_use set returns when_to_use", func(t *testing.T) {
		n := &MemoryNode{
			Content:   "user prefers Python for data processing tasks",
			WhenToUse: "当用户询问Python或数据处理问题时检索此记忆",
		}
		if got := n.EmbeddingContent(); got != n.WhenToUse {
			t.Fatalf("got %q, want WhenToUse", got)
		}
	})

	t.Run("nil node returns empty", func(t *testing.T) {
		var n *MemoryNode
		if got := n.EmbeddingContent(); got != "" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("empty content and when_to_use returns empty", func(t *testing.T) {
		n := &MemoryNode{}
		if got := n.EmbeddingContent(); got != "" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestNewMemoryNodeWithWhen(t *testing.T) {
	n := NewMemoryNodeWithWhen(MemoryTypePersonal, "alice",
		"用户喜欢用pandas处理数据",
		"当用户提到pandas或数据处理时检索",
	)
	if n.Content != "用户喜欢用pandas处理数据" {
		t.Fatal("content mismatch")
	}
	if n.WhenToUse != "当用户提到pandas或数据处理时检索" {
		t.Fatal("when_to_use mismatch")
	}
	if n.MemoryType != MemoryTypePersonal {
		t.Fatal("memory type mismatch")
	}
	if n.MemoryTarget != "alice" {
		t.Fatal("target mismatch")
	}
	if n.MemoryID == "" {
		t.Fatal("memory id empty")
	}
	if n.EmbeddingContent() != n.WhenToUse {
		t.Fatal("embedding should use when_to_use")
	}
}

func TestMarkStore(t *testing.T) {
	s := NewMarkStore()
	s.Add("m1", MarkCompressed)
	if !s.Has("m1", MarkCompressed) {
		t.Fatal()
	}
	s.Add("m1", MarkCompressed)
	if len(s.Get("m1")) != 1 {
		t.Fatal()
	}
}
