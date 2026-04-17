package shell

import (
	"runtime"
	"testing"
)

func TestPathWithinCurrentDirectory(t *testing.T) {
	if !pathWithinCurrentDirectory("./script.sh") {
		t.Fatal("expected safe")
	}
	if !pathWithinCurrentDirectory("./a/b/c.sh") {
		t.Fatal("expected safe")
	}
	if pathWithinCurrentDirectory("./../escape.sh") {
		t.Fatal("expected unsafe")
	}
	if pathWithinCurrentDirectory(".\\..\\escape.bat") {
		t.Fatal("expected unsafe")
	}
}

func TestUnixValidator_ExtractExecutable(t *testing.T) {
	v := &UnixCommandValidator{}
	if v.ExtractExecutable("  git status ") != "git" {
		t.Fatalf("unexpected: %s", v.ExtractExecutable("  git status "))
	}
	if v.ExtractExecutable(`"my script.sh" arg`) != "my script" {
		t.Fatalf("unexpected: %s", v.ExtractExecutable(`"my script.sh" arg`))
	}
}

func TestUnixValidator_ContainsMultipleCommands(t *testing.T) {
	v := &UnixCommandValidator{}
	if !v.ContainsMultipleCommands("echo a && echo b") {
		t.Fatal("expected multiple commands")
	}
	if !v.ContainsMultipleCommands(`echo "a;b" ; echo c`) {
		t.Fatal("expected multiple commands")
	}
	if v.ContainsMultipleCommands(`echo "a&&b"`) {
		t.Fatal("expected single command inside quotes")
	}
}

func TestWindowsValidator_ExtractExecutable(t *testing.T) {
	v := &WindowsCommandValidator{}
	if v.ExtractExecutable(`C:\Program Files\app.exe arg`) != "app" {
		t.Fatalf("unexpected: %s", v.ExtractExecutable(`C:\Program Files\app.exe arg`))
	}
	if v.ExtractExecutable("dir") != "dir" {
		t.Fatalf("unexpected: %s", v.ExtractExecutable("dir"))
	}
}

func TestWindowsValidator_ContainsMultipleCommands(t *testing.T) {
	v := &WindowsCommandValidator{}
	if !v.ContainsMultipleCommands("echo a & echo b") {
		t.Fatal("expected multiple commands")
	}
	if v.ContainsMultipleCommands(`echo "a&b"`) {
		t.Fatal("expected single command inside quotes")
	}
	if v.ContainsMultipleCommands("echo a ; echo b") {
		t.Fatal("semicolon is NOT a windows separator")
	}
}

func TestDefaultValidator(t *testing.T) {
	v := defaultValidator()
	if runtime.GOOS == "windows" {
		if _, ok := v.(*WindowsCommandValidator); !ok {
			t.Fatal("expected windows validator")
		}
	} else {
		if _, ok := v.(*UnixCommandValidator); !ok {
			t.Fatal("expected unix validator")
		}
	}
}
