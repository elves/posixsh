package eval

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/xiaq/posixsh/parse"
)

type Evaler struct {
	scope map[string]string
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
	return &frame{ev, []*os.File{os.Stdin, os.Stdout, os.Stderr}}
}

type frame struct {
	ev    *Evaler
	files []*os.File
}

func (fm *frame) clone() *frame {
	fm2 := *fm
	fm2.files = append([]*os.File{}, fm.files...)
	return &fm2
}

func (fm *frame) chunk(ch *parse.Chunk) {
	for _, pp := range ch.Pipelines {
		fm.pipeline(pp)
	}
}

func (fm *frame) pipeline(ch *parse.Pipeline) {
	if len(ch.Forms) != 1 {
		fmt.Println("pipeline not supported yet")
	} else {
		fm.form(ch.Forms[0])
	}
}

func (fm *frame) form(fr *parse.Form) {
	if fr.FnBody != nil {
		fmt.Println("function definition not supported yet")
	}
	if len(fr.Redirs) > 0 {
		fmt.Println("redirs not supported yet")
	}
	var words []string
	for _, cp := range fr.Words {
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
			newFm := fm.clone()
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
