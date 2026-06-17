package network

import "time"

type network struct {
	ID   string `json:"id" bson:"_id,omitempty"`
	Name string `json:"name" bson:"name"`
	Slug string `json:"slug" bson:"slug"`

	UserID string `json:"userId" bson:"user_id"`

	CreatedAt time.Time  `json:"createdAt" bson:"created_at"`
	UpdatedAt time.Time  `json:"updatedAt" bson:"updated_at"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" bson:"deleted_at,omitempty"`
}

type insertNetworkRequest struct {
	Name   string `json:"name"`
	UserID string `json:"userId"`
}

type updateNetworkRequest struct {
	Name string `json:"name"`
}
