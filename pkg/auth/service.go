package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/hasher"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/uuid"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
	defaultIssuer   = "huma"
	defaultAudience = "huma"
)

type tokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int64  `json:"expiresIn"`
}

type loginUserRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	NetworkID string `json:"networkId"`
}

type loginOrganizationUserRequest struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	NetworkID      string `json:"networkId"`
	OrganizationID string `json:"organizationId"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type meResponse struct {
	PrincipalType  string `json:"principalType"`
	ID             string `json:"id"`
	NetworkID      string `json:"networkId"`
	OrganizationID string `json:"organizationId,omitempty"`
}

type service struct {
	logger      *logger.Logger
	store       store
	hasher      hasher.Hasher
	jwtSecret   []byte
	issuer      string
	audience    string
	rateLimiter *loginRateLimiter
}

func newService(log *logger.Logger, store store, h hasher.Hasher, jwtSecret []byte) *service {
	return &service{
		logger:      log,
		store:       store,
		hasher:      h,
		jwtSecret:   jwtSecret,
		issuer:      defaultIssuer,
		audience:    defaultAudience,
		rateLimiter: newLoginRateLimiter(10, time.Minute),
	}
}

func (s *service) LoginUser(ctx context.Context, req loginUserRequest, clientIP string) (*tokenPair, error) {
	email, password, err := s.validateCredentials(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}
	networkID := strings.TrimSpace(req.NetworkID)
	if networkID == "" || !uuid.Valid(networkID) {
		return nil, apperror.NewBadRequestError("Network ID is required", nil)
	}
	if !s.rateLimiter.Allow(clientIP + "|" + email) {
		s.logger.WarnContext(ctx, "Login rate limited", logger.KeyEmail, email)
		return nil, apperror.NewBadRequestError("Too many login attempts", nil)
	}

	exists, err := s.store.NetworkExists(ctx, networkID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to check network", logger.KeyError, err)
		return nil, apperror.NewInternalError("Failed to login", err)
	}
	if !exists {
		return nil, apperror.NewBadRequestError("Network not found", nil)
	}

	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, apperror.NewUnauthorizedError("Invalid email or password", err)
		}
		s.logger.ErrorContext(ctx, "Failed to load user", logger.KeyError, err)
		return nil, apperror.NewInternalError("Failed to login", err)
	}
	ok, err := s.hasher.Compare(password, user.Password)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to compare password", logger.KeyError, err)
		return nil, apperror.NewInternalError("Failed to login", err)
	}
	if !ok {
		return nil, apperror.NewUnauthorizedError("Invalid email or password", nil)
	}

	return s.issueTokens(ctx, principal.TypeUser, user.ID, networkID, "")
}

func (s *service) LoginOrganizationUser(ctx context.Context, req loginOrganizationUserRequest, clientIP string) (*tokenPair, error) {
	email, password, err := s.validateCredentials(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}
	networkID := strings.TrimSpace(req.NetworkID)
	organizationID := strings.TrimSpace(req.OrganizationID)
	if networkID == "" || !uuid.Valid(networkID) {
		return nil, apperror.NewBadRequestError("Network ID is required", nil)
	}
	if organizationID == "" || !uuid.Valid(organizationID) {
		return nil, apperror.NewBadRequestError("Organization ID is required", nil)
	}
	if !s.rateLimiter.Allow(clientIP + "|" + email) {
		s.logger.WarnContext(ctx, "Login rate limited", logger.KeyEmail, email)
		return nil, apperror.NewBadRequestError("Too many login attempts", nil)
	}

	org, err := s.store.GetOrganizationByID(ctx, organizationID)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, apperror.NewBadRequestError("Organization not found", err)
		}
		s.logger.ErrorContext(ctx, "Failed to load organization", logger.KeyError, err)
		return nil, apperror.NewInternalError("Failed to login", err)
	}
	if org.NetworkID != networkID {
		return nil, apperror.NewBadRequestError("Organization does not belong to network", nil)
	}

	orgUser, err := s.store.GetOrganizationUserByEmail(ctx, email, networkID, organizationID)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantNotFound {
			return nil, apperror.NewUnauthorizedError("Invalid email or password", err)
		}
		s.logger.ErrorContext(ctx, "Failed to load organization user", logger.KeyError, err)
		return nil, apperror.NewInternalError("Failed to login", err)
	}
	ok, err := s.hasher.Compare(password, orgUser.Password)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to compare password", logger.KeyError, err)
		return nil, apperror.NewInternalError("Failed to login", err)
	}
	if !ok {
		return nil, apperror.NewUnauthorizedError("Invalid email or password", nil)
	}

	return s.issueTokens(ctx, principal.TypeOrganizationUser, orgUser.ID, networkID, organizationID)
}

func (s *service) Refresh(ctx context.Context, refreshToken string) (*tokenPair, error) {
	return s.refreshAndRotate(ctx, refreshToken)
}

func (s *service) Logout(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return apperror.NewBadRequestError("Refresh token is required", nil)
	}
	row, err := s.store.GetTokenByHash(ctx, hashRefreshToken(refreshToken))
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Variant == apperror.ErrorVariantUnauthorized {
			return nil
		}
		return err
	}
	return s.store.RevokeFamily(ctx, row.FamilyID, time.Now().UTC())
}

func (s *service) Me(p principal.Principal) meResponse {
	return meResponse{
		PrincipalType:  string(p.Type),
		ID:             p.ID,
		NetworkID:      p.NetworkID,
		OrganizationID: p.OrganizationID,
	}
}

func (s *service) ParseAccessToken(token string) (principal.Principal, error) {
	claims, err := parseAccessToken(s.jwtSecret, token, time.Now().UTC(), s.issuer, s.audience)
	if err != nil {
		return principal.Principal{}, apperror.NewUnauthorizedError("Invalid access token", err)
	}
	p := principal.Principal{
		Type:           principal.Type(claims.PrincipalType),
		ID:             claims.Subject,
		NetworkID:      claims.NetworkID,
		OrganizationID: claims.OrganizationID,
	}
	if p.Type != principal.TypeUser && p.Type != principal.TypeOrganizationUser {
		return principal.Principal{}, apperror.NewUnauthorizedError("Invalid access token", nil)
	}
	if p.Type == principal.TypeOrganizationUser && p.OrganizationID == "" {
		return principal.Principal{}, apperror.NewUnauthorizedError("Invalid access token", nil)
	}
	return p, nil
}

func (s *service) validateCredentials(ctx context.Context, email, password string) (string, string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", "", apperror.NewBadRequestError("Email is required", nil)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		s.logger.WarnContext(ctx, "Invalid email", logger.KeyEmail, email, logger.KeyError, err)
		return "", "", apperror.NewBadRequestError("Email is invalid", err)
	}
	if password == "" {
		return "", "", apperror.NewBadRequestError("Password is required", nil)
	}
	return email, password, nil
}

func (s *service) issueTokens(ctx context.Context, pty principal.Type, principalID, networkID, organizationID string) (*tokenPair, error) {
	familyID, err := uuid.New()
	if err != nil {
		return nil, apperror.NewInternalError("Failed to create token", err)
	}
	return s.issueTokensInFamily(ctx, pty, principalID, networkID, organizationID, familyID)
}

func (s *service) issueTokensInFamily(ctx context.Context, pty principal.Type, principalID, networkID, organizationID, familyID string) (*tokenPair, error) {
	now := time.Now().UTC()
	jti, err := uuid.New()
	if err != nil {
		return nil, apperror.NewInternalError("Failed to create token", err)
	}
	claims := accessClaims{
		Issuer:         s.issuer,
		Audience:       s.audience,
		Subject:        principalID,
		PrincipalType:  string(pty),
		NetworkID:      networkID,
		OrganizationID: organizationID,
		JWTID:          jti,
		IssuedAt:       now.Unix(),
		ExpiresAt:      now.Add(accessTokenTTL).Unix(),
	}
	access, err := signAccessToken(s.jwtSecret, claims)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to sign access token", logger.KeyError, err)
		return nil, apperror.NewInternalError("Failed to create token", err)
	}

	rawRefresh, err := newRefreshToken()
	if err != nil {
		return nil, apperror.NewInternalError("Failed to create token", err)
	}
	tokenID, err := newTokenID()
	if err != nil {
		return nil, apperror.NewInternalError("Failed to create token", err)
	}
	var orgPtr *string
	if organizationID != "" {
		orgPtr = &organizationID
	}
	row := &tokenRow{
		ID:             tokenID,
		TokenHash:      hashRefreshToken(rawRefresh),
		FamilyID:       familyID,
		PrincipalType:  string(pty),
		PrincipalID:    principalID,
		NetworkID:      networkID,
		OrganizationID: orgPtr,
		ExpiresAt:      now.Add(refreshTokenTTL),
		CreatedAt:      now,
	}
	if err := s.store.InsertToken(ctx, row); err != nil {
		s.logger.ErrorContext(ctx, "Failed to store refresh token", logger.KeyError, err)
		return nil, apperror.NewInternalError("Failed to create token", err)
	}

	return &tokenPair{
		AccessToken:  access,
		RefreshToken: rawRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
	}, nil
}

// refreshAndRotate loads the refresh token, detects reuse, issues a new pair in
// the same family, and marks the old token as replaced.
func (s *service) refreshAndRotate(ctx context.Context, refreshToken string) (*tokenPair, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, apperror.NewBadRequestError("Refresh token is required", nil)
	}
	now := time.Now().UTC()
	row, err := s.store.GetTokenByHash(ctx, hashRefreshToken(refreshToken))
	if err != nil {
		return nil, err
	}
	if row.RevokedAt != nil || row.ReplacedBy != nil {
		_ = s.store.RevokeFamily(ctx, row.FamilyID, now)
		s.logger.WarnContext(ctx, "Refresh token reuse detected", "family_id", row.FamilyID)
		return nil, apperror.NewUnauthorizedError("Invalid refresh token", nil)
	}
	if !row.ExpiresAt.After(now) {
		_ = s.store.RevokeFamily(ctx, row.FamilyID, now)
		return nil, apperror.NewUnauthorizedError("Refresh token expired", nil)
	}

	orgID := ""
	if row.OrganizationID != nil {
		orgID = *row.OrganizationID
	}

	// Issue new token first, then mark old replaced with new id.
	rawRefresh, err := newRefreshToken()
	if err != nil {
		return nil, apperror.NewInternalError("Failed to create token", err)
	}
	newID, err := newTokenID()
	if err != nil {
		return nil, apperror.NewInternalError("Failed to create token", err)
	}
	var orgPtr *string
	if orgID != "" {
		orgPtr = &orgID
	}
	newRow := &tokenRow{
		ID:             newID,
		TokenHash:      hashRefreshToken(rawRefresh),
		FamilyID:       row.FamilyID,
		PrincipalType:  row.PrincipalType,
		PrincipalID:    row.PrincipalID,
		NetworkID:      row.NetworkID,
		OrganizationID: orgPtr,
		ExpiresAt:      now.Add(refreshTokenTTL),
		CreatedAt:      now,
	}
	if err := s.store.InsertToken(ctx, newRow); err != nil {
		s.logger.ErrorContext(ctx, "Failed to store refresh token", logger.KeyError, err)
		return nil, apperror.NewInternalError("Failed to create token", err)
	}
	if err := s.store.MarkReplaced(ctx, row.ID, newID, now); err != nil {
		s.logger.ErrorContext(ctx, "Failed to rotate refresh token", logger.KeyError, err)
		return nil, apperror.NewInternalError("Failed to create token", err)
	}

	jti, err := uuid.New()
	if err != nil {
		return nil, apperror.NewInternalError("Failed to create token", err)
	}
	access, err := signAccessToken(s.jwtSecret, accessClaims{
		Issuer:         s.issuer,
		Audience:       s.audience,
		Subject:        row.PrincipalID,
		PrincipalType:  row.PrincipalType,
		NetworkID:      row.NetworkID,
		OrganizationID: orgID,
		JWTID:          jti,
		IssuedAt:       now.Unix(),
		ExpiresAt:      now.Add(accessTokenTTL).Unix(),
	})
	if err != nil {
		return nil, apperror.NewInternalError("Failed to create token", err)
	}

	return &tokenPair{
		AccessToken:  access,
		RefreshToken: rawRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
	}, nil
}
