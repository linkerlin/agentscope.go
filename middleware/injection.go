package middleware

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// DefaultInjectionInstructions is appended to the system prompt to make the
// agent aware of runtime state.
const DefaultInjectionInstructions = "\n\n## Runtime Awareness\n" +
	"You may receive periodic system-reminder hints with the current time, " +
	"task status, and context usage. Use this information to stay oriented."

// DefaultTimeFormat is Go's reference layout for ISO-8601 local time.
const DefaultTimeFormat = "2006-01-02T15:04:05"

// DefaultTimeInterval is the minimum gap between time injections.
const DefaultTimeInterval = 30 * time.Minute

// InjectionConfig controls runtime state injection into agent context.
// Aligned with Python agentscope's InjectionConfig (#29792cd9).
type InjectionConfig struct {
	// InjectRuntimeState enables time/task/context injection. Default true.
	InjectRuntimeState bool
	// Timezone for the injected time, e.g. "UTC", "Asia/Shanghai".
	Timezone string
	// TimeFormat is a Go time layout string.
	TimeFormat string
	// TimeInterval is the minimum gap between time injections.
	TimeInterval time.Duration
	// ContextBufferRatio activates context-length warning when token usage
	// approaches the compression trigger (0.0-1.0). Set to 0 to disable.
	ContextBufferRatio float64
	// Template wraps the injected state. Must contain {runtime_state}.
	Template string
	// ExtraFields are user-defined key-value pairs injected as XML tags.
	ExtraFields map[string]string
	// Instructions appended to the system prompt.
	Instructions string
}

// DefaultInjectionConfig returns a production-ready default config.
func DefaultInjectionConfig() InjectionConfig {
	return InjectionConfig{
		InjectRuntimeState: true,
		Timezone:           "UTC",
		TimeFormat:         DefaultTimeFormat,
		TimeInterval:       DefaultTimeInterval,
		ContextBufferRatio: 0.2,
		Template:           "<system-reminder>\n{runtime_state}\n</system-reminder>",
		Instructions:       DefaultInjectionInstructions,
	}
}

// InjectionMiddleware injects runtime state (current time, extra fields)
// into the agent's context at configurable intervals.
//
// It implements SystemPromptTransformer (appends awareness instructions) and
// ReasoningInterceptor (injects HintBlock with time/fields before reasoning
// steps, respecting the configured interval).
//
// Aligned with Python agentscope's runtime state injection (#29792cd9).
type InjectionMiddleware struct {
	Base
	Config InjectionConfig

	mu           sync.Mutex
	lastInjectAt time.Time
}

// NewInjectionMiddleware creates an InjectionMiddleware with default config.
func NewInjectionMiddleware() *InjectionMiddleware {
	return &InjectionMiddleware{Config: DefaultInjectionConfig()}
}

// WithConfig sets a custom InjectionConfig (builder-style).
func (m *InjectionMiddleware) WithConfig(cfg InjectionConfig) *InjectionMiddleware {
	m.Config = cfg
	return m
}

// WithTimezone overrides the timezone.
func (m *InjectionMiddleware) WithTimezone(tz string) *InjectionMiddleware {
	m.Config.Timezone = tz
	return m
}

// WithExtraFields sets user-defined extra fields.
func (m *InjectionMiddleware) WithExtraFields(fields map[string]string) *InjectionMiddleware {
	m.Config.ExtraFields = fields
	return m
}

// OnSystemPrompt appends runtime awareness instructions to the system prompt.
func (m *InjectionMiddleware) OnSystemPrompt(ctx context.Context, agent Agent, currentPrompt string) (string, error) {
	if !m.Config.InjectRuntimeState {
		return currentPrompt, nil
	}
	instr := m.Config.Instructions
	if instr == "" {
		instr = DefaultInjectionInstructions
	}
	return currentPrompt + instr, nil
}

// OnReasoning injects a runtime-state HintBlock before reasoning steps,
// respecting the configured TimeInterval to avoid flooding the context.
func (m *InjectionMiddleware) OnReasoning(ctx context.Context, agent Agent, input *ReasoningInput, next ReasoningNext) (*message.Msg, error) {
	if m.Config.InjectRuntimeState {
		hint := m.buildHint()
		if hint != "" {
			injectRuntimeHint(input, agent.AgentName(), hint)
		}
	}
	return next(ctx)
}

// buildHint returns the runtime-state text, or "" if the interval hasn't
// elapsed since the last injection.
func (m *InjectionMiddleware) buildHint() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if m.Config.Timezone != "" {
		if loc, err := time.LoadLocation(m.Config.Timezone); err == nil {
			now = now.In(loc)
		}
	}

	interval := m.Config.TimeInterval
	if interval <= 0 {
		interval = DefaultTimeInterval
	}
	if !m.lastInjectAt.IsZero() && now.Sub(m.lastInjectAt) < interval {
		return ""
	}
	m.lastInjectAt = now

	fmtStr := m.Config.TimeFormat
	if fmtStr == "" {
		fmtStr = DefaultTimeFormat
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Current time: %s", now.Format(fmtStr)))

	for k, v := range m.Config.ExtraFields {
		sb.WriteString(fmt.Sprintf("\n<%s>%s</%s>", k, v, k))
	}

	state := sb.String()
	tmpl := m.Config.Template
	if tmpl == "" {
		return state
	}
	return strings.ReplaceAll(tmpl, "{runtime_state}", state)
}

// injectRuntimeHint appends a runtime-state HintBlock to the reasoning input.
func injectRuntimeHint(input *ReasoningInput, agentName, text string) {
	hintMsg := message.NewMsg().
		Role(message.RoleAssistant).
		Name(agentName).
		Content(message.NewHintBlock(text, "runtime_state")).
		Build()
	input.Messages = append(input.Messages, hintMsg)
}
