package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
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
	if len(args) > 0 {
		f, err := os.Open(args[0])
		if err != nil {
			fmt.Println(err)
			return
		}
		defer f.Close()
		evalAll(f)
		return
	} else if sys.IsATTY(os.Stdin.Fd()) {
		repl()
	} else {
		evalAll(os.Stdin)
	}
}

func repl() {
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
		doEval(input)
	}
}

func evalAll(r io.Reader) {
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		fmt.Println(err)
	}
	doEval(string(buf))
}

func doEval(input string) {
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

	ret := eval.NewEvaler().EvalChunk(n)
	if !ret {
		os.Exit(1)
	}
}
