package main

import (
	"fmt"
	"github.com/dongbo/gope/cli"
	"net/http"
	_ "net/http/pprof"
	"os"
)

func init() {
	return
}

func main() {
	fmt.Println(os.Args)

	go func() {
		http.ListenAndServe("localhost:6060", nil)
		//http://localhost:6060/debug/pprof/
	}()
	cli.Run()
}
