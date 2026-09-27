package dingtalk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCardAPI records card/wiki calls and serves canned responses.
type fakeCardAPI struct {
	mu      sync.Mutex
	bodies  map[string]string // path -> last body
	srv     *httptest.Server
	wikiRes string
}

func newFakeCardAPI(t *testing.T) *fakeCardAPI {
	f := &fakeCardAPI{bodies: map[string]string{}, wikiRes: `{"result":[{"nodeToken":"nt1","title":"Runbook","type":"wiki","fullScreenUrl":"https://docs/x"}]}`}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0/oauth2/accessToken", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"accessToken":"tok-1","expireIn":7200}`))
	})
	capture := func(path string, reply string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			f.mu.Lock()
			f.bodies[path] = string(b)
			f.mu.Unlock()
			w.Write([]byte(reply))
		})
	}
	capture("/v1.0/card/instances/createAndDeliver", `{}`)
	capture("/v1.0/card/instances", `{}`)
	capture("/v2.0/dingtalk/wiki/nodes/search", f.wikiRes)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// TestCardSender_CreateAndUpdate verifies create-and-deliver carries the
// template/outTrack/space body, and updates target the same outTrackId.
func TestCardSender_CreateAndUpdate(t *testing.T) {
	f := newFakeCardAPI(t)
	ch := New("dt-1", "app-key", "app-secret").WithBaseURL(f.srv.URL)
	s := NewCardSender(ch)

	require.NoError(t, s.CreateAndDeliver(context.Background(), "cid-1", "tpl-ai-1", "turn-42",
		map[string]string{"title": "Deploy", "status": "running"}))
	require.NoError(t, s.UpdateCard(context.Background(), "turn-42", map[string]string{"status": "done"}))

	f.mu.Lock()
	defer f.mu.Unlock()
	created := f.bodies["/v1.0/card/instances/createAndDeliver"]
	assert.Contains(t, created, `"cardTemplateId":"tpl-ai-1"`)
	assert.Contains(t, created, `"outTrackId":"turn-42"`)
	assert.Contains(t, created, `"openSpaceId":"dt@dt"`)
	assert.Contains(t, created, `"openConversationId":"cid-1"`)
	assert.Contains(t, created, `"status":"running"`)
	updated := f.bodies["/v1.0/card/instances"]
	assert.Contains(t, updated, `"outTrackId":"turn-42"`)
	assert.Contains(t, updated, `"status":"done"`)
}

// TestParseCardCallback verifies the callback body shape decodes into the
// normalised struct.
func TestParseCardCallback(t *testing.T) {
	body := `{"cardId":"card-9","trackId":"turn-42","userId":"u1","content":{"value":{"type":"hitl_decision","session_id":"s1","confirm_id":"c1"}}}`
	req := httptest.NewRequest(http.MethodPost, "/cb", strings.NewReader(body))
	cb, err := ParseCardCallback(req)
	require.NoError(t, err)
	assert.Equal(t, "card-9", cb.CardID)
	assert.Equal(t, "turn-42", cb.TrackID)
	assert.Equal(t, "u1", cb.UserID)

	ev, err := ConfirmDecisionFromCallback(cb)
	require.NoError(t, err)
	assert.Equal(t, "c1", ev.ConfirmID)
}

// TestWikiTools verifies search and node-detail tools against the fake API.
func TestWikiTools(t *testing.T) {
	f := newFakeCardAPI(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0/oauth2/accessToken", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"accessToken":"tok-9","expireIn":7200}`))
	})
	mux.HandleFunc("/v2.0/dingtalk/wiki/nodes/search", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(f.wikiRes))
	})
	mux.HandleFunc("/v2.0/dingtalk/wiki/nodes/nt1", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"nodeToken":"nt1","title":"Runbook","type":"wiki","spaceId":"sp1","fullScreenUrl":"https://docs/x"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ch := New("dt-1", "app-key", "app-secret").WithBaseURL(srv.URL)
	search := &WikiSearchTool{Ch: ch}
	resp, err := search.Execute(context.Background(), map[string]any{"keyword": "runbook"})
	require.NoError(t, err)
	assert.Contains(t, resp.GetTextContent(), "Runbook")
	assert.Contains(t, resp.GetTextContent(), "node_token=nt1")

	node := &WikiNodeTool{Ch: ch}
	resp, err = node.Execute(context.Background(), map[string]any{"node_token": "nt1"})
	require.NoError(t, err)
	assert.Contains(t, resp.GetTextContent(), "space: sp1")
}
