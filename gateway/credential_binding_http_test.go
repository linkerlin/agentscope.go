package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/service"
)

// jsonUnmarshalInto decodes b into v (tiny helper keeping assertions tight).
func jsonUnmarshalInto(b []byte, v any) error { return json.Unmarshal(b, v) }

// credentialBindingFixture builds an authed server with a PENDING binding.
func credentialBindingFixture(t *testing.T) (*Server, *sessionIdentityFixture, string) {
	t.Helper()
	f := newSessionIdentityFixture(t)
	create := f.post(t, "/api/v1/credentials",
		`{"provider":"dingtalk","label":"corp-app","binding_ref":"flow-42"}`, "")
	if create.Code != http.StatusCreated {
		t.Fatalf("binding create: %d body=%s", create.Code, create.Body.String())
	}
	// Read back the id from the list (the create response embeds the record;
	// Encrypted is omitted but id/status/binding_ref are visible).
	var resp struct {
		ID         string `json:"id"`
		Status     string `json:"status"`
		BindingRef string `json:"binding_ref"`
	}
	if err := jsonUnmarshalInto(create.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID == "" || resp.Status != "PENDING" || resp.BindingRef != "flow-42" {
		t.Fatalf("unexpected binding create response: %+v", resp)
	}
	return f.srv, f, resp.ID
}

// TestCredentialBindingHTTPFlow walks the HTTP lifecycle: create PENDING →
// authorize delivers the secret → duplicate authorize is idempotent →
// cancel refused 409. The secret never appears in any response (22.1).
func TestCredentialBindingHTTPFlow(t *testing.T) {
	_, f, id := credentialBindingFixture(t)

	// Authorize with the secret.
	rec := f.post(t, "/api/v1/credentials/"+id+"/authorize", `{"value":"super-secret"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("authorize: %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "super-secret") {
		t.Fatal("authorize response leaks the secret")
	}

	// Duplicate authorize: idempotent success (duplicate callback).
	rec2 := f.post(t, "/api/v1/credentials/"+id+"/authorize", `{"value":"other"}`, "")
	if rec2.Code != http.StatusOK {
		t.Fatalf("duplicate authorize: %d body=%s", rec2.Code, rec2.Body.String())
	}

	// The stored secret is the FIRST delivery (idempotent no-op did not
	// re-apply), and it round-trips through storage only.
	cred, err := f.storage.GetCredential(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if cred.Encrypted != "super-secret" {
		t.Fatalf("secret mutated by duplicate callback: %q", cred.Encrypted)
	}
	if cred.NormalizedStatus() != service.CredentialAuthorized {
		t.Fatalf("status: %s", cred.NormalizedStatus())
	}

	// Cancel after authorize: 409 (terminal states are irreversible).
	rec3 := f.post(t, "/api/v1/credentials/"+id+"/cancel", `{}`, "")
	if rec3.Code != http.StatusConflict {
		t.Fatalf("cancel after authorize: expected 409, got %d", rec3.Code)
	}
}

// TestCredentialBindingHTTPCancelFlow: cancel works from PENDING and is
// idempotent; fail after cancel is 409.
func TestCredentialBindingHTTPCancelFlow(t *testing.T) {
	_, f, id := credentialBindingFixture(t)

	if rec := f.post(t, "/api/v1/credentials/"+id+"/cancel", `{}`, ""); rec.Code != http.StatusOK {
		t.Fatalf("cancel: %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := f.post(t, "/api/v1/credentials/"+id+"/cancel", `{}`, ""); rec.Code != http.StatusOK {
		t.Fatalf("repeated cancel: %d", rec.Code)
	}
	if rec := f.post(t, "/api/v1/credentials/"+id+"/fail", `{}`, ""); rec.Code != http.StatusConflict {
		t.Fatalf("fail after cancel: expected 409, got %d", rec.Code)
	}
}

// TestCredentialBindingHTTPOwnership: another user gets a uniform 404 on
// the transitions (no existence leak, 22.2 discipline).
func TestCredentialBindingHTTPOwnership(t *testing.T) {
	srv, _, id := credentialBindingFixture(t)

	// Second user.
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"name":"Bob"}`))
	regRec := httptest.NewRecorder()
	srv.ServeHTTP(regRec, regReq)
	var regResp registerResponse
	jsonUnmarshalInto(regRec.Body.Bytes(), &regResp)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"api_key":"`+regResp.APIKey+`"}`))
	loginRec := httptest.NewRecorder()
	srv.ServeHTTP(loginRec, loginReq)
	var loginResp loginResponse
	jsonUnmarshalInto(loginRec.Body.Bytes(), &loginResp)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/credentials/"+id+"/authorize", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign user authorize: expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}
