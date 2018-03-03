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

func (ev *Evaler) Eval(code string) bool {
	n := &parse.Chunk{}
	rest, err := parse.Parse(code, n)
	if rest != "" {
		fmt.Printf("trailing text: %q\n", rest)
		return false
	}
	if err != nil {
		fmt.Println("parse error", err)
		return false
	}

	return ev.EvalChunk(n)
}

func (ev *Evaler) EvalChunk(n *parse.Chunk) bool {
	return ev.frame().chunk(n)
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

func (fm *frame) chunk(ch *parse.Chunk) bool {
	var ret bool
	for _, ao := range ch.AndOrs {
		ret = fm.andOr(ao)
	}
	return ret
}

func (fm *frame) andOr(ao *parse.AndOr) bool {
	var ret bool
	for i, pp := range ao.Pipelines {
		if i > 0 && ao.AndOp[i-1] != ret {
			continue
		}
		ret = fm.pipeline(pp)
	}
	return ret
}

func (fm *frame) pipeline(ch *parse.Pipeline) bool {
	if len(ch.Forms) == 1 {
		// Short path
		return fm.cloneForRedir().form(ch.Forms[0])
	}
	var wg sync.WaitGroup
	wg.Add(len(ch.Forms))

	var nextIn *os.File
	var ppRet bool
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
		saveRet := i == len(ch.Forms)-1
		go func() {
			ret := newFm.form(theForm)
			if saveRet {
				ppRet = ret
			}
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
	return ppRet
}

func (fm *frame) form(f *parse.Form) bool {
	if f.FnBody != nil {
		fmt.Println("function definition not supported yet")
	}
	if len(f.Redirs) > 0 {
		for _, redir := range f.Redirs {
			right := fm.compound(redir.Right)
			if redir.RightFd {
				fmt.Println(">& not supported yet")
				continue
			}
			var flag, defaultDst int
			switch redir.Mode {
			case parse.RedirInput:
				flag = os.O_RDONLY
				defaultDst = 0
			case parse.RedirOutput:
				flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
				defaultDst = 1
			case parse.RedirAppend:
				flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
				defaultDst = 1
			default:
				fmt.Println("unsupported redir", redir.Mode)
				continue
			}
			f, err := os.OpenFile(right, flag, 0644)
			if err != nil {
				fmt.Println(err)
				continue
			}
			dst := redir.Left
			if dst == -1 {
				dst = defaultDst
			}
			fm.files[dst] = f
			defer f.Close()
		}
	}
	var words []string
	for _, cp := range f.Words {
		words = append(words, fm.compound(cp))
	}
	if len(words) == 0 {
		return true
	}
	path, err := exec.LookPath(words[0])
	if err != nil {
		fmt.Println("search:", err)
		return false
	}
	words[0] = path
	proc, err := os.StartProcess(path, words, &os.ProcAttr{
		Files: fm.files,
	})
	if err != nil {
		fmt.Println(err)
		return false
	}
	state, err := proc.Wait()
	if err != nil {
		fmt.Println(err)
		return false
	}
	return state.Success()
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
