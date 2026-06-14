package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Role 定义用户角色
type Role string

const (
	RoleAdmin     Role = "admin"     // 管理员：全部权限
	RoleDeveloper Role = "developer" // 开发者：创建 Agent、使用工具
	RoleViewer    Role = "viewer"    // 查看者：只读访问
)

// Permission 定义细粒度权限
type Permission string

const (
	PermAgentRead   Permission = "agent:read"
	PermAgentWrite  Permission = "agent:write"
	PermAgentDelete Permission = "agent:delete"

	PermSessionRead   Permission = "session:read"
	PermSessionWrite  Permission = "session:write"
	PermSessionDelete Permission = "session:delete"

	PermCredentialRead   Permission = "credential:read"
	PermCredentialWrite  Permission = "credential:write"
	PermCredentialDelete Permission = "credential:delete"

	PermScheduleRead   Permission = "schedule:read"
	PermScheduleWrite  Permission = "schedule:write"
	PermScheduleDelete Permission = "schedule:delete"

	PermToolExecute Permission = "tool:execute"
	PermToolAdmin   Permission = "tool:admin"

	PermUserRead   Permission = "user:read"
	PermUserWrite  Permission = "user:write"
	PermUserDelete Permission = "user:delete"

	PermSystemAdmin Permission = "system:admin"
)

// rolePermissions 定义每个角色的权限集合
var rolePermissions = map[Role][]Permission{
	RoleAdmin: {
		PermAgentRead, PermAgentWrite, PermAgentDelete,
		PermSessionRead, PermSessionWrite, PermSessionDelete,
		PermCredentialRead, PermCredentialWrite, PermCredentialDelete,
		PermScheduleRead, PermScheduleWrite, PermScheduleDelete,
		PermToolExecute, PermToolAdmin,
		PermUserRead, PermUserWrite, PermUserDelete,
		PermSystemAdmin,
	},
	RoleDeveloper: {
		PermAgentRead, PermAgentWrite,
		PermSessionRead, PermSessionWrite,
		PermCredentialRead, PermCredentialWrite,
		PermScheduleRead, PermScheduleWrite,
		PermToolExecute,
	},
	RoleViewer: {
		PermAgentRead,
		PermSessionRead,
		PermCredentialRead,
		PermScheduleRead,
	},
}

// HasPermission 检查角色是否拥有指定权限
func HasPermission(role Role, perm Permission) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// HasAnyPermission 检查角色是否拥有任意一个指定权限
func HasAnyPermission(role Role, perms ...Permission) bool {
	for _, perm := range perms {
		if HasPermission(role, perm) {
			return true
		}
	}
	return false
}

// RBACMiddleware 创建 RBAC 检查中间件
func RBACMiddleware(required Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := RoleFromContext(r.Context())
			if !HasPermission(role, required) {
				http.Error(w, `{"error":"forbidden","message":"insufficient permissions"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RoleFromContext 从 context 中提取角色
func RoleFromContext(ctx context.Context) Role {
	if r, ok := ctx.Value(ContextKeyRole).(Role); ok {
		return r
	}
	return RoleViewer // 默认只读
}

// ContextKeyRole is the context key for user role.
const ContextKeyRole ContextKey = "role"

// UserRole 用户角色关联
type UserRole struct {
	UserID    string    `json:"user_id"`
	Role      Role      `json:"role"`
	OrgID     string    `json:"org_id,omitempty"` // 组织/工作空间隔离
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`   // e.g. "agent:create", "tool:execute"
	Resource  string    `json:"resource"` // e.g. "agent:agent-123"
	Details   string    `json:"details"`
	IP        string    `json:"ip"`
	Success   bool      `json:"success"`
	ErrorMsg  string    `json:"error_msg,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// AuditLogger 审计日志接口
type AuditLogger interface {
	Log(ctx context.Context, entry *AuditLog) error
	ListByUser(ctx context.Context, userID string, limit int) ([]*AuditLog, error)
	ListByResource(ctx context.Context, resource string, limit int) ([]*AuditLog, error)
}

// MemoryAuditLogger 内存审计日志实现（开发测试）
type MemoryAuditLogger struct {
	logs []*AuditLog
}

func NewMemoryAuditLogger() *MemoryAuditLogger {
	return &MemoryAuditLogger{logs: make([]*AuditLog, 0)}
}

func (l *MemoryAuditLogger) Log(ctx context.Context, entry *AuditLog) error {
	l.logs = append(l.logs, entry)
	return nil
}

func (l *MemoryAuditLogger) ListByUser(ctx context.Context, userID string, limit int) ([]*AuditLog, error) {
	result := make([]*AuditLog, 0)
	for i := len(l.logs) - 1; i >= 0 && len(result) < limit; i-- {
		if l.logs[i].UserID == userID {
			result = append(result, l.logs[i])
		}
	}
	return result, nil
}

func (l *MemoryAuditLogger) ListByResource(ctx context.Context, resource string, limit int) ([]*AuditLog, error) {
	result := make([]*AuditLog, 0)
	for i := len(l.logs) - 1; i >= 0 && len(result) < limit; i-- {
		if strings.HasPrefix(l.logs[i].Resource, resource) {
			result = append(result, l.logs[i])
		}
	}
	return result, nil
}

// VerifyRoleAssignment 验证角色分配是否合法（防止权限提升）
func VerifyRoleAssignment(currentRole, targetRole Role) error {
	// 只有 admin 可以分配 admin 角色
	if targetRole == RoleAdmin && currentRole != RoleAdmin {
		return fmt.Errorf("only admin can assign admin role")
	}
	// 不能分配比自己更高的角色
	roleHierarchy := map[Role]int{
		RoleViewer:    1,
		RoleDeveloper: 2,
		RoleAdmin:     3,
	}
	if roleHierarchy[targetRole] > roleHierarchy[currentRole] {
		return fmt.Errorf("cannot assign role higher than your own")
	}
	return nil
}
