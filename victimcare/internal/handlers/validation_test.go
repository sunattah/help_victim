package handlers

import "testing"

func TestValidEmail(t *testing.T) {
	cases := map[string]bool{
		"amaka@example.com":      true,
		"a.b+tag@sub.domain.org": true,
		"amaka@example":          false,
		"":                       false,
		"no-at-sign.com":         false,
		"spaces in@mail.com":     false,
	}
	for input, want := range cases {
		if got := validEmail(input); got != want {
			t.Errorf("validEmail(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestValidPhone(t *testing.T) {
	cases := map[string]bool{
		"112":               true,
		"+2348012345678":    true,
		"+234 801 234 5678": true,
		"08012345678":       true,
		"":                  false,
		"abc":               false,
		"1":                 false,
	}
	for input, want := range cases {
		if got := validPhone(input); got != want {
			t.Errorf("validPhone(%q) = %v, want %v", input, got, want)
		}
	}
}
