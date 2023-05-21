package spec_test

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func parseSpecFilesInFS(fsys embed.FS) []spec {
	var specs []spec
	fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, _ error) error {
		if !d.Type().IsDir() && strings.HasSuffix(path, ".test.sh") {
			content, _ := fsys.ReadFile(path)
			specs = append(specs, parseSpecFile(path, string(content))...)
		}
		return nil
	})
	return specs
}

var (
	shellPattern = regexp.MustCompile(`^(BUG|OK|N-I) ([^ :]+ )?`)
	dashPattern  = regexp.MustCompile(`\bdash\b`)
)

// Parses a spec file using the format from the Oil shell. Notable details:
//
//   - An additional "argv-json" metadata is supported. Used for testing
//     features related to argv without also testing the "set" builtin.
//
//   - Dash-specific non-BUG metadata are treated as acceptable alternative
//     behavior. For example, if a test case specifies both "status: 0" and "N-I
//     dash status: 2", both 0 and 2 are acceptable exit statuses.
//
//   - All other shell-specific metadata are ignored. This includes "BUG dash"
//     metadata and metadata for all other shells.
func parseSpecFile(filename, content string) []spec {
	var specs []spec
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	i := 0

	warn := func(msg string) {
		fmt.Fprintf(os.Stderr, "%v:%v: %v: %v\n", filename, i+1, msg, lines[i])
	}
	readMultiLine := func() string {
		var b strings.Builder
		for i++; i < len(lines) && !strings.HasPrefix(lines[i], "## "); i++ {
			b.WriteString(lines[i])
			b.WriteByte('\n')
		}
		// Make i point to the last line of the multi-line range.
		if i < len(lines) && !strings.HasPrefix(lines[i], "## END") {
			i--
		}
		return b.String()
	}

	for i < len(lines) {
		// Skip to the name line
		for ; i < len(lines) && !isName(lines[i]); i++ {
			if isMetadata(lines[i]) {
				warn("metadata line before spec")
			} else if !isEmptyOrComment(lines[i]) {
				warn("code line before spec")
			}
		}
		if i == len(lines) {
			break
		}
		name := lines[i][len(namePrefix):]
		var codeBuilder strings.Builder
		var argv []string
		var status []int
		var stdout, stderr []string
		skipSpec := false
		// Parse code lines
		for i++; i < len(lines) && !isName(lines[i]) && !isMetadata(lines[i]); i++ {
			codeBuilder.WriteString(lines[i])
			codeBuilder.WriteByte('\n')
		}
		// Parse metadata lines, possibly with comment lines
		for ; i < len(lines) && (isMetadata(lines[i]) || isEmptyOrComment(lines[i])); i++ {
			if isEmptyOrComment(lines[i]) {
				continue
			}
			metadata := lines[i][len(metadataPrefix):]
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
			case "argv-json":
				// NOTE: My extension; not used by Oil's spec tests.
				var ss []string
				err := json.Unmarshal([]byte(value), &ss)
				if err != nil {
					warn("can't parse argv-json as JSON")
				} else {
					argv = ss
				}
			case "status":
				i, err := strconv.Atoi(value)
				if err != nil {
					warn("can't parse status as number")
				} else {
					status = append(status, i)
				}
			case "stdout":
				stdout = append(stdout, value+"\n")
			case "stdout-json":
				var s string
				err := json.Unmarshal([]byte(value), &s)
				if err != nil {
					warn("can't parse stdout-json as JSON")
				} else {
					stdout = append(stdout, s)
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
					stdout = append(stdout, s)
				}
			case "stderr":
				stderr = append(stderr, value+"\n")
			case "stderr-json":
				var s string
				err := json.Unmarshal([]byte(value), &s)
				if err != nil {
					warn("can't parse stderr-json as JSON")
				} else {
					stderr = append(stderr, s)
				}
			case "STDOUT":
				if value != "" {
					warn("trailing content")
				}
				stdout = append(stdout, readMultiLine())
			case "STDERR":
				if value != "" {
					warn("trailing content")
				}
				stderr = append(stderr, readMultiLine())
			default:
				warn("unknown key " + key)
			}
		}
		if skipSpec {
			continue
		}
		if len(status) == 0 {
			status = []int{0}
		}
		specs = append(specs, spec{
			filename, name, codeBuilder.String(), argv, status, stdout, stderr})
	}
	return specs
}

const (
	namePrefix     = "#### "
	metadataPrefix = "## "
)

func isName(line string) bool     { return strings.HasPrefix(line, namePrefix) }
func isMetadata(line string) bool { return strings.HasPrefix(line, metadataPrefix) }

func isEmptyOrComment(line string) bool {
	line = strings.TrimSpace(line)
	return line == "" || (strings.HasPrefix(line, "#") && !isMetadata(line) && !isName(line))
}
