package main

import (
	"os"

	"abrowser/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.IO{
		In:          os.Stdin,
		Out:         os.Stdout,
		Err:         os.Stderr,
		Interactive: cli.StdinIsTerminal(),
	}))
}
