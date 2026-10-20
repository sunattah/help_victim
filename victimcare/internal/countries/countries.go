// Package countries holds the country and state/region list used by the
// signup and profile forms.
//
// The list is stored as JSON and embedded into the binary, so the app has no
// runtime dependency on a network service. Countries without a known list of
// states fall back to a free-text "other" entry so the form always works.
package countries

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed data.json
var raw []byte

// Country is a country with its states, provinces or regions.
type Country struct {
	Name    string   `json:"name"`
	Regions []string `json:"regions"`
}

var (
	once    sync.Once
	ordered []Country
	byName  map[string][]string
	loadErr error
)

func load() {
	once.Do(func() {
		var payload struct {
			Countries []Country `json:"countries"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			loadErr = err
			return
		}

		byName = make(map[string][]string, len(payload.Countries))
		seen := make(map[string]bool, len(payload.Countries))
		for _, country := range payload.Countries {
			name := strings.TrimSpace(country.Name)
			if name == "" || seen[strings.ToLower(name)] {
				continue
			}
			seen[strings.ToLower(name)] = true

			regions := make([]string, 0, len(country.Regions))
			regionSeen := make(map[string]bool, len(country.Regions))
			for _, region := range country.Regions {
				region = strings.TrimSpace(region)
				key := strings.ToLower(region)
				if region == "" || regionSeen[key] {
					continue
				}
				regionSeen[key] = true
				regions = append(regions, region)
			}
			if len(regions) == 0 {
				continue
			}
			byName[strings.ToLower(name)] = regions
			ordered = append(ordered, Country{Name: name, Regions: regions})
		}
	})
}

// All returns every country in the list, sorted by name.
func All() []Country {
	load()
	out := make([]Country, len(ordered))
	copy(out, ordered)
	return out
}

// Names returns the country names in display order.
func Names() []string {
	load()
	out := make([]string, 0, len(ordered))
	for _, country := range ordered {
		out = append(out, country.Name)
	}
	return out
}

// RegionsFor returns the states/regions of a country. The second return value
// is false when the country has no known list.
func RegionsFor(name string) ([]string, bool) {
	load()
	regions, ok := byName[strings.ToLower(strings.TrimSpace(name))]
	return regions, ok
}

// Known reports whether a country name exists in the list.
func Known(name string) bool {
	load()
	_, ok := byName[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// Error returns the error that happened while loading the list.
func Error() error {
	load()
	return loadErr
}
