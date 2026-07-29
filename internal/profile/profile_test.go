package profile

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Benitoow/theia-media/internal/db"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	database, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewStore(database)
}

func TestMigrationCreatesOneLanguageNeutralDefault(t *testing.T) {
	store := newTestStore(t)

	profiles, err := store.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles = %d, want one", len(profiles))
	}
	if !profiles[0].Default || profiles[0].Name != nil {
		t.Errorf("default profile = %+v, want unnamed compatibility profile", profiles[0])
	}
	if id, err := store.DefaultID(t.Context()); err != nil || id != profiles[0].ID {
		t.Errorf("DefaultID = %d, %v; want %d", id, err, profiles[0].ID)
	}
}

func TestProfileLifecycleDoesNotDeleteTheDefault(t *testing.T) {
	store := newTestStore(t)

	created, err := store.Create(t.Context(), "  Alice  ")
	if err != nil {
		t.Fatal(err)
	}
	if created.Name == nil || *created.Name != "Alice" {
		t.Fatalf("created name = %v, want Alice", created.Name)
	}

	renamed, err := store.Rename(t.Context(), created.ID, "Alicia")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name == nil || *renamed.Name != "Alicia" {
		t.Fatalf("renamed name = %v, want Alicia", renamed.Name)
	}

	if err := store.Delete(t.Context(), created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), created.ID); !errors.Is(err, ErrNoSuchProfile) {
		t.Fatalf("Get deleted profile error = %v, want ErrNoSuchProfile", err)
	}

	defaultID, err := store.DefaultID(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(t.Context(), defaultID); !errors.Is(err, ErrDefaultProfile) {
		t.Fatalf("deleting default error = %v, want ErrDefaultProfile", err)
	}
}

func TestNamesAreBoundedAndNonEmpty(t *testing.T) {
	store := newTestStore(t)

	for _, name := range []string{"", " \t ", string(make([]byte, MaximumNameRunes+1))} {
		if _, err := store.Create(t.Context(), name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("Create(%q) error = %v, want ErrInvalidName", name, err)
		}
	}
}

func TestAvatarLifecycleAdvancesItsCacheVersion(t *testing.T) {
	store := newTestStore(t)
	p, err := store.Create(t.Context(), "Alice")
	if err != nil {
		t.Fatal(err)
	}

	image := []byte("small local image")
	withAvatar, err := store.SaveAvatar(t.Context(), p.ID, "image/webp", image)
	if err != nil {
		t.Fatal(err)
	}
	if !withAvatar.HasAvatar || withAvatar.AvatarVersion <= p.AvatarVersion {
		t.Fatalf("profile after avatar = %+v, want avatar and newer version", withAvatar)
	}

	got, err := store.Avatar(t.Context(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.MediaType != "image/webp" || !bytes.Equal(got.Data, image) {
		t.Errorf("avatar = %q %q, want image/webp and original bytes", got.MediaType, got.Data)
	}

	withoutAvatar, err := store.DeleteAvatar(t.Context(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if withoutAvatar.HasAvatar || withoutAvatar.AvatarVersion <= withAvatar.AvatarVersion {
		t.Fatalf("profile after avatar delete = %+v, want no avatar and newer version", withoutAvatar)
	}
	if _, err := store.Avatar(t.Context(), p.ID); !errors.Is(err, ErrNoAvatar) {
		t.Fatalf("Avatar after delete error = %v, want ErrNoAvatar", err)
	}
}
