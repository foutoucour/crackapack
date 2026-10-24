package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

// systemBrowser opens url in the player's default browser, with the command each OS ships for it.
// It does not wait for the browser: it returns once the command has started.
func systemBrowser(url string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{url}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		name, args = "xdg-open", []string{url}
	}
	cmd := exec.CommandContext(context.Background(), name, args...) //nolint:gosec // the command is fixed per OS and the URL is the address this process listens on
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", name, err)
	}
	// Reap the command in the background, so that it does not stay a zombie.
	go func() { _ = cmd.Wait() }()
	return nil
}
