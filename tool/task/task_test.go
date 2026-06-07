package tasktool

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/state"
)

func TestTaskTools(t *testing.T) {
	store := state.NewTaskStore()
	tools := RegisterTools(store)
	ctx := context.Background()

	create := tools[0]
	resp, err := create.Execute(ctx, map[string]any{
		"subject":     "Test",
		"description": "Do something",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}

	list := tools[2]
	resp, err = list.Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("nil list response")
	}
}
