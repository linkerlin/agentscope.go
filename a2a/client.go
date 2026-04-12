package a2a

import (
	"context"
	"errors"
)

// Message A2A 消息占位类型（具体协议字段可按规范扩展）
type Message struct {
	Role    string
	Content string
	Meta    map[string]any
}

// Client A2A 客户端抽象
type Client interface {
	Send(ctx context.Context, msg *Message) (*Message, error)
	Close() error
}

// NoopClient 占位实现，便于在无远端时编译通过
type NoopClient struct{}

func (NoopClient) Send(ctx context.Context, msg *Message) (*Message, error) {
	return nil, errors.New("a2a: noop client")
}

func (NoopClient) Close() error { return nil }
