package extensions

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func manifestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func wrapCRX2(payload []byte) []byte {
	pub := []byte("pub")
	sig := []byte("sig")
	var buf bytes.Buffer
	buf.WriteString("Cr24")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(2))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(pub)))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(sig)))
	buf.Write(pub)
	buf.Write(sig)
	buf.Write(payload)
	return buf.Bytes()
}

func wrapCRX3(payload []byte) []byte {
	header := []byte("proto-header")
	var buf bytes.Buffer
	buf.WriteString("Cr24")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(3))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(header)))
	buf.Write(header)
	buf.Write(payload)
	return buf.Bytes()
}

const sampleManifest = `{
  "manifest_version": 3,
  "name": "Sample",
  "version": "1.2.3",
  "description": "demo"
}`

func TestExtractCRX2AndCRX3(t *testing.T) {
	payload := manifestZip(t, map[string]string{
		"manifest.json": sampleManifest,
		"sw.js":         "console.log(1)\n",
	})
	for _, tc := range []struct {
		name string
		file []byte
	}{
		{"v2.crx", wrapCRX2(payload)},
		{"v3.crx", wrapCRX3(payload)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, tc.name)
			if err := os.WriteFile(src, tc.file, 0o644); err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(dir, "out")
			if err := ExtractCRX(src, dest); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(dest, "sw.js"))
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != "console.log(1)\n" {
				t.Fatalf("sw.js = %q", b)
			}
		})
	}
}

func TestExtractCRXRejectsZipSlip(t *testing.T) {
	payload := manifestZip(t, map[string]string{
		"manifest.json": sampleManifest,
		"../evil.txt":   "nope",
	})
	dir := t.TempDir()
	src := filepath.Join(dir, "bad.crx")
	if err := os.WriteFile(src, wrapCRX2(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ExtractCRX(src, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected zip slip error")
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); !os.IsNotExist(err) {
		t.Fatal("evil.txt should not exist")
	}
}

func TestAddDirAndCatalog(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "manifest.json"), []byte(sampleManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	ext, err := AddDir(root, "sample", "", src)
	if err != nil {
		t.Fatal(err)
	}
	if ext.Ref() != "sample@1.2.3" {
		t.Fatalf("ref %s", ext.Ref())
	}
	list, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ManifestName != "Sample" {
		t.Fatalf("%+v", list)
	}
	if _, err := AddDir(root, "sample", "", src); err == nil {
		t.Fatal("expected duplicate error")
	}
}
