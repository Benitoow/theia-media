package library

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ParsedName is what a filename gave away. Title is never empty; Year is zero
// when the filename genuinely did not say.
type ParsedName struct {
	Title string
	Year  int
}

// earliestYear is 1888, the year of Roundhay Garden Scene. Nothing on disk is
// older, and the bound keeps four-digit numbers in titles from being mistaken
// for release years.
const earliestYear = 1888

var (
	// Four-digit numbers that could plausibly be a release year. The range is
	// narrowed further by earliestYear and by the caller's upper bound.
	//
	// The boundaries have to be zero-width. Matching the surrounding separator
	// instead consumes it, and two adjacent years then cannot both match --
	// "1917.2019" would find 1917 and never see the release year behind it.
	yearPattern = regexp.MustCompile(`\b((?:1[89]|20)\d{2})\b`)

	// Release groups and quality annotations in brackets, which never belong to
	// a title. Parentheses are left alone at this stage because they usually
	// hold the year.
	bracketPattern = regexp.MustCompile(`\[[^\]]*\]|\{[^}]*\}`)

	whitespacePattern = regexp.MustCompile(`\s+`)
)

// technicalTags marks where a title ends and release metadata begins. The
// parser cuts at the *first* one it meets rather than deleting them wherever
// they appear, because plenty of real titles contain words like "Cut", "Extended"
// or "Multi" and deleting those everywhere mangles them.
var technicalTags = map[string]bool{
	// Resolution
	"480p": true, "576p": true, "720p": true, "1080p": true, "1080i": true,
	"1440p": true, "2160p": true, "4320p": true, "4k": true, "8k": true,
	"uhd": true, "hd": true, "sd": true, "hq": true,

	// Source
	"bluray": true, "bluray1080p": true, "brrip": true, "bdrip": true, "bdremux": true,
	"dvdrip": true, "dvdscr": true, "dvd": true, "webrip": true, "webdl": true,
	"web": true, "hdtv": true, "hdrip": true, "remux": true, "cam": true,
	"telesync": true, "vodrip": true, "ntsc": true, "pal": true,

	// Codec and encoding
	"x264": true, "x265": true, "h264": true, "h265": true, "hevc": true,
	"avc": true, "xvid": true, "divx": true, "av1": true, "vp9": true, "mpeg2": true,
	"10bit": true, "8bit": true, "hdr": true, "hdr10": true, "sdr": true,
	"dolbyvision": true, "dovi": true,

	// Audio
	"aac": true, "ac3": true, "eac3": true, "dts": true, "dtshd": true,
	"truehd": true, "atmos": true, "flac": true, "mp3": true, "opus": true,
	"ddp": true, "dd5": true, "dd": true, "aac2": true,

	// Release annotations
	"proper": true, "repack": true, "rerip": true, "internal": true, "limited": true,
	"remastered": true, "unrated": true, "uncut": true, "hybrid": true,

	// Language annotations, which in practice always follow the title
	"multi": true, "vostfr": true, "vost": true, "vff": true, "vfq": true,
	"vfi": true, "truefrench": true, "subfrench": true, "dubbed": true, "subbed": true,
}

// ParseFileName extracts a title and, where possible, a release year from a
// filename. It never fails and never returns an empty title: a name it cannot
// make sense of degrades to the filename itself, which is still far more useful
// to a human than a blank row.
func ParseFileName(name string) ParsedName {
	// A film released next year can already be on disk; anything beyond that is
	// a number in a title, not a release date.
	return parseFileName(name, time.Now().Year()+1)
}

func parseFileName(name string, maxYear int) ParsedName {
	withoutExt := stripExtension(name)

	// Kept aside so that every path out of this function has something to
	// return. This is the guarantee the rest of the parser leans on.
	fallback := tidy(normaliseSeparators(bracketPattern.ReplaceAllString(withoutExt, " ")))
	if fallback == "" {
		fallback = strings.TrimSpace(withoutExt)
	}
	if fallback == "" {
		fallback = strings.TrimSpace(name)
	}

	working := normaliseSeparators(bracketPattern.ReplaceAllString(withoutExt, " "))

	year, titlePart := extractYear(working, maxYear)

	title := tidy(stripFromFirstTag(titlePart))
	if title == "" {
		// Everything before the year was noise, which means the "year" was the
		// title all along: "1917.mkv" is a film, not a year with no name.
		if year != 0 {
			return ParsedName{Title: strconv.Itoa(year), Year: 0}
		}
		return ParsedName{Title: fallback}
	}
	return ParsedName{Title: title, Year: year}
}

// stripExtension removes a trailing file extension, but only when it looks like
// one. "Movie.2010" must keep its year, so a purely numeric suffix is left
// alone.
func stripExtension(name string) string {
	ext := filepath.Ext(name)
	if len(ext) < 3 || len(ext) > 6 {
		return name
	}
	body := ext[1:]
	digits := 0
	for _, r := range body {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return name
		}
		if unicode.IsDigit(r) {
			digits++
		}
	}
	if digits == len(body) {
		return name // ".2010" is a year, not an extension
	}
	return strings.TrimSuffix(name, ext)
}

// normaliseSeparators turns scene-style punctuation into spaces. Dots are only
// treated as separators when the name has no spaces at all, so that a title
// which legitimately contains a full stop survives.
func normaliseSeparators(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if !strings.Contains(s, " ") {
		s = strings.ReplaceAll(s, ".", " ")
	}
	return s
}

// extractYear finds the release year and returns it with the part of the string
// that precedes it.
//
// The *last* plausible year wins. "2001 A Space Odyssey 1968" and "1917 2019"
// both put the title's number first and the release year last, and preferring
// the last match gets both right.
func extractYear(s string, maxYear int) (year int, before string) {
	matches := yearPattern.FindAllStringSubmatchIndex(s, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		candidate, err := strconv.Atoi(s[m[2]:m[3]])
		if err != nil || candidate < earliestYear || candidate > maxYear {
			continue
		}
		return candidate, s[:m[2]]
	}
	return 0, s
}

// stripFromFirstTag cuts the string at the first token that is release
// metadata. Everything from there on describes the file, not the film.
func stripFromFirstTag(s string) string {
	fields := strings.Fields(s)
	for i, field := range fields {
		if technicalTags[normaliseToken(field)] {
			return strings.Join(fields[:i], " ")
		}
	}
	return s
}

// normaliseToken reduces a token to lowercase alphanumerics so that "x264",
// "X264" and "x264," all compare equal.
func normaliseToken(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// tidy collapses whitespace and trims the punctuation that separator handling
// leaves stranded at the ends.
func tidy(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '(', ')', '[', ']', '{', '}':
			return ' '
		}
		return r
	}, s)
	s = whitespacePattern.ReplaceAllString(s, " ")
	return strings.Trim(s, " -–—.,_:;|")
}
