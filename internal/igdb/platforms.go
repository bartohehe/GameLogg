package igdb

import (
	"slices"
	"strings"
)

// Etykiety platform pokazywane w UI i zapisywane w bazie. Klucz to skrót z IGDB.
var platformLabels = map[string]string{
	"PC":         "PC",
	"Mac":        "Mac",
	"Linux":      "Linux",
	"PS5":        "PS5",
	"PS4":        "PS4",
	"PS3":        "PS3",
	"PS2":        "PS2",
	"PS1":        "PS1",
	"PSV":        "PS Vita",
	"PSP":        "PSP",
	"Series X|S": "Xbox Series",
	"XONE":       "Xbox One",
	"X360":       "Xbox 360",
	"XBOX":       "Xbox",
	"Switch":     "Switch",
	"Switch 2":   "Switch 2",
	"WiiU":       "Wii U",
	"Wii":        "Wii",
	"3DS":        "3DS",
	"iOS":        "iOS",
	"Android":    "Android",
	"Stadia":     "Stadia",
}

// Kolejność platform w wyborze: najpierw najczęstsze.
var platformOrder = []string{
	"PC", "PS5", "PS4", "Xbox Series", "Xbox One", "Switch 2", "Switch",
	"Mac", "Linux", "PS3", "Xbox 360", "PS Vita", "Wii U", "3DS", "iOS", "Android",
}

// PlatformLabel zamienia platformę z IGDB na etykietę aplikacji.
func PlatformLabel(abbr, name string) string {
	if l, ok := platformLabels[abbr]; ok {
		return l
	}
	if strings.Contains(name, "Switch 2") {
		return "Switch 2"
	}
	if abbr != "" {
		return abbr
	}
	return name
}

// SortPlatforms usuwa duplikaty i puste etykiety, a potem układa platformy:
// najpierw znane w ustalonej kolejności, potem reszta alfabetycznie.
func SortPlatforms(labels []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		if l != "" && !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	rank := func(l string) int {
		if i := slices.Index(platformOrder, l); i >= 0 {
			return i
		}
		return len(platformOrder)
	}
	slices.SortStableFunc(out, func(a, b string) int {
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra - rb
		}
		return strings.Compare(a, b)
	})
	return out
}
