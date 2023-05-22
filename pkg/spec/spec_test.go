package spec_test

import (
	"embed"
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
	wantStatus []int
	wantStdout []string
	wantStderr []string
}

//go:embed oil posix
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
			if len(spec.wantStatus) != 0 && !in(status, spec.wantStatus) {
				t.Errorf("got status %v, want any of %v", status, spec.wantStatus)
			}
			testString(t, "stdout", stdout, spec.wantStdout)
			testString(t, "stderr", stderr, spec.wantStderr)
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
	if strings.Contains(s.code, "should not get here") {
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

func testString(t *testing.T, name, got string, want []string) {
	t.Helper()
	if len(want) == 0 || in(got, want) {
		return
	}
	for i, want := range want {
		t.Errorf("%v (-want%v +got):\n%v", name, i, cmp.Diff(want, got))
	}
}

func in[T comparable](x T, ys []T) bool {
	for _, y := range ys {
		if x == y {
			return true
		}
	}
	return false
}
