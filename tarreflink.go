package main

import (
	"archive/tar"
	"io"
	"os"
)

var spaces = make([]byte, 4096)

func round512(Size uint64) uint64 {
	return ((Size - 1) | 0x1ff) + 1
}

func round4k(Size uint64) uint64 {
	return ((Size - 1) | 0xfff) + 1
}

func addReflink(tw *tar.Writer, tr *tar.Reader, header *tar.Header, pos uint64) error {
	/* Headers are 512 bytes aligned */
	pos = round512(pos)

	/* Data PAX header needs two blocks */
	pos += 1024

	/* "1234 comment=\n" is 14 bytes long */
	next := round4k(pos + 14)
	rem := int64(next - pos - 14)

	if len(header.Name) > 100 {
		rem -= 511
	}

	if rem < 0 {
		rem += 4096
	}

	header.PAXRecords = make(map[string]string)
	header.PAXRecords["comment"] = string(spaces[:rem])

	err := tw.WriteHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(tw, tr)
	if err != nil {
		return err
	}

	return nil
}

func transtar(in *io.Reader, out *os.File) error {

	tr := tar.NewReader(*in)
	tw := tar.NewWriter(out)

	for i := range spaces {
		spaces[i] = ' '
	}

	for header, err := tr.Next(); err != io.EOF; header, err = tr.Next() {
		if err != nil {
			return err
		}

		header.Format = tar.FormatPAX

		if header.Typeflag == tar.TypeReg && header.Size > 0 {
			pos, err := out.Seek(0, os.SEEK_CUR)
			if err != nil {
				return err
			}

			err = addReflink(tw, tr, header, uint64(pos))
			if err != nil {
				return err
			}
			continue
		}

		err = tw.WriteHeader(header)
		if err != nil {
			return err
		}
	}

	tw.Flush()

	pos, err := out.Seek(0, os.SEEK_END)
	if err != nil {
		return err
	}

	/* Align to 4k */
	pos = int64(round4k(uint64(pos)))
	err = out.Truncate(pos)
	if err != nil {
		return err
	}

	return nil
}
