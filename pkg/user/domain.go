package user

import (
	"time"
)

type user struct {
	ID string `json:"id" bson:"_id,omitempty"`

	FirstName string `json:"firstName" bson:"first_name"`
	LastName  string `json:"lastName" bson:"last_name"`
	Email     string `json:"email" bson:"email"`
	Password  string `json:"-" bson:"password"`

	CreatedAt time.Time  `json:"createdAt" bson:"created_at"`
	UpdatedAt time.Time  `json:"updatedAt" bson:"updated_at"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" bson:"deleted_at,omitempty"`
}

type insertUserRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

type updateUserRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}
