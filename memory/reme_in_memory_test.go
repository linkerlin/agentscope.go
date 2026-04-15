package memory

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestReMeInMemoryMemoryAddAndGet(t *testing.T) {
	m := NewReMeInMemoryMemory("")
	_ = m.Add(message.NewMsg().Role(message.RoleUser).TextContent("hello").Build())
	_ = m.Add(message.NewMsg().Role(message.RoleAssistant).TextContent("world").Build())

	all, _ := m.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(all))
	}

	recent, _ := m.GetRecent(1)
	if len(recent) != 1 || recent[0].GetTextContent() != "world" {
		t.Fatalf("expected recent message 'world', got %v", recent)
	}

	if m.Size() != 2 {
		t.Fatalf("expected size 2, got %d", m.Size())
	}
}

func TestReMeInMemoryMemoryClear(t *testing.T) {
	m := NewReMeInMemoryMemory("")
	_ = m.Add(message.NewMsg().Role(message.RoleUser).TextContent("x").Build())
	_ = m.Clear()
	if m.Size() != 0 {
		t.Fatalf("expected size 0 after clear, got %d", m.Size())
	}
}

func TestReMeInMemoryMemorySnapshotRoundTrip(t *testing.T) {
	m := NewReMeInMemoryMemory("")
	m.SetCompSum("summary")
	m.SetLongTermMemory("long")
	m.Marks().Add("msg1", MarkCompressed)

	snap := m.Snapshot()
	m2 := NewReMeInMemoryMemory("")
	m2.Restore(snap)

	if m2.GetCompSum() != "summary" {
		t.Fatalf("expected compSum 'summary', got %s", m2.GetCompSum())
	}
	if m2.GetLongTermMemory() != "long" {
		t.Fatalf("expected longTerm 'long', got %s", m2.GetLongTermMemory())
	}
	if !m2.Marks().Has("msg1", MarkCompressed) {
		t.Fatal("expected mark restored")
	}
}

func TestReMeFileMemoryDelegatesToInMemory(t *testing.T) {
	m, _ := NewReMeFileMemory(DefaultReMeFileConfig(), NewSimpleTokenCounter())
	defer m.Close()

	_ = m.Add(message.NewMsg().Role(message.RoleUser).TextContent("delegated").Build())
	if m.Size() != 1 {
		t.Fatalf("expected size 1, got %d", m.Size())
	}
	m.SetLongTermMemory("lt")
	if m.GetLongTermMemory() != "lt" {
		t.Fatalf("expected longTerm 'lt', got %s", m.GetLongTermMemory())
	}
}
