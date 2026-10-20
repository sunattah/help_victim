// Package models contains the domain types shared by the store and handlers.
package models

import (
	"strings"
	"time"
	"unicode"
)

// User is a registered person who can use the app.
type User struct {
	ID            int64
	FirstName     string
	SecondName    string
	Email         string
	PasswordHash  string
	State         string
	Nation        string
	Country       string
	Theme         string
	Motion        bool
	EmergencyNo   string
	ShareLocation bool
	Vibrate       bool
	CountdownSecs int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// FullName returns the display name of the user.
func (u User) FullName() string {
	return strings.TrimSpace(strings.TrimSpace(u.FirstName) + " " + strings.TrimSpace(u.SecondName))
}

// Initials returns up to two initials, used when no profile picture is set.
func (u User) Initials() string {
	initials := ""
	for _, part := range []string{u.FirstName, u.SecondName} {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, r := range part {
			if unicode.IsLetter(r) {
				initials += strings.ToUpper(string(r))
				break
			}
		}
		if len(initials) == 2 {
			break
		}
	}
	if initials == "" {
		return "VC"
	}
	return initials
}

// PublicUser is the JSON shape of a user that is safe to send to the browser.
type PublicUser struct {
	ID            int64  `json:"id"`
	FirstName     string `json:"firstName"`
	SecondName    string `json:"secondName"`
	FullName      string `json:"fullName"`
	Email         string `json:"email"`
	State         string `json:"state"`
	Nation        string `json:"nation"`
	Country       string `json:"country"`
	Theme         string `json:"theme"`
	Motion        bool   `json:"motion"`
	EmergencyNo   string `json:"emergencyNumber"`
	ShareLocation bool   `json:"shareLocation"`
	Vibrate       bool   `json:"vibrate"`
	CountdownSecs int    `json:"countdownSeconds"`
	AvatarURL     string `json:"avatarUrl"`
	Initials      string `json:"initials"`
	CreatedAt     string `json:"createdAt"`
}

// Picture is an image the user uploaded to their profile.
type Picture struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"userId"`
	File      string    `json:"-"`
	Thumb     string    `json:"-"`
	Primary   bool      `json:"primary"`
	CreatedAt time.Time `json:"createdAt"`
	URL       string    `json:"url"`
	ThumbURL  string    `json:"thumbUrl"`
}

// Contact is someone the user can alert.
type Contact struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"userId"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Primary   bool      `json:"primary"`
	CreatedAt time.Time `json:"createdAt"`
}

// Incident is a prepared alert recorded in the report history.
type Incident struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"userId"`
	Kind           string    `json:"kind"`
	Note           string    `json:"note"`
	Latitude       *float64  `json:"latitude"`
	Longitude      *float64  `json:"longitude"`
	LocationShared bool      `json:"locationShared"`
	CreatedAt      time.Time `json:"createdAt"`
}
