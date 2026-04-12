package memory

// ReMeFileConfig ReMeLight 文件记忆配置
type ReMeFileConfig struct {
	WorkingDir              string  `json:"working_dir"`
	MaxInputLength          int     `json:"max_input_length"`
	CompactRatio            float64 `json:"compact_ratio"`
	MemoryCompactReserve    int     `json:"memory_compact_reserve"`
	ToolResultRetentionDays int     `json:"tool_result_retention_days"`
	RecentMaxBytes          int     `json:"recent_max_bytes"`
	OldMaxBytes             int     `json:"old_max_bytes"`
	Language                string  `json:"language"`
}

// DefaultReMeFileConfig 默认值
func DefaultReMeFileConfig() ReMeFileConfig {
	return ReMeFileConfig{
		WorkingDir:              ".reme",
		MaxInputLength:          128 * 1024,
		CompactRatio:            0.7,
		MemoryCompactReserve:    10000,
		ToolResultRetentionDays: 3,
		RecentMaxBytes:          100 * 1024,
		OldMaxBytes:             3000,
		Language:                "zh",
	}
}
