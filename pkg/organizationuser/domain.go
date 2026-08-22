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
	NetworkID      string `json:"networkId"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`

	// UserID is the owning network's user, used for authorization only.
	UserID string `json:"-"`
}

// LogValue implements slog.LogValuer so the password is redacted in logs.
func (u organizationUser) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("ID", u.ID),
		slog.String("firstName", u.FirstName),
		slog.String("lastName", u.LastName),
		slog.String("email", u.Email),
		slog.String("organizationID", u.OrganizationID),
		slog.String("networkID", u.NetworkID),
		slog.Time("createdAt", u.CreatedAt),
		slog.Time("updatedAt", u.UpdatedAt),
	}
	if u.DeletedAt != nil {
		attrs = append(attrs, slog.Time("deletedAt", *u.DeletedAt))
	}
	return slog.GroupValue(attrs...)
}

type insertOrganizationUserRequest struct {
	FirstName      string `json:"firstName"`
	LastName       string `json:"lastName"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	OrganizationID string `json:"organizationId"`
	NetworkID      string `json:"networkId"`
}

type patchOrganizationUserRequest struct {
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
	Email     *string `json:"email"`
	Password  *string `json:"password"`
}
