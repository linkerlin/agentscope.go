package model

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

// cheapNamedModel is a scriptable ChatModel for cost-routing tests.
type costMockModel struct {
	name  string
	chats int
	fail  bool
}

func (m *costMockModel) ModelName() string { return m.name }

func (m *costMockModel) Chat(ctx context.Context, messages []*message.Msg, options ...ChatOption) (*message.Msg, error) {
	m.chats++
	if m.fail {
		return nil, errors.New("downstream exploded")
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok-" + m.name).Build(), nil
}

func (m *costMockModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...ChatOption) (<-chan *StreamChunk, error) {
	m.chats++
	if m.fail {
		return nil, errors.New("downstream exploded")
	}
	ch := make(chan *StreamChunk, 1)
	ch <- &StreamChunk{Delta: "ok-" + m.name}
	ch <- &StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func costMessages() []*message.Msg {
	return []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent(strings.Repeat("x", 4000)).Build()}
}

func expensivePrices() (ModelPricing, ModelPricing) {
	return ModelPricing{InputPerMTok: 10, OutputPerMTok: 30}, ModelPricing{InputPerMTok: 0.5, OutputPerMTok: 1.5}
}

func TestCostRouter_NoPolicyBehavesLikeRouter(t *testing.T) {
	primary := &costMockModel{name: "flagship"}
	fallback := &costMockModel{name: "cheap"}
	p, _ := expensivePrices()
	r := NewCostRouter(primary, WithCostFallback(fallback), WithModelPrice("flagship", p))

	resp, err := r.Chat(context.Background(), costMessages())
	if err != nil {
		t.Fatal(err)
	}
	// No budget configured: always primary, even for huge inputs.
	if resp.GetTextContent() != "ok-flagship" || primary.chats != 1 || fallback.chats != 0 {
		t.Fatalf("no-policy must route to primary: primary=%d fallback=%d", primary.chats, fallback.chats)
	}
}

func TestCostRouter_DowngradesOverBudget(t *testing.T) {
	primary := &costMockModel{name: "flagship"}
	fallback := &costMockModel{name: "cheap"}
	p, c := expensivePrices()
	// 4000 chars ≈ 1000 input tokens: flagship ≈ (1000*10 + 1000*30)/1M = $0.04.
	r := NewCostRouter(primary,
		WithCostFallback(fallback),
		WithModelPrice("flagship", p),
		WithModelPrice("cheap", c),
		WithMaxCostPerTurn(0.01),
	)

	resp, err := r.Chat(context.Background(), costMessages())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "ok-cheap" || fallback.chats != 1 || primary.chats != 0 {
		t.Fatalf("over-budget must downgrade to cheap: primary=%d fallback=%d", primary.chats, fallback.chats)
	}
}

func TestCostRouter_WithinBudgetKeepsFlagship(t *testing.T) {
	primary := &costMockModel{name: "flagship"}
	fallback := &costMockModel{name: "cheap"}
	p, c := expensivePrices()
	r := NewCostRouter(primary,
		WithCostFallback(fallback),
		WithModelPrice("flagship", p),
		WithModelPrice("cheap", c),
		WithMaxCostPerTurn(1.0), // generous
	)

	if _, err := r.Chat(context.Background(), costMessages()); err != nil {
		t.Fatal(err)
	}
	if primary.chats != 1 || fallback.chats != 0 {
		t.Fatalf("within-budget must keep flagship: primary=%d fallback=%d", primary.chats, fallback.chats)
	}
}

func TestCostRouter_EscalatesToFlagshipOnFailure(t *testing.T) {
	primary := &costMockModel{name: "flagship"}
	fallback := &costMockModel{name: "cheap", fail: true}
	p, c := expensivePrices()
	r := NewCostRouter(primary,
		WithCostFallback(fallback),
		WithModelPrice("flagship", p),
		WithModelPrice("cheap", c),
		WithMaxCostPerTurn(0.01),
	)

	resp, err := r.Chat(context.Background(), costMessages())
	if err != nil {
		t.Fatal(err)
	}
	// Downgraded call failed -> flagship must still serve the turn.
	if resp.GetTextContent() != "ok-flagship" || primary.chats != 1 {
		t.Fatalf("downgrade failure must escalate to flagship: %q primary=%d", resp.GetTextContent(), primary.chats)
	}
}

func TestCostRouter_DecisionObserverAndContext(t *testing.T) {
	primary := &costMockModel{name: "flagship"}
	fallback := &costMockModel{name: "cheap"}
	p, c := expensivePrices()
	var decisions []RouteDecision
	r := NewCostRouter(primary,
		WithCostFallback(fallback),
		WithModelPrice("flagship", p),
		WithModelPrice("cheap", c),
		WithMaxCostPerTurn(0.01),
		WithRouteDecisionObserver(func(d RouteDecision) { decisions = append(decisions, d) }),
	)

	// Empty context must carry no decision.
	if _, ok := RouteDecisionFromContext(context.Background()); ok {
		t.Fatal("empty context must carry no decision")
	}

	if _, err := r.Chat(context.Background(), costMessages()); err != nil {
		t.Fatal(err)
	}
	if len(decisions) == 0 {
		t.Fatal("observer must receive the route decision")
	}
	dec := decisions[0]
	if !dec.Downgraded || dec.Chosen != "cheap" || dec.Reason == "" {
		t.Fatalf("unexpected decision: %+v", dec)
	}
}

func TestCostRouter_CardPriceRegistration(t *testing.T) {
	primary := &costMockModel{name: "card-flagship"}
	card := &ModelCard{ID: "card-flagship", Pricing: &ModelPricing{InputPerMTok: 10, OutputPerMTok: 30}}
	r := NewCostRouter(primary, WithModelCardPrice(card))
	p := r.prices["card-flagship"]
	if p.total() != 40 {
		t.Fatalf("card pricing not registered: %+v", r.prices)
	}
	// Cards without pricing are ignored.
	r2 := NewCostRouter(primary, WithModelCardPrice(&ModelCard{ID: "no-price"}))
	if _, ok := r2.prices["no-price"]; ok {
		t.Fatal("card without pricing must be ignored")
	}
}
