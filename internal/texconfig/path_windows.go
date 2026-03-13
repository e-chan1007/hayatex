//go:build windows

package texconfig

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/e-chan1007/hayatex/internal/config"
	"golang.org/x/sys/windows/registry"
)

func AddSystemLinks(config *config.Config) error {
	binDir := filepath.Join(config.TexDir, "bin", "windows")

	key := registry.Key(0)
	var err error

	if config.InstallForAllUsers {
		key, err = registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			key = registry.Key(0)
		}
	}
	if key == registry.Key(0) {
		key, err = registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	}
	if err != nil {
		return err
	}

	defer key.Close()

	oldPath, _, _ := key.GetStringValue("Path")
	pathParts := strings.Split(oldPath, ";")
	for _, part := range pathParts {
		if strings.EqualFold(part, binDir) {
			return nil // Already in PATH
		}
	}
	if err := key.SetStringValue("Path", fmt.Sprintf("%s;%s", oldPath, binDir)); err != nil {
		return err
	}

	uintPtr := func(s string) uintptr {
		p, _ := syscall.UTF16PtrFromString(s)
		return uintptr(unsafe.Pointer(p))
	}

	syscall.NewLazyDLL("user32.dll").NewProc("SendMessageTimeoutW").Call(
		uintptr(0xffff), // HWND_BROADCAST
		0x001a,          // WM_SETTINGCHANGE
		0,
		uintPtr("Environment"),
		0x0002, // SMTO_ABORTIFHUNG
		5000,
		0,
	)
	return nil
}
