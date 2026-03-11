package utils

import (
	"fmt"
	"strings"
)

// SetEnv は環境変数のスライスに対して、指定したキーの値を更新または追加します
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
