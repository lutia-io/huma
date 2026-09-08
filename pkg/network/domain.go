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

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type insertNetworkRequest struct {
	Name   string `json:"name"`
	UserID string `json:"-"`
}

type patchNetworkRequest struct {
	Name *string `json:"name"`
}
