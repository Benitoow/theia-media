package profiles

import (
	"bytes"
	"context"
	"database/sql"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Benitoow/theia-media/internal/db"
)

func newTestStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	database, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return New(database), database
}

func TestMigrationCreatesOneUnnamedDefaultProfile(t *testing.T) {
	store, _ := newTestStore(t)

	list, err := store.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("profiles = %d, want 1", len(list))
	}
	// The name is absent rather than "Profil principal": the interface writes
	// what the viewer reads, in the active language (decision 25).
	if list[0].Name != "" || !list[0].IsDefault {
		t.Errorf("default profile = %+v, want an unnamed default", list[0])
	}
}

func TestCreateRenameDeleteAndTheLimit(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := t.Context()

	created, err := store.Create(ctx, "  Mimi  ")
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "Mimi" {
		t.Errorf("name = %q, want it trimmed to Mimi", created.Name)
	}
	if created.IsDefault {
		t.Error("a created profile must not claim to be the default")
	}

	for _, bad := range []string{"", "   ", strings.Repeat("é", MaxNameRunes+1), "bad\x00name"} {
		if _, err := store.Create(ctx, bad); err != ErrInvalidName {
			t.Errorf("Create(%q) error = %v, want ErrInvalidName", bad, err)
		}
	}

	// A name of exactly the limit is counted in runes, not bytes.
	if _, err := store.Create(ctx, strings.Repeat("é", MaxNameRunes)); err != nil {
		t.Errorf("a %d-rune name was refused: %v", MaxNameRunes, err)
	}

	for len(mustList(t, store)) < MaxProfiles {
		if _, err := store.Create(ctx, "filler"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Create(ctx, "one too many"); err != ErrProfileLimit {
		t.Errorf("error past the limit = %v, want ErrProfileLimit", err)
	}

	renamed, err := store.Rename(ctx, created.ID, "Mimi 2")
	if err != nil || renamed.Name != "Mimi 2" {
		t.Fatalf("rename = %+v, %v", renamed, err)
	}
	if err := store.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, created.ID); err != ErrNoSuchProfile {
		t.Errorf("reading a deleted profile = %v, want ErrNoSuchProfile", err)
	}
}

func TestTheLastProfileCannotBeDeleted(t *testing.T) {
	store, _ := newTestStore(t)
	only, err := store.DefaultID(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(t.Context(), only); err != ErrLastProfile {
		t.Errorf("deleting the last profile = %v, want ErrLastProfile", err)
	}
}

func TestDeletingAProfileTakesItsHistoryAndNobodyElseS(t *testing.T) {
	store, database := newTestStore(t)
	ctx := t.Context()

	if _, err := database.ExecContext(ctx,
		`INSERT INTO movies (id, path, file_name, size_bytes, modified_at, title,
		     first_seen_scan, last_seen_scan, added_at, updated_at)
		 VALUES (1, '/a.mkv', 'a.mkv', 1, 0, 'A', 1, 1, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	other, err := store.Create(ctx, "Diegoat")
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.DefaultID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{first, other.ID} {
		if _, err := database.ExecContext(ctx,
			`INSERT INTO movie_progress (profile_id, movie_id, position_seconds, watched_at, finished)
			 VALUES (?, 1, 600, 100, 0)`, id); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.Delete(ctx, other.ID); err != nil {
		t.Fatal(err)
	}

	var rows, survivor int64
	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MAX(profile_id), 0) FROM movie_progress`).Scan(&rows, &survivor); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || survivor != first {
		t.Errorf("progress rows = %d owned by %d, want 1 owned by %d", rows, survivor, first)
	}
}

func TestStatsCountOnlyThatProfile(t *testing.T) {
	store, database := newTestStore(t)
	ctx := t.Context()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO movies (id, path, file_name, size_bytes, modified_at, title,
		    first_seen_scan, last_seen_scan, added_at, updated_at) VALUES
			(1, '/a.mkv', 'a.mkv', 1, 0, 'A', 1, 1, 0, 0),
			(2, '/b.mkv', 'b.mkv', 1, 0, 'B', 1, 1, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	mine, err := store.DefaultID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := store.Create(ctx, "Pablitax")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO movie_progress (profile_id, movie_id, position_seconds, watched_at, finished) VALUES
			(?, 1, 600, 500, 0),
			(?, 2, 100, 900, 1),
			(?, 1, 300, 700, 1)`, mine, mine, theirs.ID); err != nil {
		t.Fatal(err)
	}

	profile, err := store.Get(ctx, mine)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Stats.MoviesStarted != 1 || profile.Stats.MoviesFinished != 1 {
		t.Errorf("stats = %+v, want 1 started and 1 finished", profile.Stats)
	}
	if profile.Stats.LastWatchedAt == nil || profile.Stats.LastWatchedAt.Unix() != 900 {
		t.Errorf("last watched = %v, want the most recent of this profile's rows", profile.Stats.LastWatchedAt)
	}
}

func TestAvatarIsSquaredReEncodedAndVersioned(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := t.Context()
	id, err := store.DefaultID(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// A wide PNG, so both the crop and the change of format are exercised.
	wide := image.NewRGBA(image.Rect(0, 0, 1200, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 1200; x++ {
			wide.Set(x, y, color.RGBA{R: uint8(x % 256), G: 90, B: 20, A: 255})
		}
	}
	var source bytes.Buffer
	if err := png.Encode(&source, wide); err != nil {
		t.Fatal(err)
	}

	normalised, contentType, err := Normalise(bytes.NewReader(source.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "image/jpeg" {
		t.Errorf("content type = %q, want image/jpeg -- a PNG must not be stored as a PNG", contentType)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(normalised))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != AvatarSize || decoded.Bounds().Dy() != AvatarSize {
		t.Errorf("stored size = %v, want %dx%d", decoded.Bounds(), AvatarSize, AvatarSize)
	}

	before, err := store.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if before.HasAvatar {
		t.Error("a fresh profile should not report a picture")
	}
	after, err := store.SetAvatar(ctx, id, normalised, contentType)
	if err != nil {
		t.Fatal(err)
	}
	if !after.HasAvatar || after.AvatarVersion != before.AvatarVersion+1 {
		t.Errorf("after upload = %+v, want a picture and a bumped version", after)
	}

	cleared, err := store.ClearAvatar(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.HasAvatar || cleared.AvatarVersion != after.AvatarVersion+1 {
		t.Errorf("after clearing = %+v, want no picture and another bump", cleared)
	}
	if _, err := store.Avatar(ctx, id); err != ErrNoAvatar {
		t.Errorf("reading a cleared picture = %v, want ErrNoAvatar", err)
	}
}

func TestNormaliseRefusesWhatIsNotAUsablePicture(t *testing.T) {
	cases := map[string][]byte{
		"empty":        {},
		"not an image": []byte("GET /api/settings HTTP/1.1\r\n\r\n"),
		"truncated":    []byte("\xff\xd8\xff\xe0 half a jpeg"),
	}
	for name, payload := range cases {
		if _, _, err := Normalise(bytes.NewReader(payload)); err != ErrInvalidImage {
			t.Errorf("Normalise(%s) error = %v, want ErrInvalidImage", name, err)
		}
	}

	if _, _, err := Normalise(bytes.NewReader(make([]byte, MaxAvatarUpload+1))); err == nil {
		t.Error("an oversized upload was accepted")
	}
}

// A JPEG that claims to be rotated must come back upright, and the claim must
// be read from the file rather than assumed.
func TestExifOrientationIsAppliedAndBoundsAreSwapped(t *testing.T) {
	tall := image.NewRGBA(image.Rect(0, 0, 40, 80))
	var body bytes.Buffer
	if err := jpeg.Encode(&body, tall, nil); err != nil {
		t.Fatal(err)
	}
	withExif := injectOrientation(t, body.Bytes(), 6)

	if got := jpegOrientation(withExif); got != 6 {
		t.Fatalf("orientation = %d, want 6", got)
	}
	if got := jpegOrientation(body.Bytes()); got != 0 {
		t.Errorf("orientation without EXIF = %d, want 0", got)
	}

	decoded, err := jpeg.Decode(bytes.NewReader(withExif))
	if err != nil {
		t.Fatal(err)
	}
	rotated := applyOrientation(decoded, 6)
	if rotated.Bounds().Dx() != 80 || rotated.Bounds().Dy() != 40 {
		t.Errorf("rotated bounds = %v, want the axes swapped", rotated.Bounds())
	}
}

// injectOrientation splices a minimal APP1/EXIF segment carrying one tag in
// after the SOI marker.
func injectOrientation(t *testing.T, jpegBytes []byte, orientation uint16) []byte {
	t.Helper()

	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8}
	tiff = append(tiff, 0, 1) // one IFD entry
	entry := []byte{0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 0, 0, 0}
	entry[8] = byte(orientation >> 8)
	entry[9] = byte(orientation)
	tiff = append(tiff, entry...)
	tiff = append(tiff, 0, 0, 0, 0) // no next IFD

	payload := append([]byte("Exif\x00\x00"), tiff...)
	length := len(payload) + 2

	out := append([]byte{}, jpegBytes[:2]...)
	out = append(out, 0xFF, 0xE1, byte(length>>8), byte(length))
	out = append(out, payload...)
	return append(out, jpegBytes[2:]...)
}

func mustList(t *testing.T, store *Store) []Profile {
	t.Helper()
	list, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return list
}

// Deleting the default profile promotes the next oldest. The legacy columns on
// `movies` are what a rolled-back v1.5.0 reads, so they have to follow -- found
// by deleting profile 1 on the real library and watching them keep serving a
// history that no longer belonged to anybody.
func TestDeletingTheDefaultProfileRepointsTheLegacyMirror(t *testing.T) {
	store, database := newTestStore(t)
	ctx := t.Context()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO movies (id, path, file_name, size_bytes, modified_at, title,
		    first_seen_scan, last_seen_scan, added_at, updated_at, position_seconds, watched_at)
		VALUES (1, '/a.mkv', 'a.mkv', 1, 0, 'A', 1, 1, 0, 0, 900, 500)`); err != nil {
		t.Fatal(err)
	}
	first, err := store.DefaultID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(ctx, "Benjaminous")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO movie_progress (profile_id, movie_id, position_seconds, watched_at, finished)
		VALUES (?, 1, 900, 500, 0), (?, 1, 240, 800, 0)`, first, second.ID); err != nil {
		t.Fatal(err)
	}

	if err := store.Delete(ctx, first); err != nil {
		t.Fatal(err)
	}

	var position, watchedAt float64
	if err := database.QueryRowContext(ctx,
		`SELECT position_seconds, watched_at FROM movies WHERE id = 1`).Scan(&position, &watchedAt); err != nil {
		t.Fatal(err)
	}
	if position != 240 || watchedAt != 800 {
		t.Errorf("legacy mirror = %v/%v, want the promoted profile's 240/800", position, watchedAt)
	}
}

// Deleting an id that does not exist is a 404, even when a single profile
// remains. Checking the count first reported "the last profile cannot be
// deleted" -- a true sentence about the wrong subject.
func TestDeletingAnUnknownProfileIsNotTheLastProfileError(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.Delete(t.Context(), 9999); err != ErrNoSuchProfile {
		t.Errorf("deleting an unknown id = %v, want ErrNoSuchProfile", err)
	}
}
