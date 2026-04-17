package memory

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func makeMsg(text string) *message.Msg {
	return message.NewMsg().Role(message.RoleUser).TextContent(text).Build()
}

func TestInMemoryAdd(t *testing.T) {
	m := NewInMemoryMemory()
	if err := m.Add(makeMsg("hello")); err != nil {
		t.Fatal(err)
	}
	if m.Size() != 1 {
		t.Errorf("expected size 1, got %d", m.Size())
	}
}

func TestInMemoryGetAll(t *testing.T) {
	m := NewInMemoryMemory()
	_ = m.Add(makeMsg("a"))
	_ = m.Add(makeMsg("b"))
	msgs, err := m.GetAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 msgs, got %d", len(msgs))
	}
}

func TestInMemoryGetRecent(t *testing.T) {
	m := NewInMemoryMemory()
	for _, s := range []string{"a", "b", "c", "d"} {
		_ = m.Add(makeMsg(s))
	}
	recent, err := m.GetRecent(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2, got %d", len(recent))
	}
	if recent[0].GetTextContent() != "c" || recent[1].GetTextContent() != "d" {
		t.Errorf("unexpected recent messages: %v %v", recent[0].GetTextContent(), recent[1].GetTextContent())
	}
}

func TestInMemoryClear(t *testing.T) {
	m := NewInMemoryMemory()
	_ = m.Add(makeMsg("hello"))
	if err := m.Clear(); err != nil {
		t.Fatal(err)
	}
	if m.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", m.Size())
	}
}


func TestInMemoryGetRecentMoreThanSize(t *testing.T) {
	m := NewInMemoryMemory()
	_ = m.Add(makeMsg("a"))
	recent, _ := m.GetRecent(5)
	if len(recent) != 1 {
		t.Fatalf("expected 1, got %d", len(recent))
	}
}

func TestInMemoryAddNil(t *testing.T) {
	m := NewInMemoryMemory()
	if err := m.Add(nil); err != nil {
		t.Fatal(err)
	}
	// Current implementation appends nil; reflect actual behavior
	if m.Size() != 1 {
		t.Fatalf("expected size 1 (nil appended), got %d", m.Size())
	}
}
