package eval

import (
	"fmt"

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

	return ev.chunk(n)
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
}

func (ev *Evaler) compound(cp *parse.Compound) {
}

func (ev *Evaler) primary(pr *parse.Primary) {
}
