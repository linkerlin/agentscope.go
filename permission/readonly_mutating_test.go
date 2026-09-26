package permission

import "testing"

// TestIsReadOnlyCommandSingle_MutatingArguments guards the whitelist against
// prefix-only matching (PyV2 #2004/#2629/#2747/#2795): read-only commands that
// carry a mutating argument must not be auto-allowed.
func TestIsReadOnlyCommandSingle_MutatingArguments(t *testing.T) {
	readOnly := []string{
		"find . -name '*.go'",
		`find /tmp -type f`,
		"git branch",
		"git branch -a",
		"git branch --list",
		"git branch -v",
		"ls -la",
		"git status --short",
	}
	for _, cmd := range readOnly {
		if !IsReadOnlyCommandSingle(cmd) {
			t.Errorf("IsReadOnlyCommandSingle(%q) = false, want true", cmd)
		}
	}

	mutating := []string{
		"find /tmp -delete",
		"find . -exec rm {} +",
		"find . -execdir ls {} +",
		"find . -ok rm {} +",
		"find . -fprint /tmp/out.txt",
		"find . -fprintf /tmp/out.txt '%p'",
		"git branch -d feature",
		"git branch -D feature",
		"git branch -m old new",
		"git branch -M old new",
		"git branch -c copy",
		"git branch --delete feature",
		"git branch --move old new",
		"git branch --copy old new",
		"git branch --set-upstream-to=origin/main",
		`"find" /tmp -delete`,
	}
	for _, cmd := range mutating {
		if IsReadOnlyCommandSingle(cmd) {
			t.Errorf("IsReadOnlyCommandSingle(%q) = true, want false", cmd)
		}
	}
}

// TestAllReadOnlyCommands_MutatingArguments ensures compound commands inherit
// the parameter-level check.
func TestAllReadOnlyCommands_MutatingArguments(t *testing.T) {
	if AllReadOnlyCommands("find /tmp -delete && ls") {
		t.Fatal("compound command with mutating find must not be read-only")
	}
	if !AllReadOnlyCommands("find /tmp -type f && git status") {
		t.Fatal("compound command of truly read-only segments should pass")
	}
}
