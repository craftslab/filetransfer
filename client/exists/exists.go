package exists

import (
	"os"
)

// FileExists returns true iff the path name is a file (and not a directory or non-existant).
func FileExists(name string) bool {
	fi, err := os.Stat(name)
	if err != nil {
		return false
	}

	if fi.IsDir() {
		return false
	}

	return true
}
