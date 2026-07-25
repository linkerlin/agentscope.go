// Package middleware — ReflectionMiddleware
//
// AutoMem 研究方案 §1.1 (LOG 阶段) 的最小实现：每轮对话结束后，异步用便宜模型
// 抽取可记忆事实并写入持久层。失败永不阻塞主流程，延迟由 goroutine 吸收。
//
// 与 LongTermMemoryMiddleware 的分工：
//   - LTM 在 OnReply 之前 search、之后写回原始 query+reply（"对话录像"）。
//   - Reflection 在 OnReply 之后抽取"原子事实"（"反思摘要"），写入业务记忆层。
//
// 两者可同时启用，互不冲突。
package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// FactWriter consumes one extracted fact. Implementations are expected to be
// idempotent-best-effort (e.g. dedup against existing entries inside). Errors
// are logged and swallowed by the middleware.
type FactWriter interface {
	WriteFact(ctx context.Context, fact string) error
}

// FactWriterFunc adapts a closure into a FactWriter.
type FactWriterFunc func(ctx context.Context, fact string) error

func (f FactWriterFunc) WriteFact(ctx context.Context, fact string) error {
	if f == nil {
		return nil
	}
	return f(ctx, fact)
}

// ReflectionMiddleware extracts memorable facts after each reply and writes
// them via the configured FactWriter. Asynchronous, bounded by a semaphore.
type ReflectionMiddleware struct {
	Base
	Model    model.ChatModel
	Writer   FactWriter
	MaxFacts int           // per-turn cap; 0 = default 3
	Workers  int           // concurrent reflection goroutines; 0 = default 2
	Timeout  time.Duration // per-call LLM timeout; 0 = default 30s

	once sync.Once
	sem  chan struct{}
}

// ReflectionDefaultPrompt is the system prompt modeled after AutoMem's LOG_SYSTEM
// (scaffolds/inner_agent_v0/agents/memory_agent.py:332-344): favor failures,
// commitments, preferences, corrections; skip routine exchanges.
const ReflectionDefaultPrompt = `You are a memory extractor. Given one user-assistant dialogue turn, extract 0-N atomic facts worth remembering long-term about the user or the task.

MUST extract:
- User preferences (tools, languages, communication style, content taste)
- User decisions or commitments
- Negative feedback or corrections to prior beliefs
- Factual corrections ("actually X is Y")
- Project milestones, blockers, deadlines
- Personal facts (name, role, timezone, relationships)

MUST SKIP:
- Greetings, chitchat, etiquette
- Process Q&A ("how do I...") where the answer is not about the user
- Restatements of already-obvious context
- Anything time-sensitive to this exact moment (current weather, transient state)

Each fact MUST be:
- A single self-contained sentence
- Third-person about the user ("User prefers ...") or about a durable fact
- In the same language as the dialogue

Output STRICT JSON and nothing else:
{"facts": ["fact 1", "fact 2"]}
If nothing is worth remembering, output: {"facts": []}`

// OnReply intercepts after the reply completes and kicks off async reflection.
func (m *ReflectionMiddleware) OnReply(ctx context.Context, agent Agent, input *ReplyInput, next ReplyNext) (*message.Msg, error) {
	msg, err := next(ctx)
	if m == nil || m.Model == nil || m.Writer == nil || err != nil || msg == nil {
		return msg, err
	}
	userQuery := extractUserQuery(input.Messages)
	assistantText := msg.GetTextContent()
	if strings.TrimSpace(userQuery) == "" || strings.TrimSpace(assistantText) == "" {
		return msg, err
	}
	// Best-effort: never block the reply path. Drop if all workers busy.
	if !m.acquire() {
		return msg, err
	}
	go m.reflect(userQuery, assistantText, agent.AgentName())
	return msg, err
}

func (m *ReflectionMiddleware) reflect(userQuery, assistantText, agentName string) {
	defer m.release()
	timeout := m.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	maxFacts := m.MaxFacts
	if maxFacts <= 0 {
		maxFacts = 3
	}
	facts, extractErr := m.extractFacts(ctx, userQuery, assistantText)
	if extractErr != nil || len(facts) == 0 {
		return
	}
	if len(facts) > maxFacts {
		facts = facts[:maxFacts]
	}
	for _, f := range facts {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		_ = m.Writer.WriteFact(ctx, f)
	}
}

// extractFacts calls the reflect model with a strict-JSON prompt and parses.
func (m *ReflectionMiddleware) extractFacts(ctx context.Context, userQuery, assistantText string) ([]string, error) {
	prompt := buildReflectionPrompt(userQuery, assistantText)
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent(prompt).Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("Extract now.").Build(),
	}
	// Modest token budget; reflection should never produce long essays.
	resp, err := m.Model.Chat(ctx, msgs, model.WithMaxTokens(512), model.WithTemperature(0))
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	return parseFactsJSON(resp.GetTextContent())
}

func buildReflectionPrompt(userQuery, assistantText string) string {
	// Truncate to keep the prompt bounded on pathological turns.
	uq, at := userQuery, assistantText
	if len(uq) > 4000 {
		uq = uq[:4000] + "..."
	}
	if len(at) > 4000 {
		at = at[:4000] + "..."
	}
	return fmt.Sprintf("%s\n\n--- DIALOGUE ---\nUser: %s\nAssistant: %s\n--- END ---", ReflectionDefaultPrompt, uq, at)
}

// parseFactsJSON tolerates ```json fences and trailing prose.
func parseFactsJSON(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	// Strip markdown fences if present.
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```JSON")
		raw = strings.TrimPrefix(raw, "```")
		if idx := strings.LastIndex(raw, "```"); idx >= 0 {
			raw = raw[:idx]
		}
	}
	// Extract the outermost JSON object (LLMs sometimes prefix "Here are...").
	start := strings.IndexByte(raw, '{')
	end := strings.LastIndexByte(raw, '}')
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}
	var payload struct {
		Facts []string `json:"facts"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	return payload.Facts, nil
}

func (m *ReflectionMiddleware) initSem() {
	m.once.Do(func() {
		workers := m.Workers
		if workers <= 0 {
			workers = 2
		}
		m.sem = make(chan struct{}, workers)
	})
}

func (m *ReflectionMiddleware) acquire() bool {
	m.initSem()
	select {
	case m.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (m *ReflectionMiddleware) release() {
	if m.sem == nil {
		return
	}
	select {
	case <-m.sem:
	default:
	}
}
