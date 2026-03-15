package utils

import (
	"fmt"
	"strings"
)

// Update environment variable in the given env slice
func SetEnv(env []string, key, value string) []string {
	prefix := key + "="
	newEntry := fmt.Sprintf("%s=%s", key, value)

	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = newEntry
			return env
		}
	}
	return append(env, newEntry)
}
