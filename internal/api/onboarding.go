package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/discovery"
)

type onboardingAddress struct {
	URL       string `json:"url"`
	Address   string `json:"address"`
	Interface string `json:"interface"`
	Virtual   bool   `json:"virtual"`
}

type onboardingResponse struct {
	// Needed is false once the screen has been dismissed, and stays false.
	Needed bool `json:"needed"`

	// PrimaryURL is what the QR code encodes: an IP address, never the mDNS
	// name. Android does not resolve .local, which was confirmed on a real
	// phone rather than assumed, so the name can only ever be a convenience.
	PrimaryURL string `json:"primary_url"`
	QRCodeSVG  string `json:"qr_code_svg"`

	// Alternatives are the other addresses this machine answers on, offered
	// because ranking cannot be certain: a laptop running Docker or Hyper-V has
	// several private addresses and only one a phone can reach.
	Alternatives []onboardingAddress `json:"alternatives"`

	// MDNSURL is shown as a secondary line where it works at all.
	MDNSURL string `json:"mdns_url,omitempty"`
}

// handleOnboarding describes how to reach this server from another device.
func (s *Server) handleOnboarding(w http.ResponseWriter, r *http.Request) {
	needed, err := s.onboardingNeeded(r.Context())
	if err != nil {
		s.log.Error("reading the onboarding state failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "could not read the onboarding state")
		return
	}

	candidates, err := discovery.Candidates()
	if err != nil {
		s.log.Error("listing network addresses failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "could not read the network addresses")
		return
	}
	if len(candidates) == 0 {
		writeJSONError(w, http.StatusServiceUnavailable,
			"this machine has no usable network address; is it connected?")
		return
	}

	response := onboardingResponse{
		Needed:     needed,
		PrimaryURL: fmt.Sprintf("http://%s:%d", candidates[0].Text, s.cfg.Port),
		MDNSURL:    fmt.Sprintf("http://%s.local:%d", s.cfg.Hostname, s.cfg.Port),
	}
	for _, c := range candidates[1:] {
		response.Alternatives = append(response.Alternatives, onboardingAddress{
			URL:       fmt.Sprintf("http://%s:%d", c.Text, s.cfg.Port),
			Address:   c.Text,
			Interface: c.Interface,
			Virtual:   c.Virtual,
		})
	}

	svg, err := discovery.QRSVG(response.PrimaryURL)
	if err != nil {
		s.log.Error("generating the QR code failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "the QR code could not be generated")
		return
	}
	response.QRCodeSVG = svg

	writeJSON(w, http.StatusOK, response)
}

// handleCompleteOnboarding dismisses the welcome screen for good.
func (s *Server) handleCompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	if err := s.state.Set(r.Context(), db.KeyOnboardingCompleted,
		time.Now().UTC().Format(time.RFC3339)); err != nil {
		s.log.Error("recording onboarding completion failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "the state could not be saved")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// onboardingNeeded reports whether the welcome screen should be shown.
func (s *Server) onboardingNeeded(ctx context.Context) (bool, error) {
	done, err := s.state.Has(ctx, db.KeyOnboardingCompleted)
	if err != nil {
		return false, err
	}
	return !done, nil
}
