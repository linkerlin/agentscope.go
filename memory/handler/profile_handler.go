package handler

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

// ProfileHandler 管理本地用户画像文件
type ProfileHandler struct {
	ProfileDir string
}

// NewProfileHandler 创建画像处理器
func NewProfileHandler(profileDir string) *ProfileHandler {
	return &ProfileHandler{ProfileDir: profileDir}
}

func (h *ProfileHandler) userProfilePath(userName string) string {
	return filepath.Join(h.ProfileDir, userName+".json")
}

// ReadAllProfiles 读取指定用户的画像文件
func (h *ProfileHandler) ReadAllProfiles(ctx context.Context, userName string) (map[string]any, error) {
	_ = ctx
	path := h.userProfilePath(userName)
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

// AddProfile 添加画像（如果不存在则新建）
func (h *ProfileHandler) AddProfile(ctx context.Context, userName string, profile map[string]any) error {
	_ = ctx
	if err := os.MkdirAll(h.ProfileDir, 0o755); err != nil {
		return err
	}
	return h.writeProfile(userName, profile)
}

// UpdateProfile 增量更新画像（合并已有字段）
func (h *ProfileHandler) UpdateProfile(ctx context.Context, userName string, updates map[string]any) error {
	existing, err := h.ReadAllProfiles(ctx, userName)
	if err != nil {
		return err
	}
	for k, v := range updates {
		existing[k] = v
	}
	return h.writeProfile(userName, existing)
}

func (h *ProfileHandler) writeProfile(userName string, profile map[string]any) error {
	path := h.userProfilePath(userName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
