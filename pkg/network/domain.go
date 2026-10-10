package network

import (
	"time"

	"github.com/lutia-io/huma/pkg/user"
)

type network struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`

	UserID    string   `json:"userId"`
	CreatedBy user.Ref `json:"createdBy"`
	UpdatedBy user.Ref `json:"updatedBy"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type insertNetworkRequest struct {
	Name   string `json:"name"`
	UserID string `json:"-"`
}

type patchNetworkRequest struct {
	Name *string `json:"name"`
}

type listParams struct {
	UserID    string
	NetworkID string
	Query     string
	Name      string
	NameOp    string
	Slug      string
	SlugOp    string
	Sort      string
	Order     string
	Page      int
	PageSize  int
}

type listResult struct {
	Items    []*network `json:"items"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}
