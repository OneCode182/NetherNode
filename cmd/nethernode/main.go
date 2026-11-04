// Command nethernode is the NetherNode server admin CLI.
//
// S3 shipped the core packages (rcon, compose, backup, mcstatus); S4 wires
// them into the lifecycle commands (start, stop, restart, status,
// save-server, backup-server) implemented in internal/cli. S5 adds admin
// (ops.json), settings (server.properties), and plugin script delegation.
package main

import (
	"os"

	"github.com/onecode182/nethernode/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
