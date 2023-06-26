package spec_test

import (
	"embed"
	"fmt"
	"io"
	"os"
	"reflect"
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
	wantStatus       []interval
	wantStdout       []string
	wantStderr       []string
	wantStderrRegexp []*regexp.Regexp
}

//go:embed oil posix posix-ext
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
				if !matchAny(status, spec.wantStatus, interval.contains) {
					t.Errorf("got status %v, want any of %v", status, spec.wantStatus)
				}
			}
			if len(spec.wantStdout) > 0 {
				if !in(stdout, spec.wantStdout) {
					t.Errorf("got stdout %q", stdout)
					for i, want := range spec.wantStdout {
						t.Errorf("-want%v +got:\n%v", i, cmp.Diff(want, stdout))
					}
				}
			}
			if len(spec.wantStderr)+len(spec.wantStderrRegexp) > 0 {
				if !in(stderr, spec.wantStderr) && !matchAny(stderr, spec.wantStderrRegexp, (*regexp.Regexp).MatchString) {
					t.Errorf("got stderr %q", stderr)
					for i, want := range spec.wantStderr {
						t.Errorf("-want%v +got:\n%v", i, cmp.Diff(want, stderr))
					}
					for _, want := range spec.wantStderrRegexp {
						t.Errorf("or matching regexp %q", want.String())
					}
				}
			}

			if t.Failed() {
				t.Logf("code is:\n%v", spec.code)
				if len(spec.wantStderr) == 0 && stderr != "" {
					t.Logf("stderr is:\n%v", stderr)
				}
			}
		})
	}
}

var caseRegexp = regexp.MustCompile(`(?m)^case `)

func skipReason(s spec) string {
	if caseRegexp.MatchString(s.code) {
		return "code uses 'case'"
	}
	if strings.HasPrefix(s.suite, "oil/") && strings.Contains(s.code, "should not get here") {
		return "code tests error handling behavior"
	}
	switch s.suite {
	case "oil/comments.test.sh", "oil/quote.test.sh":
		return ""
	case "oil/arith.test.sh":
		if len(s.wantStatus) > 0 && !reflect.DeepEqual(s.wantStatus, []int{0}) {
			return "code tests error handling behavior"
		}
		if s.name == "Integer Overflow" {
			return "overflow should be OK instead of BUG"
		}
		if s.name == "Dynamic parsing on empty string" {
			return "not required by POSIX"
		}
		return ""
	default:
		if strings.HasPrefix(s.suite, "oil/") {
			return "suite is disabled"
		}
		return ""
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

func in[T comparable](x T, ys []T) bool {
	return matchAny(x, ys, func(x, y T) bool { return x == y })
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
