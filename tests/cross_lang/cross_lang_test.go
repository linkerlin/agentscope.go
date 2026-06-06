package crosslang

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

// TestCrossLang_MsgFromGoToPy validates that Go-generated Msg JSON can be
// parsed by Python agentscope v2.
func TestCrossLang_MsgFromGoToPy(t *testing.T) {
	if os.Getenv("CROSS_LANG") != "1" {
		t.Skip("Set CROSS_LANG=1 to run cross-language tests")
	}

	root := repoRoot(t)
	goGen := exec.Command("go", "run", "tests/cross_lang/generate_go.go")
	goGen.Dir = root
	if out, err := goGen.CombinedOutput(); err != nil {
		t.Fatalf("generate_go failed: %v\n%s", err, out)
	}

	pyVal := exec.Command("python", "tests/cross_lang/validate_py.py")
	pyVal.Dir = root
	if out, err := pyVal.CombinedOutput(); err != nil {
		t.Fatalf("validate_py failed: %v\n%s", err, out)
	}
}

// TestCrossLang_MsgFromPyToGo validates that Python-generated Msg JSON can be
// parsed by Go.
func TestCrossLang_MsgFromPyToGo(t *testing.T) {
	if os.Getenv("CROSS_LANG") != "1" {
		t.Skip("Set CROSS_LANG=1 to run cross-language tests")
	}

	root := repoRoot(t)
	pyGen := exec.Command("python", "tests/cross_lang/generate_py.py")
	pyGen.Dir = root
	if out, err := pyGen.CombinedOutput(); err != nil {
		t.Fatalf("generate_py failed: %v\n%s", err, out)
	}

	data, err := os.ReadFile(filepath.Join(root, "tests/cross_lang/fixtures/py_msg.json"))
	if err != nil {
		t.Fatalf("read py_msg.json: %v", err)
	}

	var msg message.Msg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal py_msg.json: %v", err)
	}

	if msg.Name != "PyAgent" {
		t.Fatalf("expected name PyAgent, got %s", msg.Name)
	}
	if len(msg.Content) != 7 {
		t.Fatalf("expected 7 blocks, got %d", len(msg.Content))
	}
	if msg.Content[1].BlockType() != message.TypeImage {
		t.Fatalf("expected image block, got %s", msg.Content[1].BlockType())
	}
	if msg.Content[5].BlockType() != message.TypeToolUse {
		t.Fatalf("expected tool_use block, got %s", msg.Content[5].BlockType())
	}
}

func repoRoot(t *testing.T) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is .../agentscope.go/tests/cross_lang/cross_lang_test.go
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}
