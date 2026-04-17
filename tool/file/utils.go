package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func validatePath(filePath string, baseDir string) (string, error) {
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	inputPath := filePath
	var path string
	if baseDir != "" && !filepath.IsAbs(inputPath) {
		path = filepath.Join(baseDir, inputPath)
	} else {
		var err error
		path, err = filepath.Abs(inputPath)
		if err != nil {
			return "", err
		}
	}
	path = filepath.Clean(path)

	if baseDir != "" {
		base, err := filepath.Abs(baseDir)
		if err != nil {
			return "", err
		}
		base = filepath.Clean(base)
		if !strings.HasPrefix(path, base) {
			return "", fmt.Errorf("access denied: path '%s' is outside base directory '%s'", filePath, base)
		}
	}

	return path, nil
}

func parseRanges(ranges string) (start, end int, ok bool) {
	cleaned := strings.TrimSpace(ranges)
	cleaned = strings.TrimPrefix(cleaned, "[")
	cleaned = strings.TrimSuffix(cleaned, "]")
	cleaned = strings.TrimSpace(cleaned)
	parts := strings.Split(cleaned, ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	s, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	e, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return s, e, true
}

func calculateViewRanges(originalLineCount, newLineCount, modifyStart, modifyEnd, extra int) (start, end int) {
	start = modifyStart - extra
	if start < 1 {
		start = 1
	}
	end = modifyStart + (newLineCount - originalLineCount) + (modifyEnd - modifyStart) + extra
	if end > newLineCount {
		end = newLineCount
	}
	return start, end
}

func viewTextFile(path string, startLine, endLine int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	// Remove trailing empty line caused by final \n if any
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	start := startLine - 1
	if start < 0 {
		start = 0
	}
	end := endLine
	if end > len(lines) {
		end = len(lines)
	}
	var sb strings.Builder
	for i := start; i < end; i++ {
		fmt.Fprintf(&sb, "%d: %s\n", i+1, lines[i])
	}
	return sb.String(), nil
}
