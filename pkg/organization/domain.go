package organization

import (
	"time"
)

type organization struct {
	ID string `json:"id"`

	Name string `json:"name"`
	Slug string `json:"slug"`

	NetworkID string `json:"networkId"`
	UserID    string `json:"userId"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type insertOrganizationRequest struct {
	Name      string `json:"name"`
	NetworkID string `json:"networkId"`
	UserID    string `json:"-"`
}

type patchOrganizationRequest struct {
	Name *string `json:"name"`
}

type listParams struct {
	UserID         string
	Query          string
	NetworkID      string
	OrganizationID string
	Name           string
	NameOp         string
	Slug           string
	SlugOp         string
	Network        string
	NetworkOp      string
	Sort           string
	Order          string
	Page           int
	PageSize       int
}

type listResult struct {
	Items    []*organization `json:"items"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
}
