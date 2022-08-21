package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func FreeFileName(filename string) (string, bool) {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	// checks if a .tar stands before the "actual" file ending
	if extraExt := filepath.Ext(base); extraExt == ".tar" {
		ext = extraExt + ext
		base = strings.TrimSuffix(base, extraExt)
	}
	j := 0
	for ; ; j++ {
		if _, stat := os.Stat(filename); stat != nil && !os.IsExist(stat) {
			break
		}
		filename = fmt.Sprintf("%s (%d)%s", base, j+1, ext)
	}
	return filename, j != 0
}

func GenerateFilename(name, directory string) string {
	if runtime.GOOS != "windows" {
		for _, char := range []string{"/"} {
			name = strings.ReplaceAll(name, char, "")
		}
		Log.Debug("Replaced invalid characters (not windows)")
	} else {
		// ahh i love windows :)))
		for _, char := range []string{"\\", "<", ">", ":", "\"", "/", "|", "?", "*"} {
			name = strings.ReplaceAll(name, char, "")
		}
		Log.Debug("Replaced invalid characters (windows)")
	}

	filename, changed := FreeFileName(filepath.Join(directory, name))
	if changed {
		Log.Debug("File `%s` already exists, changing name to `%s`", filepath.Base(name), filepath.Base(filename))
	}

	return filename
}
