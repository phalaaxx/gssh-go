package main

import (
	"os"
)

/* IsTerminal returns true if output device is terminal */
func IsTerminal(f *os.File) bool {
	fileInfo, err := f.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
