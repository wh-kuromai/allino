package main

import (
	"github.com/wh-kuromai/allino"
	_ "github.com/wh-kuromai/allino/example/test/handlers"
)

func main() {
	allino.RunCLI(&allino.Config{})
}
