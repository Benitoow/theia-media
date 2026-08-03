package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Benitoow/theia-media/internal/profiles"
)

// profileID resolves which viewer a request speaks for.
//
// The identity travels as ?profile=<id>, in the open, on the routes that read
// or write a playback position. That is deliberately not the header the removed
// implementation used: a header reads like a credential, and this is not one.
// Anyone on the LAN may pass any id, exactly as anyone on the LAN may already
// change every setting (decisions 6 and 48). It selects a history; it proves
// nothing.
//
// An absent or unknown id falls back to the oldest profile, which is what keeps
// the released frontend -- and any client that has never heard of profiles --
// working unchanged against this server.
func (s *Server) profileID(r *http.Request) (int64, error) {
	if s.profiles == nil {
		return 0, nil
	}

	raw := r.URL.Query().Get("profile")
	if raw == "" {
		return s.profiles.DefaultID(r.Context())
	}

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errInvalidProfileID
	}
	// An id that names nothing is refused rather than silently redirected to the
	// default: writing one viewer's position into another's history because a
	// television held a stale id is the sort of corruption nobody reports.
	exists, err := s.profiles.Exists(r.Context(), id)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, errUnknownProfile
	}
	return id, nil
}

var (
	errInvalidProfileID = errors.New("invalid profile id")
	errUnknownProfile   = errors.New("unknown profile")
)

// resolveProfile writes the error response itself, so handlers stay a single
// early return.
func (s *Server) resolveProfile(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := s.profileID(r)
	switch {
	case errors.Is(err, errInvalidProfileID):
		writeJSONError(w, http.StatusBadRequest, "invalid_profile_id")
		return 0, false
	case errors.Is(err, errUnknownProfile), errors.Is(err, profiles.ErrNoSuchProfile):
		writeJSONError(w, http.StatusNotFound, "profile_not_found")
		return 0, false
	case err != nil:
		s.log.Error("resolving the requested profile failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "profile_unavailable")
		return 0, false
	}
	return id, true
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	if s.profiles == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "profile_unavailable")
		return
	}
	list, err := s.profiles.List(r.Context())
	if err != nil {
		s.log.Error("listing profiles failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "profile_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": list})
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	if s.profiles == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "profile_unavailable")
		return
	}
	id, ok := pathID(w, r, "id", "invalid_profile_id")
	if !ok {
		return
	}
	profile, err := s.profiles.Get(r.Context(), id)
	if s.writeProfileError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

type profileNameRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	if s.profiles == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "profile_unavailable")
		return
	}
	var body profileNameRequest
	if err := decodeRemoteJSON(w, r, 4<<10, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_profile_payload")
		return
	}
	profile, err := s.profiles.Create(r.Context(), body.Name)
	if s.writeProfileError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, profile)
}

func (s *Server) handleRenameProfile(w http.ResponseWriter, r *http.Request) {
	if s.profiles == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "profile_unavailable")
		return
	}
	id, ok := pathID(w, r, "id", "invalid_profile_id")
	if !ok {
		return
	}
	var body profileNameRequest
	if err := decodeRemoteJSON(w, r, 4<<10, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_profile_payload")
		return
	}
	profile, err := s.profiles.Rename(r.Context(), id, body.Name)
	if s.writeProfileError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	if s.profiles == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "profile_unavailable")
		return
	}
	id, ok := pathID(w, r, "id", "invalid_profile_id")
	if !ok {
		return
	}
	if err := s.profiles.Delete(r.Context(), id); s.writeProfileError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSetProfileAvatar takes the raw image as the request body rather than a
// multipart form. There is one field, and multipart would add a parser and a
// temporary file for no gain.
func (s *Server) handleSetProfileAvatar(w http.ResponseWriter, r *http.Request) {
	if s.profiles == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "profile_unavailable")
		return
	}
	id, ok := pathID(w, r, "id", "invalid_profile_id")
	if !ok {
		return
	}

	image, contentType, err := profiles.Normalise(
		http.MaxBytesReader(w, r.Body, profiles.MaxAvatarUpload+1))
	switch {
	case errors.Is(err, profiles.ErrImageTooLarge):
		writeJSONError(w, http.StatusRequestEntityTooLarge, "profile_image_too_large")
		return
	case errors.Is(err, profiles.ErrInvalidImage):
		writeJSONError(w, http.StatusUnsupportedMediaType, "profile_image_unreadable")
		return
	case err != nil:
		s.log.Error("normalising a profile picture failed", "profile_id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "profile_unavailable")
		return
	}

	profile, err := s.profiles.SetAvatar(r.Context(), id, image, contentType)
	if s.writeProfileError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleDeleteProfileAvatar(w http.ResponseWriter, r *http.Request) {
	if s.profiles == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "profile_unavailable")
		return
	}
	id, ok := pathID(w, r, "id", "invalid_profile_id")
	if !ok {
		return
	}
	profile, err := s.profiles.ClearAvatar(r.Context(), id)
	if s.writeProfileError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleProfileAvatar(w http.ResponseWriter, r *http.Request) {
	if s.profiles == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "profile_unavailable")
		return
	}
	id, ok := pathID(w, r, "id", "invalid_profile_id")
	if !ok {
		return
	}
	avatar, err := s.profiles.Avatar(r.Context(), id)
	switch {
	case errors.Is(err, profiles.ErrNoAvatar):
		writeJSONError(w, http.StatusNotFound, "profile_image_not_found")
		return
	case err != nil:
		if s.writeProfileError(w, err) {
			return
		}
	}

	// The version is in the query, so the bytes behind one URL never change and
	// a television may keep them for a year.
	if r.URL.Query().Get("v") == strconv.FormatInt(avatar.Version, 10) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("Content-Type", avatar.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(avatar.Bytes)
}

func (s *Server) writeProfileError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, profiles.ErrNoSuchProfile):
		writeJSONError(w, http.StatusNotFound, "profile_not_found")
	case errors.Is(err, profiles.ErrInvalidName):
		writeJSONError(w, http.StatusBadRequest, "invalid_profile_name")
	case errors.Is(err, profiles.ErrProfileLimit):
		writeJSONError(w, http.StatusConflict, "profile_limit_reached")
	case errors.Is(err, profiles.ErrLastProfile):
		writeJSONError(w, http.StatusConflict, "profile_last_remaining")
	default:
		s.log.Error("a profile request failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "profile_unavailable")
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request, name, code string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, code)
		return 0, false
	}
	return id, true
}
