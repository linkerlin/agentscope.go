// Package handler provides high-level constructors and orchestrators that wire
// together ReMe memory components (vector stores, summarizers, compactor, etc.).
package handler

import (
	"fmt"
	"path/filepath"

	"github.com/linkerlin/agentscope.go/config"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/model"
)

// BuildReMeVectorMemory 从 ReMeMemoryConfig 一键构建完整的 ReMeVectorMemory（含可选 Orchestrator）
func BuildReMeVectorMemory(cfg *config.ReMeMemoryConfig, embed memory.EmbeddingModel, cm model.ChatModel) (*memory.ReMeVectorMemory, error) {
	fileCfg := memory.ReMeFileConfigFrom(cfg)
	counter := memory.NewSimpleTokenCounter()

	var store memory.VectorStore
	var err error
	if cfg != nil && cfg.VectorStore.Backend != "" && cfg.VectorStore.Backend != "local" {
		switch cfg.VectorStore.Backend {
		case "qdrant":
			port := cfg.VectorStore.Port
			if port <= 0 {
				port = 6334
			}
			store, err = memory.NewQdrantVectorStore(
				cfg.VectorStore.Host, port,
				cfg.VectorStore.Collection,
				uint64(cfg.VectorStore.Dimension), embed,
			)
		case "chroma":
			store, err = memory.NewChromaVectorStore(
				cfg.VectorStore.BaseURL,
				cfg.VectorStore.Collection,
				cfg.VectorStore.Dimension, embed,
			)
		case "elasticsearch", "es":
			store, err = memory.NewESVectorStore(
				cfg.VectorStore.BaseURL,
				cfg.VectorStore.Index,
				cfg.VectorStore.Dimension, embed,
			)
		case "pgvector", "pg":
			store, err = memory.NewPGVectorStore(
				cfg.VectorStore.ConnStr,
				cfg.VectorStore.Table,
				cfg.VectorStore.Dimension, embed,
			)
		default:
			return nil, fmt.Errorf("memory: unsupported vector store backend %q", cfg.VectorStore.Backend)
		}
		if err != nil {
			return nil, err
		}
	}

	v, err := memory.NewReMeVectorMemory(fileCfg, counter, store, embed)
	if err != nil {
		return nil, err
	}

	if cm != nil {
		v.InitCompactorWithModel(cm)
		v.InitSummarizerWithModel(cm)

		memTool := NewMemoryHandler(v.VectorStore())
		profileTool := NewProfileHandler(filepath.Join(fileCfg.WorkingDir, "profile"))
		historyTool := NewHistoryHandler(v.VectorStore())

		lang := "zh"
		if cfg != nil && cfg.Language != "" {
			lang = cfg.Language
		}
		ps := memory.NewPersonalSummarizer(cm, lang)
		proc := memory.NewProceduralSummarizer(cm, lang)
		ts := memory.NewToolSummarizer(cm, lang)

		orch := NewMemoryOrchestrator(ps, proc, ts, memTool, profileTool, historyTool, nil)
		v.SetOrchestrator(orch)
	}

	return v, nil
}
