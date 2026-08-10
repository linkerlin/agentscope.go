package tool

import "testing"

func TestMatchPathGlob(t *testing.T) {
	if !MatchPathGlob("", "any/path") {
		t.Fatal("empty pattern should match all")
	}
	if MatchPathGlob("src/**", "") {
		t.Fatal("empty path should not match")
	}
	if !MatchPathGlob("src/*.go", "src/main.go") {
		t.Fatal("expected glob match")
	}
	if MatchPathGlob("src/*.go", "lib/main.go") {
		t.Fatal("expected no match")
	}
}

func TestMatchGlobSearch(t *testing.T) {
	input := map[string]any{"path": "src", "pattern": "**/*.go"}
	if !MatchGlobSearch("src", input) {
		t.Fatal("expected path match")
	}
	if !MatchGlobSearch("**/*.go", input) {
		t.Fatal("expected pattern match")
	}
}

func TestMatchBashCommand(t *testing.T) {
	cases := []struct {
		pattern string
		command string
		want    bool
	}{
		{"", "git status", true},
		{"git:*", "git status", true},
		{"git:*", "git", true},
		{"git:*", "gitx status", false},
		{"git status", "prefix git status suffix", true},
		{"git *", "git", true},
		{"git *", "git add file.go", true},
		{"npm run:*", "npm run build", true},
	}
	for _, tc := range cases {
		if got := MatchBashCommand(tc.pattern, tc.command); got != tc.want {
			t.Fatalf("MatchBashCommand(%q, %q) = %v, want %v", tc.pattern, tc.command, got, tc.want)
		}
	}
}

// TestMatchBashCommandOrRegex Q1：PyV2 语义优先，正则 pattern 回退命中。
func TestMatchBashCommandOrRegex(t *testing.T) {
	// 既有 PyV2 语义不变。
	if !MatchBashCommandOrRegex("git:*", "git status") {
		t.Fatal("bash pattern must keep working")
	}
	if MatchBashCommandOrRegex("git:*", "gitx status") {
		t.Fatal("bash pattern must not over-match")
	}
	// 正则 pattern（如 guard 的 `\brm\b`）经回退命中。
	if !MatchBashCommandOrRegex(`\brm\b`, "rm -rf /tmp/x") {
		t.Fatal("regex pattern must match via fallback")
	}
	if !MatchBashCommandOrRegex(`\bsudo\b`, "sudo apt install x") {
		t.Fatal("regex sudo pattern must match")
	}
	if MatchBashCommandOrRegex(`\brm\b`, "echo farm") {
		t.Fatal("word-boundary must not match inside a word")
	}
	// 无效正则：编译失败即不匹配（不 panic）。
	if MatchBashCommandOrRegex("(unclosed", "anything") {
		t.Fatal("invalid regex must not match")
	}
}

func TestSuggestPathParentGlob(t *testing.T) {
	rules := SuggestPathParentGlob("view_text_file", "src/main.go")
	if len(rules) != 1 || rules[0].Pattern != "src/**" {
		t.Fatalf("unexpected rules: %+v", rules)
	}
}
