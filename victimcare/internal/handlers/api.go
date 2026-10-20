package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"victimcare/internal/auth"
	"victimcare/internal/imaging"
	"victimcare/internal/models"
	"victimcare/internal/store"
)

// ------------------------------------------------------------------ auth ---

type signupRequest struct {
	FirstName  string `json:"firstName"`
	SecondName string `json:"secondName"`
	Email      string `json:"email"`
	State      string `json:"state"`
	Nation     string `json:"nation"`
	Password   string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "We could not read that request. Please try again.", "")
		return
	}
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.SecondName = strings.TrimSpace(req.SecondName)
	req.Email = strings.TrimSpace(req.Email)
	req.State = strings.TrimSpace(req.State)
	req.Nation = strings.TrimSpace(req.Nation)

	switch {
	case req.FirstName == "":
		writeError(w, http.StatusBadRequest, "Enter your first name.", "firstName")
		return
	case req.SecondName == "":
		writeError(w, http.StatusBadRequest, "Enter your second name.", "secondName")
		return
	case !validEmail(req.Email):
		writeError(w, http.StatusBadRequest, "Enter a valid email address.", "email")
		return
	case req.State == "":
		writeError(w, http.StatusBadRequest, "Enter your state of residence.", "state")
		return
	case req.Nation == "":
		writeError(w, http.StatusBadRequest, "Enter your nation of origin.", "nation")
		return
	case len(req.Password) < 8:
		writeError(w, http.StatusBadRequest, "Password must be at least 8 characters.", "password")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Something went wrong. Please try again.", "")
		return
	}

	user, err := s.store.CreateUser(r.Context(), models.User{
		FirstName:     req.FirstName,
		SecondName:    req.SecondName,
		Email:         req.Email,
		PasswordHash:  hash,
		State:         req.State,
		Nation:        req.Nation,
		Theme:         "light",
		Motion:        true,
		EmergencyNo:   "112",
		ShareLocation: true,
		Vibrate:       true,
		CountdownSecs: 5,
	})
	if err != nil {
		if errors.Is(err, store.ErrEmailTaken) {
			writeError(w, http.StatusConflict, "That email address is already registered.", "email")
			return
		}
		writeError(w, http.StatusInternalServerError, "We could not create the account. Please try again.", "")
		return
	}

	if err := s.sessions.Start(r.Context(), w, user.ID, s.cfg.SessionTTL); err != nil {
		writeError(w, http.StatusInternalServerError, "We could not start your session. Please try again.", "")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": s.publicUser(r.Context(), user)})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "We could not read that request. Please try again.", "")
		return
	}
	req.Email = strings.TrimSpace(req.Email)

	user, err := s.store.UserByEmail(r.Context(), req.Email)
	if err != nil || !auth.CheckPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "That email address and password do not match.", "")
		return
	}
	if err := s.sessions.Start(r.Context(), w, user.ID, s.cfg.SessionTTL); err != nil {
		writeError(w, http.StatusInternalServerError, "We could not start your session. Please try again.", "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": s.publicUser(r.Context(), user)})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.withSession(func(w http.ResponseWriter, r *http.Request) {
		if err := s.sessions.End(r.Context(), w, r); err != nil {
			writeError(w, http.StatusInternalServerError, "We could not sign you out. Please try again.", "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})(w, r)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		session := sessionFrom(r)
		writeJSON(w, http.StatusOK, map[string]any{
			"user":     s.publicUser(r.Context(), session.User),
			"csrf":     session.CSRF,
			"pictures": s.picturesOf(r, session.User.ID),
		})
	})(w, r)
}

// ------------------------------------------------------------------ user ---

type profileRequest struct {
	FirstName  string `json:"firstName"`
	SecondName string `json:"secondName"`
	Email      string `json:"email"`
	State      string `json:"state"`
	Nation     string `json:"nation"`
	Password   string `json:"password"`
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	s.requireCSRF(func(w http.ResponseWriter, r *http.Request) {
		var req profileRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "We could not read that request. Please try again.", "")
			return
		}
		session := sessionFrom(r)
		user := session.User

		req.FirstName = strings.TrimSpace(req.FirstName)
		req.SecondName = strings.TrimSpace(req.SecondName)
		req.Email = strings.TrimSpace(req.Email)
		req.State = strings.TrimSpace(req.State)
		req.Nation = strings.TrimSpace(req.Nation)

		if req.FirstName == "" {
			writeError(w, http.StatusBadRequest, "Enter your first name.", "firstName")
			return
		}
		if req.SecondName == "" {
			writeError(w, http.StatusBadRequest, "Enter your second name.", "secondName")
			return
		}
		if !validEmail(req.Email) {
			writeError(w, http.StatusBadRequest, "Enter a valid email address.", "email")
			return
		}
		if req.State == "" {
			writeError(w, http.StatusBadRequest, "Enter your state of residence.", "state")
			return
		}
		if req.Nation == "" {
			writeError(w, http.StatusBadRequest, "Enter your nation of origin.", "nation")
			return
		}

		// Changing the e-mail address requires the current password.
		if !strings.EqualFold(req.Email, user.Email) {
			if !auth.CheckPassword(user.PasswordHash, req.Password) {
				writeError(w, http.StatusForbidden, "Enter your current password to change your email address.", "password")
				return
			}
		}

		updated, err := s.store.UpdateProfile(r.Context(), user.ID, req.FirstName, req.SecondName, req.Email, req.State, req.Nation)
		if err != nil {
			if errors.Is(err, store.ErrEmailTaken) {
				writeError(w, http.StatusConflict, "That email address is already registered.", "email")
				return
			}
			writeError(w, http.StatusInternalServerError, "We could not save your profile. Please try again.", "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": s.publicUser(r.Context(), updated)})
	})(w, r)
}

type settingsRequest struct {
	Theme         string `json:"theme"`
	Motion        *bool  `json:"motion"`
	ShareLocation *bool  `json:"shareLocation"`
	Vibrate       *bool  `json:"vibrate"`
	EmergencyNo   string `json:"emergencyNumber"`
	CountdownSecs int    `json:"countdownSeconds"`
}

var validThemes = map[string]bool{"light": true, "dark": true, "calm": true, "contrast": true, "auto": true}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		session := sessionFrom(r)
		writeJSON(w, http.StatusOK, map[string]any{"settings": s.publicUser(r.Context(), session.User)})
	})(w, r)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	s.requireCSRF(func(w http.ResponseWriter, r *http.Request) {
		var req settingsRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "We could not read that request. Please try again.", "")
			return
		}
		session := sessionFrom(r)
		user := session.User

		theme := strings.TrimSpace(req.Theme)
		if theme == "" {
			theme = user.Theme
		}
		if !validThemes[theme] {
			writeError(w, http.StatusBadRequest, "That theme is not available.", "theme")
			return
		}

		motion := user.Motion
		if req.Motion != nil {
			motion = *req.Motion
		}
		shareLocation := user.ShareLocation
		if req.ShareLocation != nil {
			shareLocation = *req.ShareLocation
		}
		vibrate := user.Vibrate
		if req.Vibrate != nil {
			vibrate = *req.Vibrate
		}

		emergencyNo := strings.TrimSpace(req.EmergencyNo)
		if emergencyNo == "" {
			emergencyNo = "112"
		}
		if !validPhone(emergencyNo) {
			writeError(w, http.StatusBadRequest, "Enter a valid emergency number, for example 112.", "emergencyNumber")
			return
		}

		countdown := req.CountdownSecs
		if countdown != 3 && countdown != 5 && countdown != 10 {
			countdown = user.CountdownSecs
		}

		updated, err := s.store.UpdateSettings(r.Context(), user.ID, theme, motion, shareLocation, vibrate, emergencyNo, countdown)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "We could not save your settings. Please try again.", "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"settings": s.publicUser(r.Context(), updated)})
	})(w, r)
}

type passwordRequest struct {
	Current string `json:"currentPassword"`
	New     string `json:"newPassword"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	s.requireCSRF(func(w http.ResponseWriter, r *http.Request) {
		var req passwordRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "We could not read that request. Please try again.", "")
			return
		}
		session := sessionFrom(r)
		if !auth.CheckPassword(session.User.PasswordHash, req.Current) {
			writeError(w, http.StatusForbidden, "Your current password is not correct.", "currentPassword")
			return
		}
		if len(req.New) < 8 {
			writeError(w, http.StatusBadRequest, "Password must be at least 8 characters.", "newPassword")
			return
		}
		hash, err := auth.HashPassword(req.New)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Something went wrong. Please try again.", "")
			return
		}
		if err := s.store.UpdatePassword(r.Context(), session.User.ID, hash); err != nil {
			writeError(w, http.StatusInternalServerError, "We could not change your password. Please try again.", "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})(w, r)
}

type deleteAccountRequest struct {
	Password string `json:"password"`
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	s.requireCSRF(func(w http.ResponseWriter, r *http.Request) {
		var req deleteAccountRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "We could not read that request. Please try again.", "")
			return
		}
		session := sessionFrom(r)
		if !auth.CheckPassword(session.User.PasswordHash, req.Password) {
			writeError(w, http.StatusForbidden, "Enter your password to delete your account.", "password")
			return
		}

		pictures, _ := s.store.Pictures(r.Context(), session.User.ID)
		for _, p := range pictures {
			imaging.Remove(s.cfg.UploadDir, p.File, p.Thumb)
		}
		if err := s.store.DeleteUser(r.Context(), session.User.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "We could not delete your account. Please try again.", "")
			return
		}
		auth.ClearSessionCookies(w, s.cfg.SecureCookies)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})(w, r)
}

// -------------------------------------------------------------- pictures ---

func (s *Server) picturesOf(r *http.Request, userID int64) []models.Picture {
	pictures, err := s.store.Pictures(r.Context(), userID)
	if err != nil {
		return []models.Picture{}
	}
	out := make([]models.Picture, 0, len(pictures))
	for _, p := range pictures {
		out = append(out, decorate(p))
	}
	return out
}

func (s *Server) handleListPictures(w http.ResponseWriter, r *http.Request) {
	s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		session := sessionFrom(r)
		writeJSON(w, http.StatusOK, map[string]any{"pictures": s.picturesOf(r, session.User.ID)})
	})(w, r)
}

func (s *Server) handleUploadPicture(w http.ResponseWriter, r *http.Request) {
	s.requireCSRF(func(w http.ResponseWriter, r *http.Request) {
		session := sessionFrom(r)

		// The multipart reader caps the amount of memory used for the upload.
		r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes+(1<<20))
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "That picture is too large. Please choose a file under 5 MB.", "file")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "Choose a picture to upload.", "file")
			return
		}
		defer file.Close()

		if header.Size > s.cfg.MaxUploadBytes {
			writeError(w, http.StatusBadRequest, "That picture is too large. Please choose a file under 5 MB.", "file")
			return
		}

		result, err := imaging.Save(s.cfg.UploadDir, file)
		if err != nil {
			switch {
			case errors.Is(err, imaging.ErrTooLarge):
				writeError(w, http.StatusBadRequest, "That picture is too large. Please choose a file under 5 MB.", "file")
			case errors.Is(err, imaging.ErrTooSmall):
				writeError(w, http.StatusBadRequest, "That picture is too small. Use at least 128 x 128 pixels.", "file")
			default:
				writeError(w, http.StatusBadRequest, "That file is not a supported image. Use JPG or PNG.", "file")
			}
			return
		}

		existing, _ := s.store.Pictures(r.Context(), session.User.ID)
		picture, err := s.store.AddPicture(r.Context(), models.Picture{
			UserID:  session.User.ID,
			File:    result.File,
			Thumb:   result.Thumb,
			Primary: len(existing) == 0,
		})
		if err != nil {
			imaging.Remove(s.cfg.UploadDir, result.File, result.Thumb)
			writeError(w, http.StatusInternalServerError, "We could not save that picture. Please try again.", "")
			return
		}
		if picture.Primary {
			_ = s.store.SetPrimaryPicture(r.Context(), session.User.ID, picture.ID)
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"picture":  decorate(picture),
			"pictures": s.picturesOf(r, session.User.ID),
		})
	})(w, r)
}

func (s *Server) handleDeletePicture(w http.ResponseWriter, r *http.Request) {
	s.requireCSRF(func(w http.ResponseWriter, r *http.Request) {
		session := sessionFrom(r)
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "That picture could not be found.", "id")
			return
		}
		picture, err := s.store.Picture(r.Context(), session.User.ID, id)
		if err != nil {
			writeError(w, http.StatusNotFound, "That picture could not be found.", "id")
			return
		}
		if err := s.store.DeletePicture(r.Context(), session.User.ID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "We could not delete that picture. Please try again.", "")
			return
		}
		imaging.Remove(s.cfg.UploadDir, picture.File, picture.Thumb)

		// Promote another picture when the profile picture was removed.
		if picture.Primary {
			if remaining, err := s.store.Pictures(r.Context(), session.User.ID); err == nil && len(remaining) > 0 {
				_ = s.store.SetPrimaryPicture(r.Context(), session.User.ID, remaining[0].ID)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"pictures": s.picturesOf(r, session.User.ID)})
	})(w, r)
}

func (s *Server) handlePrimaryPicture(w http.ResponseWriter, r *http.Request) {
	s.requireCSRF(func(w http.ResponseWriter, r *http.Request) {
		session := sessionFrom(r)
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "That picture could not be found.", "id")
			return
		}
		if _, err := s.store.Picture(r.Context(), session.User.ID, id); err != nil {
			writeError(w, http.StatusNotFound, "That picture could not be found.", "id")
			return
		}
		if err := s.store.SetPrimaryPicture(r.Context(), session.User.ID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "We could not update your profile picture. Please try again.", "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"pictures": s.picturesOf(r, session.User.ID)})
	})(w, r)
}

// -------------------------------------------------------------- contacts ---

type contactRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

func (s *Server) handleListContacts(w http.ResponseWriter, r *http.Request) {
	s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		session := sessionFrom(r)
		contacts, err := s.store.Contacts(r.Context(), session.User.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "We could not load your contacts. Please try again.", "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"contacts": contacts})
	})(w, r)
}

func (s *Server) handleSaveContact(w http.ResponseWriter, r *http.Request) {
	s.requireCSRF(func(w http.ResponseWriter, r *http.Request) {
		var req contactRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "We could not read that request. Please try again.", "")
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Phone = strings.TrimSpace(req.Phone)
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "Enter the name of your contact.", "name")
			return
		}
		if !validPhone(req.Phone) {
			writeError(w, http.StatusBadRequest, "Enter a valid phone number, for example +2348012345678.", "phone")
			return
		}
		session := sessionFrom(r)
		contact, err := s.store.SetPrimaryContact(r.Context(), models.Contact{
			UserID: session.User.ID,
			Name:   req.Name,
			Phone:  req.Phone,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "We could not save your contact. Please try again.", "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"contact": contact})
	})(w, r)
}

func (s *Server) handleDeleteContact(w http.ResponseWriter, r *http.Request) {
	s.requireCSRF(func(w http.ResponseWriter, r *http.Request) {
		session := sessionFrom(r)
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "That contact could not be found.", "id")
			return
		}
		contacts, err := s.store.Contacts(r.Context(), session.User.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "We could not load your contacts. Please try again.", "")
			return
		}
		for _, c := range contacts {
			if c.ID == id {
				if _, err := s.store.SetPrimaryContact(r.Context(), models.Contact{UserID: session.User.ID, Name: c.Name, Phone: c.Phone}); err != nil {
					writeError(w, http.StatusInternalServerError, "We could not update your contacts. Please try again.", "")
					return
				}
				break
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})(w, r)
}

// ------------------------------------------------------------- incidents ---

type incidentRequest struct {
	Kind           string   `json:"kind"`
	Note           string   `json:"note"`
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
	LocationShared bool     `json:"locationShared"`
}

var validIncidentKinds = map[string]bool{
	"Accident": true, "Medical": true, "Assault": true, "Fire": true,
	"Other": true, "SOS emergency": true,
}

func (s *Server) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		session := sessionFrom(r)
		reports, err := s.store.Incidents(r.Context(), session.User.ID, 20)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "We could not load your reports. Please try again.", "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"incidents": reports})
	})(w, r)
}

func (s *Server) handleCreateIncident(w http.ResponseWriter, r *http.Request) {
	s.requireCSRF(func(w http.ResponseWriter, r *http.Request) {
		var req incidentRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "We could not read that request. Please try again.", "")
			return
		}
		req.Kind = strings.TrimSpace(req.Kind)
		if !validIncidentKinds[req.Kind] {
			writeError(w, http.StatusBadRequest, "Choose what kind of problem this is.", "kind")
			return
		}
		req.Note = strings.TrimSpace(req.Note)
		if len(req.Note) > 1000 {
			writeError(w, http.StatusBadRequest, "Keep the description under 1000 characters.", "note")
			return
		}
		session := sessionFrom(r)
		incident, err := s.store.AddIncident(r.Context(), models.Incident{
			UserID:         session.User.ID,
			Kind:           req.Kind,
			Note:           req.Note,
			Latitude:       req.Latitude,
			Longitude:      req.Longitude,
			LocationShared: req.LocationShared && req.Latitude != nil && req.Longitude != nil,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "We could not save that report. Please try again.", "")
			return
		}
		reports, _ := s.store.Incidents(r.Context(), session.User.ID, 20)
		writeJSON(w, http.StatusCreated, map[string]any{"incident": incident, "incidents": reports})
	})(w, r)
}

func (s *Server) handleClearIncidents(w http.ResponseWriter, r *http.Request) {
	s.requireCSRF(func(w http.ResponseWriter, r *http.Request) {
		session := sessionFrom(r)
		if err := s.store.ClearIncidents(r.Context(), session.User.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "We could not clear your reports. Please try again.", "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"incidents": []models.Incident{}})
	})(w, r)
}

// ------------------------------------------------------------ validation ---

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s.]+\.[^@\s]+$`)

func validEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" || len(email) > 160 {
		return false
	}
	return emailPattern.MatchString(email)
}

var phonePattern = regexp.MustCompile(`^[0-9+][0-9\s\-().]{1,23}$`)

func validPhone(phone string) bool {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return false
	}
	if !phonePattern.MatchString(phone) {
		return false
	}
	digits := 0
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	// Emergency numbers can be as short as 112, contact numbers are longer.
	return digits >= 2
}
