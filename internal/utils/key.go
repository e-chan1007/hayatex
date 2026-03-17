package utils

import (
	"bufio"
	"fmt"
	"os"
)

func WaitForAnyKey(message string) {
	if message != "" {
		fmt.Println(message)
	}
	bufio.NewReader(os.Stdin).Read(make([]byte, 1))
}
