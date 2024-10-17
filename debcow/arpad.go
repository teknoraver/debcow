package debcow

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/DataDog/zstd"
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

type WriteSeekCloser interface {
	io.WriteCloser
	io.WriteSeeker
}

type ArWriter struct {
	io.WriteCloser

	out     WriteSeekCloser
	pos     int64
	oldsize int64
	format  string
	in      io.Reader
	verbose bool
}

func (aw *ArWriter) Write(p []byte) (n int, err error) {
	return aw.out.Write(p)
}

func (aw *ArWriter) Close() error {
	endpos, err := aw.out.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	decsize := endpos - aw.pos - 60
	if decsize != aw.oldsize {

		if aw.verbose {
			fmt.Fprintf(os.Stderr, "Data size changed from %d to %d bytes, adjusting header\n", aw.oldsize, decsize)
		}

		var newsizestr string
		newsizestr = fmt.Sprintf("%-10d", decsize)
		aw.out.Seek(aw.pos+48, io.SeekStart)
		_, err = aw.out.Write([]byte(newsizestr))
		if err != nil {
			return err
		}
	}

	aw.out.Close()

	return nil
}

func (aw *ArWriter) addPadding(out WriteSeekCloser) error {
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

	if aw.verbose {
		fmt.Fprintf(os.Stderr, "Adding %d bytes _data-pad file\n", size)
	}

	out.Write(buf)
	out.Seek(int64(newpos), io.SeekStart)

	return nil
}

func (aw *ArWriter) handleDataTar(algo string, in io.Reader, out WriteSeekCloser, buf []byte, size uint64) error {
	aw.addPadding(out)

	startpos, err := out.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	if aw.verbose {
		fmt.Fprintf(os.Stderr, "data.tar new offset is 0x%x\n", startpos+60)
	}

	aw.pos = startpos

	if algo != "" {
		copy(buf, "data.tar        ")
		if aw.verbose {
			fmt.Fprintln(os.Stderr, "Decompressing", algo[1:], "archive")
		}
	}

	_, err = out.Write(buf[:60])
	if err != nil {
		return err
	}

	switch algo {
	case "":
		aw.in = in
	case ".zst":
		aw.in = zstd.NewReader(in)
	case ".xz":
		aw.in = xz.NewReader(in)
	case ".gz":
		aw.in, err = gzip.NewReader(in)
		if err != nil {
			return err
		}
	default:
		return errors.New("Unknown algorithm: " + algo[1:])
	}

	return nil
}

func ArPadder(in io.Reader, out WriteSeekCloser, verbose bool) (*ArWriter, error) {
	buf := make([]byte, 1024)

	_, err := in.Read(buf[:8])
	if err != nil {
		return nil, err
	}

	if string(buf[:8]) != "!<arch>\n" {
		return nil, errors.New("Not an ar file")
	}

	if verbose {
		fmt.Fprintln(os.Stderr, "Transcoding deb package")
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

		if verbose {
			pos, err := out.Seek(0, io.SeekCurrent)
			if err != nil {
				return nil, err
			}

			fmt.Fprintf(os.Stderr, "%s offset 0x%x size %d\n", name, pos+60, size)
		}

		if len(name) >= 8 && name[:8] == "data.tar" {
			aw := ArWriter{
				verbose: verbose,
				out:     out,
				oldsize: int64(size),
			}

			err = aw.handleDataTar(name[8:], in, out, buf, size)
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
