package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/jamespfennell/xz"
)

func stripSpaces(buf []byte) string {
	var i int
	for i = 0; i < len(buf); i++ {
		if buf[i] == ' ' || buf[i] == 0 {
			break
		}
	}

	return string(buf[:i])
}

type arWriter struct {
	io.WriteCloser

	out     *os.File
	pos     int64
	oldsize int64
	format  string
	in      io.Reader
}

func (aw *arWriter) Write(p []byte) (n int, err error) {
	return aw.out.Write(p)
}

func (aw *arWriter) Close() error {
	endpos, err := aw.out.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "oldpos: %d, endpos: %d\n", aw.pos, endpos)

	decsize := endpos - aw.pos - 60
	if decsize != aw.oldsize {
		var newsizestr string
		newsizestr = fmt.Sprintf("%-10d", decsize)
		fmt.Fprintf(os.Stderr, "Decompressed size changed from %d to %d\n", aw.oldsize, decsize)
		aw.out.Seek(aw.pos+48, io.SeekStart)
		_, err = aw.out.Write([]byte(newsizestr))
		if err != nil {
			return err
		}
	}

	aw.out.Close()

	return nil
}

func addPadding(out *os.File) error {
	pos, err := out.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	newpos := round4k(uint64(pos)+60) - 60
	size := newpos - uint64(pos) - 60

	buf := make([]byte, 60)
	copy(buf, "_data-pad       ")
	copy(buf[16:], "0            ")
	copy(buf[28:], "0     ")
	copy(buf[34:], "0     ")
	copy(buf[40:], "100644  ")
	copy(buf[48:], fmt.Sprintf("%-10d", size))
	copy(buf[58:], "`\n")

	out.Write(buf)
	out.Truncate(int64(newpos))
	out.Seek(0, io.SeekEnd)

	return nil
}

func handleDataTar(aw *arWriter, algo string, in *os.File, out *os.File, buf []byte, size uint64) error {
	addPadding(out)

	startpos, err := out.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	aw.pos = startpos

	if algo != "" {
		copy(buf, "data.tar        ")
	}

	_, err = out.Write(buf[:60])
	if err != nil {
		return err
	}

	switch algo {
	case "":
		aw.in = in
	case ".xz":
		aw.in = xz.NewReader(in)
	}

	return nil
}

func arpad(in *os.File, out *os.File) (*arWriter, error) {
	buf := make([]byte, 1024)

	_, err := in.Read(buf[:8])
	if err != nil {
		return nil, err
	}

	if string(buf[:8]) != "!<arch>\n" {
		return nil, errors.New("Not an ar file")
	}

	_, err = out.Write(buf[:8])
	if err != nil {
		return nil, err
	}

	for {
		n, err := in.Read(buf[:60])
		if err != nil {
			return nil, err
		}

		name := stripSpaces(buf[:16])
		sizeStr := stripSpaces(buf[48:58])
		size, err := strconv.ParseUint(sizeStr, 10, 64)

		if len(name) >= 8 && name[:8] == "data.tar" {
			var aw arWriter

			aw.out = out
			aw.oldsize = int64(size)

			err = handleDataTar(&aw, name[8:], in, out, buf, size)
			if err != nil {
				return nil, err
			}
			return &aw, nil
		}

		_, err = out.Write(buf[:n])
		if err != nil {
			return nil, err
		}

		io.CopyN(out, in, int64(size))
		if size%2 != 0 {
			in.Read(buf[:1])
			out.Write(buf[:1])
		}
	}
}
