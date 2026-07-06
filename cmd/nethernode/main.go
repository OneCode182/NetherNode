// Command nethernode is the NetherNode server admin CLI.
//
// S3 ships the core packages (rcon, compose, backup, mcstatus) and this
// dispatch skeleton; lifecycle commands land in S4 and admin/settings in S5.
package main

import (
	"fmt"
	"os"
)

var version = "dev"

const usage = `NetherNode server CLI

Usage:
  nethernode help
  nethernode version

Commands:
  help       Show this help.
  version    Print CLI version.

Lifecycle commands (start, stop, restart, status, save-server,
backup-server), admin and settings management arrive in later steps;
ops/nethernode remains the operational entrypoint until then.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "help"
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	case "version", "--version":
		fmt.Printf("nethernode %s\n", version)
		return nil
	default:
		return fmt.Errorf("unknown command: %s\n\n%s", command, usage)
	}
}
