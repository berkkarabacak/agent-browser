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

func TestOpensWindow(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, true},
		{[]string{"gui"}, true},
		{[]string{"gui", "--root", "/tmp/x"}, true},
		{[]string{"--root", "/tmp/x"}, true},
		{[]string{"gui", "--help"}, false},
		{[]string{"gui", "-h"}, false},
		{[]string{"--help"}, false},
		{[]string{"version"}, false},
		{[]string{"list"}, false},
		{[]string{"help"}, false},
	}
	for _, tc := range cases {
		if got := OpensWindow(tc.args); got != tc.want {
			t.Fatalf("%v: got %v want %v", tc.args, got, tc.want)
		}
	}
}

func TestNoArgsHelpOrWindow(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run(nil, IO{Out: &out, Err: &errBuf})
	if code != 0 || !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("help exit %d out %q err %q", code, out.String(), errBuf.String())
	}
	var root string
	code = Run([]string{"--root", "/tmp/abrowser-window"}, IO{
		Out: &out,
		Err: &errBuf,
		OpenWindow: func(got string) error {
			root = got
			return nil
		},
	})
	if code != 0 || root != "/tmp/abrowser-window" {
		t.Fatalf("window exit %d root %q err %q", code, root, errBuf.String())
	}
}
