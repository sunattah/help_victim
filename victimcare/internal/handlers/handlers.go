// Package handlers wires the HTTP routes of the Victim Care server.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"victimcare/internal/auth"
	"victimcare/internal/config"
	"victimcare/internal/countries"
	"victimcare/internal/guides"
	"victimcare/internal/models"
	"victimcare/internal/store"
)

// Version is the build version shown in the About screen.
const Version = "1.0.0"

type contextKey string

const sessionKey contextKey = "session"

// Server holds every dependency of the HTTP layer.
type Server struct {
	cfg      config.Config
	store    *store.Store
	sessions *auth.Manager
	tmpl     *template.Template
	mux      *http.ServeMux
	limiter  *RateLimiter
}

// New creates a Server and registers every route.
func New(cfg config.Config, st *store.Store) (*Server, error) {
	tmpl, err := template.ParseGlob(filepath.Join(cfg.TemplateDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	s := &Server{
		cfg:      cfg,
		store:    st,
		sessions: auth.NewManager(st, cfg.Secret, cfg.SecureCookies),
		tmpl:     tmpl,
		mux:      http.NewServeMux(),
		limiter:  NewRateLimiter(60, time.Minute),
	}
	s.routes()
	return s, nil
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; img-src 'self' data: https://images.pexels.com; "+
			"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
			"font-src 'self' https://fonts.gstatic.com; script-src 'self' 'unsafe-inline'; "+
			"connect-src 'self'")
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/v1/auth/signup", s.limitAuth(s.handleSignup))
	s.mux.HandleFunc("POST /api/v1/auth/login", s.limitAuth(s.handleLogin))
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/v1/auth/me", s.handleMe)
	s.mux.HandleFunc("PATCH /api/v1/users/me", s.handleUpdateProfile)
	s.mux.HandleFunc("GET /api/v1/users/me/medical", s.handleGetMedical)
	s.mux.HandleFunc("PUT /api/v1/users/me/medical", s.handleUpdateMedical)
	s.mux.HandleFunc("GET /api/v1/users/me/pictures", s.handleListPictures)
	s.mux.HandleFunc("POST /api/v1/users/me/pictures", s.handleUploadPicture)
	s.mux.HandleFunc("DELETE /api/v1/users/me/pictures/{id}", s.handleDeletePicture)
	s.mux.HandleFunc("POST /api/v1/users/me/pictures/{id}/primary", s.handlePrimaryPicture)
	s.mux.HandleFunc("GET /api/v1/users/me/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/v1/users/me/settings", s.handleUpdateSettings)
	s.mux.HandleFunc("POST /api/v1/users/me/password", s.handleChangePassword)
	s.mux.HandleFunc("DELETE /api/v1/users/me", s.handleDeleteAccount)
	s.mux.HandleFunc("GET /api/v1/contacts", s.handleListContacts)
	s.mux.HandleFunc("POST /api/v1/contacts", s.handleSaveContact)
	s.mux.HandleFunc("DELETE /api/v1/contacts/{id}", s.handleDeleteContact)
	s.mux.HandleFunc("POST /api/v1/contacts/{id}/primary", s.handleMakePrimaryContact)
	s.mux.HandleFunc("GET /api/v1/countries", s.handleCountries)
	s.mux.HandleFunc("GET /api/v1/incidents", s.handleListIncidents)
	s.mux.HandleFunc("POST /api/v1/incidents", s.handleCreateIncident)
	s.mux.HandleFunc("DELETE /api/v1/incidents", s.handleClearIncidents)
	s.mux.HandleFunc("DELETE /api/v1/incidents/{id}", s.handleDeleteSingleIncident)
	s.mux.HandleFunc("GET /api/v1/guides", s.handleGuides)

	s.mux.HandleFunc("GET /media/picture/{id}", s.handlePictureFile)
	s.mux.HandleFunc("GET /media/picture/{id}/thumb", s.handlePictureThumb)

	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.HandleFunc("GET /static/", s.handleStatic)
}

// ---------------------------------------------------------------- helpers --

// RateLimiter tracks recent attempts by client IP using a sliding window.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates an in-memory rate limiter.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow reports whether a request from the given IP is within rate limits.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	valid := make([]time.Time, 0, len(rl.attempts[ip]))
	for _, t := range rl.attempts[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.attempts[ip] = valid
		return false
	}

	valid = append(valid, now)
	rl.attempts[ip] = valid
	return true
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// limitAuth limits attempts on sensitive authentication endpoints.
func (s *Server) limitAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !s.limiter.Allow(ip) {
			writeError(w, http.StatusTooManyRequests, "Too many attempts. Please wait a minute before trying again.", "")
			return
		}
		next(w, r)
	}
}

// sessionFrom returns the session attached to the request, if any.
func sessionFrom(r *http.Request) *auth.Session {
	if s, ok := r.Context().Value(sessionKey).(*auth.Session); ok {
		return s
	}
	return nil
}

// withSession loads the session and puts it in the request context.
func (s *Server) withSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := s.sessions.Load(r.Context(), r)
		if err != nil {
			log.Printf("session lookup failed: %v", err)
		}
		if session != nil {
			r = r.WithContext(context.WithValue(r.Context(), sessionKey, session))
		}
		next(w, r)
	}
}

// requireAuth rejects requests without a session.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return s.withSession(func(w http.ResponseWriter, r *http.Request) {
		if sessionFrom(r) == nil {
			writeError(w, http.StatusUnauthorized, "You need to sign in first.", "")
			return
		}
		next(w, r)
	})
}

// requireCSRF rejects state changing requests without a valid CSRF token.
func (s *Server) requireCSRF(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		session := sessionFrom(r)
		token := r.Header.Get("X-CSRF-Token")
		if token == "" {
			token = r.FormValue("csrf_token")
		}
		if !auth.VerifyCSRF(session.CSRF, token) {
			writeError(w, http.StatusForbidden, "Your session expired. Reload the page and try again.", "csrf")
			return
		}
		next(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message, field string) {
	writeJSON(w, status, map[string]any{"error": message, "field": field})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decode body: %w", err)
	}
	return nil
}

// publicUser converts a user into the shape that is safe to send to a browser.
func (s *Server) publicUser(ctx context.Context, u models.User) models.PublicUser {
	p := models.PublicUser{
		ID:            u.ID,
		FirstName:     u.FirstName,
		SecondName:    u.SecondName,
		FullName:      u.FullName(),
		Email:         u.Email,
		State:         u.State,
		Nation:        u.Nation,
		Country:       u.Country,
		Theme:         u.Theme,
		Motion:        u.Motion,
		EmergencyNo:   u.EmergencyNo,
		ShareLocation: u.ShareLocation,
		Vibrate:       u.Vibrate,
		CountdownSecs: u.CountdownSecs,
		BloodType:     u.BloodType,
		Allergies:     u.Allergies,
		Medications:   u.Medications,
		MedicalNotes:  u.MedicalNotes,
		ShareMedical:  u.ShareMedical,
		Initials:      u.Initials(),
		CreatedAt:     u.CreatedAt.Format(time.RFC3339),
	}
	if picture, err := s.primaryPicture(ctx, u.ID); err == nil {
		p.AvatarURL = picture.URL
	}
	return p
}

// primaryPicture returns the profile picture of a user, if there is one.
func (s *Server) primaryPicture(ctx context.Context, userID int64) (models.Picture, error) {
	pictures, err := s.store.Pictures(ctx, userID)
	if err != nil {
		return models.Picture{}, err
	}
	for _, p := range pictures {
		if p.Primary {
			return decorate(p), nil
		}
	}
	if len(pictures) > 0 {
		return decorate(pictures[0]), nil
	}
	return models.Picture{}, store.ErrNotFound
}

// decorate fills in the public URLs of a picture.
func decorate(p models.Picture) models.Picture {
	p.URL = fmt.Sprintf("/media/picture/%d", p.ID)
	p.ThumbURL = fmt.Sprintf("/media/picture/%d/thumb", p.ID)
	return p
}

// ------------------------------------------------------------------ api ----

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": Version,
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleGuides(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"guides": guides.All()})
}

// handleCountries returns the country and state list used by the signup and
// profile forms.
func (s *Server) handleCountries(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"countries": countries.All()})
}

// ---------------------------------------------------------------- pages ----

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	session, err := s.sessions.Load(r.Context(), r)
	if err != nil {
		log.Printf("session lookup failed: %v", err)
	}

	data := map[string]any{
		"Version":   Version,
		"Theme":     "light",
		"SignedIn":  false,
		"CSRF":      "",
		"User":      models.PublicUser{},
		"Pictures":  []models.Picture{},
		"Guides":    guides.All(),
		"Contact":   models.Contact{},
		"Contacts":  []models.Contact{},
		"Reports":   []models.Incident{},
		"Countries": countries.All(),
		"Year":      time.Now().Year(),
	}
	if session != nil {
		data["SignedIn"] = true
		data["CSRF"] = session.CSRF
		data["Theme"] = session.User.Theme
		data["User"] = s.publicUser(r.Context(), session.User)

		if pictures, err := s.store.Pictures(r.Context(), session.User.ID); err == nil {
			decorated := make([]models.Picture, 0, len(pictures))
			for _, p := range pictures {
				decorated = append(decorated, decorate(p))
			}
			data["Pictures"] = decorated
		}
		if contacts, err := s.store.Contacts(r.Context(), session.User.ID); err == nil && len(contacts) > 0 {
			data["Contact"] = contacts[0]
			data["Contacts"] = contacts
		}
		if reports, err := s.store.Incidents(r.Context(), session.User.ID, 20); err == nil {
			data["Reports"] = reports
		}
	}
	data["Bootstrap"] = template.JS(s.bootstrap(r.Context(), data))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("render index: %v", err)
	}
}

// bootstrap renders the initial state of the page as JSON so the browser can
// take over without a page reload.
func (s *Server) bootstrap(ctx context.Context, data map[string]any) string {
	payload := map[string]any{
		"signedIn": data["SignedIn"],
		"csrf":     data["CSRF"],
		"user":     data["User"],
		"pictures": data["Pictures"],
		"contact":  data["Contact"],
		"contacts": data["Contacts"],
		"reports":  data["Reports"],
		"version":  Version,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	dir := s.cfg.StaticDir
	path := filepath.Join(dir, filepath.Clean(strings.TrimPrefix(r.URL.Path, "/static/")))
	if !strings.HasPrefix(path, dir) {
		http.NotFound(w, r)
		return
	}
	file, err := os.Stat(path)
	if err != nil || file.IsDir() {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(path, ".css"), strings.HasSuffix(path, ".js"):
		w.Header().Set("Cache-Control", "public, max-age=300")
	case strings.HasSuffix(path, ".svg"), strings.HasSuffix(path, ".png"), strings.HasSuffix(path, ".jpg"):
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	http.ServeFile(w, r, path)
}

// ---------------------------------------------------------------- media ----

func (s *Server) handlePictureFile(w http.ResponseWriter, r *http.Request) {
	s.withSession(func(w http.ResponseWriter, r *http.Request) { s.servePicture(w, r, false) })(w, r)
}

func (s *Server) handlePictureThumb(w http.ResponseWriter, r *http.Request) {
	s.withSession(func(w http.ResponseWriter, r *http.Request) { s.servePicture(w, r, true) })(w, r)
}

func (s *Server) servePicture(w http.ResponseWriter, r *http.Request, thumb bool) {
	session := sessionFrom(r)
	if session == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	picture, err := s.store.Picture(r.Context(), session.User.ID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		log.Printf("load picture: %v", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	name := picture.File
	if thumb {
		name = picture.Thumb
	}
	if name == "" || strings.ContainsAny(name, `/\`) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=600")
	http.ServeFile(w, r, filepath.Join(s.cfg.UploadDir, name))
}
