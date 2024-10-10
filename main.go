package main

import (
	"os"
)

func main() {

	aw, err := arpad(os.Stdin, os.Stdout)
	if err != nil {
		panic(err)
	}

	err = transtar(&aw.in, aw.out)
	if err != nil {
		panic(err)
	}

	err = aw.Close()
	if err != nil {
		panic(err)
	}
}
