package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func CheckWritePermission(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create directory: %w", err)
	}

	testFile := filepath.Join(dir, ".hayatex_tmp")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}
