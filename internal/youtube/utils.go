package youtube

import (
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// ToLowerNoSymbols mirrors instab/internal.ToLowerNoSymbols for safe filenames.
func ToLowerNoSymbols(input string) string {
	return strings.ToLower(nonAlnum.ReplaceAllString(input, ""))
}
