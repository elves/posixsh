package eval

import (
	"os"
	"path/filepath"
	"strings"
)

// Like os/exec.LookPath, but
//
//   - Uses the wording directory and PATH given in the argument.
//   - Returns either [StatusCommandNotFound] or [StatusCommandNotExecutable] in
//     the second argument if the search is not successful.
//
// TODO: Windows support.
func lookPath(file, wd, paths string) (string, int) {
	if strings.Contains(file, "/") {
		if !filepath.IsAbs(file) {
			file = filepath.Join(wd, file)
		}
		status := checkExecutable(file)
		return file, status
	}
	retStatus := StatusCommandNotFound
	for _, dir := range filepath.SplitList(paths) {
		if !filepath.IsAbs(dir) {
			// Ignore any component that is not absolute for safety. This
			// behavior is slightly different from os/exec.LookPath, which will
			// proceed to check these directories but return exec.ErrDot.
			continue
		}
		fullpath := filepath.Join(dir, file)
		status := checkExecutable(fullpath)
		if status == 0 {
			return fullpath, 0
		} else if status == StatusCommandNotExecutable {
			retStatus = StatusCommandNotExecutable
		}
	}
	return "", retStatus
}

func checkExecutable(file string) int {
	info, err := os.Stat(file)
	if err == nil && !info.IsDir() {
		if info.Mode()&0o111 != 0 {
			return 0
		}
		return StatusCommandNotExecutable
	}
	return StatusCommandNotFound
}
