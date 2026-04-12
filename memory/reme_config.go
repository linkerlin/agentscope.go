package memory

import "github.com/linkerlin/agentscope.go/config"

// ReMeFileConfigFrom 将统一配置中的 ReMe 段映射为 ReMeFileConfig（零值保留默认）
func ReMeFileConfigFrom(c *config.ReMeMemoryConfig) ReMeFileConfig {
	out := DefaultReMeFileConfig()
	if c == nil {
		return out
	}
	if c.WorkingDir != "" {
		out.WorkingDir = c.WorkingDir
	}
	if c.MaxInputLength > 0 {
		out.MaxInputLength = c.MaxInputLength
	}
	if c.CompactRatio > 0 {
		out.CompactRatio = c.CompactRatio
	}
	if c.MemoryCompactReserve > 0 {
		out.MemoryCompactReserve = c.MemoryCompactReserve
	}
	if c.ToolResultRetentionDays > 0 {
		out.ToolResultRetentionDays = c.ToolResultRetentionDays
	}
	if c.RecentMaxBytes > 0 {
		out.RecentMaxBytes = c.RecentMaxBytes
	}
	if c.OldMaxBytes > 0 {
		out.OldMaxBytes = c.OldMaxBytes
	}
	return out
}
