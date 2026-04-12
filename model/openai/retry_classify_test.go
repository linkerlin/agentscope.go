package openai

import (
	"context"
	"errors"
	"testing"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/retry"
)

func TestClassifyOpenAIErrPermanent4xx(t *testing.T) {
	err := &goopenai.APIError{HTTPStatusCode: 401, Message: "bad"}
	w := classifyOpenAIErr(err)
	if !retry.IsPermanent(w) {
		t.Fatal("expected permanent")
	}
}

func TestClassifyOpenAIErrRetry429(t *testing.T) {
	err := &goopenai.APIError{HTTPStatusCode: 429, Message: "rate"}
	w := classifyOpenAIErr(err)
	if retry.IsPermanent(w) {
		t.Fatal("429 should retry")
	}
}

func TestClassifyOpenAIErrCanceled(t *testing.T) {
	err := context.Canceled
	w := classifyOpenAIErr(err)
	if !retry.IsPermanent(w) {
		t.Fatal("canceled should be permanent")
	}
}

func TestClassifyOpenAIErrNetwork(t *testing.T) {
	err := errors.New("tcp reset")
	w := classifyOpenAIErr(err)
	if retry.IsPermanent(w) {
		t.Fatal("network should retry")
	}
}
