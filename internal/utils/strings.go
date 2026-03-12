package utils

import (
	"regexp"
	"strings"
)

// Case-insensitive prefix check that also returns the remaining string
func HasPrefixIgnoreCase(s, prefix string) (bool, string) {
	if strings.EqualFold(s[:len(prefix)], prefix) {
		return true, s[len(prefix):]
	}
	return false, ""
}

// Split key=value pairs, handling quoted values
func ParseKeyValuePairs(s string) map[string]string {
	re := regexp.MustCompile(`(\w+)=("[^"]*"|[^\s]+)`)
	matches := re.FindAllStringSubmatch(s, -1)
	params := make(map[string]string)
	for _, m := range matches {
		key := m[1]
		val := strings.Trim(m[2], "\"")
		params[key] = val
	}
	return params
}
