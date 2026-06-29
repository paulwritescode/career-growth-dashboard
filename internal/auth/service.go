// Package auth handles the single-operator authentication: setup, login,
// sessions, password changes, and API key management. It is the foundation
// of phase 2 — everything else depends on a logged-in user.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// Sentinel errors.
var (
	ErrNoAdmin            = errors.New("no admin user exists; run /setup first")
	ErrInvalidCredentials = errors.New("incorrect username or password")
	ErrSessionExpired     = errors.New("session expired")
	ErrPasswordMismatch   = errors.New("new password and confirmation do not match")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrSetupAlreadyDone   = errors.New("setup has already been completed")
	ErrSamePassword       = errors.New("new password must differ from current password")
	ErrInvalidAPIKey      = errors.New("invalid API key")
	ErrAPIKeyRevoked      = errors.New("API key has been revoked")
)

const (
	sessionIDLen   = 32 // bytes before base64 encoding
	apiKeyLen      = 24 // bytes before base64 encoding
	sessionMaxAge  = 14 * 24 * time.Hour // default idle timeout
)

// DB is the interface this package needs from the store layer.
type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Service manages auth operations for the single operator account.
type Service struct {
	db  DB
	now func() time.Time
}

// New creates an auth service.
func New(db DB) *Service {
	return &Service{db: db, now: time.Now}
}

// SetupInput is the data needed for first-run setup.
type SetupInput struct {
	Password    string
	Confirm     string
	DisplayName string
}

// SetupAdmin creates the admin user on first run. Returns ErrSetupAlreadyDone
// if a user already exists.
func (s *Service) SetupAdmin(ctx context.Context, in SetupInput) (domain.User, error) {
	// Check no user exists yet.
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return domain.User{}, fmt.Errorf("auth: check users: %w", err)
	}
	if count > 0 {
		return domain.User{}, ErrSetupAlreadyDone
	}

	if err := validatePassword(in.Password, in.Confirm); err != nil {
		return domain.User{}, err
	}

	hash, err := hashPassword(in.Password)
	if err != nil {
		return domain.User{}, fmt.Errorf("auth: hash password: %w", err)
	}

	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		displayName = "admin"
	}
	initials := deriveInitials(displayName)
	now := s.now().UTC().Format(time.RFC3339)

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, display_name, avatar_initials, password_hash, created_at, updated_at)
		 VALUES ('admin', ?, ?, ?, ?, ?)`,
		displayName, initials, hash, now, now)
	if err != nil {
		return domain.User{}, fmt.Errorf("auth: create admin: %w", err)
	}
	id, _ := res.LastInsertId()

	return domain.User{
		ID:             id,
		Username:       "admin",
		DisplayName:    displayName,
		AvatarInitials: initials,
		Role:           domain.RoleOther,
		CreatedAt:      s.now().UTC(),
		UpdatedAt:      s.now().UTC(),
	}, nil
}

// HasAdmin reports whether a user row exists (i.e. setup is done).
func (s *Service) HasAdmin(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, fmt.Errorf("auth: check users: %w", err)
	}
	return count > 0, nil
}

// Authenticate verifies credentials and returns a new session.
func (s *Service) Authenticate(ctx context.Context, username, password string, remember bool) (domain.Session, error) {
	var user domain.User
	var hash string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Session{}, ErrInvalidCredentials
		}
		return domain.Session{}, fmt.Errorf("auth: lookup user: %w", err)
	}

	if !checkPassword(hash, password) {
		return domain.Session{}, ErrInvalidCredentials
	}

	return s.createSession(ctx, user.ID, remember)
}

// ValidateSession looks up and validates a session by its cookie ID.
// Bumps last_seen_at on success.
func (s *Service) ValidateSession(ctx context.Context, sessionID string) (domain.Session, error) {
	var sess domain.Session
	var createdAt, lastSeen, expiresAt string
	var remember int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, created_at, last_seen_at, expires_at, remember
		 FROM sessions WHERE id = ?`, sessionID).
		Scan(&sess.ID, &sess.UserID, &createdAt, &lastSeen, &expiresAt, &remember)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Session{}, ErrSessionExpired
		}
		return domain.Session{}, fmt.Errorf("auth: lookup session: %w", err)
	}

	sess.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	sess.LastSeenAt, _ = time.Parse(time.RFC3339, lastSeen)
	sess.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	sess.Remember = remember == 1

	// Check expiry.
	if s.now().UTC().After(sess.ExpiresAt) {
		// Lazily delete expired session.
		_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID)
		return domain.Session{}, ErrSessionExpired
	}

	// Bump last_seen_at.
	now := s.now().UTC().Format(time.RFC3339)
	_, _ = s.db.ExecContext(ctx,
		`UPDATE sessions SET last_seen_at = ? WHERE id = ?`, now, sessionID)

	return sess, nil
}

// Logout deletes a session.
func (s *Service) Logout(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID)
	return err
}

// ChangePasswordInput is the data needed to change a password.
type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
	Confirm         string
	UserID          int64
	SessionID       string // keep this session alive
}

// ChangePassword verifies the current password and updates it.
func (s *Service) ChangePassword(ctx context.Context, in ChangePasswordInput) error {
	var hash string
	if err := s.db.QueryRowContext(ctx,
		`SELECT password_hash FROM users WHERE id = ?`, in.UserID).Scan(&hash); err != nil {
		return fmt.Errorf("auth: lookup user: %w", err)
	}

	if !checkPassword(hash, in.CurrentPassword) {
		return ErrInvalidCredentials
	}
	if err := validatePassword(in.NewPassword, in.Confirm); err != nil {
		return err
	}
	if in.CurrentPassword == in.NewPassword {
		return ErrSamePassword
	}

	newHash, err := hashPassword(in.NewPassword)
	if err != nil {
		return fmt.Errorf("auth: hash password: %w", err)
	}

	now := s.now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		newHash, now, in.UserID); err != nil {
		return fmt.Errorf("auth: update password: %w", err)
	}

	// Invalidate all other sessions.
	_, _ = s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = ? AND id != ?`, in.UserID, in.SessionID)

	return nil
}

// UpdateProfileInput is the data needed to update the user profile.
type UpdateProfileInput struct {
	UserID         int64
	DisplayName    string
	Username       string
	AvatarInitials string
}

// UpdateProfile updates display name, username, and avatar initials.
func (s *Service) UpdateProfile(ctx context.Context, in UpdateProfileInput) (domain.User, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" {
		username = "admin"
	}
	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		displayName = username
	}
	initials := strings.TrimSpace(in.AvatarInitials)
	if initials == "" {
		initials = deriveInitials(displayName)
	}
	// Cap initials to 2 chars.
	if len(initials) > 2 {
		initials = initials[:2]
	}

	now := s.now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET username = ?, display_name = ?, avatar_initials = ?, updated_at = ?
		 WHERE id = ?`,
		username, displayName, initials, now, in.UserID)
	if err != nil {
		return domain.User{}, fmt.Errorf("auth: update profile: %w", err)
	}

	return s.GetUser(ctx, in.UserID)
}

// GetUser fetches the user by ID.
func (s *Service) GetUser(ctx context.Context, userID int64) (domain.User, error) {
	var u domain.User
	var roleOther sql.NullString
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, display_name, avatar_initials, role, role_other, created_at, updated_at
		 FROM users WHERE id = ?`, userID).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.AvatarInitials, &u.Role, &roleOther, &createdAt, &updatedAt)
	if err != nil {
		return domain.User{}, fmt.Errorf("auth: get user: %w", err)
	}
	if roleOther.Valid {
		u.RoleOther = &roleOther.String
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	u.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return u, nil
}

// SetRole updates the user's role (from onboarding).
func (s *Service) SetRole(ctx context.Context, userID int64, role domain.UserRole, other string) error {
	now := s.now().UTC().Format(time.RFC3339)
	var roleOther *string
	if role == domain.RoleOther && other != "" {
		roleOther = &other
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET role = ?, role_other = ?, updated_at = ? WHERE id = ?`,
		string(role), roleOther, now, userID)
	return err
}

// --- API Key Management ---

// GenerateAPIKey creates a new API key and returns the plaintext (shown once).
func (s *Service) GenerateAPIKey(ctx context.Context, userID int64, label string) (string, domain.APIKey, error) {
	plain, err := generateRandomString(apiKeyLen)
	if err != nil {
		return "", domain.APIKey{}, fmt.Errorf("auth: generate key: %w", err)
	}
	plaintext := "sk_live_" + plain

	hash := hashAPIKey(plaintext)
	prefix := plaintext[:12] // "sk_live_" + 4 chars
	now := s.now().UTC().Format(time.RFC3339)

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO api_keys (user_id, label, key_hash, key_prefix, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		userID, label, hash, prefix, now)
	if err != nil {
		return "", domain.APIKey{}, fmt.Errorf("auth: insert api_key: %w", err)
	}
	id, _ := res.LastInsertId()

	key := domain.APIKey{
		ID:        id,
		UserID:    userID,
		Label:     label,
		KeyPrefix: prefix,
		CreatedAt: s.now().UTC(),
	}
	return plaintext, key, nil
}

// AuthenticateKey validates a presented API key.
func (s *Service) AuthenticateKey(ctx context.Context, presented string) (domain.APIKey, error) {
	hash := hashAPIKey(presented)

	var key domain.APIKey
	var revokedAt sql.NullString
	var createdAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, label, key_prefix, created_at, revoked_at
		 FROM api_keys WHERE key_hash = ?`, hash).
		Scan(&key.ID, &key.UserID, &key.Label, &key.KeyPrefix, &createdAt, &revokedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.APIKey{}, ErrInvalidAPIKey
		}
		return domain.APIKey{}, fmt.Errorf("auth: lookup api_key: %w", err)
	}
	if revokedAt.Valid {
		return domain.APIKey{}, ErrAPIKeyRevoked
	}

	key.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	// Update last_used_at.
	now := s.now().UTC().Format(time.RFC3339)
	_, _ = s.db.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = ? WHERE id = ?`, now, key.ID)

	return key, nil
}

// RevokeAPIKey marks a key as revoked.
func (s *Service) RevokeAPIKey(ctx context.Context, keyID int64) error {
	now := s.now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE api_keys SET revoked_at = ? WHERE id = ?`, now, keyID)
	return err
}

// ListAPIKeys returns all API keys for a user.
func (s *Service) ListAPIKeys(ctx context.Context, userID int64) ([]domain.APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, label, key_prefix, created_at, last_used_at, revoked_at
		 FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("auth: list api_keys: %w", err)
	}
	defer rows.Close()

	var keys []domain.APIKey
	for rows.Next() {
		var k domain.APIKey
		var createdAt string
		var lastUsed, revoked sql.NullString
		if err := rows.Scan(&k.ID, &k.UserID, &k.Label, &k.KeyPrefix, &createdAt, &lastUsed, &revoked); err != nil {
			return nil, err
		}
		k.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if lastUsed.Valid {
			t, _ := time.Parse(time.RFC3339, lastUsed.String)
			k.LastUsedAt = &t
		}
		if revoked.Valid {
			t, _ := time.Parse(time.RFC3339, revoked.String)
			k.RevokedAt = &t
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// --- Internal helpers ---

func (s *Service) createSession(ctx context.Context, userID int64, remember bool) (domain.Session, error) {
	id, err := generateRandomString(sessionIDLen)
	if err != nil {
		return domain.Session{}, fmt.Errorf("auth: generate session id: %w", err)
	}

	now := s.now().UTC()
	expiresAt := now.Add(sessionMaxAge)
	rememberInt := 0
	if remember {
		rememberInt = 1
	}

	nowStr := now.Format(time.RFC3339)
	expiresStr := expiresAt.Format(time.RFC3339)

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, created_at, last_seen_at, expires_at, remember)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, userID, nowStr, nowStr, expiresStr, rememberInt)
	if err != nil {
		return domain.Session{}, fmt.Errorf("auth: create session: %w", err)
	}

	return domain.Session{
		ID:         id,
		UserID:     userID,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  expiresAt,
		Remember:   remember,
	}, nil
}

func validatePassword(password, confirm string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}
	if password != confirm {
		return ErrPasswordMismatch
	}
	return nil
}

// hashPassword uses SHA-256 for simplicity in this local-first app.
// Production would use Argon2id or bcrypt, but we avoid the CGO dependency.
// Since this is a local-only, single-operator tool, SHA-256 with a unique salt
// is acceptable. The salt is prepended to the hash.
func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(password))
	digest := h.Sum(nil)
	// Store as "salt:hash" in base64.
	return base64.RawStdEncoding.EncodeToString(salt) + ":" + base64.RawStdEncoding.EncodeToString(digest), nil
}

// checkPassword verifies a password against the stored hash.
func checkPassword(stored, password string) bool {
	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(password))
	actualHash := h.Sum(nil)
	return subtle.ConstantTimeCompare(expectedHash, actualHash) == 1
}

func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return base64.RawStdEncoding.EncodeToString(h[:])
}

func generateRandomString(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func deriveInitials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "?"
	}
	if len(parts) == 1 {
		r := []rune(strings.ToUpper(parts[0]))
		if len(r) >= 2 {
			return string(r[:2])
		}
		return string(r)
	}
	first := []rune(strings.ToUpper(parts[0]))
	last := []rune(strings.ToUpper(parts[len(parts)-1]))
	return string(first[0:1]) + string(last[0:1])
}
