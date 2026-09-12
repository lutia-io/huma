package pipeline

import (
	"context"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/slug"
	"github.com/lutia-io/huma/pkg/user"
	"github.com/lutia-io/huma/pkg/uuid"
)

type Service struct {
	logger *logger.Logger
	store  store
}

func NewService(logger *logger.Logger, store store) *Service {
	return &Service{
		logger: logger,
		store:  store,
	}
}

func optionalID(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func visibleToOrganization(networkID string, organizationID *string, principalNetworkID, principalOrganizationID string) bool {
	if networkID != principalNetworkID {
		return false
	}
	if organizationID == nil {
		return true
	}
	return *organizationID == principalOrganizationID
}

func (s *Service) Insert(ctx context.Context, req insertPipelineDefinitionRequest) (string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		s.logger.WarnContext(ctx, "Empty name")
		return "", apperror.NewBadRequestError("Name is required", nil)
	}

	slug := slug.Slugify(req.Name)
	if slug == "" {
		s.logger.WarnContext(ctx, "Empty slug")
		return "", apperror.NewBadRequestError("Slug is required", nil)
	}

	networkID := strings.TrimSpace(req.NetworkID)
	if networkID == "" {
		s.logger.WarnContext(ctx, "Empty network ID")
		return "", apperror.NewBadRequestError("Network ID is required", nil)
	}

	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		s.logger.WarnContext(ctx, "Empty user ID")
		return "", apperror.NewBadRequestError("User ID is required", nil)
	}

	organizationID := optionalID(req.OrganizationID)
	if organizationID != nil && !uuid.Valid(*organizationID) {
		s.logger.WarnContext(ctx, "Invalid organization ID")
		return "", apperror.NewBadRequestError("Invalid organization ID", nil)
	}

	if err := validateDefinition(req.Definition); err != nil {
		s.logger.WarnContext(ctx, "Invalid definition", logger.KeyError, err)
		return "", err
	}

	p := &pipelineDefinition{
		Name:           name,
		Slug:           slug,
		Description:    strings.TrimSpace(req.Description),
		Active:         req.Active,
		Internal:       req.Internal,
		Definition:     req.Definition,
		NetworkID:      networkID,
		OrganizationID: organizationID,
		UserID:         userID,
		CreatedBy:      user.Ref{ID: userID},
		UpdatedBy:      user.Ref{ID: userID},
	}

	id, err := s.store.Insert(ctx, p)
	if err != nil {
		if apperror.IsConflict(err) {
			s.logger.WarnContext(ctx, "Rejected duplicate pipeline definition", logger.KeySlug, slug)
			return "", err
		}
		s.logger.ErrorContext(ctx, "Failed to insert pipeline definition", logger.KeySlug, slug, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created pipeline definition", logger.KeyID, id)
	return id, nil
}

func (s *Service) Patch(ctx context.Context, existing *pipelineDefinition, req patchPipelineDefinitionRequest, updatedBy string) error {
	if existing.Internal {
		return apperror.NewBadRequestError("Internal pipeline definitions cannot be updated", nil)
	}

	if req.Name == nil && req.Description == nil && req.Active == nil && req.Definition == nil {
		return apperror.NewBadRequestError("No fields to update", nil)
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			s.logger.WarnContext(ctx, "Empty name")
			return apperror.NewBadRequestError("Name is required", nil)
		}
		slug := slug.Slugify(name)
		if slug == "" {
			s.logger.WarnContext(ctx, "Empty slug")
			return apperror.NewBadRequestError("Slug is required", nil)
		}
		existing.Name = name
		existing.Slug = slug
	}

	if req.Description != nil {
		existing.Description = strings.TrimSpace(*req.Description)
	}

	if req.Active != nil {
		existing.Active = *req.Active
	}

	if req.Definition != nil {
		if err := validateDefinition(*req.Definition); err != nil {
			s.logger.WarnContext(ctx, "Invalid definition", logger.KeyError, err)
			return err
		}
		existing.Definition = *req.Definition
	}

	existing.UpdatedBy.ID = updatedBy

	if err := s.store.Update(ctx, existing); err != nil {
		if apperror.IsConflict(err) || apperror.IsNotFound(err) {
			s.logger.WarnContext(ctx, "Rejected pipeline definition update", logger.KeyID, existing.ID, logger.KeyError, err)
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to update pipeline definition", logger.KeyID, existing.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully updated pipeline definition", logger.KeyID, existing.ID)
	return nil
}

func (s *Service) List(ctx context.Context, p principal.Principal, params listParams) (*listResult, error) {
	switch p.Type {
	case principal.TypeUser:
		params.UserID = p.ID
	case principal.TypeOrganizationUser:
		if p.NetworkID == "" || p.OrganizationID == "" {
			return nil, apperror.NewForbiddenError("Organization user token missing network or organization", nil)
		}
		params.UserID = ""
		params.NetworkID = p.NetworkID
		params.OrganizationID = p.OrganizationID
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	result, err := s.store.List(ctx, params)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list pipeline definitions", logger.KeyUserID, p.ID, logger.KeyError, err)
		return nil, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, p principal.Principal, id string) (*pipelineDefinition, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid pipeline definition ID", nil)
	}

	pipeline, err := s.store.GetByID(ctx, id)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get pipeline definition", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}

	switch p.Type {
	case principal.TypeUser:
		if pipeline.UserID != p.ID {
			return nil, apperror.NewNotFoundError("Pipeline definition not found", nil)
		}
	case principal.TypeOrganizationUser:
		if !visibleToOrganization(pipeline.NetworkID, pipeline.OrganizationID, p.NetworkID, p.OrganizationID) {
			return nil, apperror.NewNotFoundError("Pipeline definition not found", nil)
		}
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	return pipeline, nil
}

func (s *Service) Enqueue(ctx context.Context, req EnqueueRequest) (string, error) {
	var (
		def *pipelineDefinition
		err error
	)
	switch {
	case strings.TrimSpace(req.PipelineDefinitionID) != "":
		if !uuid.Valid(req.PipelineDefinitionID) {
			return "", apperror.NewBadRequestError("Invalid pipeline definition ID", nil)
		}
		def, err = s.store.GetByID(ctx, req.PipelineDefinitionID)
	case strings.TrimSpace(req.PipelineSlug) != "":
		def, err = s.store.GetBySlug(ctx, req.NetworkID, req.PipelineSlug, req.OrganizationID)
	default:
		return "", apperror.NewBadRequestError("Pipeline definition is required", nil)
	}
	if err != nil {
		return "", err
	}
	if def.NetworkID != req.NetworkID {
		return "", apperror.NewNotFoundError("Pipeline definition not found", nil)
	}
	if !def.MatchesOrganization(req.OrganizationID) {
		return "", apperror.NewNotFoundError("Pipeline definition not found", nil)
	}
	if !def.Active {
		return "", apperror.NewBadRequestError("Pipeline definition is not active", nil)
	}
	if err := validateDefinition(def.Definition); err != nil {
		return "", err
	}

	input := req.Input
	if input == nil {
		input = map[string]any{}
	}
	dedupeKey := strings.TrimSpace(req.DedupeKey)
	if dedupeKey == "" {
		dedupeKey = uuid.MustNew()
	}

	run := &Pipeline{
		PipelineDefinitionID: def.ID,
		NetworkID:            def.NetworkID,
		OrganizationID:       req.OrganizationID,
		OrganizationUserID:   req.OrganizationUserID,
		DedupeKey:            dedupeKey,
		Input:                input,
		Definition:           snapshotDefinition(def.Definition),
	}
	id, err := s.store.InsertPending(ctx, run)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to enqueue pipeline", logger.KeyID, def.ID, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Enqueued pipeline", logger.KeyID, id, "pipeline_definition_id", def.ID)
	return id, nil
}

func (s *Service) ListPipelines(ctx context.Context, p principal.Principal, params runListParams) (*runListResult, error) {
	switch p.Type {
	case principal.TypeUser:
		params.UserID = p.ID
	case principal.TypeOrganizationUser:
		if p.NetworkID == "" || p.OrganizationID == "" {
			return nil, apperror.NewForbiddenError("Organization user token missing network or organization", nil)
		}
		params.UserID = ""
		params.NetworkID = p.NetworkID
		params.OrganizationID = p.OrganizationID
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	result, err := s.store.ListPipelines(ctx, params)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list pipelines", logger.KeyUserID, p.ID, logger.KeyError, err)
		return nil, err
	}
	return result, nil
}

func (s *Service) GetPipeline(ctx context.Context, p principal.Principal, id string) (*Pipeline, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid pipeline ID", nil)
	}

	run, err := s.store.GetPipelineByID(ctx, id)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get pipeline", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}
	if err := s.authorizePipeline(p, run); err != nil {
		return nil, err
	}
	return run, nil
}

// Retry reopens a failed pipeline for another execution. Input stays the
// snapshot from enqueue. The node graph is replaced with the live
// definition when it still exists, so a definition fix (wrong field name,
// etc.) is what actually runs. Workers skip nodes that already completed,
// which keeps a retried pipeline from repeating successful side effects
// such as a RECORD CREATE.
func (s *Service) Retry(ctx context.Context, p principal.Principal, id string) error {
	run, err := s.GetPipeline(ctx, p, id)
	if err != nil {
		return err
	}
	if run.Status != "failed" {
		return apperror.NewBadRequestError("Only failed pipelines can be retried", nil)
	}

	definition := run.Definition
	live, err := s.store.GetByID(ctx, run.PipelineDefinitionID)
	if err != nil {
		if !apperror.IsNotFound(err) {
			s.logger.ErrorContext(ctx, "Failed to load pipeline definition for retry", logger.KeyID, run.PipelineDefinitionID, logger.KeyError, err)
			return err
		}
	} else {
		definition = snapshotDefinition(live.Definition)
	}

	if err := s.store.RetryFailed(ctx, run.ID, definition); err != nil {
		if apperror.IsBadRequest(err) {
			s.logger.WarnContext(ctx, "Rejected pipeline retry", logger.KeyID, run.ID, logger.KeyError, err)
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to retry pipeline", logger.KeyID, run.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully retried pipeline", logger.KeyID, run.ID)
	return nil
}

func (s *Service) ListPipelineNodes(ctx context.Context, p principal.Principal, pipelineID string) ([]*PipelineNode, error) {
	if _, err := s.GetPipeline(ctx, p, pipelineID); err != nil {
		return nil, err
	}
	nodes, err := s.store.ListPipelineNodesByPipelineID(ctx, pipelineID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list pipeline nodes", logger.KeyID, pipelineID, logger.KeyError, err)
		return nil, err
	}
	return nodes, nil
}

func (s *Service) GetPipelineNode(ctx context.Context, p principal.Principal, id string) (*PipelineNode, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid pipeline node ID", nil)
	}
	n, err := s.store.GetPipelineNodeByID(ctx, id)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get pipeline node", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}
	if _, err := s.GetPipeline(ctx, p, n.PipelineID); err != nil {
		return nil, err
	}
	return n, nil
}

func (s *Service) authorizePipeline(p principal.Principal, run *Pipeline) error {
	switch p.Type {
	case principal.TypeUser:
		if run.UserID != p.ID {
			return apperror.NewNotFoundError("Pipeline not found", nil)
		}
	case principal.TypeOrganizationUser:
		if run.NetworkID != p.NetworkID || run.OrganizationID != p.OrganizationID {
			return apperror.NewNotFoundError("Pipeline not found", nil)
		}
	default:
		return apperror.NewUnauthorizedError("Authentication required", nil)
	}
	return nil
}
