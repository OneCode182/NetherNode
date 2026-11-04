//go:build !linux

package cli

import "fmt"

func lockPluginUpdates(string) (func(), error) {
	return nil, fmt.Errorf("plugin update: apply and staging require the Linux EC2 host")
}
