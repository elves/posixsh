package eval

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/xiaq/posixsh/parse"
)

type Evaler struct {
	globals map[string]string
}

func NewEvaler() *Evaler {
	return &Evaler{make(map[string]string)}
}

func (ev *Evaler) Eval(code string) error {
	n := &parse.Chunk{}
	rest, err := parse.Parse(code, n)
	if rest != "" {
		return fmt.Errorf("trailing text: %q", rest)
	}
	if err != nil {
		return err
	}

	return ev.EvalChunk(n)
}

func (ev *Evaler) EvalChunk(n *parse.Chunk) error {
	ev.frame().chunk(n)
	return nil
}

func (ev *Evaler) frame() *frame {
	return &frame{ev.globals, []*os.File{os.Stdin, os.Stdout, os.Stderr}}
}

type frame struct {
	globals map[string]string
	files   []*os.File
}

func (fm *frame) cloneForRedir() *frame {
	newFm := *fm
	newFm.files = append([]*os.File{}, fm.files...)
	return &newFm
}

func (fm *frame) cloneForSubshell() *frame {
	newFm := &frame{
		make(map[string]string), append([]*os.File{}, fm.files...),
	}
	// TODO: Optimize
	for k, v := range fm.globals {
		newFm.globals[k] = v
	}
	return newFm
}

func (fm *frame) chunk(ch *parse.Chunk) {
	for _, pp := range ch.Pipelines {
		fm.pipeline(pp)
	}
}

func (fm *frame) pipeline(ch *parse.Pipeline) {
	if len(ch.Forms) == 1 {
		// Short path
		fm.form(ch.Forms[0])
		return
	}
	var wg sync.WaitGroup
	wg.Add(len(ch.Forms))

	var nextIn *os.File
	for i, f := range ch.Forms {
		var newFm *frame
		var close0, close1 bool
		if i < len(ch.Forms)-1 {
			newFm = fm.cloneForSubshell()
			r, w, err := os.Pipe()
			if err != nil {
				fmt.Println("pipe:", err)
			}
			newFm.files[1] = w
			close1 = true
			nextIn = r
		} else {
			newFm = fm.cloneForRedir()
		}
		if i > 0 {
			newFm.files[0] = nextIn
			close0 = true
		}
		theForm := f
		go func() {
			newFm.form(theForm)
			if close0 {
				newFm.files[0].Close()
			}
			if close1 {
				newFm.files[1].Close()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func (fm *frame) form(f *parse.Form) {
	if f.FnBody != nil {
		fmt.Println("function definition not supported yet")
	}
	if len(f.Redirs) > 0 {
		fmt.Println("redirs not supported yet")
	}
	var words []string
	for _, cp := range f.Words {
		words = append(words, fm.compound(cp))
	}
	if len(words) == 0 {
		return
	}
	path, err := exec.LookPath(words[0])
	if err != nil {
		fmt.Println("search:", err)
		return
	}
	words[0] = path
	proc, err := os.StartProcess(path, words, &os.ProcAttr{
		Files: fm.files,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	state, err := proc.Wait()
	if err != nil {
		fmt.Println(err)
		return
	}
	// fmt.Println("state:", state)
	_ = state
}

func (fm *frame) compound(cp *parse.Compound) string {
	if cp.TildePrefix != "" {
		fmt.Println("tilde not supported yet")
	}
	var buf bytes.Buffer
	for _, pr := range cp.Parts {
		buf.WriteString(fm.primary(pr))
	}
	return buf.String()
}

func (fm *frame) primary(pr *parse.Primary) string {
	switch pr.Type {
	case parse.BarewordPrimary, parse.SingleQuotedPrimary:
		return pr.Value
	case parse.OutputCapturePrimary:
		r, w, err := os.Pipe()
		if err != nil {
			fmt.Println("pipe:", err)
			return ""
		}
		go func() {
			newFm := fm.cloneForRedir()
			newFm.files[1] = w
			newFm.chunk(pr.Body)
			w.Close()
		}()
		// TODO: Split by $IFS
		output, err := ioutil.ReadAll(r)
		r.Close()
		if err != nil {
			fmt.Println("read:", err)
		}
		return strings.TrimSuffix(string(output), "\n")
	default:
		fmt.Println("primary of type", pr.Type, "not supported yet")
		return ""
	}
}
