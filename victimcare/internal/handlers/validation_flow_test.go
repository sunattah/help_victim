package handlers

import (
	"io"
	"net/http"
	"testing"
)

func TestSignupRejectsBadEmail(t *testing.T) {
	server, client := newTestServer(t)
	cases := []string{
		"amaka",
		"amaka@example",
		"amaka@",
		"@example.com",
		"amaka @example.com",
		"amaka@@example.com",
		"amaka..obi@example.com",
		"amaka@example..com",
		"amaka@example.c",
		"amaka@example.123",
	}
	for _, email := range cases {
		body := `{"firstName":"Amaka","secondName":"Obi","email":"` + email +
			`","country":"Nigeria","state":"Lagos","nation":"Nigeria","password":"secret1234"}`
		status, payload := postJSON(t, client, server.URL+"/api/v1/auth/signup", "", body)
		if status != http.StatusBadRequest {
			t.Errorf("signup with email %q status = %d, want %d (%v)", email, status, http.StatusBadRequest, payload)
		}
		if payload["field"] != "email" {
			t.Errorf("signup with email %q reported field %v, want email", email, payload["field"])
		}
	}
}

func TestSignupAcceptsRealEmail(t *testing.T) {
	server, client := newTestServer(t)
	body := `{"firstName":"Amaka","secondName":"Obi","email":"amaka.obi+help@example.co.uk",` +
		`"country":"Nigeria","state":"Lagos","nation":"Nigeria","password":"secret1234"}`
	status, payload := postJSON(t, client, server.URL+"/api/v1/auth/signup", "", body)
	if status != http.StatusCreated {
		t.Fatalf("signup status = %d, want %d (%v)", status, http.StatusCreated, payload)
	}
}

func TestSignupRejectsWeakPassword(t *testing.T) {
	server, client := newTestServer(t)
	cases := []string{
		"short1",     // too short
		"nodigitsat", // no number
		"12345678",   // no letter
		"abcdefgh",   // no number
	}
	for _, password := range cases {
		body := `{"firstName":"Amaka","secondName":"Obi","email":"weak@example.com",` +
			`"country":"Nigeria","state":"Lagos","nation":"Nigeria","password":"` + password + `"}`
		status, payload := postJSON(t, client, server.URL+"/api/v1/auth/signup", "", body)
		if status != http.StatusBadRequest {
			t.Errorf("signup with password %q status = %d, want %d", password, status, http.StatusBadRequest)
		}
		if payload["field"] != "password" {
			t.Errorf("signup with password %q reported field %v, want password", password, payload["field"])
		}
	}
}

func TestSignupRejectsStateFromAnotherCountry(t *testing.T) {
	server, client := newTestServer(t)
	body := `{"firstName":"Amaka","secondName":"Obi","email":"amaka@example.com",` +
		`"country":"Nigeria","state":"California","nation":"Nigeria","password":"secret1234"}`
	status, payload := postJSON(t, client, server.URL+"/api/v1/auth/signup", "", body)
	if status != http.StatusBadRequest {
		t.Fatalf("signup status = %d, want %d (%v)", status, http.StatusBadRequest, payload)
	}
	if payload["field"] != "state" {
		t.Fatalf("reported field %v, want state", payload["field"])
	}
}

func TestCountriesEndpoint(t *testing.T) {
	server, client := newTestServer(t)
	resp, err := client.Get(server.URL + "/api/v1/countries")
	if err != nil {
		t.Fatalf("get countries: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("countries status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read countries: %v", err)
	}
	if !containsString(string(body), "Nigeria") {
		t.Fatal("the country list does not contain Nigeria")
	}
}

func TestMultipleContacts(t *testing.T) {
	server, client := newTestServer(t)
	postJSON(t, client, server.URL+"/api/v1/auth/signup", "",
		`{"firstName":"Amaka","secondName":"Obi","email":"amaka@example.com","country":"Nigeria","state":"Lagos","nation":"Nigeria","password":"secret1234"}`)
	csrf := csrfToken(t, client, server.URL)

	status, payload := postJSON(t, client, server.URL+"/api/v1/contacts", csrf,
		`{"name":"Chidi","phone":"+2348012345678"}`)
	if status != http.StatusOK {
		t.Fatalf("first contact status = %d (%v)", status, payload)
	}
	contacts := payload["contacts"].([]any)
	if len(contacts) != 1 {
		t.Fatalf("expected one contact, got %d", len(contacts))
	}
	first := contacts[0].(map[string]any)
	if first["primary"] != true {
		t.Fatal("the first contact should be primary")
	}

	// A second contact is added and becomes primary only when asked.
	status, payload = postJSON(t, client, server.URL+"/api/v1/contacts", csrf,
		`{"name":"Ada","phone":"+2348098765432"}`)
	if status != http.StatusOK {
		t.Fatalf("second contact status = %d (%v)", status, payload)
	}
	contacts = payload["contacts"].([]any)
	if len(contacts) != 2 {
		t.Fatalf("expected two contacts, got %d", len(contacts))
	}

	// Switch the primary contact.
	secondID := int(contacts[1].(map[string]any)["id"].(float64))
	status, payload = postJSON(t, client, server.URL+"/api/v1/contacts/"+itoa(secondID)+"/primary", csrf, `{}`)
	if status != http.StatusOK {
		t.Fatalf("make primary status = %d (%v)", status, payload)
	}
	contacts = payload["contacts"].([]any)
	for _, raw := range contacts {
		contact := raw.(map[string]any)
		want := int(contact["id"].(float64)) == secondID
		if contact["primary"] != want {
			t.Fatalf("contact %v has primary=%v, want %v", contact["id"], contact["primary"], want)
		}
	}

	// Delete one contact again.
	status, payload = deleteJSON(t, client, server.URL+"/api/v1/contacts/"+itoa(secondID), csrf)
	if status != http.StatusOK {
		t.Fatalf("delete contact status = %d (%v)", status, payload)
	}
	contacts = payload["contacts"].([]any)
	if len(contacts) != 1 {
		t.Fatalf("expected one contact after the delete, got %d", len(contacts))
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
