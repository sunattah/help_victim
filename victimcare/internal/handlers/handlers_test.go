package handlers

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"victimcare/internal/config"
	"victimcare/internal/store"
)

// newTestServer spins up a server backed by a temporary database and upload
// directory, and returns an HTTP client that keeps the session cookie.
func newTestServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()

	root := t.TempDir()
	cfg := config.Config{
		Addr:           ":0",
		DBPath:         filepath.Join(root, "test.db"),
		UploadDir:      filepath.Join(root, "uploads"),
		Secret:         []byte("test-secret"),
		SecureCookies:  false,
		MaxUploadBytes: 5 << 20,
		SessionTTL:     time.Hour,
		TemplateDir:    filepath.Join("..", "..", "templates"),
		StaticDir:      filepath.Join("..", "..", "static"),
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	srv, err := New(cfg, st)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	testServer := httptest.NewServer(srv)
	t.Cleanup(testServer.Close)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	return testServer, client
}

// csrfToken reads the CSRF token that the server handed to the browser.
func csrfToken(t *testing.T, client *http.Client, base string) string {
	t.Helper()
	resp, err := client.Get(base + "/api/v1/auth/me")
	if err != nil {
		t.Fatalf("get me: %v", err)
	}
	defer resp.Body.Close()
	var payload struct {
		CSRF string `json:"csrf"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	return payload.CSRF
}

func postJSON(t *testing.T, client *http.Client, url, csrf, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	payload := map[string]any{}
	data, _ := io.ReadAll(resp.Body)
	json.Unmarshal(data, &payload)
	return resp.StatusCode, payload
}

func deleteJSON(t *testing.T, client *http.Client, url, csrf string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	payload := map[string]any{}
	data, _ := io.ReadAll(resp.Body)
	json.Unmarshal(data, &payload)
	return resp.StatusCode, payload
}

func putJSON(t *testing.T, client *http.Client, url, csrf, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	payload := map[string]any{}
	data, _ := io.ReadAll(resp.Body)
	json.Unmarshal(data, &payload)
	return resp.StatusCode, payload
}

func getJSON(t *testing.T, client *http.Client, url string) (int, map[string]any) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	defer resp.Body.Close()
	payload := map[string]any{}
	data, _ := io.ReadAll(resp.Body)
	json.Unmarshal(data, &payload)
	return resp.StatusCode, payload
}

func TestSignupLoginAndSession(t *testing.T) {
	server, client := newTestServer(t)

	status, payload := postJSON(t, client, server.URL+"/api/v1/auth/signup", "",
		`{"firstName":"Amaka","secondName":"Obi","email":"amaka@example.com","country":"Nigeria","state":"Lagos","nation":"Nigeria","password":"secret1234"}`)
	if status != http.StatusCreated {
		t.Fatalf("signup status = %d, payload = %v", status, payload)
	}

	user, ok := payload["user"].(map[string]any)
	if !ok {
		t.Fatalf("signup did not return a user: %v", payload)
	}
	if user["email"] != "amaka@example.com" {
		t.Fatalf("unexpected email: %v", user["email"])
	}

	csrf := csrfToken(t, client, server.URL)
	if csrf == "" {
		t.Fatal("expected a csrf token")
	}

	// A second signup with the same address must be refused.
	other, _ := cookiejar.New(nil)
	otherClient := &http.Client{Jar: other}
	status, _ = postJSON(t, otherClient, server.URL+"/api/v1/auth/signup", "",
		`{"firstName":"Amaka","secondName":"Obi","email":"amaka@example.com","country":"Nigeria","state":"Lagos","nation":"Nigeria","password":"secret1234"}`)
	if status != http.StatusConflict {
		t.Fatalf("duplicate signup status = %d, want %d", status, http.StatusConflict)
	}

	// Logging out ends the session.
	status, _ = postJSON(t, client, server.URL+"/api/v1/auth/logout", csrf, `{}`)
	if status != http.StatusOK {
		t.Fatalf("logout status = %d", status)
	}
	resp, err := client.Get(server.URL + "/api/v1/auth/me")
	if err != nil {
		t.Fatalf("get me after logout: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me after logout status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	server, client := newTestServer(t)

	postJSON(t, client, server.URL+"/api/v1/auth/signup", "",
		`{"firstName":"Tunde","secondName":"Ayo","email":"tunde@example.com","country":"Nigeria","state":"Abuja","nation":"Nigeria","password":"secret1234"}`)

	fresh, _ := cookiejar.New(nil)
	freshClient := &http.Client{Jar: fresh}
	status, _ := postJSON(t, freshClient, server.URL+"/api/v1/auth/login", "",
		`{"email":"tunde@example.com","password":"wrongpassword"}`)
	if status != http.StatusUnauthorized {
		t.Fatalf("login status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestProfileAndSettingsUpdate(t *testing.T) {
	server, client := newTestServer(t)
	postJSON(t, client, server.URL+"/api/v1/auth/signup", "",
		`{"firstName":"Amaka","secondName":"Obi","email":"amaka@example.com","country":"Nigeria","state":"Lagos","nation":"Nigeria","password":"secret1234"}`)
	csrf := csrfToken(t, client, server.URL)

	req, _ := http.NewRequest(http.MethodPatch, server.URL+"/api/v1/users/me",
		bytes.NewBufferString(`{"firstName":"Amaka","secondName":"Obi","email":"amaka@example.com","country":"Nigeria","state":"Enugu","nation":"Nigeria","password":""}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("patch profile: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch profile status = %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodPut, server.URL+"/api/v1/users/me/settings",
		bytes.NewBufferString(`{"theme":"dark","motion":false,"shareLocation":false,"vibrate":true,"emergencyNumber":"112","countdownSeconds":10}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("put settings: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put settings status = %d", resp.StatusCode)
	}

	var payload struct {
		Settings map[string]any `json:"settings"`
	}
	json.NewDecoder(resp.Body).Decode(&payload)
	if payload.Settings["theme"] != "dark" {
		t.Fatalf("theme = %v, want dark", payload.Settings["theme"])
	}
	if payload.Settings["countdownSeconds"].(float64) != 10 {
		t.Fatalf("countdown = %v, want 10", payload.Settings["countdownSeconds"])
	}

	// An unknown theme must be refused.
	req, _ = http.NewRequest(http.MethodPut, server.URL+"/api/v1/users/me/settings",
		bytes.NewBufferString(`{"theme":"neon","emergencyNumber":"112","countdownSeconds":5}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("put bad settings: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown theme status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestCSRFIsRequiredForMutations(t *testing.T) {
	server, client := newTestServer(t)
	postJSON(t, client, server.URL+"/api/v1/auth/signup", "",
		`{"firstName":"Amaka","secondName":"Obi","email":"amaka@example.com","country":"Nigeria","state":"Lagos","nation":"Nigeria","password":"secret1234"}`)

	status, payload := postJSON(t, client, server.URL+"/api/v1/contacts", "",
		`{"name":"Chidi","phone":"+2348012345678"}`)
	if status != http.StatusForbidden {
		t.Fatalf("contact without csrf status = %d, want %d (%v)", status, http.StatusForbidden, payload)
	}
}

func TestContactAndIncidentFlow(t *testing.T) {
	server, client := newTestServer(t)
	postJSON(t, client, server.URL+"/api/v1/auth/signup", "",
		`{"firstName":"Amaka","secondName":"Obi","email":"amaka@example.com","country":"Nigeria","state":"Lagos","nation":"Nigeria","password":"secret1234"}`)
	csrf := csrfToken(t, client, server.URL)

	status, payload := postJSON(t, client, server.URL+"/api/v1/contacts", csrf,
		`{"name":"Chidi","phone":"+2348012345678"}`)
	if status != http.StatusOK {
		t.Fatalf("contact status = %d (%v)", status, payload)
	}

	status, payload = postJSON(t, client, server.URL+"/api/v1/incidents", csrf,
		`{"kind":"Medical","note":"Fell on the stairs","latitude":6.52,"longitude":3.37,"locationShared":true}`)
	if status != http.StatusCreated {
		t.Fatalf("incident status = %d (%v)", status, payload)
	}
	incidents, ok := payload["incidents"].([]any)
	if !ok || len(incidents) != 1 {
		t.Fatalf("expected one incident, got %v", payload["incidents"])
	}

	// An unknown kind is refused.
	status, _ = postJSON(t, client, server.URL+"/api/v1/incidents", csrf, `{"kind":"Disaster","note":"x"}`)
	if status != http.StatusBadRequest {
		t.Fatalf("bad kind status = %d, want %d", status, http.StatusBadRequest)
	}
}

func TestPictureUpload(t *testing.T) {
	server, client := newTestServer(t)
	postJSON(t, client, server.URL+"/api/v1/auth/signup", "",
		`{"firstName":"Amaka","secondName":"Obi","email":"amaka@example.com","country":"Nigeria","state":"Lagos","nation":"Nigeria","password":"secret1234"}`)
	csrf := csrfToken(t, client, server.URL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "picture.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 300, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 300; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	png.Encode(part, img)
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/users/me/pictures", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload status = %d (%s)", resp.StatusCode, data)
	}

	var payload struct {
		Picture  map[string]any   `json:"picture"`
		Pictures []map[string]any `json:"pictures"`
	}
	json.NewDecoder(resp.Body).Decode(&payload)
	if payload.Picture["primary"] != true {
		t.Fatalf("the first picture should be the profile picture: %v", payload.Picture)
	}

	// The stored file must be readable through the media route.
	pictureID := int(payload.Picture["id"].(float64))
	resp, err = client.Get(server.URL + "/media/picture/" + itoa(pictureID))
	if err != nil {
		t.Fatalf("get picture: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("picture status = %d", resp.StatusCode)
	}
}

func TestUploadedFileMustBeAnImage(t *testing.T) {
	server, client := newTestServer(t)
	postJSON(t, client, server.URL+"/api/v1/auth/signup", "",
		`{"firstName":"Amaka","secondName":"Obi","email":"amaka@example.com","country":"Nigeria","state":"Lagos","nation":"Nigeria","password":"secret1234"}`)
	csrf := csrfToken(t, client, server.URL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "evil.png")
	part.Write([]byte("#!/bin/sh\necho pwned\n"))
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/users/me/pictures", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("upload status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestIndexPageRenders(t *testing.T) {
	server, client := newTestServer(t)

	resp, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("get index: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("index status = %d", resp.StatusCode)
	}
	for _, want := range []string{"Victim Care", "picture-area", "open-signup", "id=\"bootstrap\""} {
		if !bytes.Contains(body, []byte(want)) {
			t.Fatalf("index page is missing %q", want)
		}
	}

	postJSON(t, client, server.URL+"/api/v1/auth/signup", "",
		`{"firstName":"Amaka","secondName":"Obi","email":"amaka@example.com","country":"Nigeria","state":"Lagos","nation":"Nigeria","password":"secret1234"}`)

	resp, err = client.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("get index signed in: %v", err)
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	for _, want := range []string{"settings-card", "theme-swatch", "settings-signout", "upload-button"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Fatalf("signed in page is missing %q", want)
		}
	}
}

func TestStaticAssetsAreServed(t *testing.T) {
	server, client := newTestServer(t)
	for _, path := range []string{"/static/css/app.css", "/static/js/app.js", "/static/img/slides/care.svg"} {
		resp, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d", path, resp.StatusCode)
		}
	}
}

func TestGuidesEndpoint(t *testing.T) {
	server, client := newTestServer(t)
	resp, err := client.Get(server.URL + "/api/v1/guides")
	if err != nil {
		t.Fatalf("get guides: %v", err)
	}
	defer resp.Body.Close()
	var payload struct {
		Guides []map[string]any `json:"guides"`
	}
	json.NewDecoder(resp.Body).Decode(&payload)
	if len(payload.Guides) < 10 {
		t.Fatalf("expected at least 10 first aid guides, got %d", len(payload.Guides))
	}
	guideIDs := make(map[string]bool)
	for _, g := range payload.Guides {
		if id, ok := g["id"].(string); ok {
			guideIDs[id] = true
		}
	}
	for _, expectedID := range []string{"bleeding", "burns", "choking", "breathing", "bone", "shock", "anaphylaxis", "poisoning", "seizure", "heatstroke"} {
		if !guideIDs[expectedID] {
			t.Errorf("missing guide %s", expectedID)
		}
	}
}

func TestMedicalProfileFlow(t *testing.T) {
	server, client := newTestServer(t)
	postJSON(t, client, server.URL+"/api/v1/auth/signup", "", `{
		"firstName": "Ada",
		"secondName": "Lovelace",
		"email": "ada.medical@example.com",
		"country": "United Kingdom",
		"state": "England",
		"nation": "British",
		"password": "Password123"
	}`)

	csrf := csrfToken(t, client, server.URL)

	// Get medical profile initially empty
	status, payload := getJSON(t, client, server.URL+"/api/v1/users/me/medical")
	if status != http.StatusOK {
		t.Fatalf("get medical: status %d", status)
	}
	med, ok := payload["medical"].(map[string]any)
	if !ok || med["bloodType"] != "" {
		t.Fatalf("expected empty initial blood type: %v", payload)
	}

	// Update medical profile
	putBody := `{
		"bloodType": "O+",
		"allergies": "Penicillin, Peanuts",
		"medications": "Albuterol inhaler",
		"medicalNotes": "Mild asthma when running",
		"shareMedical": true
	}`
	status, payload = putJSON(t, client, server.URL+"/api/v1/users/me/medical", csrf, putBody)
	if status != http.StatusOK {
		t.Fatalf("put medical status = %d: %v", status, payload)
	}
	med = payload["medical"].(map[string]any)
	if med["bloodType"] != "O+" || med["allergies"] != "Penicillin, Peanuts" || med["shareMedical"] != true {
		t.Fatalf("unexpected updated medical: %v", med)
	}

	// Check /api/v1/auth/me has medical fields
	status, mePayload := getJSON(t, client, server.URL+"/api/v1/auth/me")
	if status != http.StatusOK {
		t.Fatalf("get me status = %d", status)
	}
	user := mePayload["user"].(map[string]any)
	if user["bloodType"] != "O+" || user["shareMedical"] != true {
		t.Fatalf("user in /me missing medical fields: %v", user)
	}
}

func TestSingleIncidentDeleteFlow(t *testing.T) {
	server, client := newTestServer(t)
	postJSON(t, client, server.URL+"/api/v1/auth/signup", "", `{
		"firstName": "Grace",
		"secondName": "Hopper",
		"email": "grace.incidents@example.com",
		"country": "United States",
		"state": "Virginia",
		"nation": "American",
		"password": "Password123"
	}`)

	csrf := csrfToken(t, client, server.URL)

	// Create 2 incidents
	postJSON(t, client, server.URL+"/api/v1/incidents", csrf, `{"kind": "Accident", "note": "Report 1"}`)
	_, rep2 := postJSON(t, client, server.URL+"/api/v1/incidents", csrf, `{"kind": "Medical", "note": "Report 2"}`)
	inc2 := rep2["incident"].(map[string]any)
	inc2ID := int64(inc2["id"].(float64))

	// Delete incident 2
	status, payload := deleteJSON(t, client, server.URL+"/api/v1/incidents/"+strconv.FormatInt(inc2ID, 10), csrf)
	if status != http.StatusOK {
		t.Fatalf("delete incident status = %d: %v", status, payload)
	}
	incidents := payload["incidents"].([]any)
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident after single delete, got %d", len(incidents))
	}

	// Delete again should return 404
	status, _ = deleteJSON(t, client, server.URL+"/api/v1/incidents/"+strconv.FormatInt(inc2ID, 10), csrf)
	if status != http.StatusNotFound {
		t.Fatalf("expected 404 deleting nonexistent incident, got %d", status)
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	ip := "192.168.1.100"
	if !rl.Allow(ip) || !rl.Allow(ip) || !rl.Allow(ip) {
		t.Fatal("first 3 attempts should be allowed")
	}
	if rl.Allow(ip) {
		t.Fatal("4th attempt should be blocked")
	}
	// Different IP should still be allowed
	if !rl.Allow("192.168.1.101") {
		t.Fatal("different IP should be allowed")
	}
}

func TestDatabaseFileIsCreated(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{DBPath: filepath.Join(root, "nested", "db.sqlite")}
	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if _, err := os.Stat(cfg.DBPath); err != nil {
		t.Fatalf("database file missing: %v", err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
