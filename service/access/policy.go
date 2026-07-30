// Package access provides cross-user resource access policies for multi-tenant
// agent services. Aligned with Python agentscope's ResourceAccessPolicy (#1aeb03d0).
package access

// ResourceKind categorizes shareable resources.
type ResourceKind string

const (
	KindCredential    ResourceKind = "credential"
	KindAgent         ResourceKind = "agent"
	KindKnowledgeBase ResourceKind = "knowledge_base"
)

// ResourcePermission defines the access level for a shared resource.
type ResourcePermission string

const (
	// PermRead allows viewing and using a resource (e.g. calling an agent,
	// querying a KB). READ does not allow mutation.
	PermRead ResourcePermission = "read"
	// PermEdit allows reading plus mutating the resource (updating config,
	// adding documents, etc.). EDIT implies READ.
	PermEdit ResourcePermission = "edit"
)

// ResourceRef describes a cross-owner resource reference.
type ResourceRef struct {
	Kind       ResourceKind       `json:"kind"`
	OwnerID    string             `json:"owner_id"`
	ResourceID string             `json:"resource_id"`
	Permission ResourcePermission `json:"permission"`
}

// Policy is the extension point for cross-owner resource access decisions.
// Applications subclass (implement) this to read rules from config, IAM, LDAP, etc.
//
// The policy intentionally does NOT manage users, groups, or memberships.
// It only maps viewer_id → resource references.
type Policy interface {
	// ListAccessible returns cross-owner resources visible to viewerID for
	// the given kind. Owner-owned resources are NOT included here; callers
	// merge them separately.
	ListAccessible(viewerID string, kind ResourceKind) ([]ResourceRef, error)

	// CanEdit checks whether viewerID has edit rights on a resource owned
	// by ownerID. Owners always can. Cross-owner requires EDIT permission.
	CanEdit(viewerID, ownerID, resourceID string, kind ResourceKind) (bool, error)
}

// DenyAllPolicy denies all cross-owner access, preserving owner-isolated behavior.
// This is the default when no policy is configured.
type DenyAllPolicy struct{}

func (DenyAllPolicy) ListAccessible(viewerID string, kind ResourceKind) ([]ResourceRef, error) {
	return nil, nil
}

func (DenyAllPolicy) CanEdit(viewerID, ownerID, resourceID string, kind ResourceKind) (bool, error) {
	return viewerID == ownerID, nil
}

// StaticPolicy is a simple in-memory policy backed by a list of refs.
// Useful for testing and small deployments.
type StaticPolicy struct {
	Refs []ResourceRef
}

func (s *StaticPolicy) ListAccessible(viewerID string, kind ResourceKind) ([]ResourceRef, error) {
	var out []ResourceRef
	for _, r := range s.Refs {
		if r.Kind == kind {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *StaticPolicy) CanEdit(viewerID, ownerID, resourceID string, kind ResourceKind) (bool, error) {
	if viewerID == ownerID {
		return true, nil
	}
	for _, r := range s.Refs {
		if r.Kind == kind && r.OwnerID == ownerID && r.ResourceID == resourceID && r.Permission == PermEdit {
			return true, nil
		}
	}
	return false, nil
}
