package subtitles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClassifyCodec(t *testing.T) {
	for codec, want := range map[string]Kind{
		"subrip":            KindText,
		"SubRip":            KindText,
		"ass":               KindText,
		"mov_text":          KindText,
		"webvtt":            KindText,
		"hdmv_pgs_subtitle": KindImage,
		"dvd_subtitle":      KindImage,
		"dvb_teletext":      KindImage,
		// The safe answer for something nobody has taught it about: refusing to
		// promise costs one menu entry, promising costs a blank track.
		"something_new": KindImage,
		"":              KindImage,
	} {
		if got := ClassifyCodec(codec); got != want {
			t.Errorf("ClassifyCodec(%q) = %q, want %q", codec, got, want)
		}
	}
}

const sampleSRT = "1\r\n" +
	"00:00:01,000 --> 00:00:04,000\r\n" +
	"Première réplique\r\n" +
	"\r\n" +
	"2\r\n" +
	"00:00:10,500 --> 00:00:14,250\r\n" +
	"Deuxième, sur\r\n" +
	"deux lignes\r\n" +
	"\r\n" +
	"3\r\n" +
	"00:01:00,000 --> 00:01:05,000\r\n" +
	"<i>Troisième</i>\r\n"

func TestConvertWritesWebVTT(t *testing.T) {
	var out strings.Builder
	if err := Convert(strings.NewReader(sampleSRT), 0, &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := out.String()

	if !strings.HasPrefix(got, "WEBVTT\n\n") {
		t.Fatalf("missing WebVTT header:\n%s", got)
	}
	for _, want := range []string{
		"00:00:01.000 --> 00:00:04.000",
		"Première réplique",
		"00:00:10.500 --> 00:00:14.250",
		"Deuxième, sur\ndeux lignes",
		"00:01:00.000 --> 00:01:05.000",
		// Styling survives; it is the one markup subtitle files really use.
		"<i>Troisième</i>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q:\n%s", want, got)
		}
	}
	// Sequence numbers are SRT bookkeeping and mean nothing in WebVTT.
	if strings.Contains(got, "\n1\n") {
		t.Errorf("sequence numbers leaked into the output:\n%s", got)
	}
}

// The offset is the whole reason this exists: a remux restarts ffmpeg at a
// timestamp and the element's clock begins again at zero, so text on the film's
// clock would sit as far from the picture as the viewer has travelled.
func TestConvertShiftsAndDropsPastCues(t *testing.T) {
	var out strings.Builder
	if err := Convert(strings.NewReader(sampleSRT), 10*time.Second, &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := out.String()

	if strings.Contains(got, "Première réplique") {
		t.Errorf("a cue ending before the seek survived:\n%s", got)
	}
	if !strings.Contains(got, "00:00:00.500 --> 00:00:04.250") {
		t.Errorf("cue two was not shifted by ten seconds:\n%s", got)
	}
	if !strings.Contains(got, "00:00:50.000 --> 00:00:55.000") {
		t.Errorf("cue three was not shifted by ten seconds:\n%s", got)
	}
}

// A cue straddling the seek point is clamped rather than dropped: somebody who
// jumps into the middle of a line should still read the end of it.
func TestConvertClampsStraddlingCue(t *testing.T) {
	var out strings.Builder
	if err := Convert(strings.NewReader(sampleSRT), 2*time.Second, &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !strings.Contains(out.String(), "00:00:00.000 --> 00:00:02.000") {
		t.Errorf("the straddling cue was not clamped to zero:\n%s", out.String())
	}
}

func TestConvertReadsWindows1252(t *testing.T) {
	// "père" and a curly apostrophe, as a French subtitle downloaded in 2009
	// actually arrives. Decoded as UTF-8 this is mojibake for the whole film.
	raw := []byte("1\n00:00:01,000 --> 00:00:02,000\np\xe8re, c\x92est fini\n")
	var out strings.Builder
	if err := Convert(strings.NewReader(string(raw)), 0, &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !strings.Contains(out.String(), "père, c’est fini") {
		t.Errorf("windows-1252 was not decoded:\n%q", out.String())
	}
}

func TestConvertAcceptsWebVTTInput(t *testing.T) {
	input := "WEBVTT\n\nNOTE something\n\n00:01.000 --> 00:02.000 line:90%\nDéjà du VTT\n"
	var out strings.Builder
	if err := Convert(strings.NewReader(input), 0, &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "00:00:01.000 --> 00:00:02.000") {
		t.Errorf("short mm:ss timestamps were not read:\n%s", got)
	}
	if strings.Contains(got, "NOTE") {
		t.Errorf("a NOTE block survived:\n%s", got)
	}
}

func TestConvertEscapesBareMarkup(t *testing.T) {
	input := "1\n00:00:01,000 --> 00:00:02,000\n5 < 6 & 7 > 3\n"
	var out strings.Builder
	if err := Convert(strings.NewReader(input), 0, &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !strings.Contains(out.String(), "5 &lt; 6 &amp; 7 &gt; 3") {
		t.Errorf("bare angle brackets were not escaped:\n%s", out.String())
	}
}

func TestFindSidecars(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("Quiet Harbour 2017.mp4")
	write("Quiet Harbour 2017.srt")
	write("Quiet Harbour 2017.fr.srt")
	write("Quiet Harbour 2017.ENGLISH.forced.srt")
	write("Quiet Harbour 2017.es.sdh.vtt")
	write("Another Film 2019.fr.srt") // belongs to a different film
	write("Quiet Harbour 2017.nfo")   // not a subtitle

	found, err := FindSidecars(filepath.Join(dir, "Quiet Harbour 2017.mp4"))
	if err != nil {
		t.Fatalf("FindSidecars: %v", err)
	}
	if len(found) != 4 {
		t.Fatalf("found %d sidecars, want 4: %+v", len(found), found)
	}

	byLanguage := map[string]Sidecar{}
	for _, sidecar := range found {
		byLanguage[sidecar.Language] = sidecar
	}
	if _, ok := byLanguage[""]; !ok {
		t.Errorf("the bare .srt was not found: %+v", found)
	}
	if got := byLanguage["fra"]; got.Codec != "srt" || got.Forced {
		t.Errorf("french sidecar = %+v", got)
	}
	if got := byLanguage["eng"]; !got.Forced {
		t.Errorf("the forced hint was not read: %+v", got)
	}
	if got := byLanguage["spa"]; got.Codec != "webvtt" || got.Title != "SDH" {
		t.Errorf("spanish sidecar = %+v", got)
	}
}
