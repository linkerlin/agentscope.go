package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/service"
)

func setupServiceServer(t *testing.T) (*Server, *service.MemoryStorage, string) {
	t.Helper()
	storage := service.NewMemoryStorage()
	ctx := context.Background()
	user := &service.User{ID: "u1", Name: "Alice"}
	if err := storage.SaveUser(ctx, user); err != nil {
		t.Fatal(err)
	}
	key, err := service.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	storage.SaveCredential(ctx, &service.Credential{
		ID:        "c1",
		UserID:    "u1",
		Provider:  "api_key",
		Encrypted: service.HashAPIKey(key),
	})

	apiAuth := service.NewAPIKeyAuthenticator(storage, "")
	srv := NewServer(&mockAgent{name: "test"}).WithStorage(storage).WithAuthenticator(apiAuth)
	srv.RegisterServiceRoutes()
	return srv, storage, key
}

func TestServiceHandlers_AgentCRUD(t *testing.T) {
	srv, _, key := setupServiceServer(t)

	// Create
	body := `{"name":"my-agent","model_id":"gpt-4"}`
	req := httptest.NewRequest("POST", "/api/v1/agents", bytes.NewReader([]byte(body)))
	req.Header.Set("X-API-Key", key)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var agent service.AgentConfig
	json.Unmarshal(rec.Body.Bytes(), &agent)
	if agent.Name != "my-agent" {
		t.Fatalf("unexpected agent name: %s", agent.Name)
	}

	// List
	req = httptest.NewRequest("GET", "/api/v1/agents", nil)
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rec.Code)
	}
	var agents []*service.AgentConfig
	json.Unmarshal(rec.Body.Bytes(), &agents)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	// Get
	req = httptest.NewRequest("GET", "/api/v1/agents/"+agent.ID, nil)
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Patch
	patchBody := `{"name":"renamed","model_id":"gpt-4o"}`
	req = httptest.NewRequest("PATCH", "/api/v1/agents/"+agent.ID, bytes.NewReader([]byte(patchBody)))
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var patched service.AgentConfig
	json.Unmarshal(rec.Body.Bytes(), &patched)
	if patched.Name != "renamed" || patched.ModelID != "gpt-4o" {
		t.Fatalf("unexpected patch result: %+v", patched)
	}

	// Delete
	req = httptest.NewRequest("DELETE", "/api/v1/agents/"+agent.ID, nil)
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", rec.Code)
	}
}

func TestServiceHandlers_SessionCRUD(t *testing.T) {
	srv, storage, key := setupServiceServer(t)

	// Create
	body := `{"title":"my-session"}`
	req := httptest.NewRequest("POST", "/api/v1/sessions", bytes.NewReader([]byte(body)))
	req.Header.Set("X-API-Key", key)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var sess service.Session
	json.Unmarshal(rec.Body.Bytes(), &sess)

	// List
	req = httptest.NewRequest("GET", "/api/v1/sessions", nil)
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rec.Code)
	}

	// Get
	req = httptest.NewRequest("GET", "/api/v1/sessions/"+sess.ID, nil)
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", rec.Code)
	}

	// Patch
	patchBody := `{"title":"renamed-session"}`
	req = httptest.NewRequest("PATCH", "/api/v1/sessions/"+sess.ID, bytes.NewReader([]byte(patchBody)))
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: expected 200, got %d", rec.Code)
	}

	// Messages
	_ = storage.SaveMessage(context.Background(), &service.StoredMessage{
		ID: "m1", SessionID: sess.ID, Role: "user", Content: "hi", CreatedAt: time.Now(),
	})
	req = httptest.NewRequest("GET", "/api/v1/sessions/"+sess.ID+"/messages", nil)
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("messages: expected 200, got %d", rec.Code)
	}
	var msgResp listSessionMessagesResponse
	json.Unmarshal(rec.Body.Bytes(), &msgResp)
	if msgResp.Total != 1 {
		t.Fatalf("expected 1 message, got %d", msgResp.Total)
	}

	// Delete
	req = httptest.NewRequest("DELETE", "/api/v1/sessions/"+sess.ID, nil)
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", rec.Code)
	}
}

func TestServiceHandlers_CredentialCRUD(t *testing.T) {
	srv, _, key := setupServiceServer(t)

	// Create
	body := `{"provider":"openai","label":"default","value":"sk-test"}`
	req := httptest.NewRequest("POST", "/api/v1/credentials", bytes.NewReader([]byte(body)))
	req.Header.Set("X-API-Key", key)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var cred service.Credential
	json.Unmarshal(rec.Body.Bytes(), &cred)

	// List
	req = httptest.NewRequest("GET", "/api/v1/credentials", nil)
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rec.Code)
	}

	// Get
	req = httptest.NewRequest("GET", "/api/v1/credentials/"+cred.ID, nil)
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", rec.Code)
	}

	// Patch
	patchBody := `{"label":"updated"}`
	req = httptest.NewRequest("PATCH", "/api/v1/credentials/"+cred.ID, bytes.NewReader([]byte(patchBody)))
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: expected 200, got %d", rec.Code)
	}

	// Delete
	req = httptest.NewRequest("DELETE", "/api/v1/credentials/"+cred.ID, nil)
	req.Header.Set("X-API-Key", key)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", rec.Code)
	}
}

// TestServiceHandlers_CredentialResponseRedacted locks the 22.1 rule: no
// credential endpoint response contains the stored secret.
func TestServiceHandlers_CredentialResponseRedacted(t *testing.T) {
	srv, _, key := setupServiceServer(t)

	providerValue, err := service.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"provider": "openai", "label": "default", "value": providerValue})
	req := httptest.NewRequest("POST", "/api/v1/credentials", bytes.NewReader(body))
	req.Header.Set("X-API-Key", key)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if resp := rec.Body.String(); strings.Contains(resp, "encrypted") || strings.Contains(resp, providerValue) {
		t.Fatalf("create response leaks secret: %s", resp)
	}

	var cred service.Credential
	json.Unmarshal(rec.Body.Bytes(), &cred)

	for _, tc := range []struct{ name, path string }{
		{"get", "/api/v1/credentials/" + cred.ID},
		{"list", "/api/v1/credentials"},
	} {
		req := httptest.NewRequest("GET", tc.path, nil)
		req.Header.Set("X-API-Key", key)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", tc.name, rec.Code)
		}
		if resp := rec.Body.String(); strings.Contains(resp, "encrypted") || strings.Contains(resp, providerValue) {
			t.Fatalf("%s response leaks secret: %s", tc.name, resp)
		}
	}
}

// TestServiceHandlers_ProductionRequiresCipher locks the 22.1 rule: in
// production mode credential writes are refused without a cipher.
func TestServiceHandlers_ProductionRequiresCipher(t *testing.T) {
	storage := service.NewMemoryStorage()
	ctx := context.Background()
	storage.SaveUser(ctx, &service.User{ID: "u1", Name: "Alice"})
	key, err := service.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	storage.SaveCredential(ctx, &service.Credential{
		ID: "c1", UserID: "u1", Provider: "api_key", Encrypted: service.HashAPIKey(key),
	})

	apiAuth := service.NewAPIKeyAuthenticator(storage, "")
	srv := NewServer(&mockAgent{name: "test"}).
		WithStorage(storage).
		WithAuthenticator(apiAuth).
		WithProduction(true)
	srv.RegisterServiceRoutes()

	providerValue, _ := service.GenerateAPIKey()
	body, _ := json.Marshal(map[string]string{"provider": "openai", "label": "default", "value": providerValue})
	req := httptest.NewRequest("POST", "/api/v1/credentials", bytes.NewReader(body))
	req.Header.Set("X-API-Key", key)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 in production mode without cipher, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestServiceHandlers_Forbidden(t *testing.T) {
	storage := service.NewMemoryStorage()
	ctx := context.Background()
	user1 := &service.User{ID: "u1", Name: "Alice"}
	user2 := &service.User{ID: "u2", Name: "Bob"}
	storage.SaveUser(ctx, user1)
	storage.SaveUser(ctx, user2)
	key1, err := service.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	key2, err := service.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	storage.SaveCredential(ctx, &service.Credential{ID: "c1", UserID: "u1", Provider: "api_key", Encrypted: service.HashAPIKey(key1)})
	storage.SaveCredential(ctx, &service.Credential{ID: "c2", UserID: "u2", Provider: "api_key", Encrypted: service.HashAPIKey(key2)})

	// Create an agent owned by u1
	storage.SaveAgentConfig(ctx, &service.AgentConfig{ID: "a1", UserID: "u1", Name: "agent1"})

	apiAuth := service.NewAPIKeyAuthenticator(storage, "")
	srv := NewServer(&mockAgent{name: "test"}).WithStorage(storage).WithAuthenticator(apiAuth)
	srv.RegisterServiceRoutes()

	// u2 tries to access u1's agent
	req := httptest.NewRequest("GET", "/api/v1/agents/a1", nil)
	req.Header.Set("X-API-Key", key2)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestServiceHandlers_Unauthorized(t *testing.T) {
	storage := service.NewMemoryStorage()
	apiAuth := service.NewAPIKeyAuthenticator(storage, "")
	srv := NewServer(&mockAgent{name: "test"}).WithStorage(storage).WithAuthenticator(apiAuth)
	srv.RegisterServiceRoutes()

	req := httptest.NewRequest("GET", "/api/v1/agents", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
