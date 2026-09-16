package setup

import (
	"regexp"
	"strings"
	"testing"
)

// ansiPattern matches the escape sequences a styled terminal view carries.
//
// A rendered page is full of them, and counting them as characters was how a
// measurement of "78 runes in a 74-column form" looked like clipping when the
// line was only 72 characters of text. A width check that counts escapes is a
// width check that reports on the palette.
var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

// TestNoPageDrawsWiderThanItsForm guards the one thing a screenshot cannot
// settle: whether a page overflows the width the form was given.
//
// It exists because a capture at 120 columns looked like it clipped the French
// description - and it had not. The capturing window is sized by the capture
// script and can run past the screen edge, so the picture was clipped while the
// form was exactly as wide as it should be. Measuring the rendered strings is
// what tells the two apart, which is the whole reason this is a test and not a
// second look at the screenshot.
//
// Pages are measured for both languages at the three widths the plan asks for.
// Escape sequences are stripped, and trailing spaces are ignored: Huh pads its
// card, and padding is not content.
func TestNoPageDrawsWiderThanItsForm(t *testing.T) {
	// The form itself is given a width, and Huh indents and borders inside it, so
	// a page is allowed no more than the width it was built with.
	for _, language := range []string{"fr", "en"} {
		catalogue, _ := CatalogueFor(language)
		for _, width := range []int{74, 94, 114} {
			for _, role := range []Role{RoleAllInOne, RoleServer, RolePlayer} {
				result := &FormResult{Role: role}
				pages := walkPages(t, buildForm(result, catalogue, width, 16))
				for index, page := range pages {
					for _, line := range strings.Split(page, "\n") {
						visible := ansiPattern.ReplaceAllString(line, "")
						visible = strings.TrimRight(visible, " ")
						if runes := len([]rune(visible)); runes > width {
							t.Errorf(
								"%s, width %d, %s, page %d: %d visible characters in a %d-column form:\n%q",
								language, width, role, index+1, runes, width, visible,
							)
						}
					}
				}
			}
		}
	}
}
