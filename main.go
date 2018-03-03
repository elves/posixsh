package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/elves/elvish/util"
	"github.com/xiaq/posixsh/eval"
	"github.com/xiaq/posixsh/parse"
)

func main() {
	buf, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	input := string(buf)

	ch := &parse.Chunk{}
	rest, err := parse.Parse(input, ch)
	fmt.Println("node:", parse.PprintAST(ch))
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
