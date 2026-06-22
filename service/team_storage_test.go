package service

import (
	"context"
	"testing"
)

// testTeamCRUD exercises the full Team storage contract against any Storage
// implementation: round-trip with Members, the leader-session index, the
// user index, not-found errors, and index cleanup on delete.
func testTeamCRUD(t *testing.T, s Storage) {
	ctx := context.Background()

	team := &Team{
		ID:              "team-1",
		UserID:          "u1",
		LeaderSessionID: "sess-1",
		Name:            "Research",
		Description:     "does research",
		Members: []TeamMember{
			{AgentID: "wa", Name: "worker", SessionID: "ws"},
			{AgentID: "wb", Name: "writer", SessionID: "wbs"},
		},
	}
	if err := s.SaveTeam(ctx, team); err != nil {
		t.Fatal(err)
	}

	// Round-trip including Members slice.
	got, err := s.GetTeam(ctx, "team-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Research" || got.LeaderSessionID != "sess-1" || len(got.Members) != 2 {
		t.Fatalf("team roundtrip mismatch: %+v", got)
	}
	if got.Members[0].Name != "worker" || got.Members[1].SessionID != "wbs" {
		t.Fatalf("members mismatch: %+v", got.Members)
	}

	// Leader-session index.
	byLeader, err := s.GetTeamByLeaderSession(ctx, "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if byLeader.ID != "team-1" {
		t.Fatalf("leader index mismatch: %s", byLeader.ID)
	}

	// User index.
	list, err := s.ListTeamsByUser(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "team-1" {
		t.Fatalf("user index mismatch: %+v", list)
	}

	// A second team for the same user proves the index holds multiple.
	team2 := &Team{ID: "team-2", UserID: "u1", LeaderSessionID: "sess-2", Name: "Writing"}
	if err := s.SaveTeam(ctx, team2); err != nil {
		t.Fatal(err)
	}
	list2, _ := s.ListTeamsByUser(ctx, "u1")
	if len(list2) != 2 {
		t.Fatalf("expected 2 teams for user, got %d", len(list2))
	}

	// Not-found errors.
	if _, err := s.GetTeam(ctx, "nope"); err == nil {
		t.Fatal("expected error for missing team")
	}
	if _, err := s.GetTeamByLeaderSession(ctx, "nope"); err == nil {
		t.Fatal("expected error for missing leader session")
	}

	// Update an existing team (e.g. append a member) is an upsert.
	team.Members = append(team.Members, TeamMember{AgentID: "wc", Name: "critic", SessionID: "wcs"})
	if err := s.SaveTeam(ctx, team); err != nil {
		t.Fatal(err)
	}
	got2, _ := s.GetTeam(ctx, "team-1")
	if len(got2.Members) != 3 {
		t.Fatalf("expected 3 members after upsert, got %d", len(got2.Members))
	}

	// Delete cleans all indexes.
	if err := s.DeleteTeam(ctx, "team-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetTeam(ctx, "team-1"); err == nil {
		t.Fatal("expected error after delete")
	}
	if _, err := s.GetTeamByLeaderSession(ctx, "sess-1"); err == nil {
		t.Fatal("leader index not cleaned after delete")
	}
	list3, _ := s.ListTeamsByUser(ctx, "u1")
	if len(list3) != 1 || list3[0].ID != "team-2" {
		t.Fatalf("user index should still hold team-2 only, got %+v", list3)
	}

	// Delete of a missing team is best-effort (idempotent, no error).
	if err := s.DeleteTeam(ctx, "team-1"); err != nil {
		t.Fatalf("delete of missing team should be idempotent, got %v", err)
	}
}

func TestMemoryStorage_TeamCRUD(t *testing.T) {
	testTeamCRUD(t, NewMemoryStorage())
}

func TestRedisStorage_TeamCRUD(t *testing.T) {
	s, mr := setupRedisStorage(t)
	defer mr.Close()
	testTeamCRUD(t, s)
}

// TestStorage_TeamCompileTimeGuarantee ensures both implementations satisfy
// the extended Storage interface (catches future interface drift at compile).
func TestStorage_TeamCompileTimeGuarantee(t *testing.T) {
	var _ Storage = (*MemoryStorage)(nil)
	var _ Storage = (*RedisStorage)(nil)
}
