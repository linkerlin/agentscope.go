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

func TestSuggestPathParentGlob(t *testing.T) {
	rules := SuggestPathParentGlob("view_text_file", "src/main.go")
	if len(rules) != 1 || rules[0].Pattern != "src/**" {
		t.Fatalf("unexpected rules: %+v", rules)
	}
}
