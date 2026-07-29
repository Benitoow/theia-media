package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Benitoow/theia-media/internal/profile"
)

const (
	profileHeader          = "X-Theia-Profile"
	maximumAvatarUpload    = 8 << 20
	maximumProfileBodySize = 2 << 10
)

type profilesResponse struct {
	Profiles []profile.Profile `json:"profiles"`
}

type profileNameRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := s.profiles.List(r.Context())
	if err != nil {
		s.log.Error("listing profiles failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "profiles_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, profilesResponse{Profiles: profiles})
}

func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var body profileNameRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maximumProfileBodySize)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_profile_payload")
		return
	}

	created, err := s.profiles.Create(r.Context(), body.Name)
	switch {
	case errors.Is(err, profile.ErrInvalidName):
		writeJSONError(w, http.StatusBadRequest, "invalid_profile_name")
	case errors.Is(err, profile.ErrTooManyProfiles):
		writeJSONError(w, http.StatusConflict, "profile_limit_reached")
	case err != nil:
		s.log.Error("creating a profile failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "profile_create_failed")
	default:
		writeJSON(w, http.StatusCreated, created)
	}
}

func (s *Server) handleRenameProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := profilePathID(w, r)
	if !ok {
		return
	}

	var body profileNameRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maximumProfileBodySize)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_profile_payload")
		return
	}
	renamed, err := s.profiles.Rename(r.Context(), id, body.Name)
	switch {
	case errors.Is(err, profile.ErrInvalidName):
		writeJSONError(w, http.StatusBadRequest, "invalid_profile_name")
	case errors.Is(err, profile.ErrNoSuchProfile):
		writeJSONError(w, http.StatusNotFound, "profile_not_found")
	case err != nil:
		s.log.Error("renaming a profile failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "profile_update_failed")
	default:
		writeJSON(w, http.StatusOK, renamed)
	}
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := profilePathID(w, r)
	if !ok {
		return
	}

	err := s.profiles.Delete(r.Context(), id)
	switch {
	case errors.Is(err, profile.ErrNoSuchProfile):
		writeJSONError(w, http.StatusNotFound, "profile_not_found")
	case errors.Is(err, profile.ErrDefaultProfile):
		writeJSONError(w, http.StatusConflict, "default_profile")
	case errors.Is(err, profile.ErrLastProfile):
		writeJSONError(w, http.StatusConflict, "last_profile")
	case err != nil:
		s.log.Error("deleting a profile failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "profile_delete_failed")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleSaveProfileAvatar(w http.ResponseWriter, r *http.Request) {
	id, ok := profilePathID(w, r)
	if !ok {
		return
	}

	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maximumAvatarUpload))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "avatar_too_large")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid_avatar")
		return
	}
	processed, err := profile.ProcessAvatar(data)
	if err != nil {
		writeJSONError(w, http.StatusUnsupportedMediaType, "invalid_avatar")
		return
	}

	updated, err := s.profiles.SaveAvatar(r.Context(), id, "image/jpeg", processed)
	switch {
	case errors.Is(err, profile.ErrNoSuchProfile):
		writeJSONError(w, http.StatusNotFound, "profile_not_found")
	case err != nil:
		s.log.Error("saving a profile avatar failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "avatar_save_failed")
	default:
		writeJSON(w, http.StatusOK, updated)
	}
}

func (s *Server) handleDeleteProfileAvatar(w http.ResponseWriter, r *http.Request) {
	id, ok := profilePathID(w, r)
	if !ok {
		return
	}

	updated, err := s.profiles.DeleteAvatar(r.Context(), id)
	switch {
	case errors.Is(err, profile.ErrNoSuchProfile):
		writeJSONError(w, http.StatusNotFound, "profile_not_found")
	case err != nil:
		s.log.Error("deleting a profile avatar failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "avatar_delete_failed")
	default:
		writeJSON(w, http.StatusOK, updated)
	}
}

func (s *Server) handleProfileAvatar(w http.ResponseWriter, r *http.Request) {
	id, ok := profilePathID(w, r)
	if !ok {
		return
	}
	version, err := strconv.ParseInt(r.PathValue("version"), 10, 64)
	if err != nil || version < 1 {
		writeJSONError(w, http.StatusNotFound, "avatar_not_found")
		return
	}

	avatar, err := s.profiles.Avatar(r.Context(), id)
	switch {
	case errors.Is(err, profile.ErrNoSuchProfile), errors.Is(err, profile.ErrNoAvatar):
		writeJSONError(w, http.StatusNotFound, "avatar_not_found")
		return
	case err != nil:
		s.log.Error("reading a profile avatar failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "avatar_unavailable")
		return
	case avatar.Version != version:
		writeJSONError(w, http.StatusNotFound, "avatar_not_found")
		return
	}

	w.Header().Set("Content-Type", avatar.MediaType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeContent(w, r, "profile.jpg", avatar.LastUpdated, bytes.NewReader(avatar.Data))
}

// selectedProfileID resolves the freely selectable progress namespace for one
// request. A missing header means the compatibility default. An explicit stale
// or malformed selection never falls back silently, because that would write
// one person's progress into another's history.
func (s *Server) selectedProfileID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	// Progress is freely selectable, but it is still private household state.
	// A browser or intermediary must never reuse one profile's JSON for another.
	w.Header().Add("Vary", profileHeader)
	w.Header().Set("Cache-Control", "private, no-store")

	raw := r.Header.Get(profileHeader)
	if raw == "" {
		id, err := s.profiles.DefaultID(r.Context())
		if err != nil {
			s.log.Error("reading the default profile failed", "error", err)
			writeJSONError(w, http.StatusInternalServerError, "profiles_unavailable")
			return 0, false
		}
		return id, true
	}

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		writeJSONError(w, http.StatusBadRequest, "invalid_profile")
		return 0, false
	}
	if _, err := s.profiles.Get(r.Context(), id); errors.Is(err, profile.ErrNoSuchProfile) {
		writeJSONError(w, http.StatusNotFound, "profile_not_found")
		return 0, false
	} else if err != nil {
		s.log.Error("validating a selected profile failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "profiles_unavailable")
		return 0, false
	}
	return id, true
}

func profilePathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeJSONError(w, http.StatusBadRequest, "invalid_profile")
		return 0, false
	}
	return id, true
}
