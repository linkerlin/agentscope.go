package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/gateway"
)

func TestWebUI_OffloadHintInjection(t *testing.T) {
	toolOffload := gateway.NewToolOffloadManager()
	ag := newDemoAgent()
	srv := buildGateway(ag, toolOffload)
	srv.RegisterV2Routes()

	ts := httptest.NewServer(gateway.CORSMiddleware(srv))
	defer ts.Close()

	sessionID := "webui-offload-sess"
	toolOffload.PushResult(sessionID, "<system-notification>\nResult:\nwebui-offload-ok\n</system-notification>")

	body := `{"text":"next","session_id":"` + sessionID + `"}`
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v2/chat/stream?protocol=agui", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, readBody(resp))
	}
	streamText := readBody(resp)
	if !(strings.Contains(streamText, "ad-ok") && strings.Contains(streamText, "system-n")) {
		t.Fatalf("expected offload hint in AG-UI stream, got:\n%s", streamText)
	}
}

func readBody(resp *http.Response) string {
	defer resp.Body.Close()
	b := make([]byte, 0, 4096)
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			b = append(b, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(b)
}
