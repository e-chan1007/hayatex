//go:build unix

package utils

import (
	"os"
	"strings"

	"github.com/mitchellh/go-ps"
)

func IsLaunchedByGui() bool {
	ppid := os.Getppid()
	if ppid <= 1 {
		return true
	}
	process, err := ps.FindProcess(ppid)
	if err != nil || process == nil {
		return false
	}
	parentName := strings.ToLower(process.Executable())
	return !strings.HasSuffix(parentName, "sh")
}
