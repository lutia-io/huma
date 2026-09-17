package authz

import (
	"context"

	"github.com/lutia-io/huma/pkg/principal"
)

type snapshotKey struct{}

type cachedSnapshot struct {
	key string
	snap *Snapshot
}

func snapshotCacheKey(p principal.Principal, networkID, organizationID string) string {
	return string(p.Type) + ":" + p.ID + ":" + networkID + ":" + organizationID
}

func withSnapshot(ctx context.Context, key string, snap *Snapshot) context.Context {
	return context.WithValue(ctx, snapshotKey{}, cachedSnapshot{key: key, snap: snap})
}

func snapshotFromContext(ctx context.Context, key string) *Snapshot {
	cached, ok := ctx.Value(snapshotKey{}).(cachedSnapshot)
	if !ok || cached.key != key || cached.snap == nil {
		return nil
	}
	return cached.snap
}

// Snapshot is the caller's effective access for one network/org pair.
type Snapshot struct {
	Principal      principal.Principal
	NetworkID      string
	OrganizationID string
	CreatedBy      string
	Creator        bool
	Member         bool
	Grants         []Grant
}

type ResourceActions struct {
	Create       bool `json:"create"`
	Read         bool `json:"read"`
	Update       bool `json:"update"`
	Delete       bool `json:"delete"`
	ManageAccess bool `json:"manageAccess,omitempty"`
}

type Effective struct {
	PrincipalType string                 `json:"principalType"`
	Creator       bool                   `json:"creator"`
	Network       *NetworkEffective      `json:"network,omitempty"`
	Organization  *OrganizationEffective `json:"organization,omitempty"`
}

type NetworkEffective struct {
	Member       bool                       `json:"member"`
	Creator      bool                       `json:"creator"`
	ManageAccess bool                       `json:"manageAccess"`
	Grants       map[string]ResourceActions `json:"grants"`
}

type OrganizationEffective struct {
	Grants      map[string]ResourceActions   `json:"grants"`
	FieldAccess map[string]map[string]string `json:"fieldAccess"`
}

func (s *Snapshot) allowed(action, resource, resourceID, schemaID string) bool {
	if s == nil {
		return false
	}
	if s.Principal.Type == principal.TypeUser && s.Creator && !isOrgOnlyResource(resource) {
		return true
	}
	if s.Principal.Type == principal.TypeUser && (resource == ResourceRecord || resource == ResourceFile) && s.Member {
		return true
	}
	if s.Principal.Type == principal.TypeOrganizationUser && definitionResource(resource) && action != ActionRead {
		return false
	}
	for _, g := range s.Grants {
		if g.matches(resource, resourceID, schemaID) && g.hasAction(action) {
			return true
		}
	}
	return false
}

func (s *Snapshot) ManageAccess() bool {
	return s.allowed(ActionManageAccess, ResourceNetwork, "", "")
}

func isOrgOnlyResource(resource string) bool {
	return resource == ResourceRecord || resource == ResourceFile
}

func (s *Snapshot) Effective() *Effective {
	out := &Effective{
		PrincipalType: string(s.Principal.Type),
		Creator:       s.Creator,
	}
	if s.Principal.Type == principal.TypeUser {
		out.Network = &NetworkEffective{
			Member:       s.Member,
			Creator:      s.Creator,
			ManageAccess: s.ManageAccess(),
			Grants:       resourceActionsFromGrants(s, true),
		}
	}
	if s.OrganizationID != "" && s.Principal.Type == principal.TypeOrganizationUser {
		out.Organization = &OrganizationEffective{
			Grants:      resourceActionsFromGrants(s, false),
			FieldAccess: s.fieldAccessMap(),
		}
	}
	if s.OrganizationID != "" && s.Principal.Type == principal.TypeUser && s.Member {
		out.Organization = &OrganizationEffective{
			Grants: map[string]ResourceActions{
				ResourceRecord: {Create: true, Read: true, Update: true, Delete: true},
				ResourceFile:   {Create: true, Read: true, Delete: true},
			},
			FieldAccess: map[string]map[string]string{},
		}
	}
	return out
}

func resourceActionsFromGrants(s *Snapshot, network bool) map[string]ResourceActions {
	catalog := organizationActions
	if network {
		catalog = networkActions
	}
	out := make(map[string]ResourceActions, len(catalog))
	for resource := range catalog {
		out[resource] = ResourceActions{
			Create:       s.allowed(ActionCreate, resource, "", ""),
			Read:         s.allowed(ActionRead, resource, "", ""),
			Update:       s.allowed(ActionUpdate, resource, "", ""),
			Delete:       s.allowed(ActionDelete, resource, "", ""),
			ManageAccess: resource == ResourceNetwork && s.allowed(ActionManageAccess, ResourceNetwork, "", ""),
		}
	}
	return out
}

func (s *Snapshot) fieldAccessMap() map[string]map[string]string {
	out := make(map[string]map[string]string)
	for _, g := range s.Grants {
		if g.Resource != ResourceRecord {
			continue
		}
		key := g.SchemaID
		if key == "" {
			key = "*"
		}
		fields := out[key]
		if fields == nil {
			fields = map[string]string{}
			out[key] = fields
		}
		if len(g.Fields) == 0 {
			fields["*"] = FieldWrite
			continue
		}
		for _, f := range g.Fields {
			if f.Access == FieldWrite || fields[f.Name] != FieldWrite {
				fields[f.Name] = f.Access
			}
		}
	}
	return out
}
