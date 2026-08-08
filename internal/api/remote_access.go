package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Benitoow/theia-media/internal/remoteaccess"
)

type remoteAccessUpdate struct {
	Enabled    *bool   `json:"enabled"`
	Automatic  *bool   `json:"automatic"`
	ListenPort *int    `json:"listen_port"`
	Endpoint   *string `json:"endpoint"`
}

type remotePeerCreate struct {
	Name string `json:"name"`
}

func (s *Server) handleRemoteAccessStatus(w http.ResponseWriter, r *http.Request) {
	if s.remote == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "remote_access_unavailable")
		return
	}
	status, err := s.remote.Status(r.Context())
	if err != nil {
		s.log.Error("reading remote access status failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "remote_access_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleUpdateRemoteAccess(w http.ResponseWriter, r *http.Request) {
	if s.remote == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "remote_access_unavailable")
		return
	}
	var body remoteAccessUpdate
	if err := decodeRemoteJSON(w, r, 16<<10, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_remote_access_payload")
		return
	}
	current, err := s.remote.Status(r.Context())
	if err != nil {
		s.log.Error("reading remote access configuration failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "remote_access_unavailable")
		return
	}
	cfg := current.Config
	if body.Enabled != nil {
		cfg.Enabled = *body.Enabled
	}
	if body.Automatic != nil {
		cfg.Automatic = *body.Automatic
	}
	if body.ListenPort != nil {
		cfg.ListenPort = *body.ListenPort
	}
	if body.Endpoint != nil {
		cfg.Endpoint = strings.TrimSpace(*body.Endpoint)
		// Typing an endpoint is what taking it over looks like. Leaving
		// automatic on would let the next discovery overwrite it, which reads
		// as the field not saving.
		if body.Automatic == nil && cfg.Endpoint != "" {
			cfg.Automatic = false
		}
	}
	status, err := s.remote.Update(r.Context(), cfg)
	if err != nil {
		s.writeRemoteAccessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleCreateRemotePeer(w http.ResponseWriter, r *http.Request) {
	if s.remote == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "remote_access_unavailable")
		return
	}
	var body remotePeerCreate
	if err := decodeRemoteJSON(w, r, 4<<10, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_remote_peer_payload")
		return
	}
	provision, err := s.remote.CreatePeer(r.Context(), body.Name)
	if err != nil {
		s.writeRemoteAccessError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, provision)
}

func (s *Server) handleRevokeRemotePeer(w http.ResponseWriter, r *http.Request) {
	if s.remote == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "remote_access_unavailable")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeJSONError(w, http.StatusBadRequest, "invalid_remote_peer_id")
		return
	}
	if err := s.remote.RevokePeer(r.Context(), id); err != nil {
		s.writeRemoteAccessError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoteSession(w http.ResponseWriter, r *http.Request) {
	if peer, ok := remoteaccess.PeerFromRequest(r); ok {
		writeJSON(w, http.StatusOK, remoteaccess.Session{Mode: "remote", Peer: &peer})
		return
	}
	writeJSON(w, http.StatusOK, remoteaccess.Session{Mode: "lan"})
}

func (s *Server) writeRemoteAccessError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "remote_access_unavailable"
	switch {
	case errors.Is(err, remoteaccess.ErrInvalidPort):
		status, code = http.StatusBadRequest, "invalid_remote_listen_port"
	case errors.Is(err, remoteaccess.ErrCarrierNAT):
		status, code = http.StatusConflict, remoteaccess.DiscoveryNotPublic
	case errors.Is(err, remoteaccess.ErrRouterRefused):
		status, code = http.StatusConflict, remoteaccess.DiscoveryRefused
	case errors.Is(err, remoteaccess.ErrNoAutomaticEndpoint):
		status, code = http.StatusConflict, remoteaccess.DiscoveryNoGateway
	case errors.Is(err, remoteaccess.ErrInvalidEndpoint):
		status, code = http.StatusBadRequest, "invalid_remote_endpoint"
	case errors.Is(err, remoteaccess.ErrInvalidPeerName):
		status, code = http.StatusBadRequest, "invalid_remote_peer_name"
	case errors.Is(err, remoteaccess.ErrPeerLimit):
		status, code = http.StatusConflict, "remote_peer_limit_reached"
	case errors.Is(err, remoteaccess.ErrPeerNotFound):
		status, code = http.StatusNotFound, "remote_peer_not_found"
	case errors.Is(err, remoteaccess.ErrDisabled):
		status, code = http.StatusConflict, "remote_access_disabled"
	case errors.Is(err, remoteaccess.ErrNotReady):
		status, code = http.StatusServiceUnavailable, "remote_access_not_ready"
	case errors.Is(err, remoteaccess.ErrUnavailable):
		status, code = http.StatusServiceUnavailable, "remote_access_unavailable"
	}
	if status >= 500 {
		s.log.Error("remote access request failed", "error", err)
	}
	writeJSONError(w, status, code)
}

func decodeRemoteJSON(w http.ResponseWriter, r *http.Request, limit int64, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("remote access payload contains trailing data")
	}
	return nil
}
