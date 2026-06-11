package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// DreamConfig 记忆演化配置
type DreamConfig struct {
	VaultDir              string   `json:"vault_dir"`
	Buckets               []string `json:"buckets"`
	TopK                  int      `json:"top_k"`
	ScoreThreshold        float64  `json:"score_threshold"`
	CorroborateThreshold  float64  `json:"corroborate_threshold"`
	MinContentLength      int      `json:"min_content_length"`
}

// DefaultDreamConfig 默认 Dream 配置
func DefaultDreamConfig() DreamConfig {
	return DreamConfig{
		Buckets:              []string{"procedure", "personal"},
		TopK:                 10,
		ScoreThreshold:       0.3,
		CorroborateThreshold: 0.7,
		MinContentLength:     50,
	}
}

// DreamAction 记忆整合动作
type DreamAction string

const (
	DreamCreate      DreamAction = "CREATE"
	DreamCorroborate DreamAction = "CORROBORATE"
	DreamRefine      DreamAction = "REFINE"
	DreamCorrect     DreamAction = "CORRECT"
	DreamSkip        DreamAction = "SKIP"
)

// DreamCandidate 候选记忆草案
type DreamCandidate struct {
	Bucket     string  `json:"bucket"`
	Content    string  `json:"content"`
	WhenToUse  string  `json:"when_to_use"`
	Score      float64 `json:"score"`
	SourceFile string  `json:"source_file"`
}

// DreamDecision 整合决策
type DreamDecision struct {
	Candidate *DreamCandidate  `json:"candidate"`
	Action    DreamAction      `json:"action"`
	Existing  *MemoryNode      `json:"existing,omitempty"`
	Updated   *MemoryNode      `json:"updated,omitempty"`
	Reason    string           `json:"reason"`
}

// DreamResult 记忆演化结果
type DreamResult struct {
	Created      []*MemoryNode `json:"created"`
	Corroborated []string      `json:"corroborated"`
	Refined      []*MemoryNode `json:"refined"`
	Corrected    []*MemoryNode `json:"corrected"`
	Skipped      []string      `json:"skipped"`
	TotalCandidates int        `json:"total_candidates"`
}

// DreamStep 记忆演化管线。
// 对标 ReMe4 DreamStep：两阶段提取+整合，四策略写入。
type DreamStep struct {
	Config DreamConfig
	LLM    model.ChatModel
	Store  VectorStore
}

// NewDreamStep 创建记忆演化步骤
func NewDreamStep(cfg DreamConfig, llm model.ChatModel, store VectorStore) *DreamStep {
	if len(cfg.Buckets) == 0 {
		cfg.Buckets = []string{"procedure", "personal"}
	}
	if cfg.TopK <= 0 {
		cfg.TopK = 10
	}
	if cfg.ScoreThreshold <= 0 {
		cfg.ScoreThreshold = 0.3
	}
	if cfg.CorroborateThreshold <= 0 {
		cfg.CorroborateThreshold = 0.7
	}
	if cfg.MinContentLength <= 0 {
		cfg.MinContentLength = 50
	}
	return &DreamStep{
		Config: cfg,
		LLM:    llm,
		Store:  store,
	}
}

// Execute 执行 Dream 管线
func (d *DreamStep) Execute(ctx context.Context) (*DreamResult, error) {
	candidates, err := d.extractCandidates(ctx)
	if err != nil {
		return nil, fmt.Errorf("dream extract: %w", err)
	}

	result := &DreamResult{TotalCandidates: len(candidates)}

	for _, cand := range candidates {
		decision, err := d.integrateCandidate(ctx, cand)
		if err != nil {
			continue
		}
		switch decision.Action {
		case DreamCreate:
			node := NewMemoryNodeWithWhen(MemoryType(cand.Bucket), cand.Bucket, cand.Content, cand.WhenToUse)
			node.Score = cand.Score
			node.TimeModified = time.Now()
			if d.Store != nil {
				if err := d.Store.Insert(ctx, []*MemoryNode{node}); err == nil {
					result.Created = append(result.Created, node)
				}
			} else {
				result.Created = append(result.Created, node)
			}
		case DreamCorroborate:
			if decision.Existing != nil {
				decision.Existing.Score += cand.Score * 0.1
				decision.Existing.TimeModified = time.Now()
				if d.Store != nil {
					_ = d.Store.Update(ctx, decision.Existing)
				}
				result.Corroborated = append(result.Corroborated, decision.Existing.MemoryID)
			}
		case DreamRefine:
			if decision.Updated != nil && d.Store != nil {
				_ = d.Store.Update(ctx, decision.Updated)
				result.Refined = append(result.Refined, decision.Updated)
			}
		case DreamCorrect:
			if decision.Updated != nil && d.Store != nil {
				_ = d.Store.Update(ctx, decision.Updated)
				result.Corrected = append(result.Corrected, decision.Updated)
			}
		case DreamSkip:
			result.Skipped = append(result.Skipped, cand.Content[:min(80, len(cand.Content))])
		}
	}
	return result, nil
}

func (d *DreamStep) extractCandidates(ctx context.Context) ([]*DreamCandidate, error) {
	var candidates []*DreamCandidate

	if d.Config.VaultDir == "" {
		return nil, errors.New("dream: vault_dir not configured")
	}

	dailyDir := filepath.Join(d.Config.VaultDir, "daily")
	digestDir := filepath.Join(d.Config.VaultDir, "digest")

	for _, dir := range []string{dailyDir, digestDir} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			fullPath := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			content := string(data)
			if len(content) < d.Config.MinContentLength {
				continue
			}

			fileCands, err := d.extractFromContent(ctx, content, fullPath)
			if err != nil {
				continue
			}
			candidates = append(candidates, fileCands...)
		}
	}
	return candidates, nil
}

func (d *DreamStep) extractFromContent(ctx context.Context, content string, sourceFile string) ([]*DreamCandidate, error) {
	if d.LLM == nil {
		return d.simpleExtract(content, sourceFile), nil
	}

	prompt := d.buildExtractPrompt(content)
	resp, err := d.LLM.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent("You are a memory extraction agent. Extract actionable memory candidates from the given content. Output valid JSON array only.").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return d.simpleExtract(content, sourceFile), nil
	}

	var candidates []*DreamCandidate
	if err := json.Unmarshal([]byte(resp.GetTextContent()), &candidates); err != nil {
		return d.simpleExtract(content, sourceFile), nil
	}

	for _, c := range candidates {
		c.SourceFile = sourceFile
		if c.Score <= 0 {
			c.Score = 0.5
		}
	}
	return candidates, nil
}

func (d *DreamStep) buildExtractPrompt(content string) string {
	bucketsJSON, _ := json.Marshal(d.Config.Buckets)
	truncated := content
	if len(truncated) > 4000 {
		truncated = truncated[:4000] + "\n...[truncated]"
	}
	return fmt.Sprintf(`Extract actionable memory candidates from the content below.
For each candidate, assign it to one of these buckets: %s.

Output a JSON array of objects with these fields:
- bucket: one of the bucket names
- content: the memory text (concise, standalone)
- when_to_use: condition or trigger for when this memory should be retrieved
- score: relevance score between 0.0 and 1.0

Content to analyze:
%s`, string(bucketsJSON), truncated)
}

func (d *DreamStep) simpleExtract(content string, sourceFile string) []*DreamCandidate {
	var candidates []*DreamCandidate
	lines := strings.Split(content, "\n")
	var buf strings.Builder

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			if buf.Len() >= d.Config.MinContentLength {
				for _, bucket := range d.Config.Buckets {
					candidates = append(candidates, &DreamCandidate{
						Bucket:     bucket,
						Content:    strings.TrimSpace(buf.String()),
						WhenToUse:  fmt.Sprintf("when discussing %s topics", bucket),
						Score:      0.5,
						SourceFile: sourceFile,
					})
				}
				buf.Reset()
			}
			continue
		}
		buf.WriteString(line)
		buf.WriteString("\n")
	}

	if buf.Len() >= d.Config.MinContentLength {
		for _, bucket := range d.Config.Buckets {
			candidates = append(candidates, &DreamCandidate{
				Bucket:     bucket,
				Content:    strings.TrimSpace(buf.String()),
				WhenToUse:  fmt.Sprintf("when discussing %s topics", bucket),
				Score:      0.5,
				SourceFile: sourceFile,
			})
		}
	}
	return candidates
}

func (d *DreamStep) integrateCandidate(ctx context.Context, cand *DreamCandidate) (*DreamDecision, error) {
	if cand.Score < d.Config.ScoreThreshold {
		return &DreamDecision{Candidate: cand, Action: DreamSkip, Reason: "score below threshold"}, nil
	}

	similar, err := d.findSimilar(ctx, cand)
	if err != nil || len(similar) == 0 {
		return &DreamDecision{Candidate: cand, Action: DreamCreate, Reason: "no similar memory found"}, nil
	}

	best := similar[0]

	if d.LLM != nil {
		return d.llmIntegrate(ctx, cand, best)
	}

	return d.heuristicIntegrate(cand, best), nil
}

func (d *DreamStep) findSimilar(ctx context.Context, cand *DreamCandidate) ([]*MemoryNode, error) {
	if d.Store == nil {
		return nil, nil
	}
	return d.Store.Search(ctx, cand.Content, RetrieveOptions{
		TopK:        d.Config.TopK,
		MinScore:    0,
		MemoryTypes: []MemoryType{MemoryType(cand.Bucket)},
	})
}

func (d *DreamStep) heuristicIntegrate(cand *DreamCandidate, best *MemoryNode) *DreamDecision {
	contentSimilarity := simpleTextSimilarity(cand.Content, best.Content)

	switch {
	case contentSimilarity >= 0.9:
		return &DreamDecision{
			Candidate: cand,
			Action:    DreamCorroborate,
			Existing:  best,
			Reason:    fmt.Sprintf("high similarity (%.2f) - corroborating", contentSimilarity),
		}
	case contentSimilarity >= d.Config.CorroborateThreshold:
		updated := *best
		updated.Content = best.Content + "\n\n[Updated] " + cand.Content
		updated.WhenToUse = cand.WhenToUse
		updated.TimeModified = time.Now()
		return &DreamDecision{
			Candidate: cand,
			Action:    DreamRefine,
			Existing:  best,
			Updated:   &updated,
			Reason:    fmt.Sprintf("moderate similarity (%.2f) - refining", contentSimilarity),
		}
	case contentSimilarity >= 0.3:
		return &DreamDecision{
			Candidate: cand,
			Action:    DreamCorrect,
			Existing:  best,
			Updated:   NewMemoryNodeWithWhen(MemoryType(cand.Bucket), cand.Bucket, cand.Content, cand.WhenToUse),
			Reason:    fmt.Sprintf("low similarity (%.2f) but same topic - correcting", contentSimilarity),
		}
	default:
		return &DreamDecision{
			Candidate: cand,
			Action:    DreamCreate,
			Reason:    fmt.Sprintf("distinct topic (%.2f) - creating new", contentSimilarity),
		}
	}
}

func (d *DreamStep) llmIntegrate(ctx context.Context, cand *DreamCandidate, best *MemoryNode) (*DreamDecision, error) {
	prompt := fmt.Sprintf(`Given a new memory candidate and the most similar existing memory, decide the integration action.

New candidate:
  content: %s
  when_to_use: %s
  score: %.2f

Existing memory (id: %s):
  content: %s
  score: %.2f

Decide one of: CREATE, CORROBORATE, REFINE, CORRECT, SKIP
- CREATE: completely new, no overlap → create new memory
- CORROBORATE: same information → increase existing score, no content change
- REFINE: similar topic with additional detail → merge into existing
- CORRECT: different information on same topic → replace existing
- SKIP: not useful → ignore

Output JSON: {"action": "...", "reason": "...", "updated_content": "..." (only for REFINE/CORRECT)}`,
		cand.Content, cand.WhenToUse, cand.Score,
		best.MemoryID, best.Content, best.Score)

	resp, err := d.LLM.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent("You decide how to integrate new memories. Output valid JSON only.").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return d.heuristicIntegrate(cand, best), nil
	}

	var llmDecision struct {
		Action         string `json:"action"`
		Reason         string `json:"reason"`
		UpdatedContent string `json:"updated_content"`
	}
	if err := json.Unmarshal([]byte(resp.GetTextContent()), &llmDecision); err != nil {
		return d.heuristicIntegrate(cand, best), nil
	}

	decision := &DreamDecision{
		Candidate: cand,
		Action:    DreamAction(llmDecision.Action),
		Existing:  best,
		Reason:    llmDecision.Reason,
	}

	if (decision.Action == DreamRefine || decision.Action == DreamCorrect) && llmDecision.UpdatedContent != "" {
		updated := *best
		updated.Content = llmDecision.UpdatedContent
		updated.WhenToUse = cand.WhenToUse
		updated.TimeModified = time.Now()
		decision.Updated = &updated
	}
	return decision, nil
}

func simpleTextSimilarity(a, b string) float64 {
	aWords := make(map[string]int)
	for _, w := range strings.Fields(strings.ToLower(a)) {
		aWords[w]++
	}
	bWords := make(map[string]int)
	for _, w := range strings.Fields(strings.ToLower(b)) {
		bWords[w]++
	}

	intersection := 0
	union := 0
	for w, c := range aWords {
		union += c
		if bc, ok := bWords[w]; ok {
			if c < bc {
				intersection += c
			} else {
				intersection += bc
			}
		}
	}
	for w, c := range bWords {
		if _, ok := aWords[w]; !ok {
			union += c
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
