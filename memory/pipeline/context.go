package pipeline

import "github.com/linkerlin/agentscope.go/memory"

// FlowContext 在流水线步骤之间传递数据。
// 每个步骤从 context 读取输入，将结果写回 context。
type FlowContext struct {
	Query          string
	Messages       []interface{}
	MemoryNodes    []*memory.MemoryNode
	TopK           int
	MinScore       float64
	Threshold      float64
	EnableLLMRerank   bool
	EnableScoreFilter bool
	EnableLLMRewrite  bool
	ValidationThreshold float64
	SuccessThreshold   float64

	// 由步骤填充的结果
	RetrievedNodes []*memory.MemoryNode
	RerankedNodes  []*memory.MemoryNode
	RewrittenText  string
	ValidatedNodes []*memory.MemoryNode
	DedupedNodes   []*memory.MemoryNode

	// 元数据
	Metadata map[string]any
}

// NewFlowContext 创建带默认值的 FlowContext
func NewFlowContext(query string) *FlowContext {
	return &FlowContext{
		Query:     query,
		TopK:      10,
		MinScore:  0.1,
		Threshold: 0.3,
		Metadata:  make(map[string]any),
	}
}
