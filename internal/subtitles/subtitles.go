// Package subtitles turns the subtitle tracks a file carries into something a
// browser will render, and says plainly when it cannot.
//
// Decision 3 draws the line: text tracks -- SRT and ASS inside a container,
// plus `.srt` files sitting next to the film -- are in. Image tracks -- PGS,
// VobSub, DVB -- are out, because making them visible means burning them into
// the picture, which is the full transcode pipeline decision 1 keeps out.
//
// An image track is still *listed*, marked unrenderable. A BluRay rip whose
// only subtitles are PGS would otherwise look like a file with no subtitles at
// all, and the viewer would go looking for a setting that does not exist.
package subtitles

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Kind is what a track is made of, which decides whether it can be shown.
type Kind string

const (
	// KindText is a renderable track: it converts to WebVTT.
	KindText Kind = "text"

	// KindImage is a bitmap track. Listed, never served.
	KindImage Kind = "image"
)

// textCodecs are the subtitle codecs that carry characters.
var textCodecs = map[string]bool{
	"subrip": true, "srt": true, "ass": true, "ssa": true,
	"webvtt": true, "vtt": true, "mov_text": true, "text": true,
	"microdvd": true, "sami": true, "subviewer": true, "subviewer1": true,
	"stl": true, "jacosub": true, "mpl2": true, "pjs": true, "realtext": true,
	"vplayer": true, "eia_608": true, "subrip_text": true,
}

// imageCodecs are the bitmap ones. Named rather than inferred by elimination so
// an unknown codec is reported as unknown instead of silently promised.
var imageCodecs = map[string]bool{
	"hdmv_pgs_subtitle": true, "pgssub": true, "pgs": true,
	"dvd_subtitle": true, "dvdsub": true, "vobsub": true,
	"dvb_subtitle": true, "dvbsub": true, "dvb_teletext": true,
	"xsub": true, "hdmv_text_subtitle": false,
}

// ClassifyCodec reports whether a codec can become WebVTT.
func ClassifyCodec(codec string) Kind {
	switch normalised := strings.ToLower(strings.TrimSpace(codec)); {
	case textCodecs[normalised]:
		return KindText
	case imageCodecs[normalised]:
		return KindImage
	default:
		// Unknown codecs are treated as bitmaps: refusing to promise is the
		// cheaper mistake. A track that turns out to be text is one entry in
		// textCodecs away from working.
		return KindImage
	}
}

// sidecarExtensions are the subtitle files read from disk. `.ass` is absent on
// purpose: converting its styling honestly needs ffmpeg, and Extract handles it
// there. This list is what Convert can do alone, with no binary downloaded.
var sidecarExtensions = map[string]string{
	".srt": "srt",
	".vtt": "webvtt",
}

// Sidecar is a subtitle file found beside a media file.
type Sidecar struct {
	Path     string
	Codec    string
	Language string
	Title    string
	Forced   bool
}

// hintWords are the tokens a sidecar name uses for something other than a
// language: "Film.fr.forced.srt".
var hintWords = map[string]string{
	"forced": "forced", "force": "forced", "forces": "forced", "forcé": "forced",
	"sdh": "sdh", "cc": "sdh", "hi": "sdh",
	"full": "", "complete": "", "default": "",
}

// languageNames maps the words people actually put in filenames to the ISO 639
// codes the rest of the system speaks. Only the ones a French or English
// library realistically contains: guessing wider invents facts.
var languageNames = map[string]string{
	"fr": "fra", "fre": "fra", "fra": "fra", "french": "fra", "francais": "fra",
	"français": "fra", "vf": "fra", "vff": "fra",
	"en": "eng", "eng": "eng", "english": "eng", "vo": "eng", "vost": "eng",
	"es": "spa", "spa": "spa", "spanish": "spa", "espanol": "spa", "español": "spa",
	"de": "deu", "ger": "deu", "deu": "deu", "german": "deu", "allemand": "deu",
	"it": "ita", "ita": "ita", "italian": "ita", "italien": "ita",
	"pt": "por", "por": "por", "portuguese": "por",
	"nl": "nld", "dut": "nld", "nld": "nld", "dutch": "nld",
	"ja": "jpn", "jpn": "jpn", "japanese": "jpn", "japonais": "jpn",
	"zh": "zho", "chi": "zho", "zho": "zho", "chinese": "zho",
	"ru": "rus", "rus": "rus", "russian": "rus",
	"ar": "ara", "ara": "ara", "arabic": "ara", "arabe": "ara",
	"ko": "kor", "kor": "kor", "korean": "kor",
	"pl": "pol", "pol": "pol", "polish": "pol",
	"sv": "swe", "swe": "swe", "swedish": "swe",
	"da": "dan", "dan": "dan", "danish": "dan",
	"no": "nor", "nor": "nor", "norwegian": "nor",
	"fi": "fin", "fin": "fin", "finnish": "fin",
	"tr": "tur", "tur": "tur", "turkish": "tur",
}

// FindSidecars lists the subtitle files belonging to one media file.
//
// The directory is read rather than the individual names guessed, because the
// separator, the case and the order of the hints all vary between the tools
// people use: "Film.fr.srt", "Film.FRENCH.forced.srt", "Film.fra.sdh.srt".
func FindSidecars(mediaPath string) ([]Sidecar, error) {
	dir := filepath.Dir(mediaPath)
	stem := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))
	lowerStem := strings.ToLower(stem)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("subtitles: reading %s: %w", dir, err)
	}

	var found []Sidecar
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		codec, ok := sidecarExtensions[ext]
		if !ok {
			continue
		}
		base := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
		if base != lowerStem && !strings.HasPrefix(base, lowerStem+".") &&
			!strings.HasPrefix(base, lowerStem+"_") && !strings.HasPrefix(base, lowerStem+"-") {
			continue
		}

		sidecar := Sidecar{
			Path:  filepath.Join(dir, name),
			Codec: codec,
		}
		suffix := strings.Trim(base[len(lowerStem):], "._- ")
		for _, token := range strings.FieldsFunc(suffix, func(r rune) bool {
			return r == '.' || r == '_' || r == '-' || r == ' '
		}) {
			if hint, ok := hintWords[token]; ok {
				if hint == "forced" {
					sidecar.Forced = true
				} else if hint == "sdh" {
					sidecar.Title = "SDH"
				}
				continue
			}
			if language, ok := languageNames[token]; ok && sidecar.Language == "" {
				sidecar.Language = language
			}
		}
		found = append(found, sidecar)
	}
	return found, nil
}

// Convert writes a `.srt` or `.vtt` file out as WebVTT, shifted so that
// `offset` becomes zero and cues ending before it are dropped.
//
// The shift exists because a remuxed stream is a pipe: ffmpeg is restarted at a
// timestamp and the video element's clock begins again at zero, so subtitles on
// the film's clock would sit hours away from the picture. The same number that
// seeks the video seeks the text, which is what keeps the two from drifting
// apart.
func Convert(r io.Reader, offset time.Duration, w io.Writer) error {
	raw, err := io.ReadAll(io.LimitReader(r, maxSubtitleBytes))
	if err != nil {
		return fmt.Errorf("subtitles: reading: %w", err)
	}
	text := decodeText(raw)

	if _, err := io.WriteString(w, "WEBVTT\n\n"); err != nil {
		return err
	}

	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		body    []string
		start   time.Duration
		end     time.Duration
		hasCue  bool
		written int
	)
	flush := func() error {
		if !hasCue {
			body = nil
			return nil
		}
		hasCue = false
		lines := trimTrailingBlank(body)
		body = nil
		if len(lines) == 0 || end <= offset {
			return nil
		}
		shiftedStart := start - offset
		if shiftedStart < 0 {
			shiftedStart = 0
		}
		written++
		_, err := fmt.Fprintf(w, "%s --> %s\n%s\n\n",
			formatTimestamp(shiftedStart), formatTimestamp(end-offset),
			strings.Join(lines, "\n"))
		return err
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if from, to, ok := parseTimingLine(line); ok {
			if err := flush(); err != nil {
				return err
			}
			start, end, hasCue = from, to, true
			continue
		}
		if hasCue {
			body = append(body, escapeCueText(line))
			continue
		}
		// Sequence numbers, `WEBVTT` headers, NOTE blocks and stray blank lines
		// before the first timing line are all dropped: the output header is
		// written above and nothing else outside a cue survives the shift with
		// a meaning worth keeping.
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("subtitles: reading: %w", err)
	}
	if err := flush(); err != nil {
		return err
	}
	if written == 0 {
		// A valid, empty track. Browsers accept a header-only file, and an
		// empty one is a truer answer than an error for a track whose cues all
		// sit before the current position.
		return nil
	}
	return nil
}

// maxSubtitleBytes is generous for dialogue and small enough that a mistaken
// `.srt` extension on something enormous cannot be read into memory.
const maxSubtitleBytes = 32 << 20

// parseTimingLine reads both dialects at once: SRT writes `00:00:01,000` and
// WebVTT writes `00:00:01.000` or `00:01.000`.
func parseTimingLine(line string) (time.Duration, time.Duration, bool) {
	index := strings.Index(line, "-->")
	if index < 0 {
		return 0, 0, false
	}
	from, ok := parseTimestamp(strings.TrimSpace(line[:index]))
	if !ok {
		return 0, 0, false
	}
	rest := strings.TrimSpace(line[index+3:])
	// WebVTT allows cue settings after the end time: "00:05.000 line:90%".
	if space := strings.IndexAny(rest, " \t"); space >= 0 {
		rest = rest[:space]
	}
	to, ok := parseTimestamp(rest)
	if !ok {
		return 0, 0, false
	}
	return from, to, true
}

func parseTimestamp(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	value = strings.Replace(value, ",", ".", 1)

	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	var hours, minutes int
	var err error
	if len(parts) == 3 {
		if hours, err = strconv.Atoi(strings.TrimSpace(parts[0])); err != nil {
			return 0, false
		}
		parts = parts[1:]
	}
	if minutes, err = strconv.Atoi(strings.TrimSpace(parts[0])); err != nil {
		return 0, false
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, false
	}
	total := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute +
		time.Duration(seconds*float64(time.Second))
	if total < 0 {
		return 0, false
	}
	return total, true
}

func formatTimestamp(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := d.Milliseconds()
	return fmt.Sprintf("%02d:%02d:%02d.%03d",
		total/3600000, total/60000%60, total/1000%60, total%1000)
}

// escapeCueText protects the three characters WebVTT reads as markup. Subtitle
// files in the wild contain bare `<` and `&` far more often than they contain
// intentional tags.
func escapeCueText(line string) string {
	if !strings.ContainsAny(line, "<&>") {
		return line
	}
	if isMarkupLine(line) {
		return line
	}
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(line)
}

// isMarkupLine recognises the styling both formats share -- <i>, <b>, <u>,
// <font ...> -- so an italicised line is not shown with its tags visible.
func isMarkupLine(line string) bool {
	lower := strings.ToLower(line)
	for _, tag := range []string{"<i>", "</i>", "<b>", "</b>", "<u>", "</u>", "<font", "</font>", "<c.", "<v "} {
		if strings.Contains(lower, tag) {
			return true
		}
	}
	return false
}

func trimTrailingBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	return lines
}

// decodeText makes bytes into a Go string without dragging in x/text.
//
// Subtitle files downloaded in French are windows-1252 as often as they are
// UTF-8, and a mis-decoded one is not a cosmetic problem: "père" reads as
// "p?re" across the whole film. Valid UTF-8 is left alone -- the check is
// cheap and never wrong on a real UTF-8 file -- and everything else is decoded
// as windows-1252, which is a superset of latin-1 and therefore the safest
// single guess.
func decodeText(raw []byte) string {
	raw = trimBOM(raw)
	if utf8.Valid(raw) {
		return string(raw)
	}

	var b strings.Builder
	b.Grow(len(raw) * 2)
	for _, c := range raw {
		if c < 0x80 {
			b.WriteByte(c)
			continue
		}
		if c >= 0xA0 {
			b.WriteRune(rune(c)) // latin-1 is code point == byte
			continue
		}
		if r := windows1252High[c-0x80]; r != 0 {
			b.WriteRune(r)
		} else {
			b.WriteRune('�')
		}
	}
	return b.String()
}

// windows1252High is the 0x80..0x9F range, the only place windows-1252 and
// latin-1 disagree. It is where the curly quotes, the em dash and the ellipsis
// live, which is to say: most of a subtitle file's punctuation.
var windows1252High = [32]rune{
	'€', 0, '‚', 'ƒ', '„', '…', '†', '‡',
	'ˆ', '‰', 'Š', '‹', 'Œ', 0, 'Ž', 0,
	0, '‘', '’', '“', '”', '•', '–', '—',
	'˜', '™', 'š', '›', 'œ', 0, 'ž', 'Ÿ',
}

func trimBOM(raw []byte) []byte {
	switch {
	case len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF:
		return raw[3:]
	default:
		return raw
	}
}

// ExtractArgs builds the ffmpeg invocation that pulls one embedded track out as
// WebVTT, already rebased so that startSeconds is zero.
//
// The seek is the same `-ss` the remux uses, deliberately: one mechanism moves
// both the picture and the text, so they cannot drift apart. Measured on a
// generated MKV -- a cue at 00:07.023 comes back at 00:00.023 for -ss 7.
func ExtractArgs(path string, streamIndex int, startSeconds float64) []string {
	args := []string{"-hide_banner", "-loglevel", "error"}
	if startSeconds > 0 {
		args = append(args, "-ss", strconv.FormatFloat(startSeconds, 'f', 3, 64))
	}
	return append(args,
		"-i", path,
		"-map", "0:"+strconv.Itoa(streamIndex),
		"-c:s", "webvtt",
		"-f", "webvtt",
		"pipe:1",
	)
}
