// internal/utils/arch.go (新設) または internal/utils/strings.go などへ

package utils

import (
	"path/filepath"
	"runtime"
)

func DetectTeXLiveArch() string {
	osName := runtime.GOOS
	archName := runtime.GOARCH

	switch osName {
	case "windows":
		return "windows"
	case "darwin":
		return "universal-darwin"
	case "linux":
		if isMusl() {
			return "x86_64-linuxmusl"
		}
		switch archName {
		case "amd64":
			return "x86_64-linux"
		case "arm64":
			return "aarch64-linux"
		case "386":
			return "i386-linux"
		}
	}
	return ""
}

func isMusl() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	matches, _ := filepath.Glob("/lib/ld-musl-*.so.*")
	return len(matches) > 0
}
