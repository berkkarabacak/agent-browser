package extensions

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ExtractCRX unpacks a CRX2 or CRX3 archive into dest.
// The zip payload must contain manifest.json at its root.
func ExtractCRX(src, dest string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	payload, err := crxPayload(b)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return fmt.Errorf("crx zip: %w", err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	foundManifest := false
	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasPrefix(name, "/") || path.IsAbs(name) {
			return fmt.Errorf("crx entry %q is absolute", f.Name)
		}
		clean := path.Clean(name)
		if clean == ".." || strings.HasPrefix(clean, "../") || clean == "." && f.FileInfo().IsDir() {
			if clean == ".." || strings.HasPrefix(clean, "../") {
				return fmt.Errorf("crx entry %q escapes the archive", f.Name)
			}
		}
		if clean == "." {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(clean))
		if !within(dest, target) && filepath.Clean(target) != filepath.Clean(dest) {
			return fmt.Errorf("crx entry %q escapes the archive", f.Name)
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
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
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
		if clean == "manifest.json" {
			foundManifest = true
		}
	}
	if !foundManifest {
		return fmt.Errorf("crx has no manifest.json at the archive root")
	}
	return nil
}

func crxPayload(b []byte) ([]byte, error) {
	if len(b) >= 4 && bytes.Equal(b[:4], []byte("PK\x03\x04")) {
		return nil, fmt.Errorf("file is a zip archive, not a .crx; unpack it and use --from")
	}
	if len(b) < 16 || string(b[:4]) != "Cr24" {
		return nil, fmt.Errorf("not a CRX file (missing Cr24 header)")
	}
	version := binary.LittleEndian.Uint32(b[4:8])
	var start int
	switch version {
	case 2:
		pubLen := binary.LittleEndian.Uint32(b[8:12])
		sigLen := binary.LittleEndian.Uint32(b[12:16])
		start = 16 + int(pubLen) + int(sigLen)
	case 3:
		headerLen := binary.LittleEndian.Uint32(b[8:12])
		start = 12 + int(headerLen)
	default:
		return nil, fmt.Errorf("unsupported CRX version %d", version)
	}
	if start < 0 || start > len(b) {
		return nil, fmt.Errorf("crx header length is invalid")
	}
	payload := b[start:]
	if len(payload) < 4 || !bytes.Equal(payload[:4], []byte("PK\x03\x04")) {
		return nil, fmt.Errorf("crx payload is not a zip archive")
	}
	return payload, nil
}
