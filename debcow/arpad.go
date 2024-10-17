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

	in      io.Reader
	out     WriteSeekCloser
	pos     int64
	oldsize int64
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

	/* calculate the decompressed size subtracting the end of file
	 * position from the start of the data.tar file */
	decsize := endpos - aw.pos
	if decsize != aw.oldsize {
		if aw.verbose {
			fmt.Fprintf(os.Stderr, "Data size changed from %d to %d bytes, adjusting header\n", aw.oldsize, decsize)
		}

		var newsizestr string
		newsizestr = fmt.Sprintf("%-10d", decsize)
		aw.out.Seek(aw.pos-60+48, io.SeekStart)
		_, err = aw.out.Write([]byte(newsizestr))
		if err != nil {
			return err
		}
	}

	aw.out.Close()

	return nil
}

func (aw *ArWriter) addPadding() error {
	/* pos here is the end of the control file */
	pos, err := aw.out.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	if (pos+60)%4096 == 0 {
		if aw.verbose {
			fmt.Fprintln(os.Stderr, "No padding needed")
		}
		return nil
	}

	/* calculate the next 4k boundary, and subtract 60 bytes for the header */
	newpos := round4k(pos+60) - 60
	size := newpos - pos - 60

	if size < 0 {
		size += 4096
	}

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

	aw.out.Write(buf)
	aw.out.Seek(newpos, io.SeekStart)

	return nil
}

func (aw *ArWriter) handleDataTar(algo string, buf []byte, size int64) error {
	aw.addPadding()

	if algo != "" {
		copy(buf, "data.tar        ")
		if aw.verbose {
			fmt.Fprintln(os.Stderr, "Decompressing", algo[1:], "archive")
		}
	}

	_, err := aw.out.Write(buf[:60])
	if err != nil {
		return err
	}

	/* pos here is the start of the data.tar file */
	startpos, err := aw.out.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	if aw.verbose {
		fmt.Fprintf(os.Stderr, "data.tar new offset is 0x%x\n", startpos)
	}

	aw.pos = startpos

	switch algo {
	case "":
	case ".zst":
		aw.in = zstd.NewReader(aw.in)
	case ".xz":
		aw.in = xz.NewReader(aw.in)
	case ".gz":
		aw.in, err = gzip.NewReader(aw.in)
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
		size, err := strconv.ParseInt(sizeStr, 10, 64)

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
				in:      in,
				out:     out,
				oldsize: size,
			}

			err = aw.handleDataTar(name[8:], buf, size)
			if err != nil {
				return nil, err
			}
			return &aw, nil
		}

		_, err = out.Write(buf[:n])
		if err != nil {
			return nil, err
		}

		io.CopyN(out, in, size)
		if size%2 != 0 {
			in.Read(buf[:1])
			out.Write(buf[:1])
		}
	}
}
