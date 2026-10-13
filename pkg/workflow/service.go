package workflow

import (
	"context"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/authz"
	"github.com/lutia-io/huma/pkg/criteria"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/slug"
	"github.com/lutia-io/huma/pkg/user"
	"github.com/lutia-io/huma/pkg/uuid"
)

type Service struct {
	logger     *logger.Logger
	store      store
	authz      *authz.Engine
}

func NewService(logger *logger.Logger, store store, engine *authz.Engine) *Service {
	return &Service{
		logger: logger,
		store:  store,
		authz:  engine,
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
	if req.Definition.Trigger.HasAutoIntake() {
		if schemaID == "" {
			s.logger.WarnContext(ctx, "Empty schema ID")
			return "", apperror.NewBadRequestError("Schema ID is required for record or schedule triggers", nil)
		}
		if !uuid.Valid(schemaID) {
			s.logger.WarnContext(ctx, "Invalid schema ID")
			return "", apperror.NewBadRequestError("Invalid schema ID", nil)
		}
	} else if schemaID != "" && !uuid.Valid(schemaID) {
		s.logger.WarnContext(ctx, "Invalid schema ID")
		return "", apperror.NewBadRequestError("Invalid schema ID", nil)
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

	wfd := &WorkflowDefinition{
		Name:           name,
		Slug:           slug,
		Description:    strings.TrimSpace(req.Description),
		Active:         req.Active,
		Internal:       req.Internal,
		Definition:     req.Definition,
		SchemaID:       schemaID,
		NetworkID:      networkID,
		OrganizationID: organizationID,
		UserID:         userID,
		CreatedBy:      user.Ref{ID: userID},
		UpdatedBy:      user.Ref{ID: userID},
	}

	id, err := s.store.Insert(ctx, wfd)
	if err != nil {
		if apperror.IsConflict(err) || apperror.IsBadRequest(err) {
			s.logger.WarnContext(ctx, "Rejected workflow definition insert", logger.KeySlug, slug, logger.KeyError, err)
			return "", err
		}
		s.logger.ErrorContext(ctx, "Failed to insert workflow definition", logger.KeySlug, slug, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Successfully created workflow definition", logger.KeyID, id)
	return id, nil
}

func (s *Service) Patch(ctx context.Context, existing *WorkflowDefinition, req patchWorkflowDefinitionRequest, updatedBy string) error {
	if existing.Internal {
		return apperror.NewBadRequestError("Internal workflow definitions cannot be updated", nil)
	}

	if req.Name == nil && req.Description == nil && req.Active == nil && req.Definition == nil && req.SchemaID == nil {
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

	if req.SchemaID != nil {
		schemaID := strings.TrimSpace(*req.SchemaID)
		def := existing.Definition
		if req.Definition != nil {
			def = *req.Definition
		}
		if def.Trigger.HasAutoIntake() && schemaID == "" {
			s.logger.WarnContext(ctx, "Empty schema ID")
			return apperror.NewBadRequestError("Schema ID is required for record or schedule triggers", nil)
		}
		if schemaID != "" && !uuid.Valid(schemaID) {
			s.logger.WarnContext(ctx, "Invalid schema ID")
			return apperror.NewBadRequestError("Invalid schema ID", nil)
		}
		existing.SchemaID = schemaID
	}

	existing.UpdatedBy.ID = updatedBy

	if err := s.store.Update(ctx, existing); err != nil {
		if apperror.IsConflict(err) || apperror.IsBadRequest(err) {
			s.logger.WarnContext(ctx, "Rejected workflow definition update", logger.KeyID, existing.ID, logger.KeyError, err)
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to update workflow definition", logger.KeyID, existing.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully updated workflow definition", logger.KeyID, existing.ID)
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
		s.logger.ErrorContext(ctx, "Failed to list workflow definitions", logger.KeyUserID, p.ID, logger.KeyError, err)
		return nil, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, p principal.Principal, id string) (*WorkflowDefinition, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid workflow definition ID", nil)
	}

	wf, err := s.store.GetByID(ctx, id)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get workflow definition", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}

	switch p.Type {
	case principal.TypeUser:
		if s.authz == nil {
			if wf.UserID != p.ID {
				return nil, apperror.NewNotFoundError("Workflow definition not found", nil)
			}
			break
		}
		if err := s.authz.VisibleFor(ctx, p, wf.NetworkID, apperror.NewNotFoundError("Workflow definition not found", nil)); err != nil {
			return nil, err
		}
	case principal.TypeOrganizationUser:
		if wf.NetworkID != p.NetworkID {
			return nil, apperror.NewNotFoundError("Workflow definition not found", nil)
		}
		if !visibleToOrganization(wf.NetworkID, wf.OrganizationID, p.NetworkID, p.OrganizationID) {
			return nil, apperror.NewNotFoundError("Workflow definition not found", nil)
		}
	default:
		return nil, apperror.NewUnauthorizedError("Authentication required", nil)
	}

	return wf, nil
}

func (s *Service) ListWorkflows(ctx context.Context, p principal.Principal, params runListParams) (*runListResult, error) {
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

	result, err := s.store.ListWorkflows(ctx, params)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list workflows", logger.KeyUserID, p.ID, logger.KeyError, err)
		return nil, err
	}
	return result, nil
}

func (s *Service) GetWorkflow(ctx context.Context, p principal.Principal, id string) (*Workflow, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid workflow ID", nil)
	}

	wf, err := s.store.GetWorkflowByID(ctx, id)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get workflow", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}

	if err := s.authorizeWorkflow(ctx, p, wf); err != nil {
		return nil, err
	}
	return wf, nil
}

func (s *Service) loadDefinitionForEnqueue(ctx context.Context, networkID, organizationID, definitionID, slug string) (*WorkflowDefinition, error) {
	var (
		def *WorkflowDefinition
		err error
	)
	switch {
	case strings.TrimSpace(definitionID) != "":
		if !uuid.Valid(definitionID) {
			return nil, apperror.NewBadRequestError("Invalid workflow definition ID", nil)
		}
		def, err = s.store.GetByID(ctx, definitionID)
	case strings.TrimSpace(slug) != "":
		def, err = s.store.GetBySlug(ctx, networkID, slug, organizationID)
	default:
		return nil, apperror.NewBadRequestError("Workflow definition is required", nil)
	}
	if err != nil {
		return nil, err
	}
	if def.NetworkID != networkID {
		return nil, apperror.NewNotFoundError("Workflow definition not found", nil)
	}
	if !def.MatchesOrganization(organizationID) {
		return nil, apperror.NewNotFoundError("Workflow definition not found", nil)
	}
	if !def.Active {
		return nil, apperror.NewBadRequestError("Workflow definition is not active", nil)
	}
	if err := validateDefinition(def.Definition); err != nil {
		return nil, err
	}
	return def, nil
}

func (s *Service) Enqueue(ctx context.Context, req EnqueueRequest) (string, error) {
	def, err := s.loadDefinitionForEnqueue(ctx, req.NetworkID, req.OrganizationID, req.WorkflowDefinitionID, req.WorkflowSlug)
	if err != nil {
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

	run := &Workflow{
		WorkflowDefinitionID: def.ID,
		NetworkID:            def.NetworkID,
		OrganizationID:       req.OrganizationID,
		OrganizationUserID:   req.OrganizationUserID,
		DedupeKey:            dedupeKey,
		Input:                input,
		Data:                 map[string]any{},
		Definition:           SnapshotRunDefinition(def.Definition),
	}
	id, err := s.store.InsertPending(ctx, run)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to enqueue workflow", logger.KeyID, def.ID, logger.KeyError, err)
		return "", err
	}
	s.logger.InfoContext(ctx, "Enqueued workflow", logger.KeyID, id, "workflow_definition_id", def.ID)
	return id, nil
}

func (s *Service) ListWorkflowNodes(ctx context.Context, p principal.Principal, workflowID string) ([]*WorkflowNode, error) {
	if _, err := s.GetWorkflow(ctx, p, workflowID); err != nil {
		return nil, err
	}

	nodes, err := s.store.ListWorkflowNodesByWorkflowID(ctx, workflowID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list workflow nodes", logger.KeyID, workflowID, logger.KeyError, err)
		return nil, err
	}
	return nodes, nil
}

// Retry reopens a failed workflow for another execution. Trigger data and input
// stay the snapshot from intake. The node graph is replaced with the live
// definition when it still exists. Workers skip nodes that already completed.
func (s *Service) Retry(ctx context.Context, p principal.Principal, id string) error {
	wf, err := s.GetWorkflow(ctx, p, id)
	if err != nil {
		return err
	}
	if wf.Status != "failed" {
		return apperror.NewBadRequestError("Only failed workflows can be retried", nil)
	}

	definition := wf.Definition
	live, err := s.store.GetByID(ctx, wf.WorkflowDefinitionID)
	if err != nil {
		if !apperror.IsNotFound(err) {
			s.logger.ErrorContext(ctx, "Failed to load workflow definition for retry", logger.KeyID, wf.WorkflowDefinitionID, logger.KeyError, err)
			return err
		}
	} else {
		definition = SnapshotRunDefinition(live.Definition)
	}

	if err := s.store.RetryFailed(ctx, wf.ID, definition); err != nil {
		if apperror.IsBadRequest(err) {
			s.logger.WarnContext(ctx, "Rejected workflow retry", logger.KeyID, wf.ID, logger.KeyError, err)
			return err
		}
		s.logger.ErrorContext(ctx, "Failed to retry workflow", logger.KeyID, wf.ID, logger.KeyError, err)
		return err
	}
	s.logger.InfoContext(ctx, "Successfully retried workflow", logger.KeyID, wf.ID)
	return nil
}

func (s *Service) GetWorkflowNode(ctx context.Context, p principal.Principal, id string) (*WorkflowNode, error) {
	if !uuid.Valid(id) {
		return nil, apperror.NewBadRequestError("Invalid workflow node ID", nil)
	}

	node, err := s.store.GetWorkflowNodeByID(ctx, id)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "Failed to get workflow node", logger.KeyID, id, logger.KeyError, err)
		return nil, err
	}

	if _, err := s.GetWorkflow(ctx, p, node.WorkflowID); err != nil {
		return nil, err
	}
	return node, nil
}

func (s *Service) authorizeWorkflow(ctx context.Context, p principal.Principal, wf *Workflow) error {
	switch p.Type {
	case principal.TypeUser:
		if s.authz == nil {
			if wf.UserID != p.ID {
				return apperror.NewNotFoundError("Workflow not found", nil)
			}
			return nil
		}
		return s.authz.VisibleFor(ctx, p, wf.NetworkID, apperror.NewNotFoundError("Workflow not found", nil))
	case principal.TypeOrganizationUser:
		if wf.NetworkID != p.NetworkID || wf.OrganizationID != p.OrganizationID {
			return apperror.NewNotFoundError("Workflow not found", nil)
		}
	default:
		return apperror.NewUnauthorizedError("Authentication required", nil)
	}
	return nil
}

// ValidateDefinition checks trigger, criteria, and node levels.
func ValidateDefinition(def Definition) error {
	return validateDefinition(def)
}

func validateDefinition(def Definition) error {
	if err := validateTrigger(def.Trigger); err != nil {
		return err
	}
	if err := validateCriteria(def.Criteria); err != nil {
		return err
	}
	return validateNodes(def.Nodes)
}

func validateTrigger(t Trigger) error {
	var hasCreated, hasUpdated, hasSchedule bool
	for _, on := range t.Events() {
		switch on {
		case TriggerOnCreated:
			hasCreated = true
		case TriggerOnUpdated:
			hasUpdated = true
		case TriggerOnSchedule:
			hasSchedule = true
		default:
			return apperror.NewBadRequestError("Unknown trigger event", nil)
		}
	}
	if hasSchedule && (hasCreated || hasUpdated) {
		return apperror.NewBadRequestError("A schedule trigger cannot also run on create or update", nil)
	}

	cron := strings.TrimSpace(t.Cron)
	timezone := strings.TrimSpace(t.Timezone)
	if hasSchedule {
		if cron == "" {
			return apperror.NewBadRequestError("A schedule trigger needs a cron expression", nil)
		}
		if timezone == "" {
			return apperror.NewBadRequestError("A schedule trigger needs a timezone", nil)
		}
		if _, _, err := ParseSchedule(cron, timezone); err != nil {
			return apperror.NewBadRequestError(err.Error(), err)
		}
	} else {
		if cron != "" || timezone != "" {
			return apperror.NewBadRequestError("Cron and timezone are only valid on a schedule trigger", nil)
		}
	}

	if len(t.Changed) > 0 && !hasUpdated {
		return apperror.NewBadRequestError("Changed fields are only valid when the trigger includes update", nil)
	}
	for _, field := range t.Changed {
		if strings.TrimSpace(field) == "" {
			return apperror.NewBadRequestError("Changed fields cannot be empty", nil)
		}
	}
	return nil
}

func validateCriteria(c criteria.Criteria) error {
	if c.Logic == "" && strings.TrimSpace(c.Field) == "" && c.Operator == "" && len(c.Conditions) == 0 {
		return nil
	}
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

