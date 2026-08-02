package library

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// ParsedEpisode is the identity carried by an episode release name. Matched
// distinguishes an ordinary film from an episode; Ambiguous means the marker
// was reliable but there was no series title to attach it to.
type ParsedEpisode struct {
	Matched        bool
	Ambiguous      bool
	SeriesTitle    string
	SeriesYear     int
	SeasonNumber   int
	EpisodeNumbers []int
	EpisodeTitle   string
	Pattern        string
}

type episodePattern struct {
	name         string
	start        *regexp.Regexp
	continuation *regexp.Regexp
}

var episodePatterns = []episodePattern{
	{
		name:         "sxxexx",
		start:        regexp.MustCompile(`(?i)(^|[ ._\-\[(])S([0-9]{1,2})E([0-9]{1,3})`),
		continuation: regexp.MustCompile(`(?i)^(?:[ ._\-]*E)([0-9]{1,3})`),
	},
	{
		name:         "xpattern",
		start:        regexp.MustCompile(`(?i)(^|[ ._\-\[(])([0-9]{1,2})x([0-9]{1,3})`),
		continuation: regexp.MustCompile(`(?i)^(?:[ ._\-]*x)([0-9]{1,3})`),
	},
}

var (
	seriesSeparators = regexp.MustCompile(`[._]+`)
	seasonDirectory  = regexp.MustCompile(`(?i)^(?:S[0-9]{1,2}|Season[ ._-]*[0-9]{1,2}|Saison[ ._-]*[0-9]{1,2}|Specials?|Sp[eé]ciaux)$`)
)

// ParseEpisodePath recognises only explicit release patterns. Numeric shorthand
// such as 101 and date-based episodes are deliberately left for a later parser
// backed by real examples; guessing here misclassifies ordinary films.
func ParseEpisodePath(relative string) ParsedEpisode {
	base := filepath.Base(relative)
	stem := strings.TrimSuffix(base, filepath.Ext(base))

	for _, pattern := range episodePatterns {
		index := pattern.start.FindStringSubmatchIndex(stem)
		if index == nil {
			continue
		}

		tokenStart := index[0]
		if index[2] >= 0 && index[3] > index[2] {
			tokenStart = index[3]
		}
		season, _ := strconv.Atoi(stem[index[4]:index[5]])
		firstEpisode, _ := strconv.Atoi(stem[index[6]:index[7]])
		episodes := []int{firstEpisode}
		end := index[1]

		for {
			next := pattern.continuation.FindStringSubmatchIndex(stem[end:])
			if next == nil {
				break
			}
			episode, _ := strconv.Atoi(stem[end+next[2] : end+next[3]])
			episodes = append(episodes, episode)
			end += next[1]
		}

		if end < len(stem) {
			r, _ := firstRune(stem[end:])
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				continue
			}
		}

		rawSeries := cleanReleaseText(stem[:tokenStart])
		if rawSeries == "" {
			rawSeries = seriesTitleFromParents(relative)
		}
		result := ParsedEpisode{
			Matched:        true,
			SeasonNumber:   season,
			EpisodeNumbers: canonicalEpisodeNumbers(episodes),
			EpisodeTitle:   cleanEpisodeTitle(stem[end:]),
			Pattern:        pattern.name,
		}
		if rawSeries == "" {
			result.Ambiguous = true
			return result
		}
		identity := ParseFileName(rawSeries)
		result.SeriesTitle = identity.Title
		result.SeriesYear = identity.Year
		return result
	}

	return ParsedEpisode{}
}

func firstRune(s string) (rune, int) {
	for _, r := range s {
		return r, len(string(r))
	}
	return 0, 0
}

func cleanReleaseText(s string) string {
	s = strings.Trim(s, " ._-[]()")
	s = seriesSeparators.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

func cleanEpisodeTitle(s string) string {
	s = cleanReleaseText(s)
	if s == "" {
		return ""
	}
	// Reuse the film parser's technical-tag vocabulary. Unlike ParseFileName,
	// an all-technical suffix has no useful fallback: "1080p" is not a title.
	return tidy(stripFromFirstTag(normaliseSeparators(bracketPattern.ReplaceAllString(s, " "))))
}

func seriesTitleFromParents(relative string) string {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	for i := len(parts) - 2; i >= 0; i-- {
		candidate := strings.TrimSpace(parts[i])
		if candidate == "" || seasonDirectory.MatchString(candidate) {
			continue
		}
		return cleanReleaseText(candidate)
	}
	return ""
}

func canonicalEpisodeNumbers(numbers []int) []int {
	seen := make(map[int]bool, len(numbers))
	out := make([]int, 0, len(numbers))
	for _, number := range numbers {
		if number < 0 || seen[number] {
			continue
		}
		seen[number] = true
		out = append(out, number)
	}
	sort.Ints(out)
	return out
}

func episodeKey(numbers []int) string {
	parts := make([]string, len(numbers))
	for i, number := range numbers {
		parts[i] = strconv.Itoa(number)
	}
	return strings.Join(parts, ",")
}
