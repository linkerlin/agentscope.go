package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/service"
)

func TestSchedule_PersistedCRUD(t *testing.T) {
	storage := service.NewMemoryStorage()
	reg := NewAgentRegistry()
	btm := NewBackgroundTaskManager(reg, nil).WithStorage(storage)

	srv := NewServer(&mockAgent{name: "test"})
	srv.WithStorage(storage).WithBackgroundTaskManager(btm)
	srv.RegisterScheduleRoutes()

	body, _ := json.Marshal(scheduleRequest{
		ID:       "j1",
		UserID:   "u1",
		AgentID:  "a1",
		CronExpr: "*/5 * * * *",
		Payload:  "ping",
	})
	req := httptest.NewRequest(http.MethodPost, "/schedule", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, "u1"))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	got, err := storage.GetSchedule(context.Background(), "j1")
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != "u1" || got.CronExpr != "*/5 * * * *" {
		t.Fatalf("unexpected stored schedule: %+v", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/schedule", nil)
	req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, "u1"))
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rr.Code)
	}
	var listed listSchedulesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.Total != 1 {
		t.Fatalf("expected 1 schedule, got %d", listed.Total)
	}

	patchBody, _ := json.Marshal(updateScheduleRequest{Payload: strPtr("pong")})
	req = httptest.NewRequest(http.MethodPatch, "/schedule/j1", bytes.NewReader(patchBody))
	req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, "u1"))
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	got, _ = storage.GetSchedule(context.Background(), "j1")
	if got.Payload != "pong" {
		t.Fatalf("expected payload pong, got %q", got.Payload)
	}

	req = httptest.NewRequest(http.MethodDelete, "/schedule/j1", nil)
	req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, "u1"))
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", rr.Code)
	}
	if _, err := storage.GetSchedule(context.Background(), "j1"); err == nil {
		t.Fatal("expected schedule removed from storage")
	}
}

func TestSchedule_LoadOnStart(t *testing.T) {
	storage := service.NewMemoryStorage()
	_ = storage.SaveSchedule(context.Background(), &service.Schedule{
		ID:        "boot",
		UserID:    "u1",
		AgentID:   "a1",
		CronExpr:  "0 * * * *",
		Payload:   "boot",
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	btm := NewBackgroundTaskManager(NewAgentRegistry(), nil).WithStorage(storage)
	btm.Start()
	defer btm.Stop()

	jobs := btm.List()
	if len(jobs) != 1 || jobs[0].ID != "boot" {
		t.Fatalf("expected boot job loaded, got %+v", jobs)
	}
}

func strPtr(s string) *string { return &s }
