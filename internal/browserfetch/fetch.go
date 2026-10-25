// Package browserfetch downloads Chrome for Testing into a data directory.
package browserfetch

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LastKnownGood is the Chrome for Testing version catalog.
const LastKnownGood = "https://googlechromelabs.github.io/chrome-for-testing/last-known-good-versions-with-downloads.json"

// Options selects the platform and where the zip is unpacked.
type Options struct {
	OS          string
	Arch        string
	DestDir     string
	Client      *http.Client
	ManifestURL string
	Channel     string
	Progress    func(done, total int64)
}

// Result is the unpacked Chrome executable.
type Result struct {
	Version    string
	Executable string
}

type cftDoc struct {
	Channels map[string]struct {
		Version   string `json:"version"`
		Downloads struct {
			Chrome []struct {
				Platform string `json:"platform"`
				URL      string `json:"url"`
			} `json:"chrome"`
		} `json:"downloads"`
	} `json:"channels"`
}

// Platform returns the Chrome for Testing platform key and the executable
// path inside the zip.
func Platform(goos, goarch string) (key, exe string, err error) {
	switch goos + "/" + goarch {
	case "windows/amd64":
		return "win64", "chrome-win64/chrome.exe", nil
	case "linux/amd64":
		return "linux64", "chrome-linux64/chrome", nil
	case "darwin/amd64":
		return "mac-x64", "chrome-mac-x64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing", nil
	case "darwin/arm64":
		return "mac-arm64", "chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing", nil
	default:
		return "", "", fmt.Errorf("Chrome for Testing has no build for %s/%s", goos, goarch)
	}
}

// Fetch downloads the Stable Chrome for Testing zip and unpacks it into DestDir.
func Fetch(ctx context.Context, opt Options) (Result, error) {
	if opt.OS == "" {
		opt.OS = runtime.GOOS
	}
	if opt.Arch == "" {
		opt.Arch = runtime.GOARCH
	}
	if strings.TrimSpace(opt.DestDir) == "" {
		return Result{}, fmt.Errorf("browser download needs a destination folder")
	}
	if opt.Channel == "" {
		opt.Channel = "Stable"
	}
	if opt.ManifestURL == "" {
		opt.ManifestURL = LastKnownGood
	}
	client := opt.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Minute}
	}
	key, rel, err := Platform(opt.OS, opt.Arch)
	if err != nil {
		return Result{}, err
	}
	body, err := getBytes(ctx, client, opt.ManifestURL)
	if err != nil {
		return Result{}, err
	}
	var doc cftDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return Result{}, fmt.Errorf("reading the Chrome version list: %w", err)
	}
	ch, ok := doc.Channels[opt.Channel]
	if !ok {
		return Result{}, fmt.Errorf("Chrome version list has no %s channel", opt.Channel)
	}
	zipURL := ""
	for _, item := range ch.Downloads.Chrome {
		if item.Platform == key {
			zipURL = item.URL
			break
		}
	}
	if zipURL == "" {
		return Result{}, fmt.Errorf("Chrome version list has no %s download", key)
	}
	tmp, err := os.CreateTemp("", "abrowser-chrome-*.zip")
	if err != nil {
		return Result{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := downloadFile(ctx, client, zipURL, tmp, opt.Progress); err != nil {
		tmp.Close()
		return Result{}, err
	}
	if err := tmp.Close(); err != nil {
		return Result{}, err
	}
	if err := Unzip(tmpName, opt.DestDir); err != nil {
		return Result{}, err
	}
	exe := filepath.Join(opt.DestDir, filepath.FromSlash(rel))
	st, err := os.Stat(exe)
	if err != nil || st.IsDir() {
		return Result{}, fmt.Errorf("Chrome unpacked without %s", rel)
	}
	if opt.OS != "windows" {
		if err := os.Chmod(exe, 0o755); err != nil {
			return Result{}, err
		}
	}
	return Result{Version: ch.Version, Executable: exe}, nil
}

func getBytes(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "abrowser")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

func downloadFile(ctx context.Context, client *http.Client, url string, dst *os.File, progress func(done, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "abrowser")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	const maxZip = 800 << 20
	cw := &countWriter{w: dst, total: resp.ContentLength, progress: progress}
	n, err := io.Copy(cw, io.LimitReader(resp.Body, maxZip+1))
	if err != nil {
		return err
	}
	if n > maxZip {
		return fmt.Errorf("Chrome download is larger than %d MB", maxZip>>20)
	}
	if progress != nil {
		total := resp.ContentLength
		if total <= 0 {
			total = n
		}
		progress(n, total)
	}
	return nil
}

type countWriter struct {
	w        io.Writer
	done     int64
	total    int64
	next     int64
	progress func(done, total int64)
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.done += int64(n)
	if c.progress != nil && (c.done >= c.next || err != nil) {
		c.next = c.done + (1 << 20)
		c.progress(c.done, c.total)
	}
	return n, err
}

// Unzip extracts zipPath into dest and rejects entries that escape dest.
func Unzip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	dest = filepath.Clean(dest)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	prefix := dest + string(os.PathSeparator)
	for _, f := range r.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
			return fmt.Errorf("zip entry %q is absolute", f.Name)
		}
		clean := path.Clean(name)
		if clean == ".." || strings.HasPrefix(clean, "../") || clean == "." && !f.FileInfo().IsDir() && name != "." {
			if clean == ".." || strings.HasPrefix(clean, "../") {
				return fmt.Errorf("zip entry %q escapes the archive", f.Name)
			}
		}
		if clean == "." {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(clean))
		if filepath.Clean(target) != dest && !strings.HasPrefix(filepath.Clean(target), prefix) {
			return fmt.Errorf("zip entry %q escapes the archive", f.Name)
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("zip entry %q is a symlink", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if f.Mode()&0o111 != 0 {
			mode = 0o755
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
