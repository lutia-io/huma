package user

import (
	"log/slog"
	"time"
)

type user struct {
	ID string `json:"id"`

	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Password  string `json:"-"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// LogValue implements slog.LogValuer so the password is redacted in logs.
func (u user) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("ID", u.ID),
		slog.String("firstName", u.FirstName),
		slog.String("lastName", u.LastName),
		slog.String("email", u.Email),
		slog.Time("createdAt", u.CreatedAt),
		slog.Time("updatedAt", u.UpdatedAt),
	}
	if u.DeletedAt != nil {
		attrs = append(attrs, slog.Time("deletedAt", *u.DeletedAt))
	}
	return slog.GroupValue(attrs...)
}

type insertUserRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}
