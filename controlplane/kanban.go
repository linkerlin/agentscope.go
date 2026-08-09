package controlplane

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RowLifecycle is the projection lifecycle state of a Kanban card. It is
// derived from a todo's supersession fields and state, and is the data-encoded
// lineage a sink renders instead of requiring a reader to consult a private
// source document (LoopX "Projection Sink Design": row lifecycle as data).
type RowLifecycle string

const (
	// RowCurrent is an active card with no successor.
	RowCurrent RowLifecycle = "current"
	// RowSuperseded is a card replaced by a successor (SupersededBy set).
	RowSuperseded RowLifecycle = "superseded"
	// RowRetired is a deferred card with no successor (cancelled, not replaced).
	RowRetired RowLifecycle = "retired"
)

// KanbanCard is one projection row carrying the full lineage metadata an
// operator or external sink needs to render "why did this card change".
type KanbanCard struct {
	Todo         *Todo        `json:"todo"`
	Lifecycle    RowLifecycle `json:"lifecycle"`
	Supersedes   string       `json:"supersedes,omitempty"`
	SupersededBy string       `json:"superseded_by,omitempty"`
	SourceID     string       `json:"source_id,omitempty"`
	HasOpenGate  bool         `json:"has_open_gate,omitempty"`
}

// LineageEdge is one supersession relationship, for audit/replay.
type LineageEdge struct {
	From   string    `json:"from"` // superseded todo id
	To     string    `json:"to"`   // successor todo id
	Reason string    `json:"reason,omitempty"`
	At     time.Time `json:"at"`
}

// Kanban is the read-only board projection: cards grouped by column (TodoState)
// plus the supersession lineage. The board is a PROJECTION — the stores remain
// the source of truth (LoopX "Agent-Native Kanban Is A Projection"). Mutating
// it changes nothing.
type Kanban struct {
	GoalID    string                     `json:"goal_id"`
	Objective string                     `json:"objective"`
	Columns   map[TodoState][]KanbanCard `json:"columns"`
	Lineage   []LineageEdge              `json:"lineage"`
}

// SupersedeTodo defers oldTodoID and creates a successor todo linked by
// supersession. The old todo becomes RowSuperseded (SupersededBy = new id); the
// new todo carries Supersedes = old id and inherits the old owner/continuation.
// The supersession is recorded as a row_lifecycle decision event in the ledger.
//
// This is the operator behind replans: "this approach is replaced by that
// approach" — expressed as data, not prose. A deferred-without-successor todo
// (plain Defer, not Supersede) becomes RowRetired instead.
//
// ponytail: successor inherits Order/Continuation/TaskClass from the old todo
// for a smooth handoff; callers can mutate the returned todo before any
// further upsert if they need to diverge.
func (k *Kernel) SupersedeTodo(ctx context.Context, goalID, oldTodoID, agentID, newDesc, reason string) (*Todo, error) {
	old, err := k.todos.Get(ctx, goalID, oldTodoID)
	if err != nil {
		return nil, err
	}
	if old.SupersededBy != "" {
		return nil, fmt.Errorf("controlplane: todo %s already superseded by %s", oldTodoID, old.SupersededBy)
	}
	if !LegalTodoTransition(old.State, TodoDeferred) {
		return nil, fmt.Errorf("controlplane: todo %s in state %s cannot be superseded", oldTodoID, old.State)
	}

	// Create the successor, inheriting the lane context.
	successor := &Todo{
		ID:           uuid.NewString(),
		GoalID:       goalID,
		Description:  newDesc,
		TaskClass:    old.TaskClass,
		State:        TodoOpen,
		ClaimedBy:    old.ClaimedBy, // smooth handoff
		Continuation: old.Continuation,
		Order:        old.Order,
		Supersedes:   old.ID,
	}
	// Run the three durable writes (successor, old-link, goal-advance) in one
	// transaction so a mid-way failure rolls back ALL of them (#4 round-3).
	// Without this, a goal could end up pointing at a deleted successor.
	err = k.runTx(ctx, func(tctx context.Context) error {
		if err := k.todos.Upsert(tctx, successor); err != nil {
			return err
		}
		old.State = TodoDeferred
		old.SupersededBy = successor.ID
		if err := k.todos.Upsert(tctx, old); err != nil {
			return err
		}
		if g, gerr := k.goals.Get(tctx, goalID); gerr == nil && g.CurrentTodoID == old.ID {
			g.CurrentTodoID = successor.ID
			if err := k.goals.Upsert(tctx, g); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	k.record(ctx, Event{
		Kind: EventDecision, Type: "todo_superseded",
		GoalID: goalID, TodoID: successor.ID,
		Detail: map[string]any{
			"superseded": old.ID, "by": agentID, "reason": reason,
			"row_lifecycle": string(RowSuperseded),
		},
	})
	return successor, nil
}

// Kanban builds the read-only board projection for a goal: cards grouped into
// columns by TodoState, each card carrying its row-lifecycle metadata, plus the
// supersession lineage edges. Pending gates are folded onto the matching todo
// card (HasOpenGate) so an operator sees blockers in place.
func (k *Kernel) Kanban(ctx context.Context, goalID string) (*Kanban, error) {
	g, err := k.goals.Get(ctx, goalID)
	if err != nil {
		return nil, err
	}
	todos, _ := k.todos.List(ctx, goalID)

	// Index unresolved gates by todo id to fold onto cards.
	gateTodos := map[string]bool{}
	if gates, _ := k.gates.ListUnresolved(ctx, goalID); gates != nil {
		for _, gt := range gates {
			if gt.TodoID != "" {
				gateTodos[gt.TodoID] = true
			}
		}
	}

	board := &Kanban{
		GoalID:    goalID,
		Objective: g.Objective,
		Columns:   map[TodoState][]KanbanCard{},
	}
	for _, t := range todos {
		// Redact private source refs from projected evidence (#3a round-4).
		t.Evidence = RedactEvidenceSlice(t.Evidence)
		card := KanbanCard{
			Todo:         t,
			Lifecycle:    rowLifecycleOf(t),
			Supersedes:   t.Supersedes,
			SupersededBy: t.SupersededBy,
			SourceID:     t.ID,
			HasOpenGate:  gateTodos[t.ID],
		}
		col := t.State
		board.Columns[col] = append(board.Columns[col], card)
		if t.SupersededBy != "" {
			board.Lineage = append(board.Lineage, LineageEdge{
				From: t.ID, To: t.SupersededBy, At: t.UpdatedAt,
			})
		}
	}
	return board, nil
}

// rowLifecycleOf derives the projection lifecycle from a todo's state and
// supersession fields.
func rowLifecycleOf(t *Todo) RowLifecycle {
	if t.SupersededBy != "" {
		return RowSuperseded
	}
	if t.State == TodoDeferred {
		return RowRetired
	}
	return RowCurrent
}
