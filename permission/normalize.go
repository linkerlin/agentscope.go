package permission

import "strings"

// NormalizeDepth caps recursive wrapper expansion (sh -c / eval / sudo / ...)
// so a maliciously nested command cannot cause unbounded work.
const NormalizeDepth = 8

// NormalizeCommand unwraps common command wrappers and strips quoting so the
// inner command can be matched against permission rules and safety checks.
// It is a heuristic, not a shell parser: it exists to make permission
// decisions see through `sudo rm -rf /`, `sh -c "rm -rf /"`, `eval ...`,
// `env -S ...` and similar indirection, not to execute anything.
func NormalizeCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	for i := 0; i < NormalizeDepth; i++ {
		next := unwrapOnce(cmd)
		if next == cmd {
			return cmd
		}
		cmd = next
	}
	return cmd
}

// sudoValueFlags are sudo/doas options that consume a separate value argument
// (e.g. `sudo -u root rm -rf /`). Boolean flags (-n, -s, -i, -v, ...) are
// stripped singly. Without this, `sudo -u root -- rm -rf /` never unwraps to
// `rm -rf /` and the safety checks miss the dangerous inner command.
var sudoValueFlags = map[string]bool{
	"-u": true, "-g": true, "-p": true, "-h": true, "-D": true,
	"-U": true, "-C": true, "-T": true, "-R": true,
}

// unwrapOnce applies a single normalization step: strip a leading wrapper
// token, or strip quoting when no wrapper is present.
func unwrapOnce(cmd string) string {
	tokens := ShellTokenize(cmd)
	for len(tokens) > 0 && isEnvAssignment(tokens[0]) {
		tokens = tokens[1:]
	}
	if len(tokens) == 0 {
		return cmd
	}
	switch tokens[0] {
	case "sudo", "doas":
		rest := tokens[1:]
		for len(rest) > 0 && strings.HasPrefix(rest[0], "-") && rest[0] != "-" {
			f := rest[0]
			rest = rest[1:]
			if sudoValueFlags[f] && len(rest) > 0 {
				rest = rest[1:] // consume the option's value
			}
		}
		return joinTokens(rest)
	case "sh", "bash", "dash", "zsh", "ksh":
		if len(tokens) > 1 && tokens[1] == "-c" {
			return joinTokens(tokens[2:])
		}
	case "eval":
		return joinTokens(tokens[1:])
	case "env":
		if len(tokens) > 1 && tokens[1] == "-S" {
			return joinTokens(tokens[2:])
		}
		return joinTokens(tokens[1:])
	case "xargs", "nice", "ionice", "time", "nohup", "command":
		rest := tokens[1:]
		for len(rest) > 0 && strings.HasPrefix(rest[0], "-") && rest[0] != "-" {
			if rest[0] == "-n" && len(rest) > 1 {
				rest = rest[2:]
				continue
			}
			rest = rest[1:]
		}
		return joinTokens(rest)
	}
	// No wrapper: drop quoting / escapes only.
	return joinTokens(tokens)
}

func joinTokens(tokens []string) string {
	return strings.Join(tokens, " ")
}
