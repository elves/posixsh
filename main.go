package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/elves/posixsh/pkg/eval"
	"github.com/elves/posixsh/pkg/parse"
	"src.elv.sh/pkg/diag"
	"src.elv.sh/pkg/sys"
)

var (
	printAST = flag.Bool("print-ast", false, "print AST")
)

func main() {
	flag.Parse()
	args := flag.Args()
	ev := eval.NewEvaler(eval.StdFiles)
	if len(args) > 0 {
		f, err := os.Open(args[0])
		if err != nil {
			fmt.Println(err)
			return
		}
		defer f.Close()
		evalAll(ev, f)
		return
	} else if sys.IsATTY(os.Stdin.Fd()) {
		repl(ev)
	} else {
		evalAll(ev, os.Stdin)
	}
}

func repl(ev *eval.Evaler) {
	stdin := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("posixsh> ")
		input, err := stdin.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				fmt.Println(err)
			}
			break
		}
		doEval(ev, input)
	}
}

func evalAll(ev *eval.Evaler, r io.Reader) {
	buf, err := io.ReadAll(r)
	if err != nil {
		fmt.Println(err)
	}
	doEval(ev, string(buf))
}

func doEval(ev *eval.Evaler, input string) {
	n, err := parse.Parse(input)
	if *printAST {
		fmt.Println("node:", parse.PprintAST(n))
	}
	if err != nil {
		fmt.Println("err:", err)
		for _, entry := range err.(parse.Error).Errors {
			sr := diag.NewContext("input", input, diag.PointRanging(entry.Position))
			fmt.Printf("  %s\n", entry.Message)
			fmt.Printf("    %s\n", sr.ShowCompact(""))
		}
	}

	status := ev.EvalChunk(n)
	if status != 0 {
		fmt.Println("status:", status)
	}
}
