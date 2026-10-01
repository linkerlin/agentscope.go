package hub

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	mcpserver "github.com/linkerlin/agentscope.go/toolkit/mcp"
)

// InstallMCPs connects every card's MCP server spec via the resilient
// toolkit/mcp.ConnectServers loader: binaries that are not installed or
// servers that fail to connect are skipped, never breaking the others.
// Returns the connected manager plus per-card results.
func InstallMCPs(ctx context.Context, cards []MCPCard) (*mcpserver.Manager, []mcpserver.ConnectResult) {
	specs := make([]mcpserver.ServerSpec, 0, len(cards))
	for _, c := range cards {
		spec := c.Spec
		if spec.Name == "" {
			spec.Name = c.ID
		}
		specs = append(specs, spec)
	}
	return mcpserver.ConnectServers(ctx, specs)
}

// maxArchiveBytes caps a skill archive download to protect against unbounded
// downloads (a marketplace entry could point anywhere).
const maxArchiveBytes = 64 << 20 // 64 MiB

// installFetcher backs InstallSkill's downloads (18.7): the same retry /
// Retry-After policy as the catalog fetches, and injectable for tests.
var installFetcher = newFetcher()

// InstallSkill downloads the card's archive and extracts it into destDir.
// The download retries 429/5xx with backoff (18.7 — marketplaces rate-limit
// bursts of installs); extraction is hardened against zip-slip / tar-slip
// (path traversal) and the total extracted size stays bounded.
func InstallSkill(ctx context.Context, card SkillCard, destDir string) error {
	if card.ArchiveURL == "" {
		return fmt.Errorf("hub: skill %q has no archive url", card.ID)
	}
	_, body, err := installFetcher.fetchLimit(ctx, card.ArchiveURL, "", maxArchiveBytes)
	if err != nil {
		return err
	}
	return extractArchive(body, destDir)
}

// extractArchive unpacks zip / tar / tar.gz bytes into destDir, sanitising
// every entry path to stay inside destDir and bounding the total size.
func extractArchive(data []byte, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	// zip
	if len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && data[2] == 0x03 && data[3] == 0x04 {
		return extractZip(data, destDir)
	}
	// tar / tar.gz
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("hub: gzip: %w", err)
		}
		defer gz.Close()
		return extractTar(gz, destDir)
	}
	return extractTar(bytes.NewReader(data), destDir)
}

func extractZip(data []byte, destDir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("hub: zip: %w", err)
	}
	var total int64
	for _, f := range zr.File {
		path, ok := safeJoin(destDir, f.Name)
		if !ok {
			return fmt.Errorf("hub: zip entry escapes dest dir: %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			continue
		}
		total += int64(f.UncompressedSize64)
		if total > maxArchiveBytes {
			return fmt.Errorf("hub: extracted size exceeds %d bytes", maxArchiveBytes)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(path)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTar(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("hub: tar: %w", err)
		}
		path, ok := safeJoin(destDir, hdr.Name)
		if !ok {
			return fmt.Errorf("hub: tar entry escapes dest dir: %q", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			total += hdr.Size
			if total > maxArchiveBytes {
				return fmt.Errorf("hub: extracted size exceeds %d bytes", maxArchiveBytes)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			out, err := os.Create(path)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			if err != nil {
				return err
			}
		}
	}
}

// safeJoin joins name under root, rejecting absolute paths and ".." escapes.
// Both Windows and POSIX absolute forms are rejected (filepath.IsAbs misses
// "/x" on Windows, so the leading-slash check is the cross-platform guard).
func safeJoin(root, name string) (string, bool) {
	clean := filepath.Clean(strings.ReplaceAll(name, "\\", "/"))
	if clean == "." || clean == "" {
		return "", false
	}
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, `\`) || strings.HasPrefix(clean, "..") {
		return "", false
	}
	return filepath.Join(root, clean), true
}
