package spec_test

import (
	"embed"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/elves/posixsh/pkg/eval"
	"github.com/google/go-cmp/cmp"
	"src.elv.sh/pkg/must"
	"src.elv.sh/pkg/testutil"
)

type spec struct {
	suite string
	name  string
	code  string
	argv  []string

	// No elements = don't check.
	// Multiple elements = any one is OK.
	wantStatus []interval
	wantStdout []regexpOrString
	wantStderr []regexpOrString
}

type regexpOrString struct {
	re *regexp.Regexp
	s  string
}

func (rs regexpOrString) match(s string) bool {
	if rs.re != nil {
		return rs.re.MatchString(s)
	}
	return rs.s == s
}

//go:embed impl oil posix posix-ext
var specFiles embed.FS

var specs = parseSpecFilesInFS(specFiles)

func TestSpecs(t *testing.T) {
	for _, spec := range specs {
		t.Run(spec.suite+"/"+spec.name, func(t *testing.T) {
			if reason := skipReason(spec); reason != "" {
				t.Skip(reason)
			}
			testutil.InTempDir(t)
			files, read := makeFiles()
			argv := spec.argv
			if len(argv) == 0 {
				argv = []string{"/bin/sh"}
			}
			ev := eval.NewEvaler(argv, files)
			status := ev.Eval(spec.code)
			stdout, stderr := read()

			if len(spec.wantStatus) > 0 {
				if strings.HasPrefix(spec.suite, "oil/") {
					// Relax status constraint for tests in oil/: as long as we
					// are testing a non-zero status, admit any non-zero status.
					relaxStatus(&spec)
				}
				if !matchAny(status, spec.wantStatus, interval.contains) {
					t.Errorf("got status %v, want any of %v", status, spec.wantStatus)
				}
			}
			testOutput(t, "stdout", stdout, spec.wantStdout)
			testOutput(t, "stderr", stderr, spec.wantStderr)

			if t.Failed() {
				t.Logf("code is:\n%v", spec.code)
				if len(spec.wantStderr) == 0 && stderr != "" {
					t.Logf("stderr is:\n%v", stderr)
				}
			}
		})
	}
}

func skipReason(s spec) string {
	if !strings.HasPrefix(s.suite, "oil/") {
		return ""
	}
	if strings.Contains(s.code, "typeset") {
		return "code uses typeset"
	}
	if strings.Contains(s.code, "$?") {
		return "code tests exact value of $?"
	}
	switch s.suite {
	case "oil/comments.test.sh", "oil/quote.test.sh":
		return ""
	case "oil/arith.test.sh":
		if s.name == "Integer Overflow" {
			return "overflow should be OK instead of BUG"
		}
		if s.name == "Dynamic parsing on empty string" {
			return "not required by POSIX"
		}
		return ""
	default:
		return "suite is disabled"
	}
}

var devNull = must.OK1(os.Open(os.DevNull))

func makeFiles() ([]*os.File, func() (string, string)) {
	file1, read1 := outputPipe()
	file2, read2 := outputPipe()
	return []*os.File{devNull, file1, file2}, func() (string, string) {
		return read1(), read2()
	}
}

func outputPipe() (*os.File, func() string) {
	r, w := must.Pipe()
	ch := make(chan string)
	go func() {
		ch <- string(must.OK1(io.ReadAll(r)))
		r.Close()
	}()
	return w, func() string {
		w.Close()
		return <-ch
	}
}

func relaxStatus(s *spec) {
	hasNonZero := false
	for _, status := range s.wantStatus {
		if status != (interval{0, 0}) {
			hasNonZero = true
		}
	}
	if hasNonZero {
		s.wantStatus = append(s.wantStatus, interval{1, 127})
	}
}

func testOutput(t *testing.T, what, got string, wants []regexpOrString) {
	t.Helper()
	if len(wants) == 0 {
		return
	}
	if !matchAny(got, wants, regexpOrString.match) {
		t.Errorf("%v doesn't match any of %v choices:\n%s", what, len(wants), got)
		for i, want := range wants {
			if want.re != nil {
				t.Errorf("#%d: string matching regexp %q", i, want.re.String())
			} else {
				t.Errorf("#%d: -want +got:\n%v", i, cmp.Diff(want.s, got))
			}
		}
	}
}

func matchAny[V, M any](value V, matchers []M, match func(M, V) bool) bool {
	for _, matcher := range matchers {
		if match(matcher, value) {
			return true
		}
	}
	return false
}

type interval [2]int

func (i interval) String() string      { return fmt.Sprintf("%v..%v", i[0], i[1]) }
func (i interval) contains(j int) bool { return i[0] <= j && j <= i[1] }
