package output

// JSONSchema 简化版 JSON Schema（用于结构化输出提示）
type JSONSchema struct {
	Type        string                 `json:"type"`
	Properties  map[string]*SchemaProp `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Description string                 `json:"description,omitempty"`
}

// SchemaProp 描述单个字段
type SchemaProp struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description,omitempty"`
	Properties  map[string]*SchemaProp `json:"properties,omitempty"`
	Items       *SchemaProp            `json:"items,omitempty"`
	Enum        []any                  `json:"enum,omitempty"`
}
