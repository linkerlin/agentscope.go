package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/linkerlin/agentscope.go/memory"
)

// ProfileBackend 画像存储后端接口
type ProfileBackend interface {
	ReadAll(ctx context.Context, userName string) (map[string]any, error)
	Update(ctx context.Context, userName string, updates map[string]any) error
	Delete(ctx context.Context, userName string, key string) error
	Retrieve(ctx context.Context, userName, query string, topK int) (map[string]any, error)
}

// ProfileHandler 管理用户画像，支持可插拔后端
type ProfileHandler struct {
	Backend ProfileBackend
}

// NewProfileHandler 创建文件后端画像处理器（向后兼容）
func NewProfileHandler(profileDir string) *ProfileHandler {
	return &ProfileHandler{Backend: NewFileProfileBackend(profileDir)}
}

// NewProfileHandlerWithBackend 使用自定义后端创建画像处理器
func NewProfileHandlerWithBackend(backend ProfileBackend) *ProfileHandler {
	return &ProfileHandler{Backend: backend}
}

// ReadAllProfiles 读取用户全部画像
func (h *ProfileHandler) ReadAllProfiles(ctx context.Context, userName string) (map[string]any, error) {
	if h.Backend == nil {
		return make(map[string]any), nil
	}
	return h.Backend.ReadAll(ctx, userName)
}

// AddProfile 添加画像
func (h *ProfileHandler) AddProfile(ctx context.Context, userName string, profile map[string]any) error {
	if h.Backend == nil {
		return fmt.Errorf("profile backend not configured")
	}
	return h.Backend.Update(ctx, userName, profile)
}

// UpdateProfile 增量更新画像
func (h *ProfileHandler) UpdateProfile(ctx context.Context, userName string, updates map[string]any) error {
	if h.Backend == nil {
		return fmt.Errorf("profile backend not configured")
	}
	return h.Backend.Update(ctx, userName, updates)
}

// RetrieveProfile 语义检索画像
func (h *ProfileHandler) RetrieveProfile(ctx context.Context, userName, query string, topK int) (map[string]any, error) {
	if h.Backend == nil {
		return make(map[string]any), nil
	}
	return h.Backend.Retrieve(ctx, userName, query, topK)
}

// FileProfileBackend 文件系统画像后端
type FileProfileBackend struct {
	ProfileDir string
}

func NewFileProfileBackend(profileDir string) *FileProfileBackend {
	return &FileProfileBackend{ProfileDir: profileDir}
}

func (b *FileProfileBackend) userProfilePath(userName string) string {
	return filepath.Join(b.ProfileDir, userName+".json")
}

func (b *FileProfileBackend) ReadAll(ctx context.Context, userName string) (map[string]any, error) {
	_ = ctx
	path := b.userProfilePath(userName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, err
	}
	var profile map[string]any
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func (b *FileProfileBackend) Update(ctx context.Context, userName string, updates map[string]any) error {
	existing, err := b.ReadAll(ctx, userName)
	if err != nil {
		return err
	}
	for k, v := range updates {
		existing[k] = v
	}
	if err := os.MkdirAll(b.ProfileDir, 0o755); err != nil {
		return err
	}
	path := b.userProfilePath(userName)
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (b *FileProfileBackend) Delete(ctx context.Context, userName string, key string) error {
	existing, err := b.ReadAll(ctx, userName)
	if err != nil {
		return err
	}
	delete(existing, key)
	path := b.userProfilePath(userName)
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (b *FileProfileBackend) Retrieve(ctx context.Context, userName, query string, topK int) (map[string]any, error) {
	return b.ReadAll(ctx, userName)
}

// VectorProfileBackend 向量存储画像后端
type VectorProfileBackend struct {
	Store       memory.VectorStore
	MemoryType  memory.MemoryType
	MaxProfiles int
}

func NewVectorProfileBackend(store memory.VectorStore) *VectorProfileBackend {
	return &VectorProfileBackend{
		Store:       store,
		MemoryType:  memory.MemoryTypePersonal,
		MaxProfiles: 100,
	}
}

func (b *VectorProfileBackend) ReadAll(ctx context.Context, userName string) (map[string]any, error) {
	nodes, err := b.Store.Search(ctx, "", memory.RetrieveOptions{
		TopK:          b.MaxProfiles,
		MinScore:      0,
		MemoryTypes:   []memory.MemoryType{b.MemoryType},
		MemoryTargets: []string{userName},
	})
	if err != nil {
		return nil, err
	}
	profile := make(map[string]any, len(nodes))
	for _, n := range nodes {
		profile[n.WhenToUse] = n.Content
		if n.WhenToUse == "" {
			if kw, ok := n.Metadata["keywords"].(string); ok {
				profile[kw] = n.Content
			}
		}
	}
	return profile, nil
}

func (b *VectorProfileBackend) Update(ctx context.Context, userName string, updates map[string]any) error {
	for key, value := range updates {
		content := ""
		switch v := value.(type) {
		case string:
			content = v
		default:
			content = fmt.Sprintf("%v", v)
		}
		node := memory.NewMemoryNodeWithWhen(b.MemoryType, userName, content, key)
		if err := b.Store.Insert(ctx, []*memory.MemoryNode{node}); err != nil {
			return err
		}
	}
	return nil
}

func (b *VectorProfileBackend) Delete(ctx context.Context, userName string, key string) error {
	return b.Store.Delete(ctx, key)
}

func (b *VectorProfileBackend) Retrieve(ctx context.Context, userName, query string, topK int) (map[string]any, error) {
	nodes, err := b.Store.Search(ctx, query, memory.RetrieveOptions{
		TopK:          topK,
		MinScore:      0,
		MemoryTypes:   []memory.MemoryType{b.MemoryType},
		MemoryTargets: []string{userName},
	})
	if err != nil {
		return nil, err
	}
	profile := make(map[string]any, len(nodes))
	for _, n := range nodes {
		key := n.WhenToUse
		if key == "" {
			key = n.MemoryID
		}
		profile[key] = n.Content
	}
	return profile, nil
}
