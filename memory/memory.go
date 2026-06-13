// Package memory provides conversation storage, long-term memory management,
// vector stores, full-text search, and ReMe-specific memory orchestration.
package memory

import "github.com/linkerlin/agentscope.go/message"

// Memory is the interface for conversation storage
type Memory interface {
	Add(msg *message.Msg) error
	GetAll() ([]*message.Msg, error)
	GetRecent(n int) ([]*message.Msg, error)
	Clear() error
	Size() int
}
