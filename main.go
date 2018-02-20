package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/xiaq/posixsh/parse"
)

func main() {
	buf, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	ch := &parse.Chunk{}
	rest, err := parse.Parse(string(buf), ch)
	fmt.Println("node:", parse.PprintAST(ch))
	if rest != "" {
		fmt.Printf("parsed %d, rest %d\n", len(buf)-len(rest), len(rest))
	}
	if err != nil {
		fmt.Println("err:", err)
		for _, entry := range err.(parse.Error).Errors {
			fmt.Printf("  at %d: %s\n", entry.Position, entry.Message)
		}
	}
}
