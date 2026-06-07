package permission

// DefaultDangerousFiles is the built-in list of sensitive files that should be
// protected from auto-editing. Matched by basename (case-insensitive).
var DefaultDangerousFiles = []string{
	".gitconfig",
	".gitmodules",
	".bashrc",
	".bash_profile",
	".zshrc",
	".zprofile",
	".profile",
	".ssh/config",
	".ssh/authorized_keys",
	".netrc",
	".npmrc",
	".pypirc",
	".env",
	".envrc",
	".env.local",
	".env.development",
	".env.development.local",
	".env.test",
	".env.test.local",
	".env.staging",
	".env.production",
	".env.production.local",
}

// DefaultDangerousDirs is the built-in list of sensitive directories that
// should be protected. Matched when any path segment equals an entry
// (case-insensitive).
var DefaultDangerousDirs = []string{
	".git",
	".vscode",
	".idea",
	".ssh",
}

// DefaultDangerousCommands is the built-in list of dangerous command patterns
// that require explicit user approval.
var DefaultDangerousCommands = []string{
	"rm -rf",
	"sudo rm",
	"dd",
	"mkfs",
	"fdisk",
	"format",
	"chmod 777",
	"chmod -R 777",
	"chown -R",
	"kill -9",
	"> /dev/",
}

// DefaultReadOnlyCommands is the built-in list of safe read-only commands that
// can be auto-allowed.
var DefaultReadOnlyCommands = []string{
	"ls",
	"cat",
	"pwd",
	"echo",
	"find",
	"grep",
	"head",
	"tail",
	"wc",
	"which",
	"whoami",
	"uname",
	"df",
	"du",
	"stat",
	"file",
	"date",
	"env",
	"printenv",
	"git status",
	"git log",
	"git diff",
	"git show",
	"git branch",
}
