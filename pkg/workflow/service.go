package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/criteria"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/slug"
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

func (s *Service) Insert(ctx context.Context, req insertWorkflowDefinitionRequest) (string, error) {
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

	schemaID := strings.TrimSpace(req.SchemaID)
	if schemaID == "" {
		s.logger.WarnContext(ctx, "Empty schema ID")
		return "", apperror.NewBadRequestError("Schema ID is required", nil)
	}
	if !uuid.Valid(schemaID) {
		s.logger.WarnContext(ctx, "Invalid schema ID")
		return "", apperror.NewBadRequestError("Invalid schema ID", nil)
	}

	if err := validateDefinition(req.Definition); err != nil {
		s.logger.WarnContext(ctx, "Invalid definition", logger.KeyError, err)
		return "", err
	}

	wfd := &WorkflowDefinition{
		Name:       name,
		Slug:       slug,
		Active:     req.Active,
		Internal:   req.Internal,
		Definition: req.Definition,
		SchemaID:   schemaID,
		NetworkID:  networkID,
		UserID:     userID,
	}

	id, err := s.store.Insert(ctx, wfd)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && (appErr.Variant == apperror.ErrorVariantConflict || appErr.Variant == apperror.ErrorVariantBadRequest) {
			s.logger.WarnContext(ctx, "Rejected workflow definition insert", logger.KeySlug, slug, logger.KeyError, err)
			return "", err
		}
		s.logger.ErrorContext(ctx, "Failed to insert workflow definition", logger.KeySlug, slug, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created workflow definition", logger.KeyID, id)
	return id, nil
}

func (s *Service) Patch(ctx context.Context, existing *WorkflowDefinition, req patchWorkflowDefinitionRequest) error {
	if existing.Internal {
		return apperror.NewBadRequestError("Internal workflow definitions cannot be updated", nil)
	}

	if req.Name == nil && req.Active == nil && req.Definition == nil && req.SchemaID == nil {
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

	if req.SchemaID != nil {
		schemaID := strings.TrimSpace(*req.SchemaID)
		if schemaID == "" {
			s.logger.WarnContext(ctx, "Empty schema ID")
			return apperror.NewBadRequestError("Schema ID is required", nil)
		}
		if !uuid.Valid(schemaID) {
			s.logger.WarnContext(ctx, "Invalid schema ID")
			return apperror.NewBadRequestError("Invalid schema ID", nil)
		}
		existing.SchemaID = schemaID
	}

	if err := s.store.Update(ctx, existing); err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && (appErr.Variant == apperror.ErrorVariantConflict || appErr.Variant == apperror.ErrorVariantBadRequest) {
			s.logger.WarnContext(ctx, "Rejected workflow definition update", logger.KeyID, existing.ID, logger.KeyError, err)
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to update workflow definition", logger.KeyID, existing.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully updated workflow definition", logger.KeyID, existing.ID)
	return nil
}

func (s *Service) List(ctx context.Context, p principal.Principal) ([]*WorkflowDefinition, error) {
	switch p.Type {
	case principal.TypeUser:
		workflows, err := s.store.ListByUserID(ctx, p.ID)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to list workflow definitions", logger.KeyUserID, p.ID, logger.KeyError, err)
			return nil, err
		}
		return workflows, nil
	case principal.TypeOrganizationUser:
		if p.NetworkID == "" || p.OrganizationID == "" {
			return nil, apperror.NewUnauthorizedError("Organization user token missing network or organization", nil)
		}
		workflows, err := s.store.ListVisibleToOrganization(ctx, p.NetworkID, p.OrganizationID)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to list workflow definitions", logger.KeyError, err)
			return nil, err
		}
		return workflows, nil
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}
}

func (s *Service) Get(ctx context.Context, p principal.Principal, id string) (*WorkflowDefinition, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid workflow definition ID", nil)
	}

	wf, err := s.store.GetByID(ctx, id)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get workflow definition", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}

	switch p.Type {
	case principal.TypeUser:
		if wf.UserID != p.ID {
			return nil, apperror.NewNotFoundError("Workflow definition not found", nil)
		}
	case principal.TypeOrganizationUser:
		if wf.NetworkID != p.NetworkID {
			return nil, apperror.NewNotFoundError("Workflow definition not found", nil)
		}
		visible, err := s.store.SchemaVisibleToOrganization(ctx, wf.SchemaID, p.NetworkID, p.OrganizationID)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to authorize workflow definition", logger.KeyID, id, logger.KeyError, err)
			return nil, err
		}
		if !visible {
			return nil, apperror.NewNotFoundError("Workflow definition not found", nil)
		}
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	return wf, nil
}

func (s *Service) ListWorkflows(ctx context.Context, p principal.Principal) ([]*Workflow, error) {
	switch p.Type {
	case principal.TypeUser:
		workflows, err := s.store.ListWorkflowsByUserID(ctx, p.ID)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to list workflows", logger.KeyUserID, p.ID, logger.KeyError, err)
			return nil, err
		}
		return workflows, nil
	case principal.TypeOrganizationUser:
		if p.NetworkID == "" || p.OrganizationID == "" {
			return nil, apperror.NewUnauthorizedError("Organization user token missing network or organization", nil)
		}
		workflows, err := s.store.ListWorkflowsByOrganization(ctx, p.NetworkID, p.OrganizationID)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to list workflows", logger.KeyError, err)
			return nil, err
		}
		return workflows, nil
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}
}

func (s *Service) GetWorkflow(ctx context.Context, p principal.Principal, id string) (*Workflow, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid workflow ID", nil)
	}

	wf, err := s.store.GetWorkflowByID(ctx, id)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get workflow", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}

	if err := s.authorizeWorkflow(p, wf); err != nil {
		return nil, err
	}
	return wf, nil
}

func (s *Service) ListWorkflowActions(ctx context.Context, p principal.Principal, workflowID string) ([]*WorkflowAction, error) {
	if _, err := s.GetWorkflow(ctx, p, workflowID); err != nil {
		return nil, err
	}

	actions, err := s.store.ListWorkflowActionsByWorkflowID(ctx, workflowID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list workflow actions", logger.KeyID, workflowID, logger.KeyError, err)
		return nil, err
	}
	return actions, nil
}

func (s *Service) GetWorkflowAction(ctx context.Context, p principal.Principal, id string) (*WorkflowAction, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid workflow action ID", nil)
	}

	wfAction, err := s.store.GetWorkflowActionByID(ctx, id)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get workflow action", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}

	if _, err := s.GetWorkflow(ctx, p, wfAction.WorkflowID); err != nil {
		return nil, err
	}
	return wfAction, nil
}

func (s *Service) authorizeWorkflow(p principal.Principal, wf *Workflow) error {
	switch p.Type {
	case principal.TypeUser:
		if wf.UserID != p.ID {
			return apperror.NewNotFoundError("Workflow not found", nil)
		}
	case principal.TypeOrganizationUser:
		if wf.NetworkID != p.NetworkID || wf.OrganizationID != p.OrganizationID {
			return apperror.NewNotFoundError("Workflow not found", nil)
		}
	default:
		return apperror.NewUnauthorizedError("Authentication required", nil)
	}
	return nil
}

func validateDefinition(def Definition) error {
	if err := validateCriteria(def.Criteria); err != nil {
		return err
	}
	return validateActions(def.Actions)
}

func validateCriteria(c criteria.Criteria) error {
	if c.Logic != "" {
		switch c.Logic {
		case criteria.LogicAnd, criteria.LogicOr:
			if len(c.Conditions) == 0 {
				return apperror.NewBadRequestError("A condition group needs at least one condition", nil)
			}
		case criteria.LogicNot:
			if len(c.Conditions) != 1 {
				return apperror.NewBadRequestError("A none-of group needs exactly one nested condition", nil)
			}
		default:
			return apperror.NewBadRequestError("Unknown condition logic", nil)
		}
		for _, child := range c.Conditions {
			if err := validateCriteria(child); err != nil {
				return err
			}
		}
		return nil
	}

	if strings.TrimSpace(c.Field) == "" || c.Operator == "" {
		return apperror.NewBadRequestError("Each condition needs a field and an operator", nil)
	}
	switch c.Operator {
	case criteria.OpEq, criteria.OpNeq, criteria.OpGt, criteria.OpGte, criteria.OpLt, criteria.OpLte, criteria.OpIn:
		return nil
	default:
		return apperror.NewBadRequestError("Unknown condition operator", nil)
	}
}

func validateActions(actions []action.Action) error {
	if len(actions) == 0 {
		return apperror.NewBadRequestError("At least one action is required", nil)
	}
	for i, act := range actions {
		n := i + 1
		switch ctx := act.Context.(type) {
		case action.CreateRecordContext:
			if strings.TrimSpace(ctx.SchemaID) == "" {
				return apperror.NewBadRequestError(fmt.Sprintf("Action %d needs a schema to create a record", n), nil)
			}
		case action.UpdateRecordContext:
			if strings.TrimSpace(ctx.RecordID) == "" {
				return apperror.NewBadRequestError(fmt.Sprintf("Action %d needs a record to update", n), nil)
			}
		case action.UpsertRecordContext:
			if strings.TrimSpace(ctx.SchemaID) == "" {
				return apperror.NewBadRequestError(fmt.Sprintf("Action %d needs a schema to create or update a record", n), nil)
			}
		case action.TriggerPipelineContext:
			if strings.TrimSpace(ctx.Pipeline) == "" {
				return apperror.NewBadRequestError(fmt.Sprintf("Action %d needs a pipeline to run", n), nil)
			}
		default:
			return apperror.NewBadRequestError(fmt.Sprintf("Action %d has an invalid type", n), nil)
		}
	}
	return nil
}
