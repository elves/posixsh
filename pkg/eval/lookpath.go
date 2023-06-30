package eval

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Like os/exec.LookPath, but
//
//   - Uses the working directory and PATH given in the argument.
//
//   - Also returns whether any non-directory file is found.
//
// TODO: Windows support.
func lookPath(file, wd, paths string, perm fs.FileMode) (path string, ok, existsAny bool) {
	if strings.Contains(file, "/") {
		if !filepath.IsAbs(file) {
			file = filepath.Join(wd, file)
		}
		ok, exists := checkPerm(file, perm)
		return file, ok, exists
	}
	for _, dir := range filepath.SplitList(paths) {
		if !filepath.IsAbs(dir) {
			// Ignore any component that is not absolute for safety. This
			// behavior is slightly different from os/exec.LookPath, which will
			// proceed to check these directories but return exec.ErrDot.
			continue
		}
		fullpath := filepath.Join(dir, file)
		ok, exists := checkPerm(fullpath, perm)
		if ok {
			return fullpath, true, true
		} else if exists {
			existsAny = true
		}
	}
	return "", false, existsAny
}

func checkPerm(file string, perm fs.FileMode) (ok, exists bool) {
	info, err := os.Stat(file)
	if err == nil && !info.IsDir() {
		return true, info.Mode()&perm != 0
	}
	return false, false
}
