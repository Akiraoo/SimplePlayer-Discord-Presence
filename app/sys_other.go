//go:build !windows

package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
)

// Used only for testing on Linux/macOS.
func openPipe(i int) (io.ReadWriteCloser, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	return net.Dial("unix", filepath.Join(base, fmt.Sprintf("discord-ipc-%d", i)))
}

func pid() int                 { return os.Getpid() }
func hideWindow(*exec.Cmd)     {}
func alert(text string)        { fmt.Println(text) }
func confirm(text string) bool { fmt.Println(text); return false }
