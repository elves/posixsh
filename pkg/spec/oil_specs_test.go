package spec_test

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

func parseOilSpecFilesInFS(fs embed.FS, dir string) []spec {
	var specs []spec
	entries, _ := fs.ReadDir(dir)
	for _, entry := range entries {
		filename := path.Join(dir, entry.Name())
		content, _ := fs.ReadFile(filename)
		specs = append(specs, parseOilSpecFile(filename, string(content))...)
	}
	return specs
}

const namePrefix = "#### "

var shellPattern = regexp.MustCompile(`^(?:BUG|OK|N-I) (?:[^ :]+ )?`)

func parseOilSpecFile(filename, content string) []spec {
	var specs []spec
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	i := 0

	warn := func(msg string) {
		fmt.Fprintf(os.Stderr, "%v:%v: %v: %v\n", filename, i+1, msg, lines[i])
	}
	readMultiLine := func() string {
		var b strings.Builder
		for i++; i < len(lines) && lines[i] != "## END"; i++ {
			b.WriteString(lines[i])
			b.WriteByte('\n')
		}
		return b.String()
	}

	// Skip empty and comment lines before the first spec
	for ; i < len(lines) && !strings.HasPrefix(lines[i], namePrefix); i++ {
		if lines[i] != "" && !strings.HasPrefix(lines[i], "#") {
			warn("non-empty, non-comment line before first spec")
		}
	}

	for i < len(lines) {
		// At the start of each iteration, lines[i] is guaranteed start with namePrefix
		name := lines[i][len(namePrefix):]
		var codeBuilder strings.Builder
		wantStatus := 0
		wantStdout := ""
		wantStderr := ""
		for i++; i < len(lines) && !strings.HasPrefix(lines[i], namePrefix); i++ {
			if metadata, ok := cutPrefix(lines[i], "## "); ok {
				annotation := shellPattern.FindString(metadata)
				metadata = metadata[len(annotation):]
				if key, value, ok := strings.Cut(metadata, ":"); ok {
					if annotation != "" {
						// Consume STDOUT and STDERR; ignore others
						if key == "STDOUT" || key == "STDERR" {
							readMultiLine()
						}
						continue
					}
					value = strings.TrimLeft(value, " ")
					switch key {
					case "code":
						codeBuilder.WriteString(value)
						codeBuilder.WriteByte('\n')
					case "status":
						i, err := strconv.Atoi(value)
						if err != nil {
							warn("can't parse status as number")
						} else {
							wantStatus = i
						}
					case "stdout":
						wantStdout = value + "\n"
					case "stdout-json":
						var s string
						err := json.Unmarshal([]byte(value), &s)
						if err != nil {
							warn("can't parse stdout-json as JSON")
						} else {
							wantStdout = s
						}
					case "stdout-repr":
						if value[0] == '\'' {
							value = "\"" + value[1:len(value)-1] + "\""
						}
						value = strings.ReplaceAll(value, `\0`, `\x00`)
						s, err := strconv.Unquote(value)
						if err != nil {
							warn("can't parse status-repr")
						} else {
							wantStdout = s
						}
					case "stderr":
						wantStderr = value + "\n"
					case "stderr-json":
						var s string
						err := json.Unmarshal([]byte(value), &s)
						if err != nil {
							warn("can't parse stderr-json as JSON")
						} else {
							wantStderr = s
						}
					case "STDOUT":
						if value != "" {
							warn("trailing content")
						}
						wantStdout = readMultiLine()
					case "STDERR":
						if value != "" {
							warn("trailing content")
						}
						wantStderr = readMultiLine()
					default:
						warn("unknown key " + key)
					}
				} else {
					warn("can't parse key from metadata")
				}
			} else {
				codeBuilder.WriteString(lines[i])
				codeBuilder.WriteByte('\n')
			}
		}
		specs = append(specs,
			spec{filename, name, codeBuilder.String(), wantStatus, wantStdout, wantStderr})
	}
	return specs
}

func cutPrefix(s, prefix string) (string, bool) {
	if strings.HasPrefix(s, prefix) {
		return s[len(prefix):], true
	}
	return "", false
}
