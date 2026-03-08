package utils

import (
	"fmt"
	"slices"
)

var units = []string{"B", "KB", "MB", "GB", "TB"}

// Converts size in inputUnit to proper unit (KB, MB, GB, etc.) and return a formatted string
func FormatBytes(size uint64, inputUnit string) string {
	inputUnitExponent := slices.Index(units, inputUnit)
	if inputUnitExponent == -1 {
		return "Invalid Size"
	}

	sizeInBytes := float64(size) * float64(int(1)<<(10*inputUnitExponent))

	exponent := 0
	for sizeInBytes >= 1024 && exponent < len(units)-1 {
		sizeInBytes /= 1024
		exponent++
	}

	return fmt.Sprintf("%.2f %s", sizeInBytes, units[exponent])
}
