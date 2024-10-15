package main

import (
	"os"

	"github.com/teknoraver/deb2extents/debcow"
)

func main() {
	aw, err := debcow.ArPadder(os.Stdin, os.Stdout)
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
