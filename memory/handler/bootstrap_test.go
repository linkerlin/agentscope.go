package handler

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/config"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

type mockBootstrapModel struct{}

func (m *mockBootstrapModel) Chat(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}
func (m *mockBootstrapModel) ChatStream(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, nil
}
func (m *mockBootstrapModel) ModelName() string { return "mock" }

type fixedBootstrapEmbed struct{}

func (f fixedBootstrapEmbed) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}
func (f fixedBootstrapEmbed) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = []float32{0.1, 0.2}
	}
	return out, nil
}

func TestBuildReMeVectorMemoryLocal(t *testing.T) {
	cfg := &config.ReMeMemoryConfig{
		WorkingDir: t.TempDir(),
		VectorStore: struct {
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
		}{
			Backend:   "local",
			Dimension: 2,
		},
	}
	v, err := BuildReMeVectorMemory(cfg, fixedBootstrapEmbed{}, &mockBootstrapModel{})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	ctx := context.Background()
	if err := v.AddMemory(ctx, memory.NewMemoryNode(memory.MemoryTypePersonal, "alice", "hello")); err != nil {
		t.Fatal(err)
	}
	res, err := v.RetrievePersonal(ctx, "alice", "hello", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
}

func TestBuildReMeVectorMemoryUnsupportedBackend(t *testing.T) {
	cfg := &config.ReMeMemoryConfig{
		VectorStore: struct {
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
		}{
			Backend: "unknown",
		},
	}
	_, err := BuildReMeVectorMemory(cfg, fixedBootstrapEmbed{}, nil)
	if err == nil {
		t.Fatal("expected error for unsupported backend")
	}
}
