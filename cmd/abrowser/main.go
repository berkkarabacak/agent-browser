package main

import (
	"os"

	"abrowser/internal/cli"
	"abrowser/internal/desktop"
)

func main() {
	args := os.Args[1:]
	if cli.OpensWindow(args) {
		os.Exit(cli.Run(args, cli.IO{
			In:          os.Stdin,
			Out:         os.Stdout,
			Err:         os.Stderr,
			Interactive: cli.StdinIsTerminal(),
			OpenWindow:  desktop.Run,
		}))
	}
	attachConsole()
	os.Exit(cli.Run(args, cli.IO{
		In:          os.Stdin,
		Out:         os.Stdout,
		Err:         os.Stderr,
		Interactive: cli.StdinIsTerminal(),
	}))
}
