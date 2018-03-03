package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/elves/elvish/sys"
	"github.com/elves/elvish/util"
	"github.com/xiaq/posixsh/eval"
	"github.com/xiaq/posixsh/parse"
)

var (
	printAST = flag.Bool("print-ast", false, "print AST")
)

func main() {
	if len(os.Args) > 1 {
		f, err := os.Open(os.Args[1])
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
		sr := util.NewSourceRange("input", input, parsedLen, parsedLen)
		fmt.Println(sr.PprintCompact(""))
	}
	if err != nil {
		fmt.Println("err:", err)
		for _, entry := range err.(parse.Error).Errors {
			sr := util.NewSourceRange("input", input, entry.Position,
				entry.Position)
			fmt.Printf("  %s\n", entry.Message)
			fmt.Printf("    %s\n", sr.PprintCompact(""))
		}
	}

	err = eval.NewEvaler().EvalChunk(ch)
	if err != nil {
		fmt.Println("eval error:", err)
	}
}
