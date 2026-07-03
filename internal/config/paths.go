// Package config resolves the crew home directory and well-known paths.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Home returns the crew state directory (default ~/.crew, override via
// CREW_HOME), creating it and the logs subdirectory if needed.
func Home() (string, error) {
	dir := os.Getenv("CREW_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".crew")
	}
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o700); err != nil {
		return "", fmt.Errorf("create crew home: %w", err)
	}
	return dir, nil
}

func SocketPath(home string) string { return filepath.Join(home, "crew.sock") }
func DBPath(home string) string     { return filepath.Join(home, "crew.db") }
func DaemonLog(home string) string  { return filepath.Join(home, "daemon.log") }
func AgentLog(home, name string) string {
	return filepath.Join(home, "logs", name+".log")
}
