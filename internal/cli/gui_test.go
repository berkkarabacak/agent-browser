package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestGUIHelp(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run([]string{"gui", "--help"}, IO{Out: &out, Err: &errBuf})
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	text := out.String() + errBuf.String()
	if !strings.Contains(text, "abrowser gui --root PATH") {
		t.Fatalf("help missing gui --root:\n%s", text)
	}
	if !strings.Contains(text, "Launch and Close") {
		t.Fatalf("help missing launch/close:\n%s", text)
	}
}
