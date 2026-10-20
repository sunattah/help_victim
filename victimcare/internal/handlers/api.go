package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"victimcare/internal/auth"
	"victimcare/internal/countries"
	"victimcare/internal/imaging"
	"victimcare/internal/models"
	"victimcare/internal/store"
)

// ------------------------------------------------------------------ auth ---

type signupRequest struct {
	FirstName  string `json:"firstName"`
	SecondName string `json:"secondName"`
	Email      string `json:"email"`
	Country    string `json:"country"`
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
	req.Country = strings.TrimSpace(req.Country)
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
		writeError(w, http.StatusBadRequest, "Enter a valid email address, for example name@example.com.", "email")
		return
	case req.Country == "":
		writeError(w, http.StatusBadRequest, "Choose your country of residence.", "country")
		return
	case !validState(req.Country, req.State):
		writeError(w, http.StatusBadRequest, "Choose a state or region for the country you picked.", "state")
		return
	case req.Nation == "":
		writeError(w, http.StatusBadRequest, "Enter your nation of origin.", "nation")
		return
	}
	if problem := passwordProblem(req.Password); problem != "" {
		writeError(w, http.StatusBadRequest, problem, "password")
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
		Country:       req.Country,
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
	Country    string `json:"country"`
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
		req.Country = strings.TrimSpace(req.Country)
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
			writeError(w, http.StatusBadRequest, "Enter a valid email address, for example name@example.com.", "email")
			return
		}
		if req.Country == "" {
			writeError(w, http.StatusBadRequest, "Choose your country of residence.", "country")
			return
		}
		if !validState(req.Country, req.State) {
			writeError(w, http.StatusBadRequest, "Choose a state or region for the country you picked.", "state")
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

		updated, err := s.store.UpdateProfile(r.Context(), user.ID, req.FirstName, req.SecondName, req.Email, req.State, req.Nation, req.Country)
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
		if problem := passwordProblem(req.New); problem != "" {
			writeError(w, http.StatusBadRequest, problem, "newPassword")
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
	ID    int64  `json:"id"`
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

// handleSaveContact adds a new contact or updates an existing one. The first
// contact a user saves becomes the primary contact used for text alerts.
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

		var contact models.Contact
		var err error
		if req.ID > 0 {
			contact, err = s.store.UpdateContact(r.Context(), session.User.ID, req.ID, req.Name, req.Phone)
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "That contact could not be found.", "id")
				return
			}
		} else {
			contact, err = s.store.AddContact(r.Context(), models.Contact{
				UserID: session.User.ID,
				Name:   req.Name,
				Phone:  req.Phone,
			})
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "We could not save your contact. Please try again.", "")
			return
		}
		contacts, _ := s.store.Contacts(r.Context(), session.User.ID)
		writeJSON(w, http.StatusOK, map[string]any{"contact": contact, "contacts": contacts})
	})(w, r)
}

// handleMakePrimaryContact picks the contact that text alerts go to.
func (s *Server) handleMakePrimaryContact(w http.ResponseWriter, r *http.Request) {
	s.requireCSRF(func(w http.ResponseWriter, r *http.Request) {
		session := sessionFrom(r)
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "That contact could not be found.", "id")
			return
		}
		if _, err := s.store.ContactByID(r.Context(), session.User.ID, id); err != nil {
			writeError(w, http.StatusNotFound, "That contact could not be found.", "id")
			return
		}
		if err := s.store.MakePrimaryContact(r.Context(), session.User.ID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "We could not update your contacts. Please try again.", "")
			return
		}
		contacts, _ := s.store.Contacts(r.Context(), session.User.ID)
		writeJSON(w, http.StatusOK, map[string]any{"contacts": contacts})
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
		if err := s.store.DeleteContact(r.Context(), session.User.ID, id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "That contact could not be found.", "id")
				return
			}
			writeError(w, http.StatusInternalServerError, "We could not delete that contact. Please try again.", "")
			return
		}
		contacts, _ := s.store.Contacts(r.Context(), session.User.ID)
		writeJSON(w, http.StatusOK, map[string]any{"contacts": contacts})
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

// emailLocal and emailDomain describe what a real email address may contain.
// The address must be an email address and nothing else: no spaces, no
// second @, no missing or invented top level domain.
var (
	emailLocalPattern  = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._%+-]*[A-Za-z0-9])?$`)
	emailDomainPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)*$`)
	emailTLDPattern    = regexp.MustCompile(`^[A-Za-z]{2,24}$`)
)

// validEmail reports whether the value is a well formed email address.
func validEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" || len(email) > 160 {
		return false
	}
	if strings.ContainsAny(email, " \t\r\n") {
		return false
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	local, domain := parts[0], parts[1]
	if local == "" || len(local) > 64 || domain == "" || len(domain) > 255 {
		return false
	}
	if strings.Contains(local, "..") || strings.Contains(domain, "..") {
		return false
	}
	if !emailLocalPattern.MatchString(local) {
		return false
	}
	if !emailDomainPattern.MatchString(domain) {
		return false
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}
	return emailTLDPattern.MatchString(labels[len(labels)-1])
}

// passwordProblem checks a password against the account rules and returns the
// reason it is not acceptable, or an empty string when the password is fine.
//
// A password must be at least 8 characters long and contain both letters and
// digits.
func passwordProblem(password string) string {
	if len(password) < 8 {
		return "Password is incorrect. It must be at least 8 characters long."
	}
	if len(password) > 128 {
		return "Password is incorrect. It must be at most 128 characters long."
	}
	if !hasLetter(password) {
		return "Password is incorrect. It must contain at least one letter."
	}
	if !hasDigit(password) {
		return "Password is incorrect. It must contain at least one number."
	}
	return ""
}

func hasLetter(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func hasDigit(value string) bool {
	for _, r := range value {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// validState checks that a state or region belongs to the chosen country.
// Countries that have no known list of states accept any short free text.
func validState(country, state string) bool {
	state = strings.TrimSpace(state)
	if state == "" || len(state) > 80 {
		return false
	}
	regions, known := countries.RegionsFor(country)
	if !known {
		return true
	}
	for _, region := range regions {
		if strings.EqualFold(region, state) {
			return true
		}
	}
	return false
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
