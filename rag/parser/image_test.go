package parser

import (
	"bytes"
	"context"
	"testing"
)

func TestSniffImageMediaType(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D}, "image/png"},
		{"jpeg", []byte{0xff, 0xd8, 0xff, 0xe0}, "image/jpeg"},
		{"gif", []byte{'G', 'I', 'F', '8', '9', 'a'}, "image/gif"},
		{"webp", []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'}, "image/webp"},
		{"unknown", []byte{1, 2, 3}, "application/octet-stream"},
	}
	for _, c := range cases {
		if got := sniffImageMediaType(c.data); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestImageParser_DataBlockOnly(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A}
	p := NewImageParser()
	secs, err := p.Parse(context.Background(), bytes.NewReader(png), "logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 1 {
		t.Fatalf("expected 1 section, got %d", len(secs))
	}
	if !secs[0].IsData() {
		t.Fatal("expected DataBlock section")
	}
	if secs[0].Metadata["media_type"] != "image/png" {
		t.Fatalf("media type = %v", secs[0].Metadata["media_type"])
	}
}

func TestImageParser_WithOCR(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G'}
	p := &ImageParser{
		OCR: func(ctx context.Context, mediaType string, data []byte) (string, error) {
			return "OCR'd text", nil
		},
	}
	secs, err := p.Parse(context.Background(), bytes.NewReader(png), "scan.png")
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 2 {
		t.Fatalf("expected text + data sections, got %d", len(secs))
	}
	if secs[0].Text() != "OCR'd text" {
		t.Fatalf("first section text = %q", secs[0].Text())
	}
	if !secs[1].IsData() {
		t.Fatal("second section should be the image DataBlock")
	}
}

// compile-time assertion
var _ Parser = (*ImageParser)(nil)
