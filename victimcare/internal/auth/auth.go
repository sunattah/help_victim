// Package auth handles password hashing, session tokens and CSRF protection.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"victimcare/internal/models"
	"victimcare/internal/store"
)

// Cookie names used by the server.
const (
	SessionCookie = "victimcare_session"
	CSRFCookie    = "victimcare_csrf"
)

// Session keeps the authenticated state of a single request.
type Session struct {
	User  models.User
	CSRF  string
	Token string
}

// HashPassword returns a bcrypt hash of the given password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword reports whether the password matches the stored hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// NewToken returns a fresh random session token.
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken returns the hex SHA-256 digest of a session token. Only the digest
// is stored, so a leaked database cannot be replayed against the server.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewCSRF returns a random CSRF token.
func NewCSRF() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate csrf token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// VerifyCSRF compares a submitted token with the expected one.
func VerifyCSRF(expected, got string) bool {
	return expected != "" && hmac.Equal([]byte(expected), []byte(got))
}

// SetSessionCookies writes the session and CSRF cookies.
func SetSessionCookies(w http.ResponseWriter, token, csrf string, secure bool, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookie,
		Value:    csrf,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
	})
}

// ClearSessionCookies removes the session and CSRF cookies.
func ClearSessionCookies(w http.ResponseWriter, secure bool) {
	for _, name := range []string{SessionCookie, CSRFCookie} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: name == SessionCookie,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
	}
}

// Manager resolves sessions for incoming requests.
type Manager struct {
	store  *store.Store
	secret []byte
	secure bool
}

// NewManager creates a session manager.
func NewManager(st *store.Store, secret []byte, secure bool) *Manager {
	return &Manager{store: st, secret: secret, secure: secure}
}

// Load reads the session cookie and returns the matching user, if any.
func (m *Manager) Load(ctx context.Context, r *http.Request) (*Session, error) {
	cookie, err := r.Cookie(SessionCookie)
	if err != nil || cookie.Value == "" {
		return nil, nil
	}
	tokenHash := HashToken(cookie.Value)
	user, csrf, err := m.store.SessionUser(ctx, tokenHash)
	if err != nil {
		return nil, nil // treat any lookup problem as "not signed in"
	}
	return &Session{User: user, CSRF: csrf, Token: cookie.Value}, nil
}

// Start creates a new session for a user and sets the cookies.
func (m *Manager) Start(ctx context.Context, w http.ResponseWriter, userID int64, ttl time.Duration) error {
	token, err := NewToken()
	if err != nil {
		return err
	}
	csrf, err := NewCSRF()
	if err != nil {
		return err
	}
	if err := m.store.CreateSession(ctx, HashToken(token), csrf, userID, time.Now().Add(ttl)); err != nil {
		return err
	}
	SetSessionCookies(w, token, csrf, m.secure, ttl)
	return nil
}

// End removes the session of the request and clears the cookies.
func (m *Manager) End(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if cookie, err := r.Cookie(SessionCookie); err == nil && cookie.Value != "" {
		if err := m.store.DeleteSession(ctx, HashToken(cookie.Value)); err != nil {
			return err
		}
	}
	ClearSessionCookies(w, m.secure)
	return nil
}
