package spec_test

import (
	"embed"
	"io"
	"os"
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

	wantStatus int
	wantStdout string
	wantStderr string
}

//go:embed oil/*.test.sh
var oilFiles embed.FS

var specs = parseOilSpecFilesInFS(oilFiles, "oil")

func TestSpecs(t *testing.T) {
	for _, spec := range specs {
		if !shouldRunSuite(spec.suite) {
			continue
		}
		t.Run(spec.suite+"/"+spec.name, func(t *testing.T) {
			testutil.InTempDir(t)
			files, read := makeFiles()
			ev := eval.NewEvaler(files)
			status := ev.Eval(spec.code)
			stdout, stderr := read()
			if status != spec.wantStatus {
				t.Errorf("got status %v, want %v", status, spec.wantStatus)
			}
			if diff := cmp.Diff(spec.wantStdout, stdout); diff != "" {
				t.Errorf("stdout (-want+got):\n%v", diff)
			}
			if diff := cmp.Diff(spec.wantStderr, stderr); diff != "" {
				t.Errorf("stderr (-want+got):\n%v", diff)
			}
			if t.Failed() {
				t.Logf("code is:\n%v", spec.code)
			}
		})
	}
}

func shouldRunSuite(name string) bool {
	switch name {
	case "oil/quote.test.sh":
		return true
	default:
		return false
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
