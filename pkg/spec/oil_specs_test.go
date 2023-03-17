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

var (
	shellPattern = regexp.MustCompile(`^(BUG|OK|N-I) +([^ :]+ +)?`)
	dashPattern  = regexp.MustCompile(`\bdash\b`)
)

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
		checkStdout := false
		wantStdout := ""
		checkStderr := false
		wantStderr := ""
		skipSpec := false
		for i++; i < len(lines) && !strings.HasPrefix(lines[i], namePrefix); i++ {
			metadata, ok := cutPrefix(lines[i], "## ")
			if !ok {
				codeBuilder.WriteString(lines[i])
				codeBuilder.WriteByte('\n')
				continue
			}

			annotation := shellPattern.FindStringSubmatch(metadata)
			if annotation != nil {
				metadata = metadata[len(annotation[0]):]
			}
			key, value, ok := strings.Cut(metadata, ":")
			if !ok {
				warn("can't parse key from metadata")
				continue
			}

			if annotation != nil {
				dash := dashPattern.FindString(annotation[2]) != ""
				if annotation[1] == "N-I" && dash {
					skipSpec = true
				} else if annotation[1] == "BUG" || !dash {
					// Ignore if the annotation is about a non-dash shell or a
					// dash bug. Consume STDOUT and STDERR.
					if key == "STDOUT" || key == "STDERR" {
						readMultiLine()
					}
					continue
				}
				// If we have reached here, dash has a different behavior and it
				// is not a bug. Proceed to use dash's behavior as an override.
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
				checkStdout = true
				wantStdout = value + "\n"
			case "stdout-json":
				var s string
				err := json.Unmarshal([]byte(value), &s)
				if err != nil {
					warn("can't parse stdout-json as JSON")
				} else {
					checkStdout = true
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
					checkStdout = true
					wantStdout = s
				}
			case "stderr":
				checkStderr = true
				wantStderr = value + "\n"
			case "stderr-json":
				var s string
				err := json.Unmarshal([]byte(value), &s)
				if err != nil {
					warn("can't parse stderr-json as JSON")
				} else {
					checkStderr = true
					wantStderr = s
				}
			case "STDOUT":
				if value != "" {
					warn("trailing content")
				}
				checkStdout = true
				wantStdout = readMultiLine()
			case "STDERR":
				if value != "" {
					warn("trailing content")
				}
				checkStderr = true
				wantStderr = readMultiLine()
			default:
				warn("unknown key " + key)
			}
		}
		if skipSpec {
			continue
		}
		specs = append(specs, spec{
			filename, name, codeBuilder.String(),
			wantStatus, checkStdout, wantStdout, checkStderr, wantStderr})
	}
	return specs
}

func cutPrefix(s, prefix string) (string, bool) {
	if strings.HasPrefix(s, prefix) {
		return s[len(prefix):], true
	}
	return "", false
}
