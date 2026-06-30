package organizationuser

import (
	"log/slog"
	"time"
)

type organizationUser struct {
	ID string `json:"id"`

	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Password  string `json:"-"`

	OrganizationID string `json:"organizationId"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// LogValue implements slog.LogValuer so the password is redacted in logs.
func (u organizationUser) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("ID", u.ID),
		slog.String("firstName", u.FirstName),
		slog.String("lastName", u.LastName),
		slog.String("email", u.Email),
		slog.String("organizationID", u.OrganizationID),
		slog.Time("createdAt", u.CreatedAt),
		slog.Time("updatedAt", u.UpdatedAt),
	}
	if u.DeletedAt != nil {
		attrs = append(attrs, slog.Time("deletedAt", *u.DeletedAt))
	}
	return slog.GroupValue(attrs...)
}

type insertOrganizationUserRequest struct {
	OrganizationID string `json:"organizationId"`
	FirstName      string `json:"firstName"`
	LastName       string `json:"lastName"`
	Email          string `json:"email"`
	Password       string `json:"password"`
}
