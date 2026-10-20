// Package config holds the runtime configuration of the Victim Care server.
//
// Every value has a working default so the binary can be started with no
// environment set up at all.
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config is the full server configuration.
type Config struct {
	// Addr is the listen address, e.g. ":8080".
	Addr string
	// DBPath is the SQLite database file.
	DBPath string
	// UploadDir is where user uploaded pictures are stored.
	UploadDir string
	// Secret is used to derive CSRF tokens and session token hashes.
	Secret []byte
	// SecureCookies marks session cookies as Secure (use behind HTTPS).
	SecureCookies bool
	// MaxUploadBytes caps the size of a single uploaded picture.
	MaxUploadBytes int64
	// SessionTTL is how long a session stays valid.
	SessionTTL time.Duration
	// TemplateDir and StaticDir allow overriding the bundled asset folders.
	TemplateDir string
	StaticDir   string
}

// Load reads the configuration from the environment, falling back to defaults.
func Load() Config {
	root := env("VICTIMCARE_ROOT", defaultRoot())
	cfg := Config{
		Addr:           env("VICTIMCARE_ADDR", ":8080"),
		DBPath:         env("VICTIMCARE_DB", filepath.Join(root, "data", "victimcare.db")),
		UploadDir:      env("VICTIMCARE_UPLOADS", filepath.Join(root, "data", "uploads")),
		Secret:         []byte(env("VICTIMCARE_SECRET", "victim-care-development-secret-change-me")),
		SecureCookies:  envBool("VICTIMCARE_SECURE_COOKIES", false),
		MaxUploadBytes: int64(envInt("VICTIMCARE_MAX_UPLOAD_BYTES", 5<<20)),
		SessionTTL:     time.Duration(envInt("VICTIMCARE_SESSION_HOURS", 24*14)) * time.Hour,
		TemplateDir:    env("VICTIMCARE_TEMPLATES", defaultDir(root, "templates")),
		StaticDir:      env("VICTIMCARE_STATIC", defaultDir(root, "static")),
	}
	return cfg
}

// defaultDir returns the first existing directory among the candidates. It lets
// the server run from the repository root as well as from inside victimcare/.
func defaultDir(root, name string) string {
	candidates := []string{
		filepath.Join(root, "victimcare", name),
		filepath.Join(root, name),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return candidates[0]
}

func defaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
