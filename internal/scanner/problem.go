package scanner

// Problem is something that went wrong during a scan, in a shape the interface
// can turn into a sentence.
//
// It carries a kind rather than a message on purpose. The first version put
// fmt.Errorf output straight into the report, and the settings page ended up
// showing people "GetFileAttributesEx D:\Films: Le fichier spécifié est
// introuvable." -- a syscall name, in the middle of a French interface, telling
// somebody whose USB drive is unplugged nothing they can act on.
//
// The technical detail still exists; it goes to the log, where it belongs.
type Problem struct {
	Kind ProblemKind `json:"kind"`

	// Path is the file or directory involved, shown to the user because it is
	// the one piece of technical detail that helps: it says which drive.
	Path string `json:"path,omitempty"`
}

// ProblemKind is the machine-readable half. The interface owns the wording.
type ProblemKind string

const (
	// KindDirectoryUnreadable is a configured library folder that cannot be
	// opened. In practice: an external drive that is not plugged in.
	KindDirectoryUnreadable ProblemKind = "directory_unreadable"

	// KindNotADirectory is a library path pointing at a file.
	KindNotADirectory ProblemKind = "not_a_directory"

	// KindSubdirectoryUnreadable is a folder inside the library that could not
	// be walked -- permissions, a broken symlink, a network share that dropped.
	KindSubdirectoryUnreadable ProblemKind = "subdirectory_unreadable"

	// KindFileUnreadable is a single file whose details could not be read.
	KindFileUnreadable ProblemKind = "file_unreadable"

	// KindSaveFailed is a film that could not be written to the database.
	KindSaveFailed ProblemKind = "save_failed"

	// KindMetadataUnavailable is a TMDB lookup that failed for a reason that is
	// not the film's fault.
	KindMetadataUnavailable ProblemKind = "metadata_unavailable"

	// KindMetadataKeyRejected is TMDB refusing the API key, which stops the
	// whole metadata pass and needs a human.
	KindMetadataKeyRejected ProblemKind = "metadata_key_rejected"

	// KindEpisodeSeriesUnknown is a filename with a reliable episode marker but
	// no series title in either the filename or its parent directories.
	KindEpisodeSeriesUnknown ProblemKind = "episode_series_unknown"
)
