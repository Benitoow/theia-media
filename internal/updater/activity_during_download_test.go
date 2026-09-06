package updater

import (
	"errors"
	"github.com/Benitoow/theia-media/internal/activity"
	"net/http"
	"testing"
)

type auditRoundTrip func(*http.Request) (*http.Response, error)

func (f auditRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAuditPlaybackStartsDuringDownload(t *testing.T) {
	inst := newInstallation(t)
	binary := buildHelper(t, "1.1.0")
	stub := stubGitHub(t, "v1.1.0", binary, digestOf(t, binary))
	defer stub.Close()
	tracker := activity.New()
	u := newUpdater(t, inst, stub.URL, tracker, func() {})
	var end func()
	u.http.Transport = auditRoundTrip(func(r *http.Request) (*http.Response, error) {
		if end == nil {
			end = tracker.Begin()
		}
		return http.DefaultTransport.RoundTrip(r)
	})
	err := u.Apply(t.Context())
	if end != nil {
		defer end()
	}
	if !errors.Is(err, ErrPlaybackInProgress) {
		t.Errorf("CONFIRMED: Apply=%v while activity busy=%v; installed state=%s", err, tracker.Busy(), u.Status().State)
	}

}
