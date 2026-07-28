package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
)

// ErrNoRelease means the repository has published nothing yet, which is an
// ordinary state for a young project rather than a failure.
var ErrNoRelease = errors.New("updater: the repository has no published release")

// release is the part of the GitHub API response Theia uses.
type release struct {
	TagName    string  `json:"tag_name"`
	Name       string  `json:"name"`
	HTMLURL    string  `json:"html_url"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

type asset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`

	// Digest is reported by GitHub for every asset as "sha256:<hex>". It is
	// what M4 used to pin ffmpeg without downloading it, and it means a release
	// needs no separate checksum file to be verifiable.
	Digest string `json:"digest"`
}

// assetName is what the release workflow names the binary for a platform. It
// has to match the CI build step exactly; a mismatch means an update that can
// never find itself.
func assetName(goos, goarch string) string {
	name := fmt.Sprintf("theia-%s-%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// latestRelease fetches the most recent published release.
func (u *Updater) latestRelease(ctx context.Context) (*release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", u.apiBase, u.repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("updater: building the request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	res, err := u.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("updater: contacting GitHub: %w", err)
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusNotFound:
		// No releases yet, or a private repository. Neither is worth alarming
		// anybody about.
		return nil, ErrNoRelease
	case res.StatusCode == http.StatusForbidden:
		// Unauthenticated GitHub API calls are rate limited per IP. A media
		// server checking a few times a day never gets near it, but say so
		// plainly rather than reporting a mystery.
		return nil, fmt.Errorf("updater: GitHub declined the request, possibly rate limited")
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("updater: GitHub returned %d", res.StatusCode)
	}

	var rel release
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("updater: reading the release: %w", err)
	}
	if rel.Draft {
		return nil, ErrNoRelease
	}
	return &rel, nil
}

// assetFor finds the binary for this platform and the digest to verify it with.
//
// A release with no digest for its asset is refused outright. Installing an
// unverified binary is exactly the failure this whole milestone is supposed to
// avoid, and "we could not check it" is not a reason to proceed.
func assetFor(rel *release, goos, goarch string) (asset, string, error) {
	want := assetName(goos, goarch)

	for _, a := range rel.Assets {
		if a.Name != want {
			continue
		}
		digest, ok := strings.CutPrefix(a.Digest, "sha256:")
		if !ok || len(digest) != 64 {
			return asset{}, "", fmt.Errorf(
				"updater: release %s publishes %s without a usable SHA-256 digest",
				rel.TagName, a.Name)
		}
		return a, digest, nil
	}

	return asset{}, "", fmt.Errorf("updater: release %s has no binary for %s/%s",
		rel.TagName, goos, goarch)
}

// currentPlatform is split out so tests can ask about other platforms.
func currentPlatform() (string, string) { return runtime.GOOS, runtime.GOARCH }
