package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/elves/elvish/pkg/diag"
	"github.com/elves/elvish/pkg/sys"
	"github.com/elves/posixsh/pkg/eval"
	"github.com/elves/posixsh/pkg/parse"
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
	} else if sys.IsATTY(os.Stdin) {
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
	ch := &parse.Chunk{}
	rest, err := parse.Parse(input, ch)
	if *printAST {
		fmt.Println("node:", parse.PprintAST(ch))
	}
	if rest != "" {
		parsedLen := len(input) - len(rest)
		fmt.Printf("parsed %d, rest %d\n", parsedLen, len(rest))
		fmt.Println("parsing stopped here:")
		sr := diag.NewContext("input", input, diag.PointRanging(parsedLen))
		fmt.Println(sr.ShowCompact(""))
	}
	if err != nil {
		fmt.Println("err:", err)
		for _, entry := range err.(parse.Error).Errors {
			sr := diag.NewContext("input", input, diag.PointRanging(entry.Position))
			fmt.Printf("  %s\n", entry.Message)
			fmt.Printf("    %s\n", sr.ShowCompact(""))
		}
	}

	ret := eval.NewEvaler().EvalChunk(ch)
	if !ret {
		os.Exit(1)
	}
}
