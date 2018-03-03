package eval

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/xiaq/posixsh/parse"
)

type Evaler struct {
	Variables map[string]string
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
	ev.chunk(n)
	return nil
}

func (ev *Evaler) chunk(ch *parse.Chunk) {
	for _, pp := range ch.Pipelines {
		ev.pipeline(pp)
	}
}

func (ev *Evaler) pipeline(ch *parse.Pipeline) {
	if len(ch.Forms) != 1 {
		fmt.Println("pipeline not supported yet")
	} else {
		ev.form(ch.Forms[0])
	}
}

func (ev *Evaler) form(fm *parse.Form) {
	if fm.FnBody != nil {
		fmt.Println("function definition not supported yet")
	}
	if len(fm.Redirs) > 0 {
		fmt.Println("redirs not supported yet")
	}
	var words []string
	for _, cp := range fm.Words {
		words = append(words, ev.compound(cp))
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
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
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

func (ev *Evaler) compound(cp *parse.Compound) string {
	if cp.TildePrefix != "" {
		fmt.Println("tilde not supported yet")
	}
	var buf bytes.Buffer
	for _, pr := range cp.Parts {
		buf.WriteString(ev.primary(pr))
	}
	return buf.String()
}

func (ev *Evaler) primary(pr *parse.Primary) string {
	switch pr.Type {
	case parse.BarewordPrimary, parse.SingleQuotedPrimary:
		return pr.Value
	default:
		fmt.Println("primary of type", pr.Type, "not supported yet")
		return ""
	}
}
