package authz

import (
	"context"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/render"
)

type Engine struct {
	store store
}

func New(pool *pgxpool.Pool) *Engine {
	return &Engine{store: newPostgresStore(pool)}
}

func NewEngineWithStore(store store) *Engine {
	return &Engine{store: store}
}

func RegisterHTTP(engine *Engine, mux *http.ServeMux) {
	mux.HandleFunc("GET /authorization", func(w http.ResponseWriter, r *http.Request) {
		p, ok := principal.FromContext(r.Context())
		if !ok {
			render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
			return
		}
		networkID := p.NetworkID
		if networkID == "" {
			networkID = strings.TrimSpace(r.Header.Get("X-Network-Id"))
		}
		if networkID == "" {
			networkID = strings.TrimSpace(r.URL.Query().Get("networkId"))
		}
		organizationID := p.OrganizationID
		if organizationID == "" {
			organizationID = strings.TrimSpace(r.Header.Get("X-Organization-Id"))
		}
		if organizationID == "" {
			organizationID = strings.TrimSpace(r.URL.Query().Get("organizationId"))
		}
		snap, err := engine.Load(r.Context(), p, networkID, organizationID)
		if err != nil {
			render.WriteError(w, err)
			return
		}
		render.WriteJSON(w, http.StatusOK, snap.Effective())
	})
}

func (e *Engine) SeedNetwork(ctx context.Context, networkID, creatorUserID string) error {
	return e.store.SeedNetwork(ctx, networkID, creatorUserID)
}

func (e *Engine) SeedOrganization(ctx context.Context, organizationID, networkID string) error {
	return e.store.SeedOrganization(ctx, organizationID, networkID)
}

func (e *Engine) Load(ctx context.Context, p principal.Principal, networkID, organizationID string) (*Snapshot, error) {
	key := snapshotCacheKey(p, networkID, organizationID)
	if snap := snapshotFromContext(ctx, key); snap != nil {
		return snap, nil
	}
	snap := &Snapshot{
		Principal:      p,
		NetworkID:      networkID,
		OrganizationID: organizationID,
	}
	if networkID != "" {
		createdBy, err := e.store.NetworkCreatedBy(ctx, networkID)
		if err != nil {
			return nil, err
		}
		snap.CreatedBy = createdBy
		if p.Type == principal.TypeUser {
			snap.Creator = p.ID == createdBy
			member, err := e.store.IsNetworkMember(ctx, p.ID, networkID)
			if err != nil {
				return nil, err
			}
			snap.Member = member || snap.Creator
			if snap.Member {
				grants, err := e.store.NetworkGrants(ctx, p.ID, networkID)
				if err != nil {
					return nil, err
				}
				snap.Grants = grants
			}
		}
		if p.Type == principal.TypeOrganizationUser {
			if p.NetworkID != "" && p.NetworkID != networkID {
				return snap, nil
			}
			snap.Member = p.OrganizationID != ""
		}
	}
	if p.Type == principal.TypeOrganizationUser && organizationID != "" {
		if p.OrganizationID != "" && p.OrganizationID != organizationID {
			snap.Member = false
			return snap, nil
		}
		grants, err := e.store.OrganizationGrants(ctx, p.ID, organizationID)
		if err != nil {
			return nil, err
		}
		snap.Grants = grants
		snap.Member = true
	}
	return snap, nil
}

func (e *Engine) LoadContext(ctx context.Context, p principal.Principal, networkID, organizationID string) (context.Context, *Snapshot, error) {
	snap, err := e.Load(ctx, p, networkID, organizationID)
	if err != nil {
		return ctx, nil, err
	}
	return withSnapshot(ctx, snapshotCacheKey(p, networkID, organizationID), snap), snap, nil
}

func (e *Engine) IsNetworkMember(ctx context.Context, userID, networkID string) (bool, error) {
	if userID == "" || networkID == "" {
		return false, nil
	}
	return e.store.IsNetworkMember(ctx, userID, networkID)
}

func (e *Engine) VisibleNetwork(ctx context.Context, p principal.Principal, networkID string) error {
	switch p.Type {
	case principal.TypeUser:
		ok, err := e.IsNetworkMember(ctx, p.ID, networkID)
		if err != nil {
			return err
		}
		if !ok {
			return apperror.NewNotFoundError("Network not found", nil)
		}
		return nil
	case principal.TypeOrganizationUser:
		if p.NetworkID != networkID {
			return apperror.NewNotFoundError("Network not found", nil)
		}
		return nil
	default:
		return apperror.NewUnauthorizedError("Authentication required", nil)
	}
}

func (e *Engine) VisibleFor(ctx context.Context, p principal.Principal, networkID string, notFound error) error {
	err := e.VisibleNetwork(ctx, p, networkID)
	if err != nil && apperror.IsNotFound(err) {
		return notFound
	}
	return err
}

func notFoundFor(resource string) error {
	switch resource {
	case ResourceNetwork:
		return apperror.NewNotFoundError("Network not found", nil)
	case ResourceOrganization:
		return apperror.NewNotFoundError("Organization not found", nil)
	case ResourceOrganizationUser:
		return apperror.NewNotFoundError("Organization user not found", nil)
	case ResourceSchema:
		return apperror.NewNotFoundError("Schema not found", nil)
	case ResourceWorkflowDefinition:
		return apperror.NewNotFoundError("Workflow definition not found", nil)
	case ResourcePipelineDefinition:
		return apperror.NewNotFoundError("Pipeline definition not found", nil)
	case ResourceRecord:
		return apperror.NewNotFoundError("Record not found", nil)
	case ResourceFile:
		return apperror.NewNotFoundError("File not found", nil)
	default:
		return apperror.NewNotFoundError("Not found", nil)
	}
}

func (e *Engine) Allow(ctx context.Context, p principal.Principal, action, resource, resourceID, networkID, organizationID string) error {
	snap, err := e.Load(ctx, p, networkID, organizationID)
	if err != nil {
		return err
	}
	if p.Type == principal.TypeUser && !snap.Member {
		return notFoundFor(resource)
	}
	if p.Type == principal.TypeOrganizationUser {
		if organizationID != "" && p.OrganizationID != organizationID {
			return notFoundFor(resource)
		}
		if networkID != "" && p.NetworkID != networkID {
			return notFoundFor(resource)
		}
	}
	schemaID := ""
	if resource == ResourceRecord {
		schemaID = resourceID
		resourceID = ""
	}
	if snap.allowed(action, resource, resourceID, schemaID) {
		return nil
	}
	return apperror.NewForbiddenError("Permission denied", nil)
}

func (e *Engine) RequireCreator(ctx context.Context, p principal.Principal, networkID string) error {
	if err := principal.RequireUser(p, networkID); err != nil {
		return err
	}
	snap, err := e.Load(ctx, p, networkID, "")
	if err != nil {
		return err
	}
	if !snap.Member {
		return apperror.NewNotFoundError("Network not found", nil)
	}
	if !snap.Creator {
		return apperror.NewForbiddenError("Only the network creator can perform this action", nil)
	}
	return nil
}

func (e *Engine) RequireManageAccess(ctx context.Context, p principal.Principal, networkID string) error {
	if err := principal.RequireUser(p, networkID); err != nil {
		return err
	}
	return e.Allow(ctx, p, ActionManageAccess, ResourceNetwork, "", networkID, "")
}
