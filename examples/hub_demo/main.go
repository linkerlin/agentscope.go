// Command hub_demo demonstrates the Hub marketplace subsystem: a
// filesystem-backed hub (builtin.FSHub) serves a card catalog; a skill card
// is downloaded (over HTTP) and extracted into a skills directory.
//
// Run:
//
//	go run ./examples/hub_demo/
//
// The demo creates a local hub catalog (mcps.json + skills.json), serves the
// skill archive over an in-process HTTP server, and installs it.
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/linkerlin/agentscope.go/hub"
	"github.com/linkerlin/agentscope.go/hub/builtin"
)

func main() {
	// 1. Build a catalog directory with mcps.json + skills.json.
	dir, err := os.MkdirTemp("", "hub-demo-catalog")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	// 2. Serve a real skill archive (zip with SKILL.md) over httptest.
	zipBytes := skillZip("code-review", "# Code Review Skill\n\nReview patches for style and safety.")
	archive := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipBytes)
	}))
	defer archive.Close()

	os.WriteFile(filepath.Join(dir, "mcps.json"), []byte(`[
	  {"id":"filesystem","name":"Filesystem","description":"Local file access","spec":{"name":"filesystem","command":"npx","args":["-y","@modelcontextprotocol/server-filesystem","$WORKDIR"]}},
	  {"id":"github","name":"GitHub","description":"GitHub operations","spec":{"name":"github","command":"npx","args":["-y","@modelcontextprotocol/server-github"],"env":{"GITHUB_PERSONAL_ACCESS_TOKEN":"$GITHUB_TOKEN"}}}
	]`), 0o644)
	os.WriteFile(filepath.Join(dir, "skills.json"), []byte(fmt.Sprintf(`[
	  {"id":"code-review","name":"Code Review","description":"Review code patches","archive_url":"%s/code-review.zip"}
	]`, archive.URL)), 0o644)

	// 3. Load the hub from the catalog.
	h, err := builtin.NewFSHub(dir, "demo", "Demo Hub", "A filesystem-backed marketplace")
	if err != nil {
		panic(err)
	}

	// 4. Browse the catalog.
	ctx := context.Background()
	mcps, _, _ := h.ListMCPCards(ctx, "", 0, 10)
	fmt.Printf("Hub %q has %d MCP cards:\n", h.DisplayName(), len(mcps))
	for _, c := range mcps {
		fmt.Printf("  - %-12s %s\n", c.Name, c.Description)
	}
	skills, _, _ := h.ListSkillCards(ctx, "", 0, 10)
	fmt.Printf("and %d skill card(s):\n", len(skills))
	for _, c := range skills {
		fmt.Printf("  - %-12s %s\n", c.Name, c.Description)
	}

	// 5. Install the skill (download + extract, zip-slip hardened).
	dest := filepath.Join(os.TempDir(), "hub-demo-skills")
	if err := hub.InstallSkill(ctx, skills[0], dest); err != nil {
		panic(err)
	}
	fmt.Printf("\nInstalled skill to %s:\n", dest)
	_ = filepath.Walk(dest, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fmt.Printf("  - %s\n", path)
		}
		return nil
	})

	// 6. Gateway wiring (documented pattern):
	fmt.Println("\nGateway wiring:")
	fmt.Println("  srv := gateway.NewServer(agent)")
	fmt.Println("  srv.WithHubs(h)")
	fmt.Println("  srv.RegisterHubRoutes()  // GET /api/v1/hubs, /hubs/{id}/mcps, POST /hubs/{id}/skills/{card}/install")
}

// skillZip builds a valid zip archive containing <name>/SKILL.md.
func skillZip(name, content string) []byte {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create(name + "/SKILL.md")
	_, _ = w.Write([]byte(content))
	_ = zw.Close()
	return buf.Bytes()
}
