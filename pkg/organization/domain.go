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
