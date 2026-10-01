package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/linkerlin/agentscope.go/event"
)

// CardSender drives DingTalk AI cards: create-and-deliver into a group chat,
// incremental updates (the streaming-card loop), and callback payloads
// normalised into HITL confirm events.
type CardSender struct {
	Ch *Channel
}

// NewCardSender builds a CardSender over a channel's OpenAPI credentials.
func NewCardSender(ch *Channel) *CardSender { return &CardSender{Ch: ch} }

// CreateAndDeliver delivers an AI card instance into a group chat.
// Docs: POST /v1.0/card/instances/createAndDeliver
//
// cardData keys become the template's cardParamMap entries; outTrackID is the
// caller-chosen stable id used later for streaming updates.
func (s *CardSender) CreateAndDeliver(ctx context.Context, openConversationID, cardTemplateID, outTrackID string, cardData map[string]string) error {
	token, err := s.Ch.accessToken(ctx)
	if err != nil {
		return err
	}
	param := map[string]string{}
	for k, v := range cardData {
		param[k] = v
	}
	body, _ := json.Marshal(map[string]any{
		"cardTemplateId": cardTemplateID,
		"outTrackId":     outTrackID,
		"openSpaceId":    "dt@dt",
		"openSpaceBody": map[string]string{
			"openConversationId": openConversationID,
			"robotCode":          s.Ch.appKey,
		},
		"openCardData": map[string]any{"cardParamMap": param},
		"imGroupOpenSpaceModel": map[string]any{
			"supportForward": false,
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.Ch.apiBase+"/v1.0/card/instances/createAndDeliver?access_token="+token, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.Ch.doJSON(req, nil)
}

// UpdateCard streams one card update: changed cardData entries are merged
// into the card instance identified by outTrackID.
// Docs: PUT /v1.0/card/instances
func (s *CardSender) UpdateCard(ctx context.Context, outTrackID string, cardData map[string]string) error {
	token, err := s.Ch.accessToken(ctx)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{
		"outTrackId": outTrackID,
		"cardData": map[string]any{
			"cardParamMap": cardData,
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		s.Ch.apiBase+"/v1.0/card/instances?access_token="+token, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.Ch.doJSON(req, nil)
}

// --- callback → HITL normalisation ---

// CardCallback is a normalised DingTalk card-callback payload. DingTalk
// delivers button/private-data interactions as JSON; this struct carries the
// fields the HITL bridge needs.
type CardCallback struct {
	// CardID is the platform card instance id.
	CardID string
	// TrackID echoes the outTrackID chosen at CreateAndDeliver — the stable
	// cross-reference to the originating turn/session.
	TrackID string
	// UserID is the operator who clicked.
	UserID string
	// ActionValue is the button's private data (arbitrary JSON chosen by the
	// card author).
	ActionValue json.RawMessage
}

// ParseCardCallback decodes a card-callback request body. The plain JSON
// shape covers enterprise-app HTTP callbacks; the encrypted envelope is a
// deployment concern handled before this point.
func ParseCardCallback(r *http.Request) (*CardCallback, error) {
	var raw struct {
		CardID  string `json:"cardId"`
		TrackID string `json:"trackId"`
		UserID  string `json:"userId"`
		Content struct {
			Value json.RawMessage `json:"value"`
		} `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("dingtalk: card callback decode: %w", err)
	}
	return &CardCallback{
		CardID:      raw.CardID,
		TrackID:     raw.TrackID,
		UserID:      raw.UserID,
		ActionValue: raw.Content.Value,
	}, nil
}

// hitlAction is the ActionValue convention for HITL confirm decisions. The
// card author embeds this JSON on the decision buttons; SessionID ties the
// callback back to the agent session awaiting the decision.
type hitlAction struct {
	Type      string `json:"type"` // "hitl_decision"
	SessionID string `json:"session_id"`
	ReplyID   string `json:"reply_id"`
	ConfirmID string `json:"confirm_id"`
	Decisions []struct {
		ToolCallID string `json:"tool_call_id"`
		Decision   string `json:"decision"`
	} `json:"decisions"`
}

// HitlAction is the public shape of a HITL decision embedded in a card
// callback (18.3): the gateway bridge needs SessionID to route the resume,
// which ConfirmDecisionFromCallback's event alone does not carry.
type HitlAction struct {
	SessionID string
	ReplyID   string
	ConfirmID string
	Decisions []event.ConfirmDecision
}

// ErrNotHITLAction is returned when a callback's ActionValue is not a HITL
// decision (custom card actions are free to exist alongside).
var ErrNotHITLAction = fmt.Errorf("dingtalk: callback action is not a hitl_decision")

// ParseHitlAction decodes a card callback into the public HITL action shape.
// Non-HITL actions return ErrNotHITLAction so callers can route them
// elsewhere (custom buttons, bot commands).
func ParseHitlAction(cb *CardCallback) (*HitlAction, error) {
	var action hitlAction
	if len(cb.ActionValue) == 0 {
		return nil, ErrNotHITLAction
	}
	if err := json.Unmarshal(cb.ActionValue, &action); err != nil {
		return nil, fmt.Errorf("dingtalk: decode action value: %w", err)
	}
	if action.Type != "hitl_decision" {
		return nil, ErrNotHITLAction
	}
	if action.SessionID == "" || action.ConfirmID == "" {
		return nil, fmt.Errorf("dingtalk: hitl action missing session_id/confirm_id")
	}
	out := &HitlAction{
		SessionID: action.SessionID,
		ReplyID:   action.ReplyID,
		ConfirmID: action.ConfirmID,
	}
	for _, d := range action.Decisions {
		out.Decisions = append(out.Decisions, event.ConfirmDecision{
			ToolCallID: d.ToolCallID,
			Decision:   d.Decision,
		})
	}
	return out, nil
}

// ConfirmDecisionFromCallback maps a card callback onto the UserConfirmResult
// event that resumes a HITL-suspended agent. Non-HITL actions return
// ErrNotHITLAction so callers can route them elsewhere.
func ConfirmDecisionFromCallback(cb *CardCallback) (*event.UserConfirmResultEvent, error) {
	action, err := ParseHitlAction(cb)
	if err != nil {
		return nil, err
	}
	return event.NewUserConfirmResult(action.ReplyID, action.ConfirmID, action.Decisions), nil
}
