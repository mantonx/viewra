package auth

import (
	"context"
	"time"

	"github.com/mantonx/viewra/internal/domain/user"
	"github.com/mantonx/viewra/internal/infrastructure/auth"
	"go.jetify.com/typeid"
)

// Service handles authentication operations.
type Service struct {
	userRepo      user.UserRepository
	sessionRepo   user.SessionRepository
	passwordHash  *auth.PasswordHasher
	tokenService  *auth.TokenService
	maxSessions   int
}

// NewService creates a new auth service.
func NewService(
	userRepo user.UserRepository,
	sessionRepo user.SessionRepository,
	passwordHash *auth.PasswordHasher,
	tokenService *auth.TokenService,
	maxSessions int,
) *Service {
	return &Service{
		userRepo:     userRepo,
		sessionRepo:  sessionRepo,
		passwordHash: passwordHash,
		tokenService: tokenService,
		maxSessions:  maxSessions,
	}
}

// LoginRequest represents a login request.
type LoginRequest struct {
	Username  string
	Password  string
	UserAgent string
	IPAddress string
}

// LoginResponse contains the tokens returned after successful authentication.
type LoginResponse struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // seconds until access token expires
	User         *UserResponse
}

// UserResponse is a safe user representation (no password hash).
type UserResponse struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	IsAdmin     bool      `json:"is_admin"`
	CreatedAt   time.Time `json:"created_at"`
}

// Login authenticates a user and returns tokens.
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// Find user by username
	u, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, user.ErrInvalidCredentials
	}

	// Check if user can authenticate
	if !u.CanAuthenticate() {
		return nil, user.ErrUserDisabled
	}

	// Verify password
	if err := s.passwordHash.Verify(req.Password, u.PasswordHash); err != nil {
		return nil, user.ErrInvalidCredentials
	}

	// Generate tokens
	accessToken, err := s.tokenService.GenerateAccessToken(u.ID, u.PublicID, u.IsAdmin)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.tokenService.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	// Create session
	session := user.NewSession(
		generateID("sess"),
		u.ID, // Internal ID for FK
		s.tokenService.HashRefreshToken(refreshToken),
		req.UserAgent,
		req.IPAddress,
		time.Now().Add(s.tokenService.GetRefreshTokenTTL()),
	)

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, err
	}

	// Enforce max sessions (delete oldest if exceeded)
	if err := s.enforceMaxSessions(ctx, u.ID); err != nil {
		// Log but don't fail the login
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.tokenService.GetAccessTokenTTL().Seconds()),
		User:         toUserResponse(u),
	}, nil
}

// RegisterRequest represents a user registration request.
type RegisterRequest struct {
	Username    string
	DisplayName string
	Password    string
	IsAdmin     bool
}

// Register creates a new user.
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*UserResponse, error) {
	// Validate request
	regReq := &user.RegistrationRequest{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Password:    req.Password,
	}
	if err := regReq.Validate(); err != nil {
		return nil, err
	}

	// Hash password
	hash, err := s.passwordHash.Hash(req.Password)
	if err != nil {
		return nil, err
	}

	// Create user
	u := user.NewUser(
		generateID("usr"),
		req.Username,
		req.DisplayName,
		hash,
		req.IsAdmin,
	)

	if err := s.userRepo.Create(ctx, u); err != nil {
		return nil, err
	}

	return toUserResponse(u), nil
}

// RefreshRequest represents a token refresh request.
type RefreshRequest struct {
	RefreshToken string
	UserAgent    string
	IPAddress    string
}

// Refresh exchanges a refresh token for new tokens.
func (s *Service) Refresh(ctx context.Context, req *RefreshRequest) (*LoginResponse, error) {
	// Find session by token hash
	tokenHash := s.tokenService.HashRefreshToken(req.RefreshToken)
	session, err := s.sessionRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, user.ErrRefreshTokenInvalid
	}

	// Check if session is expired
	if session.IsExpired() {
		// Delete the expired session
		_ = s.sessionRepo.Delete(ctx, session.ID)
		return nil, user.ErrRefreshTokenInvalid
	}

	// Get user by internal ID
	u, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		return nil, user.ErrRefreshTokenInvalid
	}

	// Check if user can still authenticate
	if !u.CanAuthenticate() {
		return nil, user.ErrUserDisabled
	}

	// Update session last used
	if err := s.sessionRepo.UpdateLastUsed(ctx, session.ID); err != nil {
		// Log but continue
	}

	// Generate new access token (but keep same refresh token/session)
	accessToken, err := s.tokenService.GenerateAccessToken(u.ID, u.PublicID, u.IsAdmin)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: req.RefreshToken, // Return same refresh token
		ExpiresIn:    int64(s.tokenService.GetAccessTokenTTL().Seconds()),
		User:         toUserResponse(u),
	}, nil
}

// Logout invalidates the current session.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := s.tokenService.HashRefreshToken(refreshToken)
	session, err := s.sessionRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil // Session not found is ok for logout
	}
	return s.sessionRepo.Delete(ctx, session.ID)
}

// LogoutAll invalidates all sessions for a user.
func (s *Service) LogoutAll(ctx context.Context, userID int64) error {
	return s.sessionRepo.DeleteByUserID(ctx, userID)
}

// GetCurrentUser returns the user for the given internal ID.
func (s *Service) GetCurrentUser(ctx context.Context, userID int64) (*UserResponse, error) {
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toUserResponse(u), nil
}

// ChangePasswordRequest represents a password change request.
type ChangePasswordRequest struct {
	UserID          int64
	CurrentPassword string
	NewPassword     string
}

// ChangePassword changes a user's password.
func (s *Service) ChangePassword(ctx context.Context, req *ChangePasswordRequest) error {
	// Validate
	pwReq := &user.PasswordChangeRequest{
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}
	if err := pwReq.Validate(); err != nil {
		return err
	}

	// Get user by internal ID
	u, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return err
	}

	// Verify current password
	if err := s.passwordHash.Verify(req.CurrentPassword, u.PasswordHash); err != nil {
		return user.ErrInvalidCredentials
	}

	// Hash new password
	hash, err := s.passwordHash.Hash(req.NewPassword)
	if err != nil {
		return err
	}

	// Update password using UpdatePassword repository method
	return s.userRepo.UpdatePassword(ctx, u.ID, hash)
}

// SessionResponse contains session information for API responses.
type SessionResponse struct {
	ID         string    `json:"id"` // Public ID for API
	UserAgent  string    `json:"user_agent"`
	IPAddress  string    `json:"ip_address"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	IsCurrent  bool      `json:"is_current"`
}

// ListSessions returns all active sessions for a user.
func (s *Service) ListSessions(ctx context.Context, userID int64, currentTokenHash string) ([]*SessionResponse, error) {
	sessions, err := s.sessionRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]*SessionResponse, 0, len(sessions))
	for _, sess := range sessions {
		if sess.IsExpired() {
			continue // Skip expired sessions
		}
		result = append(result, &SessionResponse{
			ID:         sess.PublicID, // Return public ID to API
			UserAgent:  sess.UserAgent,
			IPAddress:  sess.IPAddress,
			CreatedAt:  sess.CreatedAt,
			LastUsedAt: sess.LastUsedAt,
			IsCurrent:  sess.RefreshTokenHash == currentTokenHash,
		})
	}
	return result, nil
}

// RevokeSession revokes a specific session by public ID.
func (s *Service) RevokeSession(ctx context.Context, userID int64, sessionPublicID string) error {
	session, err := s.sessionRepo.GetByPublicID(ctx, sessionPublicID)
	if err != nil {
		return err
	}

	// Ensure user owns this session
	if session.UserID != userID {
		return user.ErrSessionNotFound
	}

	return s.sessionRepo.Delete(ctx, session.ID)
}

// NeedsSetup returns true if no users exist yet.
func (s *Service) NeedsSetup(ctx context.Context) (bool, error) {
	exists, err := s.userRepo.ExistsAny(ctx)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// ValidateAccessToken validates an access token and returns claims.
func (s *Service) ValidateAccessToken(token string) (*auth.AccessClaims, error) {
	return s.tokenService.ValidateAccessToken(token)
}

// Helper functions

func generateID(prefix string) string {
	tid, err := typeid.WithPrefix(prefix)
	if err != nil {
		// Fallback should never happen with valid prefix
		panic("invalid typeid prefix: " + prefix)
	}
	return tid.String()
}

func toUserResponse(u *user.User) *UserResponse {
	return &UserResponse{
		ID:          u.PublicID, // Return public ID to API
		Username:    u.Username,
		DisplayName: u.DisplayName,
		IsAdmin:     u.IsAdmin,
		CreatedAt:   u.CreatedAt,
	}
}

func (s *Service) enforceMaxSessions(ctx context.Context, userID int64) error {
	if s.maxSessions <= 0 {
		return nil
	}

	count, err := s.sessionRepo.CountByUserID(ctx, userID)
	if err != nil {
		return err
	}

	if count <= int64(s.maxSessions) {
		return nil
	}

	// Get all sessions and delete oldest ones
	sessions, err := s.sessionRepo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}

	// Sessions are ordered by last_used_at DESC, so delete from the end
	for i := s.maxSessions; i < len(sessions); i++ {
		_ = s.sessionRepo.Delete(ctx, sessions[i].ID)
	}

	return nil
}

// CleanExpiredSessions deletes all expired sessions.
func (s *Service) CleanExpiredSessions(ctx context.Context) (int64, error) {
	return s.sessionRepo.DeleteExpired(ctx)
}
