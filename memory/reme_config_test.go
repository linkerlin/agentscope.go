package memory

import (
	"testing"

	"github.com/linkerlin/agentscope.go/config"
)

func TestReMeFileConfigFrom(t *testing.T) {
	c := &config.ReMeMemoryConfig{
		WorkingDir:              "/tmp/reme",
		MaxInputLength:          999,
		CompactRatio:            0.8,
		MemoryCompactReserve:    5000,
		ToolResultRetentionDays: 7,
		RecentMaxBytes:          50_000,
		OldMaxBytes:             2000,
	}
	out := ReMeFileConfigFrom(c)
	if out.WorkingDir != "/tmp/reme" || out.MaxInputLength != 999 {
		t.Fatal(out)
	}
	if out.CompactRatio != 0.8 || out.MemoryCompactReserve != 5000 {
		t.Fatal(out)
	}
	if out.ToolResultRetentionDays != 7 || out.RecentMaxBytes != 50_000 || out.OldMaxBytes != 2000 {
		t.Fatal(out)
	}
}

func TestReMeFileConfigFromNil(t *testing.T) {
	out := ReMeFileConfigFrom(nil)
	def := DefaultReMeFileConfig()
	if out.WorkingDir != def.WorkingDir {
		t.Fatal(out)
	}
}
