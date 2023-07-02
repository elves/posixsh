package eval

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/elves/posixsh/pkg/arith"
	"github.com/elves/posixsh/pkg/parse"
)

type Evaler struct {
	files     []*os.File
	arguments []string
	variables variables
	functions map[string]*parse.Command
	aliases   map[string]string
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
		files,
		args,
		initVariablesFromEnv(os.Environ()),
		make(map[string]*parse.Command),
		make(map[string]string),
	}
}

func (ev *Evaler) Eval(code string) int {
	n, err := parse.Parse(code)
	if err != nil {
		// TODO: Add range information.
		fmt.Fprintln(ev.files[2], "syntax error:", err)
		return StatusSyntaxError
	}
	return ev.EvalChunk(n)
}

func (ev *Evaler) EvalChunk(n *parse.Chunk) int {
	status, _ := ev.frame().topChunk(n)
	return status
}

func (ev *Evaler) frame() *frame {
	wd, err := os.Getwd()
	if err != nil {
		wd = "/"
	}
	ev.variables.values["PWD"] = wd
	return &frame{
		ev.files, ev.arguments, ev.variables, ev.functions, ev.aliases,
		ev.files[2], wd,
		0, 0, 0, nil, 0, nil, 0, false}
}

type frame struct {
	files     []*os.File
	arguments []string
	variables variables
	functions map[string]*parse.Command
	aliases   map[string]string
	// POSIX requires all cases except "special built-in utility error" and
	// "other utility (not a special builtin-in error)" to print a shell
	// diagnostic message to the stderr, ignoring all active redirections. We
	// save the initial stderr (files[2]) in this field for that purpose.
	diagFile *os.File
	// Virtualized working directory. Necessary to emulate subshells.
	wd string
	// Shell options.
	options options
	// Used for $?.
	lastPipelineStatus int
	// Used as the status of simple commands with only assignments.
	lastCmdSubstStatus int
	// Set during a command call. Useful for diagnostic messages written from
	// (special) builtins.
	currentCommand *parse.Command
	// The following two fields are used to implement break/continue inside
	// for/while/until loops:
	//
	// - loopDepth is maintained by for/while/until and stores the number of
	//   enclosing loops. It is examined by break/continue to decide which target
	//   loop to break/continue to.
	//
	// - loopAbort is set by break/continue and examined by for/while/until,
	//   which act accordingly when the loopAbort.dest matches the current loop
	//   depth.
	//
	// The implementation is purely dynamic: it does not know which loops
	// lexically enclose the break/continue command. POSIX leaves it unspecified
	// whether break/continue should act on non-lexically enclosing loops, so
	// this behavior is compliant. This behavior is only shared with zsh; dash,
	// bash and ksh all only recognize lexically enclosing loops.
	loopDepth int
	loopAbort *loopAbort
	// Used to implement return:
	//
	// - fnLevel is incremented by (*frame).runSimple when calling a function.
	// - fnAbort is set to true by the return command.
	fnLevel int
	fnAbort bool
}

type loopAbort struct {
	dest int  // Destination value of loopDepth
	next bool // True for continue, false for break
}

func (fm *frame) cloneForSubshell() *frame {
	// TODO: Optimize with copy on write
	return &frame{
		cloneSlice(fm.files),
		cloneSlice(fm.arguments),
		fm.variables.clone(),
		cloneMap(fm.functions),
		cloneMap(fm.aliases),
		fm.diagFile,
		fm.wd,
		fm.options,
		// POSIX doesn't explicitly specify whether subshells inherit $?, but
		// all of dash, bash, ksh and zsh let subshells inherit $?, so we follow
		// their behavior.
		fm.lastPipelineStatus,
		0, nil, 0, nil, 0, false,
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

// Runs a top-level chunk. The top-level is special in that the verbose option
// causes it to print every AndOr node before executing.
func (fm *frame) topChunk(ch *parse.Chunk) (int, bool) {
	var lastStatus int
	for _, ao := range ch.AndOrs {
		if fm.options.has(verbose) {
			fmt.Fprintln(fm.files[2], strings.TrimRight(ao.Source(), "\n"))
		}
		status, ok := fm.andOr(ao)
		if !ok {
			return status, false
		}
		lastStatus = status
	}
	return lastStatus, true
}

func (fm *frame) chunk(ch *parse.Chunk) (int, bool) {
	return fm.andOrs(ch.AndOrs)
}

func (fm *frame) andOrs(aos []*parse.AndOr) (int, bool) {
	var lastStatus int
	for _, ao := range aos {
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
		fm.lastPipelineStatus = status
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

func (fm *frame) pipeline(pl *parse.Pipeline) (int, bool) {
	n := len(pl.Commands)
	if n == 1 {
		// Short path
		f := pl.Commands[0]
		if len(f.Redirs) > 0 {
			files := cloneSlice(fm.files)
			defer func() { fm.files = files }()
		}
		return fm.command(f)
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
			fm.diag(pl, "unable to create pipe for pipeline:", err)
			return StatusPipeError, true
		}
		pipes[i][0], pipes[i][1] = r, w
	}

	var wg sync.WaitGroup
	wg.Add(n)

	var lastStatus int
	var lastOK bool
	for i, f := range pl.Commands {
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
		go func(i int, f *parse.Command) {
			status, ok := newFm.command(f)
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
	if pl.Not {
		return not(lastStatus), lastOK
	}
	return lastStatus, lastOK
}

func not(status int) int {
	if status == 0 {
		return 1
	}
	return 0
}

func (fm *frame) command(c *parse.Command) (int, bool) {
	switch data := c.Data.(type) {
	case parse.Simple:
		return fm.runSimple(c, data)
	case parse.FnDef:
		return fm.runFnDef(c, data)
	default:
		// For the rest of command types, redirections are always performed
		// first and never permanent.
		//
		// This duplicates code in (*frame).runSimple.
		if len(c.Redirs) > 0 {
			savedFiles := cloneSlice(fm.files)
			defer func() {
				fm.files = savedFiles
			}()
		}
		for _, rd := range c.Redirs {
			status, ok, cleanup := fm.redir(rd)
			if cleanup != nil {
				defer cleanup()
			}
			if status != 0 {
				return status, ok
			}
		}
		switch data := c.Data.(type) {
		case parse.Group:
			return fm.chunk(data.Body)
		case parse.SubshellGroup:
			// Fatal errors from subshells are turned into non-fatal errors.
			status, _ := fm.cloneForSubshell().chunk(data.Body)
			return status, true
		case parse.For:
			return fm.runFor(c, data)
		case parse.Case:
			return fm.runCase(c, data)
		case parse.If:
			return fm.runIf(c, data)
		case parse.While:
			return fm.runWhile(c, data)
		case parse.Until:
			return fm.runUntil(c, data)
		default:
			fm.diag(c, "bug: unknown command type %T", c.Data)
			return StatusShellBug, false
		}
	}
}

func (fm *frame) runSimple(c *parse.Command, data parse.Simple) (int, bool) {
	// See comment on the code path using this field.
	fm.lastCmdSubstStatus = 0

	// The order of arguments > redirections > assignments is specified in
	// 2.9.1 Simple Commands. POSIX allows for redirections and assignments to
	// swap position if the command is a special builtin, but we don't do that.

	// POSIX requires alias substitution to happen after tokenizing but before
	// further parsing; this behavior is followed by all of dash, bash, ksh and
	// zsh.
	//
	// However, we parse most things statically and don't feed the expanded
	// alias back to the parser, and the alias command explicitly only supports
	// aliases that consist of barewords. For this subset of aliases we do
	// support, we can match the POSIX behavior by expanding aliases as early as
	// possible, so that they can - for example - shadow special builtins.
	head, tail := expandAlias(data.Words, fm.aliases)

	tailWords, ok := fm.expandCompounds(tail)
	if !ok {
		return StatusExpansionError, false
	}
	words := append(head, tailWords...)

	isSpecial := len(words) > 0 && specialBuiltins[words[0]] != nil

	// Redirections are only permanent when we're running "exec".
	permRedir := len(words) > 0 && words[0] == "exec"
	if len(c.Redirs) > 0 && !permRedir {
		savedFiles := cloneSlice(fm.files)
		defer func() {
			fm.files = savedFiles
		}()
	}

	for _, rd := range c.Redirs {
		status, ok, cleanup := fm.redir(rd)
		if cleanup != nil {
			defer cleanup()
		}
		if status != 0 {
			if isSpecial {
				// POSIX specifies that redirection errors are fatal when
				// running a special builtin.
				return status, false
			}
			return status, ok
		}
	}

	// Assignments are permanent if there is no command, or if the command is a
	// special builtin.
	permAssign := len(words) == 0 || isSpecial
	for _, assign := range c.Assigns {
		if fm.variables.readonly.has(assign.LHS) {
			fm.diag(assign, "%v is readonly", assign.LHS)
			// Assigning to a readonly error is a fatal error according to
			// POSIX.
			return StatusAssignmentError, false
		}
	}
	for _, assign := range c.Assigns {
		exp, ok := fm.compound(assign.RHS)
		if !ok {
			return StatusExpansionError, false
		}
		name := assign.LHS
		if !permAssign {
			value, isSet := fm.variables.values[name]
			exported := fm.variables.exported.has(name)
			if !isSet {
				defer delete(fm.variables.values, name)
			} else {
				defer func() {
					fm.variables.values[name] = value
				}()
			}
			// When the allexport option is active, setting a variable will also
			// export it, so undo it.
			//
			// TODO: if we are calling a function that explicitly exports the
			// variable, it should not be undone.
			if !exported {
				defer delete(fm.variables.exported, name)
			}
		}
		// We have already checked that all variables are not readonly.
		fm.SetVar(name, exp.expandOneString())
	}

	// POSIX specifies that setting xtrace causes each "command" to print a
	// "trace" after expansion but before execution, without further details.
	// All of dash, bash, ksh and zsh interprete "command" as "simple command",
	// and use a leading + to indicate trace lines, but they don't agree on the
	// exact format of the trace. For example, bash adds one + for one level of
	// command substitutions, and zsh includes filenames and line numbers. They
	// also don't agree on how temporary assignments and redirections should be
	// printed.
	//
	// We mostly follow dash's behavior: one +, print assignments but not
	// redirections.
	if fm.options.has(xtrace) {
		fm.files[2].WriteString("+")
		for _, assign := range c.Assigns {
			fmt.Fprintf(fm.files[2], " %v=%v", assign.LHS, quote(fm.getVar(assign.LHS)))
		}
		for _, word := range words {
			fmt.Fprintf(fm.files[2], " %v", word)
		}
		fm.files[2].WriteString("\n")
	}

	if len(words) == 0 {
		// 2.9.1 Simple Commands:
		//
		// If there is no command name, but the command contained a command
		// substitution, the command shall complete with the exit status of the
		// last command substitution performed. Otherwise, the command shall
		// complete with a zero exit status.
		return fm.lastCmdSubstStatus, true
	}

	return fm.callCommand(words, c, true)
}

func (fm *frame) callCommand(words []string, c *parse.Command, callFn bool) (int, bool) {
	prevCommand := fm.currentCommand
	fm.currentCommand = c
	defer func() {
		fm.currentCommand = prevCommand
	}()

	// The order of special builtin > function > non-special builtin > external
	// is specified in 2.9.1 Simple Commands. The function step can be skipped
	// for the "command" builtin.

	if builtin, ok := specialBuiltins[words[0]]; ok {
		return builtin(fm, words[1:])
	}

	// Functions?
	if callFn {
		if fn, ok := fm.functions[words[0]]; ok {
			return fm.callFuncLike(words[1:], func() (int, bool) {
				return fm.command(fn)
			})
		}
	}

	// Builtins?
	if builtin, ok := builtins[words[0]]; ok {
		return builtin(fm, words[1:]), true
	}

	// External commands?
	path, status := fm.lookExecutable(words[0], fm.getVar("PATH"))
	if status != 0 {
		return status, true
	}
	words[0] = path

	proc, err := fm.startProcess(words)
	if errors.Is(err, syscall.ENOEXEC) {
		// POSIX requires the shell to handle ENOEXEC by using a shell to run
		// the file.
		proc, err = fm.startProcess(append([]string{"/bin/sh"}, words...))
	}
	if err != nil {
		fm.diag(c, "command not executable: %v", err)
		return StatusCommandNotExecutable, true
	}

	state, err := proc.Wait()
	if err != nil {
		fm.diag(c, "error waiting for process to finish: %v", err)
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

func (fm *frame) callFuncLike(args []string, f func() (int, bool)) (int, bool) {
	oldArgs := fm.arguments
	// POSIX specifies that $0 is unchanged during a function call, but
	// position parameters are changed to be function arguments.
	fm.arguments = append([]string{fm.arguments[0]}, args...)
	fm.fnLevel++
	status, ok := f()
	fm.fnLevel--
	fm.arguments = oldArgs
	if fm.fnAbort {
		fm.fnAbort = false
		return status, true
	}
	return status, ok
}

// Looks for executable. Handles error reporting, using fm.currentCommand for
// range information in diagnostics.
func (fm *frame) lookExecutable(name, path string) (string, int) {
	path, ok, exists := lookPath(name, fm.wd, path, 0o111)
	if ok {
		return path, 0
	}
	if exists {
		fm.diag(fm.currentCommand, "command not executable: %v", name)
		return "", StatusCommandNotExecutable
	} else {
		fm.diag(fm.currentCommand, "command not found: %v", name)
		return "", StatusCommandNotFound
	}
}

func (fm *frame) startProcess(words []string) (*os.Process, error) {
	return os.StartProcess(words[0], words, &os.ProcAttr{
		Dir:   fm.wd,
		Env:   fm.variables.serializeEnvEntries(),
		Files: fm.files,
	})
}

func (fm *frame) runFnDef(c *parse.Command, data parse.FnDef) (int, bool) {
	exp, ok := fm.compound(data.Name)
	if !ok {
		return StatusExpansionError, false
	}
	name := exp.expandOneString()
	if _, isSpecial := specialBuiltins[name]; isSpecial {
		fm.diag(c, "invalid function name %v", name)
		return StatusInvalidFunctionName, false
	}
	fm.functions[name] = data.Body
	return 0, true
}

func (fm *frame) runFor(c *parse.Command, data parse.For) (int, bool) {
	exp, ok := fm.compound(data.VarName)
	if !ok {
		return StatusExpansionError, false
	}
	varName := exp.expandOneString()
	var values []string
	if data.Values == nil {
		values = fm.arguments[1:]
	} else {
		var ok bool
		values, ok = fm.expandCompounds(data.Values)
		if !ok {
			return StatusExpansionError, false
		}
	}

	var lastStatus int
	for _, value := range values {
		err := fm.SetVar(varName, value)
		if err != nil {
			fm.diag(data.VarName, "%v", err)
			// Assigning to a readonly error is a fatal error according to
			// POSIX.
			return StatusAssignmentError, false
		}
		status, ok, breaking := fm.runLoopBody(data.Body)
		if breaking {
			return 0, true
		}
		if !ok {
			return status, false
		}
		lastStatus = status
	}
	return lastStatus, true
}

// Runs a loop body and handles break/continue if it's the correct level:
//   - break causes the last return value to be true.
//   - continue is turned into (0, true).
func (fm *frame) runLoopBody(body []*parse.AndOr) (status int, ok, breaking bool) {
	fm.loopDepth++
	status, ok = fm.andOrs(body)
	fm.loopDepth--
	if !ok && fm.loopAbort != nil && fm.loopAbort.dest == fm.loopDepth {
		abort := fm.loopAbort
		fm.loopAbort = nil
		if abort.next {
			return 0, true, false
		}
		return 0, true, true
	}
	return status, ok, false
}

func (fm *frame) runCase(c *parse.Command, data parse.Case) (int, bool) {
	exp, ok := fm.compound(data.Word)
	if !ok {
		return StatusExpansionError, false
	}
	word := exp.expandOneString()
	for i, pattern := range data.Patterns {
		for _, choice := range pattern {
			exp, ok := fm.compound(choice)
			if !ok {
				return StatusExpansionError, false
			}
			choice := regexp.MustCompile("^" + regexpPatternFromWord(exp.expandOneWord(), false) + "$")
			if choice.MatchString(word) {
				return fm.andOrs(data.Bodies[i])
			}
		}
	}
	// No patterns are matched.
	return 0, true
}

func (fm *frame) runIf(c *parse.Command, data parse.If) (int, bool) {
	for i, condition := range data.Conditions {
		status, ok := fm.andOrs(condition)
		if !ok {
			return status, false
		}
		if status == 0 {
			return fm.andOrs(data.Bodies[i])
		}
	}
	if data.ElseBody != nil {
		return fm.andOrs(data.ElseBody)
	}
	return 0, true
}

func (fm *frame) runWhile(c *parse.Command, data parse.While) (int, bool) {
	return fm.runWhileUntil(c, data.Condition, data.Body, true)
}

func (fm *frame) runUntil(c *parse.Command, data parse.Until) (int, bool) {
	return fm.runWhileUntil(c, data.Condition, data.Body, false)
}

func (fm *frame) runWhileUntil(c *parse.Command, condition, body []*parse.AndOr, wantZero bool) (int, bool) {
	lastStatus := 0
	for {
		status, ok := fm.andOrs(condition)
		if !ok {
			return status, false
		}
		if (status == 0) != wantZero {
			break
		}
		status, ok, breaking := fm.runLoopBody(body)
		if breaking {
			return 0, true
		}
		if !ok {
			return status, false
		}
		lastStatus = status
	}
	return lastStatus, true
}

// Returns a status code, whether there is an error that should always be
// considered fatal (an expansion error), and a clean up function (which may be
// nil).
func (fm *frame) redir(rd *parse.Redir) (int, bool, func() error) {
	var flag, defaultDst int
	switch rd.Mode {
	case parse.RedirInput:
		flag = os.O_RDONLY
		defaultDst = 0
	case parse.RedirOutput, parse.RedirOutputOverwrite:
		flag = os.O_WRONLY | os.O_CREATE
		if rd.Mode == parse.RedirOutput && fm.options.has(noclobber) {
			flag |= os.O_EXCL
		} else {
			flag |= os.O_TRUNC
		}
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
		text := rd.Heredoc.Text
		if rd.Heredoc.Segments != nil {
			exp, ok := fm.segments(rd.Heredoc.Segments)
			if !ok {
				return StatusExpansionError, false, nil
			}
			text = exp.expandOneString()
		}
		go func() {
			n, err := w.WriteString(text)
			if err != nil {
				fm.diag(rd, "error writing to heredoc pipe: %v", err)
			} else if n < len(text) {
				fm.diag(rd, "short write on heredoc pipe: %v < %v", n, len(text))
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
		right := exp.expandOneString()

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
			// Use virtual working directory as the base for relative paths.
			if !filepath.IsAbs(right) {
				right = filepath.Join(fm.wd, right)
			}
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

func (fm *frame) expandCompounds(cps []*parse.Compound) ([]string, bool) {
	var result []string
	for _, cp := range cps {
		exp, ok := fm.compound(cp)
		if !ok {
			return nil, false
		}
		words := exp.expand(fm.ifs())
		if fm.options.has(noglob) {
			result = append(result, each(stringifyWord, words)...)
		} else {
			result = append(result, generateFilenames(words)...)
		}
	}
	return result, true
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
		if home, set := fm.variables.values["HOME"]; set {
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
	case parse.BarewordPrimary:
		return bareword{pr.Value}, true
	case parse.EscapedPrimary, parse.SingleQuotedPrimary:
		return literal{pr.Value}, true
	case parse.DoubleQuotedPrimary:
		return fm.segments(pr.Segments)
	case parse.ArithmeticPrimary:
		exp, ok := fm.segments(pr.Segments)
		if !ok {
			return nil, false
		}
		result, err := arith.Eval(exp.expandOneString(), fm)
		if err != nil {
			if errors.As(err, &arith.VarError{}) {
				fm.diag(pr, "%v", err)
			} else {
				fm.diag(pr, "bad arithmetic expression: %v", err)
			}
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
		return expanded{strconv.FormatInt(result, 10)}, true
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
			fm.lastCmdSubstStatus, _ = newFm.chunk(pr.Body)
			w.Close()
		}()
		output, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			fmt.Fprintln(fm.files[2], "read:", err)
		}
		// Removal of trailing newlines happens independently of and before word
		// splitting.
		return expanded{strings.TrimRight(string(output), "\n")}, true
	case parse.VariablePrimary:
		return fm.variable(pr.Variable)
	default:
		fm.diag(pr, "shell bug: unknown primary type %v", pr.Type)
		return literal{}, false
	}
}

func (fm *frame) segments(segs []parse.Segment) (expander, bool) {
	var elems []expander
	for _, seg := range segs {
		expansion, text := seg.Segment()
		if expansion != nil {
			exp, ok := fm.primary(expansion)
			if !ok {
				return nil, false
			}
			elems = append(elems, exp)
		} else {
			elems = append(elems, literal{text})
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
		variable, set := fm.variables.values[name]
		info = scalarVarInfo(variable, set, true)
	}

	if !info.set && fm.options.has(nounset) && !hasSubstitutionOp(v) {
		fm.diag(v, "%v is unset", name)
		return nil, false
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
		return expanded{strconv.Itoa(n)}, true
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
		case "#", "##", "%", "%%":
			exp, ok := fm.compound(mod.Argument)
			if !ok {
				return nil, false
			}
			w := exp.expandOneWord()

			// The function to apply to a single scalar variable, or each
			// element of an array variable.
			var transform func(string) string
			switch mod.Operator {
			case "#":
				pattern := regexp.MustCompile("^" + regexpPatternFromWord(w, true))
				transform = func(s string) string {
					return pattern.ReplaceAllLiteralString(s, "")
				}
			case "##":
				pattern := regexp.MustCompile("^" + regexpPatternFromWord(w, false))
				transform = func(s string) string {
					return pattern.ReplaceAllLiteralString(s, "")
				}
			case "%":
				// Since Go's regexp engine always prefers the leftmost match,
				// when removing the shortest suffix, it is not sufficient to
				// translate * to .*? and anchor the pattern on the end with $;
				// we also need to consume an arbitrary prefix as large as
				// possible.
				pattern := regexp.MustCompile("^((?s).*)" + regexpPatternFromWord(w, true) + "$")
				transform = func(s string) string {
					return pattern.ReplaceAllString(s, "$1")
				}
			case "%%":
				pattern := regexp.MustCompile(regexpPatternFromWord(w, false) + "$")
				transform = func(s string) string {
					return pattern.ReplaceAllLiteralString(s, "")
				}
			}

			if name == "*" || name == "@" {
				elems := make([]string, len(fm.arguments)-1)
				for i, arg := range fm.arguments[1:] {
					elems[i] = transform(arg)
				}
				return array{elems, fm.ifs, name == "@"}, true
			} else {
				return expanded{transform(info.scalarVal)}, true
			}
		default:
			// The parser doesn't parse other modifiers.
			fm.diag(v, "bug: unknown operator %v", mod.Operator)
			return literal{}, false
		}
		if useArg {
			arg, ok := fm.compound(mod.Argument)
			if !ok {
				return nil, false
			}
			if assignIfUse {
				if info.normal {
					err := fm.SetVar(v.Name, arg.expandOneString())
					if err != nil {
						fm.diag(v, "%v", err)
						return nil, false
					}
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
		return expanded{info.scalarVal}, true
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
		return strconv.Itoa(fm.lastPipelineStatus), true, true
	case "-":
		return fm.options.dash(), true, true
	case "$":
		return strconv.Itoa(os.Getpid()), true, true
	case "!":
		// TODO
		return "", false, true
	default:
		return "", false, false
	}
}

func hasSubstitutionOp(v *parse.Variable) bool {
	if v.Modifier == nil {
		return false
	}
	switch v.Modifier.Operator {
	case "-", ":-", "=", ":=", "+", ":+", "?", ":?":
		return true
	}
	return false
}

func (fm *frame) complainBadVar(name, what string, argNode *parse.Compound) {
	exp, ok := fm.compound(argNode)
	if !ok {
		return
	}
	arg := exp.expandOneString()
	// This intentionally uses files[2] rather than diagFile, because this is
	// not a "shell diagnostic message" and should respect active redirections.
	if arg == "" {
		fmt.Fprintf(fm.files[2], "%v is %v\n", name, what)
	} else {
		fmt.Fprintf(fm.files[2], "%v is %v: %v\n", name, what, arg)
	}
}

func (fm *frame) ifs() string {
	// Default value specified in section 2.6.5 "Field splitting".
	return fm.getVarOr("IFS", " \t\n")
}

func (fm *frame) ps2() string {
	// Default value specified in section 2.5.4 "Shell variables".
	return fm.getVarOr("PS2", "> ")
}

func (fm *frame) getVar(name string) string {
	return fm.getVarOr(name, "")
}

func (fm *frame) getVarOr(name, fallback string) string {
	value, set := fm.variables.values[name]
	if !set {
		return fallback
	}
	return value
}
