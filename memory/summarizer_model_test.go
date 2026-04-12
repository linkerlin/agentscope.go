package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestSummarizeToDailyFile(t *testing.T) {
	dir := t.TempDir()
	s := &Summarizer{
		Model:      &mockChatModel{reply: "## Notes\n- point"},
		WorkingDir: dir,
	}
	err := s.SummarizeToDailyFile(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("chat").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSummarizeToDailyFileNoModel(t *testing.T) {
	s := &Summarizer{WorkingDir: t.TempDir()}
	err := s.SummarizeToDailyFile(context.Background(), nil)
	if !errors.Is(err, ErrCompactorNoModel) {
		t.Fatalf("got %v", err)
	}
}
