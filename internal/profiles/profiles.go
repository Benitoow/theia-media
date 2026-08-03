// Package profiles separates one household's viewing from another's.
//
// A profile is not an account. There is no password, no role, no permission and
// nothing to log out of: anyone who can reach Theia on the LAN may select and
// edit every profile, exactly as they may already change every setting
// (decisions 6 and 48). What a profile owns is a name, an optional picture and
// its own playback history.
package profiles

import (
	"errors"
	"time"
)

// MaxProfiles bounds an unauthenticated LAN surface, and the number comes from
// the screen rather than from taste. The chooser is one horizontal row read
// from three metres, where a card may not fall below the 160px legibility floor
// of the design system; past eight the row either wraps or shrinks under it.
// The previous implementation allowed twelve, which was chosen before that
// screen existed.
const MaxProfiles = 8

// MaxNameRunes counts runes, not bytes: "Benjaminous" and "Bénjaminoüs" must
// have the same budget.
const MaxNameRunes = 40

// MaxAvatarUpload is what the request body reader will accept before giving up.
// It is a bound on an unauthenticated endpoint, not a quality setting -- the
// stored picture is re-encoded to AvatarSize regardless.
const MaxAvatarUpload = 8 << 20

// AvatarSize is the square every stored picture is reduced to. Large enough for
// a television card, small enough that eight of them do not weigh on the row.
const AvatarSize = 512

var (
	ErrNoSuchProfile = errors.New("no such profile")
	ErrInvalidName   = errors.New("invalid profile name")
	ErrProfileLimit  = errors.New("profile limit reached")
	ErrLastProfile   = errors.New("the last profile cannot be deleted")
	ErrNoAvatar      = errors.New("this profile has no picture")
	ErrInvalidImage  = errors.New("the uploaded file is not a usable image")
	ErrImageTooLarge = errors.New("the uploaded file is too large")
)

// Profile is one viewer.
//
// Name is empty for the default profile created by the migration. That absence
// is deliberate and load-bearing: the interface names it in the active language
// rather than SQLite storing a French string somebody would later have to
// translate (decision 25).
type Profile struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name,omitempty"`
	IsDefault     bool      `json:"is_default"`
	HasAvatar     bool      `json:"has_avatar"`
	AvatarVersion int64     `json:"avatar_version,omitempty"`
	CreatedAt     time.Time `json:"created_at"`

	// Stats is filled only when a caller asks for one profile, because the
	// chooser shows eight cards and none of them displays a count.
	Stats *Stats `json:"stats,omitempty"`
}

// Stats are the local facts the profile page shows. There is deliberately no
// email, role, status or subscription here: the reference screens carried them,
// Theia has none of them, and inventing them would be a lie with a nice layout.
type Stats struct {
	MoviesStarted    int        `json:"movies_started"`
	MoviesFinished   int        `json:"movies_finished"`
	EpisodesStarted  int        `json:"episodes_started"`
	EpisodesFinished int        `json:"episodes_finished"`
	LastWatchedAt    *time.Time `json:"last_watched_at,omitempty"`
}

// Avatar is a stored picture, already normalised.
type Avatar struct {
	Bytes       []byte
	ContentType string
	Version     int64
}
