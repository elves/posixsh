package eval

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/elves/posixsh/pkg/arith"
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
		// According to POSIX, redirections in a function definition form apply
		// when the function is defined.
		//
		// TODO: Implement this.
		for _, word := range f.Words {
			name := fm.compound(word).expandOneWord()
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
			// POSIX specifies the following in
			// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_07:
			//
			// > For the other redirection operators [that are not "<<" or
			// > "<<-"], the word that follows the redirection operator shall be
			// > subjected to tilde expansion, parameter expansion, command
			// > substitution, arithmetic expansion, and quote removal.
			// > Pathname expansion shall not be performed on the word by a
			// > non-interactive shell; an interactive shell may perform it, but
			// > shall do so only when the expansion would result in one word.
			//
			// In other words, both field splitting and pathname expansion
			// should be suppressed when evaluating the RHS of redirection
			// operators.
			//
			// In reality, this only describes the behavior of dash and ksh.
			// Bash by default doesn't suppress either, and errors when the RHS
			// expands to multiple words. Setting POSIXLY_CORRECT causes bash to
			// suppress pathname expansion, but not field splitting.
			right := fm.compound(redir.Right).expandOneWord()
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
		fm.variables[assign.LHS] = fm.compound(assign.RHS).expandOneWord()
	}

	var words []string
	for _, cp := range f.Words {
		words = append(words, fm.glob(fm.compound(cp).expand(fm.ifs()))...)
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

func (fm *frame) compound(cp *parse.Compound) expander {
	if cp.TildePrefix != "" {
		fmt.Fprintln(fm.files[2], "tilde not supported yet")
	}
	c := compound{}
	for _, pr := range cp.Parts {
		c.elems = append(c.elems, fm.primary(pr))
	}
	return c
}

func (fm *frame) primary(pr *parse.Primary) expander {
	switch pr.Type {
	case parse.BarewordPrimary, parse.SingleQuotedPrimary:
		// Literals don't undergo word splitting. Barewords are considered
		// "quoted" for this purpose because any metacharacter has to be escaped
		// to be considered part of a bareword.
		return literal{pr.Value}
	case parse.DoubleQuotedPrimary:
		return fm.dqSegments(pr.Segments)
	case parse.ArithmeticPrimary:
		result, err := arith.Eval(fm.dqSegments(pr.Segments).expandOneWord(), fm.variables)
		if err != nil {
			fmt.Fprintln(fm.files[2], "bad arithmetic expression:", err)
			// TODO: Exit?
		}
		// Arithmetic expressions undergo word splitting.
		//
		// This seems unlikely to be useful (the result is a single number), but
		// it's specified by POSIX and implemented by dash, bash and ksh. The
		// following writes "1 1": "IFS=0; echo $(( 101 ))"
		//
		// Interestingly, zsh doesn't perform word splitting on the result of
		// arithmetic expressions even with "setopt sh_word_split".
		return scalar{strconv.FormatInt(result, 10)}
	case parse.WildcardCharPrimary:
		return globMeta{pr.Value[0]}
	case parse.OutputCapturePrimary:
		r, w, err := os.Pipe()
		if err != nil {
			fmt.Fprintln(fm.files[2], "pipe:", err)
			return nil
		}
		go func() {
			stdout := fm.files[1]
			fm.files[1] = w
			fm.chunk(pr.Body)
			fm.files[1] = stdout
			w.Close()
		}()
		output, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			fmt.Fprintln(fm.files[2], "read:", err)
		}
		// Removal of trailing newlines happens independently of and before word
		// splitting.
		return scalar{strings.TrimRight(string(output), "\n")}
	case parse.VariablePrimary:
		return fm.variable(pr.Variable)
	default:
		fmt.Fprintln(fm.files[2], "primary of type", pr.Type, "not supported yet")
		return literal{""}
	}
}

type varInfo struct {
	set       bool
	null      bool
	normal    bool
	scalar    bool
	scalarVal string
}

func (fm *frame) variable(v *parse.Variable) expander {
	name := v.Name
	// We categorize suffix operators into two classes:
	//
	//  - Substitution operators: "-", ":-", "=", ":=", "+", ":+", "?" and ":?".
	//  - Trimming operators: "%", "%%", "#" and "##".
	//
	// There is also one prefix operator "#". POSIX doesn't say explicitly
	// whether it can be combined with a prefix operator. Dash, bash and ksh all
	// error when a combination is detected; our parser does the same, so we
	// assume that they are mutually exclusive.

	// Get enough information about the variable for the substitution operators.
	var info varInfo
	if name == "*" || name == "@" {
		// $* or $@
		//
		// POSIX doesn't specify whether $* and $@ should be considered set or
		// null for the substitution operators. No two shells agree completely
		// (arg list values in JSON):
		//
		// | shell | set?   | null?                    |
		// | ----- | ------ | ------------------------ |
		// | dash  | always | [] or [""]               |
		// | bash  | not [] | [] or [""]               |
		// | ksh   | not [] | $1 null                  |
		// | zsh   | always | not []                   |
		//
		// We follow what zsh does, interpreting these two tests as tests of the
		// array. This is consistent with our handling of ${#*} and ${#@}.
		info = varInfo{
			set:  true,
			null: len(fm.arguments) <= 1,
		}
	} else if value, set, ok := fm.specialScalarVar(name); ok {
		// Special scalar, like $#
		info = scalarVarInfo(value, set, false)
	} else if i, err := strconv.Atoi(name); err == nil && i >= 0 {
		// Positional parameter, like $1. We also treat $0 as a positional
		// parameter instead of a special parameter, meaning that ${00} and the
		// like are allowed; this is unspecified in POSIX, but it's harmless to
		// support and makes the code slightly simpler.
		if i < len(fm.arguments) {
			info = scalarVarInfo(fm.arguments[i], true, false)
		} else {
			info = scalarVarInfo("", false, false)
		}
	} else {
		// Normal variable, like $foo.
		value, set := fm.variables[name]
		info = scalarVarInfo(value, set, true)
	}

	if v.LengthOp {
		var n int
		if info.scalar {
			n = len(info.scalarVal)
		} else {
			// POSIX doesn't specify the value of ${#*} or ${#@}. Both bash and
			// zsh expand them like $# (the length of the array), which we
			// follow here. Dash seems to use the length of "$*" instead.
			n = len(fm.arguments) - 1
		}
		return scalar{strconv.Itoa(n)}
	}
	if v.Modifier != nil {
		mod := v.Modifier
		var useArg, assignIfUse bool
		switch mod.Operator {
		case "-":
			useArg = !info.set
		case ":-":
			useArg = info.null
		case "=":
			useArg = !info.set
			assignIfUse = true
		case ":=":
			useArg = info.null
			assignIfUse = true
		case "+":
			useArg = info.set
		case ":+":
			useArg = !info.null
		case "?":
			if !info.set {
				argument := fm.compound(mod.Argument).expandOneWord()
				if argument == "" {
					fmt.Fprintf(fm.files[2], "%v is unset\n", v.Name)
				} else {
					fmt.Fprintf(fm.files[2], "%v is unset: %v\n", v.Name, argument)
				}
				// TODO: exit
			}
		case ":?":
			if info.null {
				argument := fm.compound(mod.Argument).expandOneWord()
				if argument == "" {
					fmt.Fprintf(fm.files[2], "%v is null or unset\n", v.Name)
				} else {
					fmt.Fprintf(fm.files[2], "%v is null or unset: %v\n", v.Name, argument)
				}
				// TODO: exit
			}
		case "#", "##":
			return fm.trimVariable(name, info.scalarVal, mod.Argument, strings.TrimPrefix)
		case "%", "%%":
			return fm.trimVariable(name, info.scalarVal, mod.Argument, strings.TrimSuffix)
		default:
			// The parser doesn't parse other modifiers.
			panic(fmt.Sprintf("bug: unknown operator %v", mod.Operator))
		}
		if useArg {
			arg := fm.compound(mod.Argument)
			if assignIfUse {
				if info.normal {
					fm.variables[v.Name] = arg.expandOneWord()
				} else {
					fmt.Fprintf(fm.files[2], "%v cannot be assigned\n", v.Name)
					// TODO: Is this a fatal error?
				}
			}
			return arg
		}
	}
	// If we reach here, expand the variable itself.
	if info.scalar {
		return scalar{info.scalarVal}
	}
	return array{fm.arguments[1:], fm.ifs, name == "@"}
}

func scalarVarInfo(value string, set, normal bool) varInfo {
	return varInfo{
		set:       set,
		null:      value == "",
		normal:    normal,
		scalar:    true,
		scalarVal: value,
	}
}

func (fm *frame) specialScalarVar(name string) (value string, set, ok bool) {
	switch name {
	case "#":
		return strconv.Itoa(len(fm.arguments) - 1), true, true
	case "?":
		// TODO: Actually return $?
		return "0", true, true
	case "-":
		// TODO
		return "", true, true
	case "$":
		return strconv.Itoa(os.Getpid()), true, true
	case "!":
		// TODO
		return "", false, true
	default:
		return "", false, false
	}
}

func (fm *frame) trimVariable(name, scalarVal string, argNode *parse.Compound, f func(string, string) string) expander {
	// TODO: Implement pattern
	pattern := fm.compound(argNode).expandOneWord()
	if name == "*" || name == "@" {
		elems := make([]string, len(fm.arguments)-1)
		for i, arg := range fm.arguments[1:] {
			elems[i] = f(arg, pattern)
		}
		return array{elems, fm.ifs, name == "@"}
	} else {
		return scalar{f(scalarVal, pattern)}
	}
}

func (fm *frame) dqSegments(segs []*parse.Segment) expander {
	var elems []expander
	for _, seg := range segs {
		switch seg.Type {
		case parse.StringSegment:
			elems = append(elems, literal{seg.Value})
		case parse.ExpansionSegment:
			elems = append(elems, fm.primary(seg.Expansion))
		default:
			fmt.Fprintln(fm.files[2], "unknown DQ segment type", seg.Type)
		}
	}
	return doubleQuoted{elems}
}

func (fm *frame) ifs() string {
	ifs, set := fm.variables["IFS"]
	if !set {
		// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_06_05
		return " \t\n"
	}
	return ifs
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
