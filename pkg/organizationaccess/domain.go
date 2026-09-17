package organizationaccess

import "time"

type group struct {
	ID            string    `json:"id"`
	OrganizationID string   `json:"organizationId"`
	NetworkID     string    `json:"networkId"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Description   string    `json:"description"`
	System        bool      `json:"system"`
	Members       []member  `json:"members"`
	PermissionIDs []string  `json:"permissionIds"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type member struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}

type permission struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	NetworkID      string    `json:"networkId"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	Description    string    `json:"description"`
	System         bool      `json:"system"`
	Grants         []grant   `json:"grants"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type fieldGrant struct {
	Name   string `json:"name"`
	Access string `json:"access"`
}

type grant struct {
	Resource   string       `json:"resource"`
	Actions    []string     `json:"actions"`
	ResourceID string       `json:"resourceId,omitempty"`
	SchemaID   string       `json:"schemaId,omitempty"`
	Fields     []fieldGrant `json:"fields,omitempty"`
}

type insertGroupRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	OrganizationID string `json:"organizationId"`
	NetworkID      string `json:"networkId"`
}

type patchGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type insertPermissionRequest struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	OrganizationID string  `json:"organizationId"`
	NetworkID      string  `json:"networkId"`
	Grants         []grant `json:"grants"`
}

type patchPermissionRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Grants      []grant `json:"grants"`
}

type addMemberRequest struct {
	OrganizationUserID string `json:"organizationUserId"`
}

type assignPermissionRequest struct {
	PermissionID string `json:"permissionId"`
}
