package main

import (
	"flag"
	"os"

	"github.com/teknoraver/debcow/debcow"
)

func main() {
	verbose := flag.Bool("v", false, "verbose")
	flag.Parse()

	aw, err := debcow.ArPadder(os.Stdin, os.Stdout, *verbose)
	if err != nil {
		panic(err)
	}

	err = aw.TarTar()
	if err != nil {
		panic(err)
	}

	err = aw.Close()
	if err != nil {
		panic(err)
	}
}
