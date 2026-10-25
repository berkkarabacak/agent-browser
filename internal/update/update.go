// Package update checks a GitHub releases feed for a newer Agent Browser.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"abrowser/internal/extensions"
)

// DefaultURL is the releases feed used when ABROWSER_UPDATE_URL is empty.
const DefaultURL = "https://api.github.com/repos/berkkarabacak/agent-browser/releases/latest"

// Info is a newer release, when one exists.
type Info struct {
	Newer  bool
	Latest string
	URL    string
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Check looks up the latest release. A missing feed (HTTP 404) is not an error.
// endpoint overrides the default URL when it is not empty.
func Check(ctx context.Context, current, endpoint string) (Info, error) {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = DefaultURL
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Info{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "abrowser")
	resp, err := client.Do(req)
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Info{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return Info{}, fmt.Errorf("update check: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Info{}, err
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return Info{}, err
	}
	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	if latest == "" {
		return Info{}, nil
	}
	newer := extensions.CompareVersion(strings.TrimPrefix(current, "v"), latest) < 0
	return Info{Newer: newer, Latest: latest, URL: rel.HTMLURL}, nil
}
