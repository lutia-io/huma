package schema

import (
	"encoding/json"
	"time"

	"github.com/lutia-io/huma/pkg/user"
)

type schema struct {
	ID string `json:"id"`

	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Internal bool   `json:"internal"`

	Definition json.RawMessage `json:"definition"`

	NetworkID      string   `json:"networkId"`
	OrganizationID *string  `json:"organizationId,omitempty"`
	UserID         string   `json:"userId"`
	CreatedBy      user.Ref `json:"createdBy"`
	UpdatedBy      user.Ref `json:"updatedBy"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type insertSchemaRequest struct {
	Name           string          `json:"name"`
	Internal       bool            `json:"internal"`
	Definition     json.RawMessage `json:"definition"`
	NetworkID      string          `json:"networkId"`
	OrganizationID string          `json:"organizationId"`
	UserID         string          `json:"-"`
}

type patchSchemaRequest struct {
	Name       *string         `json:"name"`
	Definition json.RawMessage `json:"definition"`
}

type listParams struct {
	UserID         string
	Query          string
	NetworkID      string
	OrganizationID string
	Scope          string
	Name           string
	NameOp         string
	Slug           string
	SlugOp         string
	Properties     *int
	PropertiesOp   string
	Sort           string
	Order          string
	Page           int
	PageSize       int
}

type listResult struct {
	Items    []*schema `json:"items"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
}
