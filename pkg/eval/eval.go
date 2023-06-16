package eval

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
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
	// POSIX requires all cases except "special built-in utility error" and
	// "other utility (not a special builtin-in error)" to print a shell
	// diagnostic message to the stderr, ignoring all active redirections. We
	// save the initial stderr (files[2]) in this field for that purpose.
	diagFile *os.File
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
		files[2],
	}
}

func (ev *Evaler) Eval(code string) int {
	n, err := parse.Parse(code)
	if err != nil {
		fmt.Fprintln(ev.diagFile, "syntax error:", err)
		return StatusSyntaxError
	}
	return ev.EvalChunk(n)
}

func (ev *Evaler) EvalChunk(n *parse.Chunk) int {
	status, _ := ev.frame().chunk(n)
	return status
}

func (ev *Evaler) frame() *frame {
	return &frame{ev.arguments, ev.variables, ev.functions, ev.files, ev.diagFile, 0}
}

type frame struct {
	arguments    []string
	variables    map[string]string
	functions    map[string]*parse.CompoundCommand
	files        []*os.File
	diagFile     *os.File
	lastCmdSubst int
}

func (fm *frame) cloneForSubshell() *frame {
	// TODO: Optimize with copy on write
	return &frame{
		cloneSlice(fm.arguments),
		cloneMap(fm.variables), cloneMap(fm.functions),
		cloneSlice(fm.files),
		fm.diagFile,
		0,
	}
}

// Prints a diagnostic message.
func (fm *frame) diag(n parse.Node, format string, args ...any) {
	// TODO: Incorporate range information in the error message.
	fmt.Fprintf(fm.diagFile, format+"\n", args...)
}

// The rest of this file contains methods on (*frame) that implement the
// execution of commands and expansion of words. The former group of methods
// return (int, bool), and the latter return (expander, bool).
//
// The boolean flag is false iff there was a fatal error - an error that should
// abort the evaluation process. This includes all the "shall exit" errors in
// the "non-interactive shell" column of the table in
// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_08_01:
//
//  - Shell language syntax error (implemented in the Eval function)
//  - Special built-in utility error
//  - Redirection error with special built-in utilities
//  - Variable assignment error
//  - Expansion error
//
// The following errors can be fatal depending on relevant options:
//
//  - Eligible non-zero exit codes when "set -e" is active
//  - File already exits when "set -C" is active
//
// The following errors don't abort evaluation:
//
//  - Other utility (not a special built-in) error
//  - Redirection error with compound commands
//  - Redirection error with function execution
//  - Redirection error with other utilities (not special built-ins)
//  - Command not found
//
// Regardless of whether the error is fatal, the site that generates the error
// prints a suitable message.
//
// Error conditions that are not covered by POSIX may or may not be treated as
// fatal; they will have comments near them.
//
// POSIX also requires that interactive shells don't exit when there is an
// error. That behavior is outside the scope of this package: evaluation always
// stops when there is a fatal error, and it's up to the caller of this package
// to decide whether that causes the process to exit.

func (fm *frame) chunk(ch *parse.Chunk) (int, bool) {
	var lastStatus int
	for _, ao := range ch.AndOrs {
		status, ok := fm.andOr(ao)
		if !ok {
			return status, false
		}
		lastStatus = status
	}
	return lastStatus, true
}

func (fm *frame) andOr(ao *parse.AndOr) (int, bool) {
	var lastStatus int
	for i, pp := range ao.Pipelines {
		if i > 0 && shouldSkipAndOr(ao.AndOp[i-1], lastStatus) {
			continue
		}
		status, ok := fm.pipeline(pp)
		if !ok {
			return status, false
		}
		lastStatus = status
	}
	return lastStatus, true
}

func shouldSkipAndOr(and bool, lastStatus int) bool {
	return (and && lastStatus != 0) || (!and && lastStatus == 0)
}

func (fm *frame) pipeline(ch *parse.Pipeline) (int, bool) {
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
			// How to handle failure to create pipes is not covered by POSIX.
			// We write the error message to diagFile, but treat it as a
			// non-fatal error so that the script may recover from it.
			for j := 0; j < i; j++ {
				pipes[j][0].Close()
				pipes[j][1].Close()
			}
			fm.diag(ch, "unable to create pipe for pipeline:", err)
			return StatusPipeError, true
		}
		pipes[i][0], pipes[i][1] = r, w
	}

	var wg sync.WaitGroup
	wg.Add(n)

	var lastStatus int
	var lastOK bool
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
			status, ok := newFm.form(f)
			// All but the last form is run in a subshell, so even fatal errors
			// in them don't terminate evaluation.
			if i == n-1 {
				lastStatus, lastOK = status, ok
			}
			// Close the pipes associated with this command. Use the files
			// stored in pipes rather than newFm.files because the latter may
			// have been modified due to redirections.
			//
			// TODO: Maybe the pipes should be closed when the redirection
			// happened instead?
			if i > 0 {
				pipes[i-1][0].Close()
			}
			if i < n-1 {
				pipes[i][1].Close()
			}
			wg.Done()
		}(i, f)
	}
	wg.Wait()
	return lastStatus, lastOK
}

func (fm *frame) compoundCommand(cc *parse.CompoundCommand) (int, bool) {
	if cc.Subshell {
		fm = fm.cloneForSubshell()
	}
	return fm.chunk(cc.Body)
}

func (fm *frame) form(f *parse.Form) (int, bool) {
	switch f.Type {
	case parse.CompoundCommandForm:
		return fm.compoundCommand(f.Body)
	case parse.FnDefinitionForm:
		// According to POSIX, redirections in a function definition form apply
		// when the function is defined.
		//
		// TODO: Implement this.
		for _, word := range f.Words {
			exp, ok := fm.compound(word)
			if !ok {
				return StatusExpansionError, false
			}
			name := exp.expandOneWord()
			fm.functions[name] = f.Body
		}
		return 0, true
	}

	// See comment on the code path using this field.
	fm.lastCmdSubst = 0

	for _, rd := range f.Redirs {
		status, ok, cleanup := fm.redir(rd)
		if cleanup != nil {
			defer cleanup()
		}
		if status != 0 {
			// TODO: Make the error fatal if command is special builtin.
			return status, ok
		}
	}

	// TODO: Temp assignment
	for _, assign := range f.Assigns {
		exp, ok := fm.compound(assign.RHS)
		if !ok {
			return StatusExpansionError, false
		}
		fm.variables[assign.LHS] = exp.expandOneWord()
	}

	var words []string
	for _, cp := range f.Words {
		exp, ok := fm.compound(cp)
		if !ok {
			return StatusExpansionError, false
		}
		words = append(words, fm.glob(exp.expand(fm.ifs()))...)
	}
	if len(words) == 0 {
		// 2.9.1 Simple Commands:
		//
		// If there is no command name, but the command contained a command
		// substitution, the command shall complete with the exit status of the
		// last command substitution performed. Otherwise, the command shall
		// complete with a zero exit status.
		return fm.lastCmdSubst, true
	}

	// Functions?
	if fn, ok := fm.functions[words[0]]; ok {
		oldArgs := fm.arguments
		fm.arguments = words
		status, ok := fm.compoundCommand(fn)
		fm.arguments = oldArgs
		return status, ok
	}

	// Builtins?
	if builtin, ok := builtins[words[0]]; ok {
		return builtin(fm, words[1:]), true
	}

	// External commands?
	path, err := exec.LookPath(words[0])
	if err != nil {
		// TODO: Return StatusCommandNotExecutable if file exists but is not
		// executable.
		fm.diag(f, "command not found: %v", err)
		return StatusCommandNotFound, true
	}
	words[0] = path

	proc, err := os.StartProcess(path, words, &os.ProcAttr{
		Files: fm.files,
	})
	if err != nil {
		fm.diag(f, "command not executable: %v", err)
		return StatusCommandNotExecutable, true
	}

	state, err := proc.Wait()
	if err != nil {
		fm.diag(f, "error waiting for process to finish: %v", err)
		return StatusWaitError, true
	}
	if state.Exited() {
		return state.ExitCode(), true
	} else {
		waitStatus := state.Sys().(syscall.WaitStatus)
		if waitStatus.Signaled() {
			return StatusSignalBase + int(waitStatus.Signal()), true
		}
		return StatusWaitOther, true
	}
}

// Returns a status code, whether to continue, and a clean up function (the
// latter may be nil).
func (fm *frame) redir(rd *parse.Redir) (int, bool, func() error) {
	var flag, defaultDst int
	switch rd.Mode {
	case parse.RedirInput:
		flag = os.O_RDONLY
		defaultDst = 0
	case parse.RedirOutput:
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		defaultDst = 1
	case parse.RedirInputOutput:
		flag = os.O_RDWR | os.O_CREATE
		defaultDst = 0
	case parse.RedirAppend:
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
		defaultDst = 1
	case parse.RedirHeredoc:
		// flag is not used for RedirHeredoc
		defaultDst = 0
	default:
		fm.diag(rd, "bug: unkown redir mode: %v", rd.Mode)
		return StatusShellBug, false, nil
	}
	var src *os.File
	var cleanup func() error
	if rd.Mode == parse.RedirHeredoc {
		r, w, err := os.Pipe()
		if err != nil {
			fm.diag(rd, "unable to create pipe for heredoc: %v", err)
			return StatusPipeError, true, nil
		}
		content := rd.Heredoc.Value
		go func() {
			n, err := w.WriteString(content)
			if err != nil {
				fm.diag(rd, "error writing to heredoc pipe: %v", err)
			} else if n < len(content) {
				fm.diag(rd, "short write on heredoc pipe: %v < %v", n, len(content))
			}
			w.Close()
		}()
		src = r
	} else {
		// POSIX specifies that the RHS of redirections do not undergo field
		// splitting or pathname expansion, with the exception that
		// interactive shells may perform pathname expansion if the result
		// is one word
		// (https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_07).
		//
		// Dash and ksh follow this behavior.
		//
		// Bash by default doesn't suppress either, and errors when the RHS
		// expands to multiple words. Setting POSIXLY_CORRECT causes bash to
		// suppress pathname expansion, but not field splitting.
		exp, ok := fm.compound(rd.Right)
		if !ok {
			return StatusExpansionError, false, nil
		}
		right := exp.expandOneWord()

		if rd.RightFd {
			if right == "-" {
				// A nil src signifies that dst should be closed.
				src = nil
			} else if fd64, err := strconv.ParseInt(right, 10, 0); err == nil {
				fd := int(fd64)
				if 0 <= fd && fd < len(fm.files) {
					src = fm.files[fd]
				} else {
					fm.diag(rd, "source FD is out of range: %v", right)
					return StatusRedirectionError, true, nil
				}
			} else {
				fm.diag(rd, "source is not FD: %v", right)
				return StatusRedirectionError, true, nil
			}
		} else {
			f, err := os.OpenFile(right, flag, 0644)
			if err != nil {
				fm.diag(rd, "can't open redirection source: %v", err)
				return StatusRedirectionError, true, nil
			}
			cleanup = f.Close
			src = f
		}
	}
	dst := rd.Left
	if dst == -1 {
		dst = defaultDst
	}
	if dst >= len(fm.files) {
		newFiles := make([]*os.File, dst+1)
		copy(newFiles, fm.files)
		fm.files = newFiles
	}
	if src == nil {
		// TODO
		fm.diag(rd, "closing FD not implemented yet")
		return StatusRedirectionError, false, nil
	}
	fm.files[dst] = src
	return 0, true, cleanup
}

func (fm *frame) compound(cp *parse.Compound) (expander, bool) {
	c := compound{}
	if cp.TildePrefix != "" {
		// The result of tilde expansion is considered "quoted" and not subject
		// to further expansions.
		home, ok := fm.home(cp, cp.TildePrefix[1:])
		if !ok {
			return nil, false
		}
		c.elems = append(c.elems, literal{home})
	}
	for _, pr := range cp.Parts {
		elem, ok := fm.primary(pr)
		if !ok {
			return nil, false
		}
		c.elems = append(c.elems, elem)
	}
	return c, true
}

var (
	userCurrent = user.Current
	userLookup  = user.Lookup
)

func (fm *frame) home(n parse.Node, uname string) (string, bool) {
	if uname == "" {
		if home, set := fm.variables["HOME"]; set {
			return home, true
		}
	}
	var u *user.User
	var err error
	if uname == "" {
		u, err = userCurrent()
	} else {
		u, err = userLookup(uname)
	}
	if err != nil {
		if uname == "" {
			fm.diag(n, "can't get home of current user: %v\n", err)
		} else {
			fm.diag(n, "can't get home of %v: %v\n", uname, err)
		}
		return "", false
	}
	return u.HomeDir, true
}

func (fm *frame) primary(pr *parse.Primary) (expander, bool) {
	switch pr.Type {
	case parse.BarewordPrimary, parse.SingleQuotedPrimary:
		// Literals don't undergo word splitting. Barewords are considered
		// "quoted" for this purpose because any metacharacter has to be escaped
		// to be considered part of a bareword.
		return literal{pr.Value}, true
	case parse.DoubleQuotedPrimary:
		return fm.dqSegments(pr.Segments)
	case parse.ArithmeticPrimary:
		exp, ok := fm.dqSegments(pr.Segments)
		if !ok {
			return nil, false
		}
		result, err := arith.Eval(exp.expandOneWord(), fm.variables)
		if err != nil {
			fm.diag(pr, "bad arithmetic expression: %v", err)
			return nil, false
		}
		// Arithmetic expressions undergo word splitting.
		//
		// This seems unlikely to be useful (the result is a single number), but
		// it's specified by POSIX and implemented by dash, bash and ksh. The
		// following writes "1 1": "IFS=0; echo $(( 101 ))"
		//
		// Interestingly, zsh doesn't perform word splitting on the result of
		// arithmetic expressions even with "setopt sh_word_split".
		return scalar{strconv.FormatInt(result, 10)}, true
	case parse.WildcardCharPrimary:
		return globMeta{pr.Value[0]}, true
	case parse.OutputCapturePrimary:
		r, w, err := os.Pipe()
		if err != nil {
			fm.diag(pr, "unable to create pipe for command substitution: %v", err)
			return nil, false
		}
		newFm := fm.cloneForSubshell()
		newFm.files[1] = w
		// TODO: Save exit status for use in commands that only have command
		// substitutions
		go func() {
			fm.lastCmdSubst, _ = newFm.chunk(pr.Body)
			w.Close()
		}()
		output, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			fmt.Fprintln(fm.files[2], "read:", err)
		}
		// Removal of trailing newlines happens independently of and before word
		// splitting.
		return scalar{strings.TrimRight(string(output), "\n")}, true
	case parse.VariablePrimary:
		return fm.variable(pr.Variable)
	default:
		// TODO
		fm.diag(pr, "shell bug: unknown primary type %v", pr.Type)
		return nil, false
	}
}

func (fm *frame) dqSegments(segs []*parse.Segment) (expander, bool) {
	var elems []expander
	for _, seg := range segs {
		switch seg.Type {
		case parse.StringSegment:
			elems = append(elems, literal{seg.Value})
		case parse.ExpansionSegment:
			exp, ok := fm.primary(seg.Expansion)
			if !ok {
				return nil, false
			}
			elems = append(elems, exp)
		default:
			fmt.Fprintln(fm.files[2], "unknown DQ segment type", seg.Type)
		}
	}
	return doubleQuoted{elems}, true
}

type varInfo struct {
	set       bool
	null      bool
	normal    bool
	scalar    bool
	scalarVal string
}

func (fm *frame) variable(v *parse.Variable) (expander, bool) {
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
		return scalar{strconv.Itoa(n)}, true
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
				fm.complainBadVar(v.Name, "unset", mod.Argument)
				return nil, false
			}
		case ":?":
			if info.null {
				fm.complainBadVar(v.Name, "null or unset", mod.Argument)
				return nil, false
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
			arg, ok := fm.compound(mod.Argument)
			if !ok {
				return nil, false
			}
			if assignIfUse {
				if info.normal {
					fm.variables[v.Name] = arg.expandOneWord()
				} else {
					fm.diag(v, "cannot assign to $%v", v.Name)
					return nil, false
				}
			}
			return arg, true
		}
	}
	// If we reach here, expand the variable itself.
	if info.scalar {
		return scalar{info.scalarVal}, true
	}
	return array{fm.arguments[1:], fm.ifs, name == "@"}, true
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

func (fm *frame) complainBadVar(name, what string, argNode *parse.Compound) {
	exp, ok := fm.compound(argNode)
	if !ok {
		return
	}
	arg := exp.expandOneWord()
	// This intentionally uses files[2] rather than diagFile, because this is
	// not a "shell diagnostic message" and should respect active redirections.
	if arg == "" {
		fmt.Fprintf(fm.files[2], "%v is %v\n", name, what)
	} else {
		fmt.Fprintf(fm.files[2], "%v is %v: %v\n", name, what, arg)
	}
}

func (fm *frame) trimVariable(name, scalarVal string, argNode *parse.Compound, f func(string, string) string) (expander, bool) {
	// TODO: Implement pattern
	exp, ok := fm.compound(argNode)
	if !ok {
		return nil, false
	}
	pattern := exp.expandOneWord()
	if name == "*" || name == "@" {
		elems := make([]string, len(fm.arguments)-1)
		for i, arg := range fm.arguments[1:] {
			elems[i] = f(arg, pattern)
		}
		return array{elems, fm.ifs, name == "@"}, true
	} else {
		return scalar{f(scalarVal, pattern)}, true
	}
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
