package network

import "time"

type network struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`

	UserID string `json:"userId"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type insertNetworkRequest struct {
	Name   string `json:"name"`
	UserID string `json:"userId"`
}
