package eval

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/elves/posixsh/pkg/parse"
)

type Evaler struct {
	arguments []string
	variables map[string]string
	functions map[string]*parse.CompoundCommand
}

func NewEvaler() *Evaler {
	return &Evaler{
		nil,
		make(map[string]string),
		make(map[string]*parse.CompoundCommand),
	}
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
	return &frame{
		ev.arguments, ev.variables, ev.functions,
		[]*os.File{os.Stdin, os.Stdout, os.Stderr},
	}
}

type frame struct {
	arguments []string
	variables map[string]string
	functions map[string]*parse.CompoundCommand
	files     []*os.File
}

func (fm *frame) cloneForRedir() *frame {
	return &frame{
		fm.arguments, fm.variables, fm.functions,
		append([]*os.File{}, fm.files...),
	}
}

func (fm *frame) cloneForSubshell() *frame {
	newFm := &frame{
		append([]string{}, fm.arguments...),
		make(map[string]string), make(map[string]*parse.CompoundCommand),
		append([]*os.File{}, fm.files...),
	}
	// TODO: Optimize
	for k, v := range fm.variables {
		newFm.variables[k] = v
	}
	for k, v := range fm.functions {
		newFm.functions[k] = v
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

func (fm *frame) compoundCommand(cc *parse.CompoundCommand) bool {
	if cc.Subshell {
		fm = fm.cloneForSubshell()
	}
	return fm.chunk(cc.Body)
}

func (fm *frame) form(f *parse.Form) bool {
	switch f.Type {
	case parse.CompoundCommandForm:
		return fm.compoundCommand(f.Body)
	case parse.FnDefinitionForm:
		// TODO What to do with redirs?
		for _, word := range f.Words {
			name := fm.compound(word)
			fm.functions[name] = f.Body
		}
		return true
	}
	if len(f.Redirs) > 0 {
		for _, redir := range f.Redirs {
			if redir.RightFd {
				fmt.Println(">& not supported yet")
				continue
			}
			var flag, defaultDst int
			switch redir.Mode {
			case parse.RedirInput, parse.RedirHeredoc:
				flag = os.O_RDONLY
				defaultDst = 0
			case parse.RedirOutput:
				flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
				defaultDst = 1
			case parse.RedirAppend:
				flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
				defaultDst = 1
			default:
				fmt.Println("unsupported redir", redir.Mode)
				continue
			}
			var src *os.File
			if redir.Mode == parse.RedirHeredoc {
				r, w, err := os.Pipe()
				if err != nil {
					fmt.Println(err)
					continue
				}
				content := redir.Heredoc.Value
				go func() {
					n, err := w.WriteString(content)
					if n < len(content) {
						fmt.Println("short write", n, "<", len(content))
					}
					if err != nil {
						fmt.Println(err)
					}
					w.Close()
				}()
				src = r
			} else {
				right := fm.compound(redir.Right)
				f, err := os.OpenFile(right, flag, 0644)
				if err != nil {
					continue
				}
				src = f
			}
			dst := redir.Left
			if dst == -1 {
				dst = defaultDst
			}
			if dst >= len(fm.files) {
				newFiles := make([]*os.File, dst+1)
				copy(newFiles, fm.files)
				fm.files = newFiles
			}
			fm.files[dst] = src
			defer src.Close()
		}
	}

	// TODO: Temp assignment
	for _, assign := range f.Assigns {
		fm.variables[assign.LHS] = fm.compound(assign.RHS)
	}

	var words []string
	for _, cp := range f.Words {
		words = append(words, fm.compound(cp))
	}
	if len(words) == 0 {
		return true
	}

	// Functions?
	if fn, ok := fm.functions[words[0]]; ok {
		oldArgs := fm.arguments
		fm.arguments = words
		ret := fm.compoundCommand(fn)
		fm.arguments = oldArgs
		return ret
	}

	// Builtins?
	if builtin, ok := builtins[words[0]]; ok {
		return builtin(fm, words[1:]) == 0
	}

	// External commands?
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
	case parse.DoubleQuotedPrimary:
		var buf bytes.Buffer
		for _, seg := range pr.DQSegments {
			switch seg.Type {
			case parse.DQStringSegment:
				buf.WriteString(seg.Value)
			case parse.DQExpansionSegment:
				buf.WriteString(fm.primary(seg.Expansion))
			default:
				fmt.Println("unknown DQ segment type", seg.Type)
			}
		}
		return buf.String()
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
	case parse.VariablePrimary:
		v := pr.Variable
		value := fm.getVar(v.Name)
		if v.Modifier != nil {
			mod := v.Modifier
			// TODO Implement operators
			switch mod.Operator {
			default:
				fmt.Println("unknown operator", mod.Operator)
			}
			if mod.Operator[len(mod.Operator)-1] == '=' {
				fm.variables[v.Name] = value
			}
		}
		if v.LengthOp {
			value = strconv.Itoa(len(value))
		}
		return value
	default:
		fmt.Println("primary of type", pr.Type, "not supported yet")
		return ""
	}
}

func (fm *frame) getVar(name string) string {
	switch name {
	case "*", "@":
		return strings.Join(fm.arguments[1:], " ")
	default:
		if i, err := strconv.Atoi(name); err == nil && i >= 0 {
			if i < len(fm.arguments) {
				return fm.arguments[i]
			}
			return ""
		}
		return fm.variables[name]
	}
}
