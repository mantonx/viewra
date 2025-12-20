package auth

import (
	"context"

	"github.com/mantonx/viewra/internal/domain/user"
	"github.com/mantonx/viewra/internal/infrastructure/auth"
)

// AdminService handles user management operations (admin only).
type AdminService struct {
	userRepo     user.UserRepository
	sessionRepo  user.SessionRepository
	passwordHash *auth.PasswordHasher
}

// NewAdminService creates a new admin service.
func NewAdminService(
	userRepo user.UserRepository,
	sessionRepo user.SessionRepository,
	passwordHash *auth.PasswordHasher,
) *AdminService {
	return &AdminService{
		userRepo:     userRepo,
		sessionRepo:  sessionRepo,
		passwordHash: passwordHash,
	}
}

// ListUsersResponse contains paginated user list.
type ListUsersResponse struct {
	Users []*UserResponse `json:"users"`
	Total int64           `json:"total"`
}

// ListUsers returns all users with pagination.
func (s *AdminService) ListUsers(ctx context.Context, limit, offset int) (*ListUsersResponse, error) {
	users, err := s.userRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	total, err := s.userRepo.Count(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*UserResponse, len(users))
	for i, u := range users {
		result[i] = toUserResponse(u)
	}

	return &ListUsersResponse{
		Users: result,
		Total: total,
	}, nil
}

// GetUser returns a user by public ID.
func (s *AdminService) GetUser(ctx context.Context, publicID string) (*UserResponse, error) {
	u, err := s.userRepo.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	return toUserResponse(u), nil
}

// CreateUserRequest represents an admin user creation request.
type CreateUserRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	IsAdmin     bool   `json:"is_admin"`
}

// CreateUser creates a new user (admin only).
func (s *AdminService) CreateUser(ctx context.Context, req *CreateUserRequest) (*UserResponse, error) {
	// Validate
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
	userID, err := generateID("usr")
	if err != nil {
		return nil, err
	}
	u := user.NewUser(
		userID,
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

// UpdateUserRequest represents an admin user update request.
type UpdateUserRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	IsAdmin     *bool   `json:"is_admin,omitempty"`
	IsDisabled  *bool   `json:"is_disabled,omitempty"`
}

// UpdateUser updates a user by public ID (admin only).
func (s *AdminService) UpdateUser(ctx context.Context, publicID string, req *UpdateUserRequest) (*UserResponse, error) {
	u, err := s.userRepo.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}

	if req.DisplayName != nil {
		if err := u.UpdateDisplayName(*req.DisplayName); err != nil {
			return nil, err
		}
	}

	if req.IsAdmin != nil {
		u.SetAdmin(*req.IsAdmin)
	}

	if req.IsDisabled != nil {
		if *req.IsDisabled {
			u.Disable()
		} else {
			u.Enable()
		}
	}

	if err := s.userRepo.Update(ctx, u); err != nil {
		return nil, err
	}

	return toUserResponse(u), nil
}

// DeleteUser deletes a user by public ID (admin only).
func (s *AdminService) DeleteUser(ctx context.Context, publicID string) error {
	// Get user to get internal ID
	u, err := s.userRepo.GetByPublicID(ctx, publicID)
	if err != nil {
		return err
	}

	// Delete all sessions first
	if err := s.sessionRepo.DeleteByUserID(ctx, u.ID); err != nil {
		// Log but continue
	}

	return s.userRepo.Delete(ctx, u.ID)
}

// ResetPasswordRequest represents an admin password reset.
type ResetPasswordRequest struct {
	UserID      string `json:"user_id"` // Public ID
	NewPassword string `json:"new_password"`
}

// ResetPassword resets a user's password (admin only).
func (s *AdminService) ResetPassword(ctx context.Context, req *ResetPasswordRequest) error {
	// Validate password
	if err := user.ValidatePassword(req.NewPassword); err != nil {
		return err
	}

	// Hash password
	hash, err := s.passwordHash.Hash(req.NewPassword)
	if err != nil {
		return err
	}

	// Get user by public ID to verify it exists and get internal ID
	u, err := s.userRepo.GetByPublicID(ctx, req.UserID)
	if err != nil {
		return err
	}

	// Update password using repository method
	if err := s.userRepo.UpdatePassword(ctx, u.ID, hash); err != nil {
		return err
	}

	// Invalidate all sessions (force re-login)
	return s.sessionRepo.DeleteByUserID(ctx, u.ID)
}
