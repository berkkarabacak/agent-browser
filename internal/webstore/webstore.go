// Package webstore downloads an extension from a Chrome Web Store link or id.
package webstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"abrowser/internal/names"
)

// idRe matches a Chrome extension id (32 characters from a to p).
var idRe = regexp.MustCompile(`^[a-p]{32}$`)

// CRXEndpoint is the update service that returns a CRX. Tests replace it.
var CRXEndpoint = "https://clients2.google.com/service/update2/crx"

const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.8078.47 Safari/537.36"

// ParseID accepts a bare extension id or a Chrome Web Store URL.
func ParseID(input string) (string, error) {
	s := strings.TrimSpace(input)
	if idRe.MatchString(s) {
		return s, nil
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("paste a Chrome Web Store link or a 32-character extension id")
	}
	host := strings.ToLower(u.Host)
	if host != "chromewebstore.google.com" && host != "chrome.google.com" && !strings.HasSuffix(host, ".chromewebstore.google.com") {
		return "", fmt.Errorf("that link is not from the Chrome Web Store")
	}
	if q := u.Query().Get("id"); idRe.MatchString(q) {
		return q, nil
	}
	for _, part := range strings.Split(u.Path, "/") {
		if idRe.MatchString(part) {
			return part, nil
		}
	}
	return "", fmt.Errorf("that Chrome Web Store link has no extension id")
}

// DownloadURL is the CRX request for id.
func DownloadURL(id string) string {
	return CRXEndpoint + "?response=redirect&os=win&arch=x64&os_arch=x86_64&nacl_arch=x86-64&prod=chromecrx&prodchannel=unknown&prodversion=156.0.8078.47&acceptformat=crx2,crx3&x=" + url.QueryEscape("id="+id+"&uc")
}

// Download saves the CRX for id into dest.
func Download(ctx context.Context, id, dest string, client *http.Client) error {
	if !idRe.MatchString(id) {
		return fmt.Errorf("invalid extension id %q", id)
	}
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DownloadURL(id), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", chromeUA)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach the Chrome Web Store: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Chrome Web Store returned %s for that extension", resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, 4)
	n, err := io.ReadFull(resp.Body, buf)
	if err != nil {
		return fmt.Errorf("Chrome Web Store did not return an extension")
	}
	if string(buf[:n]) != "Cr24" {
		return fmt.Errorf("Chrome Web Store did not return an extension")
	}
	if _, err := f.Write(buf[:n]); err != nil {
		return err
	}
	written, err := io.Copy(f, io.LimitReader(resp.Body, 80<<20))
	if err != nil {
		return err
	}
	if written >= 80<<20 {
		return fmt.Errorf("that extension download is too large")
	}
	return f.Close()
}

// Manifest is the name and version from an unpacked extension.
type Manifest struct {
	Name    string
	Version string
}

// ReadManifest reads manifest.json.
func ReadManifest(path string) (Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var raw struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return Manifest{}, fmt.Errorf("reading the extension name: %w", err)
	}
	if strings.TrimSpace(raw.Version) == "" {
		return Manifest{}, fmt.Errorf("the extension has no version")
	}
	return Manifest{Name: raw.Name, Version: raw.Version}, nil
}

// ExtensionSlug turns a manifest name into a store directory name.
// A name that cannot be used becomes ext- plus the Web Store id.
func ExtensionSlug(manifestName, id string) string {
	if strings.Contains(manifestName, "__MSG") {
		return extFallback(id)
	}
	var b strings.Builder
	prev := false
	for _, r := range strings.ToLower(strings.TrimSpace(manifestName)) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prev = false
		case r == '_' || r == '-' || unicode.IsSpace(r):
			if !prev && b.Len() > 0 {
				b.WriteByte('-')
				prev = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 64 {
		slug = strings.Trim(slug[:64], "-")
	}
	if names.ValidExtName(slug) != nil {
		return extFallback(id)
	}
	return slug
}

func extFallback(id string) string {
	s := "ext-" + id
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}
