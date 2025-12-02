package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenConfig holds configuration for the token service.
type TokenConfig struct {
	// Secret is the signing key for JWTs
	Secret []byte
	// AccessTokenTTL is the access token lifetime
	AccessTokenTTL time.Duration
	// RefreshTokenTTL is the refresh token lifetime
	RefreshTokenTTL time.Duration
	// Issuer is the JWT issuer claim
	Issuer string
}

// DefaultTokenConfig returns default token configuration.
func DefaultTokenConfig(secret []byte) *TokenConfig {
	return &TokenConfig{
		Secret:          secret,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour, // 7 days
		Issuer:          "viewra",
	}
}

// AccessClaims represents the claims in an access token.
type AccessClaims struct {
	jwt.RegisteredClaims
	UserID  int64 `json:"uid"` // Internal user ID for efficient database lookups
	IsAdmin bool  `json:"adm"`
}

// TokenService handles JWT generation and validation.
type TokenService struct {
	config *TokenConfig
}

// NewTokenService creates a new token service.
func NewTokenService(config *TokenConfig) *TokenService {
	return &TokenService{config: config}
}

// GenerateAccessToken creates a new access token for a user.
// The publicID is used in the Subject claim for external representation.
// The userID (int64) is stored in the uid claim for efficient internal lookups.
func (s *TokenService) GenerateAccessToken(userID int64, publicID string, isAdmin bool) (string, error) {
	now := time.Now()
	claims := &AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.config.Issuer,
			Subject:   publicID, // Public ID for external representation
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.AccessTokenTTL)),
		},
		UserID:  userID, // Internal ID for efficient DB lookups
		IsAdmin: isAdmin,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.config.Secret)
}

// ValidateAccessToken validates an access token and returns its claims.
func (s *TokenService) ValidateAccessToken(tokenString string) (*AccessClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AccessClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidSigningMethod
		}
		return s.config.Secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GenerateRefreshToken creates a new cryptographically secure refresh token.
// Returns the raw token (to be sent to client) and should be hashed before storage.
func (s *TokenService) GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// HashRefreshToken creates a SHA-256 hash of a refresh token for storage.
func (s *TokenService) HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// GetRefreshTokenTTL returns the refresh token time-to-live.
func (s *TokenService) GetRefreshTokenTTL() time.Duration {
	return s.config.RefreshTokenTTL
}

// GetAccessTokenTTL returns the access token time-to-live.
func (s *TokenService) GetAccessTokenTTL() time.Duration {
	return s.config.AccessTokenTTL
}

// Token errors
var (
	ErrInvalidToken         = errors.New("invalid token")
	ErrTokenExpired         = errors.New("token has expired")
	ErrInvalidSigningMethod = errors.New("invalid signing method")
)

// GenerateSecret generates a random secret for JWT signing.
// Should be called once and stored persistently.
func GenerateSecret() ([]byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	return secret, nil
}
