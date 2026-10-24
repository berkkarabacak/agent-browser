package cli

import "abrowser/internal/desktop"

func (c *CLI) guiCmd(args []string) int {
	fs := c.newFlags("gui", guiHelp)
	root := fs.String("root", "", "data root, if not passed before the command")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() != 0 {
		return c.usage(guiHelp)
	}
	if *root != "" {
		c.Root = *root
	}
	if err := desktop.Run(c.Root); err != nil {
		return c.fail(err)
	}
	return 0
}
