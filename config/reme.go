package config

// ReMeMemoryConfig ReMe 记忆配置（与演进方案 6.2 对齐）
type ReMeMemoryConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	Type       string `json:"type" yaml:"type"`
	WorkingDir string `json:"working_dir" yaml:"working_dir"`
	Language   string `json:"language" yaml:"language"`

	MaxInputLength       int     `json:"max_input_length" yaml:"max_input_length"`
	CompactRatio         float64 `json:"compact_ratio" yaml:"compact_ratio"`
	MemoryCompactReserve int     `json:"memory_compact_reserve" yaml:"memory_compact_reserve"`

	ToolResultRetentionDays int `json:"tool_result_retention_days" yaml:"tool_result_retention_days"`
	RecentMaxBytes          int `json:"recent_max_bytes" yaml:"recent_max_bytes"`
	OldMaxBytes             int `json:"old_max_bytes" yaml:"old_max_bytes"`

	VectorStore struct {
		Backend    string `json:"backend" yaml:"backend"`
		Dimension  int    `json:"dimension" yaml:"dimension"`
		DBPath     string `json:"db_path" yaml:"db_path"`
		Host       string `json:"host" yaml:"host"`
		Port       int    `json:"port" yaml:"port"`
		Collection string `json:"collection" yaml:"collection"`
		BaseURL    string `json:"base_url" yaml:"base_url"`
		Index      string `json:"index" yaml:"index"`
		ConnStr    string `json:"conn_str" yaml:"conn_str"`
		Table      string `json:"table" yaml:"table"`
	} `json:"vector_store" yaml:"vector_store"`

	Embedding struct {
		Backend   string `json:"backend" yaml:"backend"`
		ModelName string `json:"model_name" yaml:"model_name"`
		APIKey    string `json:"api_key" yaml:"api_key"`
		BaseURL   string `json:"base_url" yaml:"base_url"`
	} `json:"embedding" yaml:"embedding"`

	Compactor struct {
		ModelName string `json:"model_name" yaml:"model_name"`
		APIKey    string `json:"api_key" yaml:"api_key"`
	} `json:"compactor" yaml:"compactor"`
}
