// Command controlplane_http drives the evolved control-plane HTTP API end to
// end: goal/todo creation, authorize (turn-token minting), writeback with
// evidence, spend with the token, gate open/resolve, kanban projection, the
// capability catalog, and maintenance. Run:
//
//	go run ./examples/controlplane_http
//
// Uses an in-process httptest server backed by an in-memory control-plane
// Kernel — no external services, no ports required.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/linkerlin/agentscope.go/controlplane"
	"github.com/linkerlin/agentscope.go/gateway"
)

func main() {
	// Build the gateway with the control plane attached (memory kernel).
	k := controlplane.NewKernel(nil, nil, nil).WithTicketEnforcement()
	srv := gateway.NewServer(nil).WithControlPlane(k)
	srv.RegisterControlPlaneRoutes()
	// Expose governance counters as Prometheus metrics at /metrics.
	srv.WithMetricsRegistry(prometheus.NewRegistry())
	ts := httptest.NewServer(srv)
	defer ts.Close()
	ctx := context.Background()
	_ = ctx

	fmt.Println("== HTTP control-plane flow ==")

	// 1. Create a goal + todo.
	goal := postJSON(ts.URL, "POST", "/api/v1/controlplane/goals",
		map[string]any{"objective": "HTTP demo goal", "scope": []string{"safe"}})
	gid := goal["id"].(string)
	fmt.Printf("  created goal: %s\n", gid)
	todo := postJSON(ts.URL, "POST", "/api/v1/controlplane/goals/"+gid+"/todos",
		map[string]any{"description": "do the work", "current_todo": true})
	tid := todo["id"].(string)
	fmt.Printf("  created todo: %s\n", tid)

	// 2. Authorize a turn -> get the TurnToken.
	dec := getJSON(ts.URL, "POST", "/api/v1/controlplane/goals/"+gid+"/authorize?turn=turn-http&agent=worker",
		nil)
	token := dec["turn_token"].(string)
	fmt.Printf("  authorize: should_run=%v token=%s…\n", dec["should_run"], token[:8])

	// 3. Writeback with evidence.
	delivery := postJSON(ts.URL, "POST", "/api/v1/controlplane/goals/"+gid+"/writeback", map[string]any{
		"todo_id": tid, "turn_id": "turn-http",
		"outcome":  map[string]any{"status": "progress"},
		"evidence": []any{map[string]any{"id": "ev-h", "kind": "diff", "summary": "http demo evidence"}},
	})
	fmt.Printf("  writeback: delivery=%v\n", delivery["delivery_id"])

	// 4. Spend WITH the token (enforcement on -> required).
	spent := postJSON(ts.URL, "POST", "/api/v1/controlplane/goals/"+gid+"/spend",
		map[string]any{"turn_id": "turn-http", "turn_token": token, "execute": true})
	fmt.Printf("  spend with token: spent_in_window=%v\n", spent["spent_in_window"])

	// 5. Open a gate, watch should-run flip to blocked, then resolve.
	gate := postJSON(ts.URL, "POST", "/api/v1/controlplane/goals/"+gid+"/gates",
		map[string]any{"question": "approve ship?", "scope": map[string]any{"kind": "production", "scope_key": "prod"}})
	gid_ := gate["gate_id"].(string)
	blocked := getJSON(ts.URL, "GET", "/api/v1/controlplane/goals/"+gid+"/should-run?agent=worker", nil)
	fmt.Printf("  gate opened -> should_run=%v state=%s question=%q\n",
		blocked["should_run"], blocked["state"], blocked["question"])
	postJSON(ts.URL, "POST", "/api/v1/controlplane/goals/"+gid+"/gates/"+gid_+"/resolve",
		map[string]any{"decision": "approve", "by": "operator"})
	ok := getJSON(ts.URL, "GET", "/api/v1/controlplane/goals/"+gid+"/should-run?agent=worker", nil)
	fmt.Printf("  gate resolved -> should_run=%v state=%s\n", ok["should_run"], ok["state"])

	// 6. Review packet + kanban projection.
	review := getJSON(ts.URL, "GET", "/api/v1/controlplane/goals/"+gid+"/review", nil)
	fmt.Printf("  review: goal=%s open_todos=%d lineage=%d\n",
		review["goal"].(map[string]any)["id"], len(review["open_todos"].([]any)), len(review["decision_lineage"].([]any)))
	kanban := getJSON(ts.URL, "GET", "/api/v1/controlplane/goals/"+gid+"/kanban", nil)
	fmt.Printf("  kanban columns: %v\n", mapKeys(kanban["columns"].(map[string]any)))

	// 7. Capability catalog.
	caps := getJSON(ts.URL, "GET", "/api/v1/controlplane/capabilities", nil)
	fmt.Printf("  capabilities: %d (issue-fix lane: %d stages)\n",
		len(caps["capabilities"].([]any)),
		len(caps["capabilities"].([]any)[0].(map[string]any)["lane"].([]any)))

	// 8. Maintenance endpoint.
	mt := postJSON(ts.URL, "POST", "/api/v1/controlplane/maintenance",
		map[string]any{"older_days": 1, "keep_last_n": 100})
	fmt.Printf("  maintenance: reaped=%v keep_last_n=%v\n", mt["reaped"], mt["keep_last_n"])

	// 9. Prometheus /metrics exports the governance counters.
	res := getText(ts.URL + "/metrics")
	for _, line := range strings.Split(res, "\n") {
		if strings.HasPrefix(line, "agentscope_controlplane_") {
			fmt.Printf("  %s\n", line)
		}
	}

	fmt.Println("\nHTTP flow completed.")
}

// getText fetches a URL and returns the body as text.
func getText(url string) string {
	res, err := http.Get(url)
	must(err)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		panic(fmt.Sprintf("GET %s -> %d", url, res.StatusCode))
	}
	return string(b)
}

// --- tiny helpers (no external deps) ---

func postJSON(base, method, path string, body any) map[string]any {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(body)
	req, _ := http.NewRequest(method, base+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	must(err)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var out map[string]any
	if len(b) > 0 {
		_ = json.Unmarshal(b, &out)
	}
	if res.StatusCode >= 300 {
		panic(fmt.Sprintf("%s %s -> %d: %s", method, path, res.StatusCode, string(b)))
	}
	return out
}

func getJSON(base, method, path string, body any) map[string]any {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, base+path, &buf)
	res, err := http.DefaultClient.Do(req)
	must(err)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var out map[string]any
	if len(b) > 0 {
		_ = json.Unmarshal(b, &out)
	}
	if res.StatusCode >= 300 {
		panic(fmt.Sprintf("%s %s -> %d: %s", method, path, res.StatusCode, string(b)))
	}
	return out
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
