//go:build unix

package client

import "syscall"

// detachedProcAttr detaches the daemon from the CLI's session so it
// survives the terminal closing.
func detachedProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
