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
	"syscall"

	"github.com/elves/posixsh/pkg/parse"
)

type Evaler struct {
	arguments []string
	variables map[string]string
	functions map[string]*parse.CompoundCommand
	files     []*os.File
}

var StdFiles = []*os.File{os.Stdin, os.Stdout, os.Stderr}

func NewEvaler(args []string, files []*os.File) *Evaler {
	if len(args) < 1 {
		panic("args must have at least 1 element")
	}
	if len(files) < 3 {
		panic("files must have at least 3 elements")
	}
	return &Evaler{
		args,
		make(map[string]string),
		make(map[string]*parse.CompoundCommand),
		files,
	}
}

func (ev *Evaler) Eval(code string) int {
	n, err := parse.Parse(code)
	if err != nil {
		fmt.Fprintln(ev.files[2], "parse error", err)
		return StatusSyntaxError
	}

	return ev.EvalChunk(n)
}

func (ev *Evaler) EvalChunk(n *parse.Chunk) int {
	return ev.frame().chunk(n)
}

func (ev *Evaler) frame() *frame {
	return &frame{ev.arguments, ev.variables, ev.functions, ev.files}
}

type frame struct {
	arguments []string
	variables map[string]string
	functions map[string]*parse.CompoundCommand
	files     []*os.File
}

func (fm *frame) cloneForSubshell() *frame {
	// TODO: Optimize with copy on write
	return &frame{
		cloneSlice(fm.arguments),
		cloneMap(fm.variables), cloneMap(fm.functions),
		cloneSlice(fm.files),
	}
}

func (fm *frame) chunk(ch *parse.Chunk) int {
	var ret int
	for _, ao := range ch.AndOrs {
		ret = fm.andOr(ao)
	}
	return ret
}

func (fm *frame) andOr(ao *parse.AndOr) int {
	var ret int
	for i, pp := range ao.Pipelines {
		if i > 0 {
			and := ao.AndOp[i-1]
			if (and && ret != 0) || (!and && ret == 0) {
				continue
			}
		}
		ret = fm.pipeline(pp)
	}
	return ret
}

func (fm *frame) pipeline(ch *parse.Pipeline) int {
	n := len(ch.Forms)
	if n == 1 {
		// Short path
		f := ch.Forms[0]
		if len(f.Redirs) > 0 {
			files := cloneSlice(fm.files)
			defer func() { fm.files = files }()
		}
		return fm.form(f)
	}

	pipes := make([][2]*os.File, n-1)
	for i := 0; i < n-1; i++ {
		r, w, err := os.Pipe()
		if err != nil {
			fmt.Fprintln(fm.files[2], "pipe:", err)
			for j := 0; j < i; j++ {
				pipes[j][0].Close()
				pipes[j][1].Close()
			}
			return StatusPipeError
		}
		pipes[i][0], pipes[i][1] = r, w
	}

	var wg sync.WaitGroup
	wg.Add(n)

	var ppRet int
	for i, f := range ch.Forms {
		var newFm *frame
		if i < n-1 {
			newFm = fm.cloneForSubshell()
			newFm.files[1] = pipes[i][1]
		} else {
			files := cloneSlice(fm.files)
			defer func() { fm.files = files }()
			newFm = fm
		}
		if i > 0 {
			newFm.files[0] = pipes[i-1][0]
		}
		go func(i int, f *parse.Form) {
			ret := newFm.form(f)
			if i == n-1 {
				ppRet = ret
			}
			if i > 0 {
				newFm.files[0].Close()
			}
			if i < n-1 {
				newFm.files[1].Close()
			}
			wg.Done()
		}(i, f)
	}
	wg.Wait()
	return ppRet
}

func (fm *frame) compoundCommand(cc *parse.CompoundCommand) int {
	if cc.Subshell {
		fm = fm.cloneForSubshell()
	}
	return fm.chunk(cc.Body)
}

func (fm *frame) form(f *parse.Form) int {
	switch f.Type {
	case parse.CompoundCommandForm:
		return fm.compoundCommand(f.Body)
	case parse.FnDefinitionForm:
		// TODO What to do with redirs?
		for _, word := range f.Words {
			name := fm.compound(word)
			fm.functions[name] = f.Body
		}
		return 0
	}
	for _, redir := range f.Redirs {
		if redir.RightFd {
			fmt.Fprintln(fm.files[2], ">& not supported yet")
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
			fmt.Fprintln(fm.files[2], "unsupported redir", redir.Mode)
			continue
		}
		var src *os.File
		if redir.Mode == parse.RedirHeredoc {
			r, w, err := os.Pipe()
			if err != nil {
				fmt.Fprintln(fm.files[2], "pipe:", err)
				continue
			}
			content := redir.Heredoc.Value
			go func() {
				n, err := w.WriteString(content)
				if n < len(content) {
					fmt.Fprintln(fm.files[2], "short write", n, "<", len(content))
				}
				if err != nil {
					fmt.Fprintln(fm.files[2], err)
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

	// TODO: Temp assignment
	for _, assign := range f.Assigns {
		fm.variables[assign.LHS] = fm.compound(assign.RHS)
	}

	var words []string
	for _, cp := range f.Words {
		words = append(words, fm.compound(cp))
	}
	if len(words) == 0 {
		return 0
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
		return builtin(fm, words[1:])
	}

	// External commands?
	path, err := exec.LookPath(words[0])
	if err != nil {
		fmt.Fprintln(fm.files[2], "search:", err)
		// TODO: Return StatusCommandNotExecutable if file exists but is not
		// executable.
		return StatusCommandNotFound
	}
	words[0] = path

	proc, err := os.StartProcess(path, words, &os.ProcAttr{
		Files: fm.files,
	})
	if err != nil {
		fmt.Fprintln(fm.files[2], err)
		return StatusCommandNotExecutable
	}

	state, err := proc.Wait()
	if err != nil {
		fmt.Fprintln(fm.files[2], err)
		return StatusWaitError
	}
	if state.Exited() {
		return state.ExitCode()
	} else {
		waitStatus := state.Sys().(syscall.WaitStatus)
		if waitStatus.Signaled() {
			return StatusSignalBase + int(waitStatus.Signal())
		}
		return StatusWaitOther
	}
}

func (fm *frame) compound(cp *parse.Compound) string {
	if cp.TildePrefix != "" {
		fmt.Fprintln(fm.files[2], "tilde not supported yet")
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
				fmt.Fprintln(fm.files[2], "unknown DQ segment type", seg.Type)
			}
		}
		return buf.String()
	case parse.OutputCapturePrimary:
		r, w, err := os.Pipe()
		if err != nil {
			fmt.Fprintln(fm.files[2], "pipe:", err)
			return ""
		}
		go func() {
			stdout := fm.files[1]
			fm.files[1] = w
			fm.chunk(pr.Body)
			fm.files[1] = stdout
			w.Close()
		}()
		// TODO: Split by $IFS
		output, err := ioutil.ReadAll(r)
		r.Close()
		if err != nil {
			fmt.Fprintln(fm.files[2], "read:", err)
		}
		return strings.TrimSuffix(string(output), "\n")
	case parse.VariablePrimary:
		v := pr.Variable
		value, set := fm.getVar(v.Name)
		if v.Modifier != nil {
			mod := v.Modifier
			argument := fm.compound(mod.Argument)
			assign := false
			switch mod.Operator {
			case "-":
				if !set {
					value = argument
				}
			case "=":
				if !set {
					value = argument
					assign = true
				}
			case ":-":
				if value == "" {
					value = argument
				}
			case ":=":
				if value == "" {
					value = argument
					assign = true
				}
			case "?":
				if !set {
					if argument == "" {
						fmt.Fprintf(fm.files[2], "%v is unset\n", v.Name)
					} else {
						fmt.Fprintf(fm.files[2], "%v is unset: %v\n", v.Name, argument)
					}
					// TODO: exit
				}
			case ":?":
				if value == "" {
					if argument == "" {
						fmt.Fprintf(fm.files[2], "%v is null or unset\n", v.Name)
					} else {
						fmt.Fprintf(fm.files[2], "%v is null or unset: %v\n", v.Name, argument)
					}
					// TODO: exit
				}
			case "+":
				if set {
					value = argument
				}
			case ":+":
				if value != "" {
					value = argument
				}
			case "#", "##":
				// TODO: Implement pattern
				value = strings.TrimPrefix(value, argument)
			case "%", "%%":
				// TODO: Implement pattern
				value = strings.TrimSuffix(value, argument)
			default:
				// The parser doesn't parse other modifiers.
				panic(fmt.Sprintf("bug: unknown operator %v", mod.Operator))
			}
			if assign {
				fm.variables[v.Name] = value
			}
		}
		if v.LengthOp {
			value = strconv.Itoa(len(value))
		}
		return value
	default:
		fmt.Fprintln(fm.files[2], "primary of type", pr.Type, "not supported yet")
		return ""
	}
}

func (fm *frame) getVar(name string) (string, bool) {
	switch name {
	case "*", "@":
		// TODO
		return strings.Join(fm.arguments[1:], " "), true
	case "#":
		return strconv.Itoa(len(fm.arguments) - 1), true
	case "?":
		// TODO: Actually return $?
		return "0", true
	case "-":
		// TODO
		return "", true
	case "$":
		return strconv.Itoa(os.Getpid()), true
	case "!":
		// TODO
		return "", false
	default:
		if i, err := strconv.Atoi(name); err == nil && i >= 0 {
			if i < len(fm.arguments) {
				return fm.arguments[i], true
			}
			return "", false
		}
		value, ok := fm.variables[name]
		return value, ok
	}
}

func cloneSlice[T any](s []T) []T {
	return append([]T(nil), s...)
}

func cloneMap[K comparable, V any](m map[K]V) map[K]V {
	mm := make(map[K]V, len(m))
	for k, v := range m {
		mm[k] = v
	}
	return mm
}
