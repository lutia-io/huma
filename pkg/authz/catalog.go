package authz

import "github.com/lutia-io/huma/pkg/apperror"

const (
	ActionCreate       = "create"
	ActionRead         = "read"
	ActionUpdate       = "update"
	ActionDelete       = "delete"
	ActionManageAccess = "manage_access"

	ResourceNetwork             = "network"
	ResourceOrganization        = "organization"
	ResourceOrganizationUser    = "organization_user"
	ResourceSchema              = "schema"
	ResourceWorkflowDefinition  = "workflow_definition"
	ResourcePipelineDefinition  = "pipeline_definition"
	ResourceRecord              = "record"
	ResourceFile                = "file"

	FieldRead  = "read"
	FieldWrite = "write"

	SlugOwners   = "owners"
	SlugMembers  = "members"
	SlugEveryone = "everyone"
	SlugAdmins   = "admins"
)

var networkActions = map[string]map[string]struct{}{
	ResourceNetwork: {
		ActionRead: {}, ActionUpdate: {}, ActionDelete: {}, ActionManageAccess: {},
	},
	ResourceOrganization: {
		ActionCreate: {}, ActionRead: {}, ActionUpdate: {}, ActionDelete: {},
	},
	ResourceOrganizationUser: {
		ActionCreate: {}, ActionRead: {}, ActionUpdate: {}, ActionDelete: {},
	},
	ResourceSchema: {
		ActionCreate: {}, ActionRead: {}, ActionUpdate: {}, ActionDelete: {},
	},
	ResourceWorkflowDefinition: {
		ActionCreate: {}, ActionRead: {}, ActionUpdate: {}, ActionDelete: {},
	},
	ResourcePipelineDefinition: {
		ActionCreate: {}, ActionRead: {}, ActionUpdate: {}, ActionDelete: {},
	},
}

var organizationActions = map[string]map[string]struct{}{
	ResourceRecord: {
		ActionCreate: {}, ActionRead: {}, ActionUpdate: {}, ActionDelete: {},
	},
	ResourceFile: {
		ActionCreate: {}, ActionRead: {}, ActionDelete: {},
	},
	ResourceOrganizationUser: {
		ActionCreate: {}, ActionRead: {}, ActionUpdate: {},
	},
}

func allNetworkActions() []Grant {
	grants := make([]Grant, 0, len(networkActions))
	for resource, actions := range networkActions {
		g := Grant{Resource: resource}
		for action := range actions {
			g.Actions = append(g.Actions, action)
		}
		grants = append(grants, g)
	}
	return grants
}

func readNetworkActions() []Grant {
	grants := make([]Grant, 0, len(networkActions))
	for resource := range networkActions {
		grants = append(grants, Grant{Resource: resource, Actions: []string{ActionRead}})
	}
	return grants
}

func ValidateNetworkGrant(resource string, actions []string) error {
	allowed, ok := networkActions[resource]
	if !ok {
		return apperror.NewBadRequestError("Invalid network resource", nil)
	}
	if len(actions) == 0 {
		return apperror.NewBadRequestError("Actions are required", nil)
	}
	for _, action := range actions {
		if _, ok := allowed[action]; !ok {
			return apperror.NewBadRequestError("Invalid network action", nil)
		}
	}
	return nil
}

func ValidateOrganizationGrant(resource string, actions []string) error {
	allowed, ok := organizationActions[resource]
	if !ok {
		return apperror.NewBadRequestError("Invalid organization resource", nil)
	}
	if len(actions) == 0 {
		return apperror.NewBadRequestError("Actions are required", nil)
	}
	for _, action := range actions {
		if _, ok := allowed[action]; !ok {
			return apperror.NewBadRequestError("Invalid organization action", nil)
		}
	}
	return nil
}

func definitionResource(resource string) bool {
	return resource == ResourceSchema || resource == ResourceWorkflowDefinition || resource == ResourcePipelineDefinition
}

func PointerID(id *string) string {
	if id == nil {
		return ""
	}
	return *id
}
