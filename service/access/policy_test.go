package access

import "testing"

func TestDenyAllPolicy_ListAccessible(t *testing.T) {
	p := DenyAllPolicy{}
	refs, err := p.ListAccessible("user1", KindAgent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("DenyAll should return empty list, got %d", len(refs))
	}
}

func TestDenyAllPolicy_CanEdit_Owner(t *testing.T) {
	p := DenyAllPolicy{}
	ok, err := p.CanEdit("user1", "user1", "agent-123", KindAgent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("owner should always be able to edit their own resource")
	}
}

func TestDenyAllPolicy_CanEdit_CrossUser(t *testing.T) {
	p := DenyAllPolicy{}
	ok, err := p.CanEdit("user1", "user2", "agent-123", KindAgent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("DenyAll should deny cross-user edit")
	}
}

func TestStaticPolicy_ListAccessible(t *testing.T) {
	p := &StaticPolicy{
		Refs: []ResourceRef{
			{Kind: KindAgent, OwnerID: "user2", ResourceID: "a1", Permission: PermRead},
			{Kind: KindKnowledgeBase, OwnerID: "user3", ResourceID: "kb1", Permission: PermEdit},
			{Kind: KindAgent, OwnerID: "user4", ResourceID: "a2", Permission: PermEdit},
		},
	}
	agents, _ := p.ListAccessible("user1", KindAgent)
	if len(agents) != 2 {
		t.Fatalf("expected 2 agent refs, got %d", len(agents))
	}
	kbs, _ := p.ListAccessible("user1", KindKnowledgeBase)
	if len(kbs) != 1 {
		t.Fatalf("expected 1 KB ref, got %d", len(kbs))
	}
}

func TestStaticPolicy_CanEdit(t *testing.T) {
	p := &StaticPolicy{
		Refs: []ResourceRef{
			{Kind: KindAgent, OwnerID: "user2", ResourceID: "a1", Permission: PermRead},
			{Kind: KindAgent, OwnerID: "user2", ResourceID: "a2", Permission: PermEdit},
		},
	}
	// Owner always can
	ok, _ := p.CanEdit("user2", "user2", "a1", KindAgent)
	if !ok {
		t.Fatal("owner should always be able to edit")
	}
	// Cross-user with READ only → cannot edit
	ok, _ = p.CanEdit("user1", "user2", "a1", KindAgent)
	if ok {
		t.Fatal("READ permission should not allow edit")
	}
	// Cross-user with EDIT → can edit
	ok, _ = p.CanEdit("user1", "user2", "a2", KindAgent)
	if !ok {
		t.Fatal("EDIT permission should allow edit")
	}
	// Cross-user with no ref → cannot edit
	ok, _ = p.CanEdit("user1", "user2", "a3", KindAgent)
	if ok {
		t.Fatal("no ref should deny edit")
	}
}
