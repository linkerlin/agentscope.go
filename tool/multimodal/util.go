package multimodal

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func downloadURLToBase64(ctx context.Context, client *http.Client, rawURL string) (string, string, error) {
	if client == nil {
		client = http.DefaultClient
	}

	var data []byte
	var mime string

	if strings.HasPrefix(rawURL, "file://") {
		path := strings.TrimPrefix(rawURL, "file://")
		path = filepath.Clean(path)
		b, err := os.ReadFile(path)
		if err != nil {
			return "", "", err
		}
		data = b
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".png":
			mime = "image/png"
		case ".jpg", ".jpeg":
			mime = "image/jpeg"
		case ".gif":
			mime = "image/gif"
		case ".webp":
			mime = "image/webp"
		default:
			mime = "application/octet-stream"
		}
	} else if strings.HasPrefix(rawURL, "data:") {
		m, b64, err := parseDataURL(rawURL)
		if err != nil {
			return "", "", err
		}
		return b64, m, nil
	} else {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return "", "", err
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", "", fmt.Errorf("download failed: status=%d", resp.StatusCode)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", "", err
		}
		data = b
		mime = resp.Header.Get("Content-Type")
		if mime == "" {
			mime = guessMimeTypeByURL(rawURL)
		}
	}

	if mime == "" {
		mime = "application/octet-stream"
	}
	return base64.StdEncoding.EncodeToString(data), mime, nil
}

func guessMimeTypeByURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(u.Path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	}
	return ""
}

// parseDataURL parses a data URL like data:image/png;base64,ABC...
// and returns (mimeType, base64Data, error).
func parseDataURL(s string) (string, string, error) {
	if !strings.HasPrefix(s, "data:") {
		return "", "", fmt.Errorf("not a data URL")
	}
	s = s[5:] // strip "data:"
	comma := strings.Index(s, ",")
	if comma < 0 {
		return "", "", fmt.Errorf("invalid data URL format")
	}
	head := s[:comma]
	data := s[comma+1:]
	parts := strings.Split(head, ";")
	var mime string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "base64" {
			continue
		}
		if p != "" && mime == "" {
			mime = p
		}
	}
	if mime == "" {
		mime = "text/plain"
	}
	return mime, data, nil
}
