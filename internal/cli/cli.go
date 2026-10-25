// Package cli is the abrowser command line.
package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"abrowser/internal/app"
	"abrowser/internal/version"
)

// Version is the abrowser release.
const Version = version.Current

// IO is the process stdio plus whether prompts are allowed.
// OpenWindow, when set, runs for an invocation with no command so a
// double-clicked app opens the desktop window. Tests leave it nil and
// still get the help text.
type IO struct {
	In          io.Reader
	Out         io.Writer
	Err         io.Writer
	Interactive bool
	OpenWindow  func(root string) error
}

// CLI runs one invocation.
type CLI struct {
	IO   IO
	JSON bool
	Root string
}

// Run executes args (os.Args without the program name) and returns an exit code.
func Run(args []string, io IO) int {
	if io.Out == nil {
		io.Out = os.Stdout
	}
	if io.Err == nil {
		io.Err = os.Stderr
	}
	if io.In == nil {
		io.In = os.Stdin
	}
	root, jsonOut, rest, err := splitGlobals(args)
	if err != nil {
		fmt.Fprintln(io.Err, "error:", err)
		return 2
	}
	if root == "" {
		root = os.Getenv("ABROWSER_ROOT")
	}
	c := &CLI{IO: io, JSON: jsonOut, Root: root}
	return c.dispatch(rest)
}

// OpensWindow reports whether this invocation should open the desktop window.
// gui --help stays on the command line.
func OpensWindow(args []string) bool {
	rest, err := dropGlobals(args)
	if err != nil || len(rest) == 0 {
		return err == nil
	}
	if rest[0] != "gui" {
		return false
	}
	for _, a := range rest[1:] {
		if a == "--help" || a == "-h" || a == "help" {
			return false
		}
	}
	return true
}

func dropGlobals(args []string) ([]string, error) {
	_, _, rest, err := splitGlobals(args)
	return rest, err
}

func (c *CLI) dispatch(args []string) int {
	if len(args) == 0 {
		if c.IO.OpenWindow != nil {
			if err := c.IO.OpenWindow(c.Root); err != nil {
				return c.fail(err)
			}
			return 0
		}
		fmt.Fprint(c.IO.Out, rootHelp)
		return 0
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		if len(args) > 1 && args[0] == "help" {
			return c.dispatch(append(append([]string{}, args[1:]...), "--help"))
		}
		fmt.Fprint(c.IO.Out, rootHelp)
		return 0
	}
	switch args[0] {
	case "init":
		return c.initCmd(args[1:])
	case "version", "--version", "-v":
		fmt.Fprintf(c.IO.Out, "abrowser %s\n", Version)
		return 0
	case "config":
		return c.configCmd(args[1:])
	case "ext":
		return c.extCmd(args[1:])
	case "group":
		return c.groupCmd(args[1:])
	case "template":
		return c.templateCmd(args[1:])
	case "clone":
		return c.cloneCmd(args[1:])
	case "list":
		return c.listCmd(args[1:])
	case "launch":
		return c.launchCmd(args[1:])
	case "close":
		return c.closeCmd(args[1:])
	case "delete":
		return c.deleteCmd(args[1:])
	case "status":
		return c.statusCmd(args[1:])
	case "profile":
		return c.profileCmd(args[1:])
	case "gui":
		return c.guiCmd(args[1:])
	default:
		fmt.Fprintf(c.IO.Err, "error: unknown command %q\n\n", args[0])
		fmt.Fprint(c.IO.Err, rootHelp)
		return 2
	}
}

func splitGlobals(args []string) (root string, jsonOut bool, rest []string, err error) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--json":
			jsonOut = true
			i++
		case a == "--root":
			if i+1 >= len(args) {
				return "", false, nil, fmt.Errorf("--root requires a path")
			}
			root = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--root="):
			root = strings.TrimPrefix(a, "--root=")
			if root == "" {
				return "", false, nil, fmt.Errorf("--root requires a path")
			}
			i++
		default:
			return root, jsonOut, args[i:], nil
		}
	}
	return root, jsonOut, nil, nil
}

func (c *CLI) open() (*app.App, error) {
	return app.Open(c.Root)
}

func (c *CLI) fail(err error) int {
	if err == nil {
		return 0
	}
	fmt.Fprintln(c.IO.Err, "error:", err)
	return 1
}

func (c *CLI) help(text string) int {
	fmt.Fprint(c.IO.Out, text)
	return 0
}

func (c *CLI) usage(text string) int {
	fmt.Fprint(c.IO.Err, text)
	return 2
}

func (c *CLI) newFlags(name, helpText string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(c.IO.Err)
	fs.Usage = func() { fmt.Fprint(c.IO.Err, helpText) }
	return fs
}

func (c *CLI) parse(fs *flag.FlagSet, args []string) (int, bool) {
	// The standard flag parser stops at the first positional argument.
	// Commands are documented as `cmd <id> --flag value`, so move flags forward.
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, false
		}
		return 2, false
	}
	return 0, true
}

// reorderFlags lets flags appear before or after positional arguments.
func reorderFlags(fs *flag.FlagSet, args []string) []string {
	takesValue := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) {
		isBool := false
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok {
			isBool = bf.IsBoolFlag()
		}
		takesValue[f.Name] = !isBool
	})
	var flags []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		name := strings.TrimLeft(arg, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		flags = append(flags, arg)
		if takesValue[name] && !strings.Contains(arg, "=") && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

func (c *CLI) printJSON(v any) int {
	enc := json.NewEncoder(c.IO.Out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return c.fail(err)
	}
	return 0
}

func (c *CLI) confirmExact(want, prompt string) bool {
	if !c.IO.Interactive {
		return false
	}
	fmt.Fprint(c.IO.Out, prompt)
	reader := bufio.NewReader(c.IO.In)
	line, err := reader.ReadString('\n')
	if err != nil && len(strings.TrimSpace(line)) == 0 {
		return false
	}
	return strings.TrimSpace(line) == want
}

func (c *CLI) confirmYes(prompt string) bool {
	if !c.IO.Interactive {
		return false
	}
	fmt.Fprint(c.IO.Out, prompt)
	reader := bufio.NewReader(c.IO.In)
	line, _ := reader.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func flagSeen(fs *flag.FlagSet, name string) bool {
	seen := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			seen = true
		}
	})
	return seen
}

func strPtr(s string) *string { return &s }

func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "t", "true", "y", "yes", "on":
		return true, nil
	case "0", "f", "false", "n", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("expected true or false, got %q", s)
	}
}

// StdinIsTerminal reports whether stdin is a character device.
func StdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func isHelpToken(s string) bool {
	return s == "--help" || s == "-h" || s == "help"
}
