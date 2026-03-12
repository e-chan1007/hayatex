package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var resolvedExecutables = make(map[string]string)

func ResolveExecutable(paths ...string) (string, error) {
	path := filepath.Join(paths...)
	if runtime.GOOS != "windows" {
		return path, nil
	}

	if resolved, ok := resolvedExecutables[path]; ok {
		return resolved, nil
	}

	pathExt := os.Getenv("PATHEXT")
	extensions := strings.Split(strings.ToUpper(pathExt), ";")
	candidates := append([]string{""}, extensions...)

	for _, ext := range candidates {
		fullPath := filepath.Join(filepath.Dir(path), filepath.Base(path)+ext)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			resolvedExecutables[path] = fullPath
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("executable not found: %s", path)
}
