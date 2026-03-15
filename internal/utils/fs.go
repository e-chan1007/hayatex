package utils

import (
	"fmt"
	"os"
)

func CheckWritePermission(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create directory: %w", err)
	}

	f, err := os.CreateTemp(dir, ".hayatex_tmp_*")
	if err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}
	_ = os.Remove(name)

	return nil
}
