package memory

import (
	"context"
	"testing"
)

func TestPersonalRetrieverNil(t *testing.T) {
	var p *PersonalRetriever
	out, err := p.Retrieve(context.Background(), "u", "q", 3)
	if err != nil || out != nil {
		t.Fatal(out, err)
	}
	p = &PersonalRetriever{}
	out, err = p.Retrieve(context.Background(), "u", "q", 3)
	if err != nil || out != nil {
		t.Fatal(out, err)
	}
}

func TestRetrieversWithVector(t *testing.T) {
	e := fixedEmbed{dim: 4}
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	v, err := NewReMeVectorMemory(cfg, NewSimpleTokenCounter(), nil, e)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = v.AddMemory(ctx, NewMemoryNode(MemoryTypeProcedural, "t1", "step one"))

	pr := &PersonalRetriever{V: v}
	_, _ = pr.Retrieve(ctx, "x", "q", 1)

	proc := &ProceduralRetriever{V: v}
	out, err := proc.Retrieve(ctx, "t1", "step", 1)
	if err != nil || len(out) != 1 {
		t.Fatal(out, err)
	}

	tr := &ToolRetriever{V: v}
	_ = v.AddMemory(ctx, NewMemoryNode(MemoryTypeTool, "grep", "use grep for search"))
	out2, err := tr.Retrieve(ctx, "grep", "search", 2)
	if err != nil || len(out2) != 1 {
		t.Fatal(out2, err)
	}
}
