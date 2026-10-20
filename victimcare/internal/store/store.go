// Package store persists everything in a single SQLite database file.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"victimcare/internal/config"
	"victimcare/internal/models"
)

// Sentinel errors returned by the store.
var (
	ErrNotFound      = errors.New("not found")
	ErrEmailTaken    = errors.New("email already registered")
	ErrSessionExpiry = errors.New("session expired")
)

// Store is the SQLite backed persistence layer.
type Store struct {
	db *sql.DB
}

// Open prepares the database file, runs the migrations and returns a Store.
func Open(cfg config.Config) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	dsn := fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)",
		cfg.DBPath,
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    first_name      TEXT    NOT NULL DEFAULT '',
    second_name     TEXT    NOT NULL DEFAULT '',
    email           TEXT    NOT NULL UNIQUE,
    password_hash   TEXT    NOT NULL,
    state           TEXT    NOT NULL DEFAULT '',
    nation          TEXT    NOT NULL DEFAULT '',
    country         TEXT    NOT NULL DEFAULT '',
    theme           TEXT    NOT NULL DEFAULT 'light',
    motion          INTEGER NOT NULL DEFAULT 1,
    emergency_no    TEXT    NOT NULL DEFAULT '112',
    share_location  INTEGER NOT NULL DEFAULT 1,
    vibrate         INTEGER NOT NULL DEFAULT 1,
    countdown_secs  INTEGER NOT NULL DEFAULT 5,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf       TEXT    NOT NULL,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS pictures (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file       TEXT    NOT NULL,
    thumb      TEXT    NOT NULL,
    is_primary INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS contacts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    phone      TEXT    NOT NULL,
    is_primary INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS incidents (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind            TEXT    NOT NULL,
    note            TEXT    NOT NULL DEFAULT '',
    latitude        REAL,
    longitude       REAL,
    location_shared INTEGER NOT NULL DEFAULT 0,
    created_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_pictures_user ON pictures(user_id);
CREATE INDEX IF NOT EXISTS idx_incidents_user ON incidents(user_id);
`

func migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	// Older databases get the country column added on start up.
	if !hasColumn(db, "users", "country") {
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN country TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

// hasColumn reports whether a table already has a column.
func hasColumn(db *sql.DB, table, column string) bool {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- users ----

// CreateUser inserts a new account. The email is stored lower-cased.
func (s *Store) CreateUser(ctx context.Context, u models.User) (models.User, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO users (first_name, second_name, email, password_hash, state, nation, country,
		                   theme, motion, emergency_no, share_location, vibrate, countdown_secs,
		                   created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.FirstName, u.SecondName, strings.ToLower(u.Email), u.PasswordHash, u.State, u.Nation, u.Country,
		u.Theme, boolToInt(u.Motion), u.EmergencyNo, boolToInt(u.ShareLocation),
		boolToInt(u.Vibrate), u.CountdownSecs, now, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return models.User{}, ErrEmailTaken
		}
		return models.User{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.User{}, err
	}
	return s.UserByID(ctx, id)
}

// UserByID loads a user by primary key.
func (s *Store) UserByID(ctx context.Context, id int64) (models.User, error) {
	row := s.db.QueryRowContext(ctx, userSelect+` WHERE id = ?`, id)
	return scanUser(row)
}

// UserByEmail loads a user by e-mail address.
func (s *Store) UserByEmail(ctx context.Context, email string) (models.User, error) {
	row := s.db.QueryRowContext(ctx, userSelect+` WHERE email = ?`, strings.ToLower(strings.TrimSpace(email)))
	return scanUser(row)
}

// UpdateProfile writes the editable profile fields of a user.
func (s *Store) UpdateProfile(ctx context.Context, id int64, first, second, email, state, nation, country string) (models.User, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		   SET first_name = ?, second_name = ?, email = ?, state = ?, nation = ?, country = ?, updated_at = ?
		 WHERE id = ?`,
		first, second, strings.ToLower(strings.TrimSpace(email)), state, nation, country, time.Now().Unix(), id)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return models.User{}, ErrEmailTaken
		}
		return models.User{}, err
	}
	return s.UserByID(ctx, id)
}

// UpdateSettings stores the appearance and alert preferences of a user.
func (s *Store) UpdateSettings(ctx context.Context, id int64, theme string, motion, shareLocation, vibrate bool, emergencyNo string, countdown int) (models.User, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		   SET theme = ?, motion = ?, share_location = ?, vibrate = ?, emergency_no = ?,
		       countdown_secs = ?, updated_at = ?
		 WHERE id = ?`,
		theme, boolToInt(motion), boolToInt(shareLocation), boolToInt(vibrate), emergencyNo, countdown,
		time.Now().Unix(), id)
	if err != nil {
		return models.User{}, err
	}
	return s.UserByID(ctx, id)
}

// UpdatePassword replaces the stored password hash.
func (s *Store) UpdatePassword(ctx context.Context, id int64, hash string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		hash, time.Now().Unix(), id)
	return err
}

// DeleteUser removes the account and every row that belongs to it.
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

const userSelect = `SELECT id, first_name, second_name, email, password_hash, state, nation, country,
	theme, motion, emergency_no, share_location, vibrate, countdown_secs, created_at, updated_at FROM users`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (models.User, error) {
	var u models.User
	var motion, share, vibrate int
	var created, updated int64
	err := row.Scan(&u.ID, &u.FirstName, &u.SecondName, &u.Email, &u.PasswordHash, &u.State, &u.Nation, &u.Country,
		&u.Theme, &motion, &u.EmergencyNo, &share, &vibrate, &u.CountdownSecs, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, err
	}
	u.Motion = motion == 1
	u.ShareLocation = share == 1
	u.Vibrate = vibrate == 1
	u.CreatedAt = time.Unix(created, 0)
	u.UpdatedAt = time.Unix(updated, 0)
	return u, nil
}

// ------------------------------------------------------------- sessions ----

// CreateSession stores a hashed session token.
func (s *Store) CreateSession(ctx context.Context, tokenHash, csrf string, userID int64, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, user_id, csrf, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		tokenHash, userID, csrf, expires.Unix(), time.Now().Unix())
	return err
}

// SessionUser resolves a session token hash to the owning user.
func (s *Store) SessionUser(ctx context.Context, tokenHash string) (models.User, string, error) {
	var userID, expires int64
	var csrf string
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, csrf, expires_at FROM sessions WHERE token_hash = ?`, tokenHash).
		Scan(&userID, &csrf, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, "", ErrSessionExpiry
	}
	if err != nil {
		return models.User{}, "", err
	}
	if time.Now().Unix() > expires {
		_ = s.DeleteSession(ctx, tokenHash)
		return models.User{}, "", ErrSessionExpiry
	}
	user, err := s.UserByID(ctx, userID)
	if err != nil {
		return models.User{}, "", err
	}
	return user, csrf, nil
}

// DeleteSession removes a single session.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteSessionsOfUser removes every session of a user (used on logout-all).
func (s *Store) DeleteSessionsOfUser(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// ------------------------------------------------------------- pictures ----

// AddPicture stores a new uploaded picture for a user.
func (s *Store) AddPicture(ctx context.Context, p models.Picture) (models.Picture, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO pictures (user_id, file, thumb, is_primary, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		p.UserID, p.File, p.Thumb, boolToInt(p.Primary), time.Now().Unix())
	if err != nil {
		return models.Picture{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.Picture{}, err
	}
	p.ID = id
	p.CreatedAt = time.Now()
	return p, nil
}

// Pictures lists every picture of a user, oldest first.
func (s *Store) Pictures(ctx context.Context, userID int64) ([]models.Picture, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, file, thumb, is_primary, created_at FROM pictures WHERE user_id = ? ORDER BY id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Picture
	for rows.Next() {
		var p models.Picture
		var primary int
		var created int64
		if err := rows.Scan(&p.ID, &p.UserID, &p.File, &p.Thumb, &primary, &created); err != nil {
			return nil, err
		}
		p.Primary = primary == 1
		p.CreatedAt = time.Unix(created, 0)
		out = append(out, p)
	}
	return out, rows.Err()
}

// Picture loads one picture of a user.
func (s *Store) Picture(ctx context.Context, userID, id int64) (models.Picture, error) {
	var p models.Picture
	var primary int
	var created int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, file, thumb, is_primary, created_at FROM pictures WHERE id = ? AND user_id = ?`,
		id, userID).Scan(&p.ID, &p.UserID, &p.File, &p.Thumb, &primary, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Picture{}, ErrNotFound
	}
	if err != nil {
		return models.Picture{}, err
	}
	p.Primary = primary == 1
	p.CreatedAt = time.Unix(created, 0)
	return p, nil
}

// SetPrimaryPicture marks one picture as the profile picture.
func (s *Store) SetPrimaryPicture(ctx context.Context, userID, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `UPDATE pictures SET is_primary = 0 WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pictures SET is_primary = 1 WHERE id = ? AND user_id = ?`, id, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// DeletePicture removes a picture row.
func (s *Store) DeletePicture(ctx context.Context, userID, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM pictures WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// ------------------------------------------------------------- contacts ----

// AddContact stores a new contact and makes it the primary one when the user
// has no contact yet.
func (s *Store) AddContact(ctx context.Context, c models.Contact) (models.Contact, error) {
	primary := c.Primary
	if !primary {
		existing, err := s.Contacts(ctx, c.UserID)
		if err != nil {
			return models.Contact{}, err
		}
		// The first contact of a user is the primary one.
		for _, contact := range existing {
			if contact.Primary {
				primary = false
				break
			}
			primary = true
		}
		if len(existing) == 0 {
			primary = true
		}
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO contacts (user_id, name, phone, is_primary, created_at) VALUES (?, ?, ?, ?, ?)`,
		c.UserID, c.Name, c.Phone, boolToInt(primary), time.Now().Unix())
	if err != nil {
		return models.Contact{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.Contact{}, err
	}
	c.ID = id
	c.Primary = primary
	c.CreatedAt = time.Now()
	return c, nil
}

// UpdateContact changes the name and phone of an existing contact.
func (s *Store) UpdateContact(ctx context.Context, userID, id int64, name, phone string) (models.Contact, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE contacts SET name = ?, phone = ? WHERE id = ? AND user_id = ?`, name, phone, id, userID)
	if err != nil {
		return models.Contact{}, err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return models.Contact{}, ErrNotFound
	}
	return s.ContactByID(ctx, userID, id)
}

// ContactByID loads one contact of a user.
func (s *Store) ContactByID(ctx context.Context, userID, id int64) (models.Contact, error) {
	var c models.Contact
	var primary int
	var created int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, phone, is_primary, created_at FROM contacts WHERE id = ? AND user_id = ?`,
		id, userID).Scan(&c.ID, &c.UserID, &c.Name, &c.Phone, &primary, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Contact{}, ErrNotFound
	}
	if err != nil {
		return models.Contact{}, err
	}
	c.Primary = primary == 1
	c.CreatedAt = time.Unix(created, 0)
	return c, nil
}

// MakePrimaryContact promotes a contact to be the one used for text alerts.
func (s *Store) MakePrimaryContact(ctx context.Context, userID, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `UPDATE contacts SET is_primary = 0 WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE contacts SET is_primary = 1 WHERE id = ? AND user_id = ?`, id, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteContact removes a contact of a user. When the primary contact is
// deleted another contact takes over.
func (s *Store) DeleteContact(ctx context.Context, userID, id int64) error {
	contact, err := s.ContactByID(ctx, userID, id)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM contacts WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	if contact.Primary {
		remaining, err := s.Contacts(ctx, userID)
		if err != nil {
			return err
		}
		for _, other := range remaining {
			if !other.Primary {
				return s.MakePrimaryContact(ctx, userID, other.ID)
			}
		}
	}
	return nil
}

// SetPrimaryContact stores the user's emergency contact (name + phone).
func (s *Store) SetPrimaryContact(ctx context.Context, c models.Contact) (models.Contact, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Contact{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `DELETE FROM contacts WHERE user_id = ?`, c.UserID); err != nil {
		return models.Contact{}, err
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO contacts (user_id, name, phone, is_primary, created_at) VALUES (?, ?, ?, 1, ?)`,
		c.UserID, c.Name, c.Phone, time.Now().Unix())
	if err != nil {
		return models.Contact{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.Contact{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.Contact{}, err
	}
	c.ID = id
	c.Primary = true
	c.CreatedAt = time.Now()
	return c, nil
}

// Contacts lists the contacts of a user.
func (s *Store) Contacts(ctx context.Context, userID int64) ([]models.Contact, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, phone, is_primary, created_at FROM contacts WHERE user_id = ? ORDER BY id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Contact
	for rows.Next() {
		var c models.Contact
		var primary int
		var created int64
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Phone, &primary, &created); err != nil {
			return nil, err
		}
		c.Primary = primary == 1
		c.CreatedAt = time.Unix(created, 0)
		out = append(out, c)
	}
	return out, rows.Err()
}

// ------------------------------------------------------------ incidents ----

// AddIncident records a prepared alert.
func (s *Store) AddIncident(ctx context.Context, i models.Incident) (models.Incident, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO incidents (user_id, kind, note, latitude, longitude, location_shared, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		i.UserID, i.Kind, i.Note, i.Latitude, i.Longitude, boolToInt(i.LocationShared), time.Now().Unix())
	if err != nil {
		return models.Incident{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.Incident{}, err
	}
	i.ID = id
	i.CreatedAt = time.Now()
	return i, nil
}

// Incidents lists the report history of a user, newest first.
func (s *Store) Incidents(ctx context.Context, userID int64, limit int) ([]models.Incident, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, kind, note, latitude, longitude, location_shared, created_at
		  FROM incidents WHERE user_id = ? ORDER BY id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Incident
	for rows.Next() {
		var i models.Incident
		var shared int
		var created int64
		if err := rows.Scan(&i.ID, &i.UserID, &i.Kind, &i.Note, &i.Latitude, &i.Longitude, &shared, &created); err != nil {
			return nil, err
		}
		i.LocationShared = shared == 1
		i.CreatedAt = time.Unix(created, 0)
		out = append(out, i)
	}
	return out, rows.Err()
}

// ClearIncidents removes the whole report history of a user.
func (s *Store) ClearIncidents(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM incidents WHERE user_id = ?`, userID)
	return err
}

// ---------------------------------------------------------------- stats ----

// Stats counts the rows in the database, used by the About screen.
func (s *Store) Stats(ctx context.Context) (map[string]int, error) {
	out := map[string]int{}
	for _, table := range []string{"users", "incidents", "pictures", "contacts"} {
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&n); err != nil {
			return nil, err
		}
		out[table] = n
	}
	return out, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
