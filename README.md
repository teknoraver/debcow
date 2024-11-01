# debcow
`debcow` is a tool to adapt Debian packages so they are suitable for copy-on-write installation.

## How it works
The idea is to use reflinks, the [filesystem copy-on-write feature](https://man7.org/linux/man-pages/man2/ioctl_ficlone.2.html), to link the file content from the downloaded archive to the destination files, instead of copying the data.  
Copy-on-write requires the data to be aligned on the filesystem boundary, typically 4k. This is not the case for Debian packages, which are typically AR archives containing a compressed `data.tar.xz` archive with the actual files.  
To have the data uncompressed and aligned, `debcow` do the following:
1. Insert a dummy file named `_data-pad` before the `data.tar` archive, to have the data.tar archive aligned.
1. Uncompress `data.tar.xz` to have the data uncompressed on disk.
1. Filter the `data.tar`, convert the tar archive from the GNU format to the POSIX format, and insert a comment in every file header to align the files on the filesystem boundary.

## How to use it
To extract the data with copy-on-write the requirements are:
1. a filesystem that supports copy-on-write, like BtrFS or XFS.
2. a custom `dpkg` which can read POSIX data.tar content and extracts file by using copy-on-write. This dpkg version can be found here:
https://github.com/teknoraver/dpkg/tree/cow

Ideally during the download phase, `apt` will pipe the downloaded packages to `debcow`, so the transcoding will go in parallel with the download.  
The transcoded package is still compatible with standard dpkg, so it can be installed even with Debian stock binaries.  
If the system has the custom `dpkg` binary, the alignd package will be extracted by using filesystem reflinks.

## Benchmarks
A package like linux-firmware is a good example because it contains a lot of big files. First we transcode the package manually:
```
# time debcow -v <linux-firmware_20240318.git3b128b60-0ubuntu2.4_amd64.deb >linux-firmware_20240318.git3b128b60-0ubuntu2.4_amd64.debc
Transcoding deb package
debian-binary offset 0x44 size 4
control.tar.zst offset 0x84 size 66707
data.tar.zst offset 0x10554 size 483787509
Adding 2672 bytes _data-pad file
Decompressing zst archive
data.tar new offset is 0x11000
Data size changed from 483787509 to 501596160 bytes, adjusting header

real    0m4,939s
user    0m0,596s
sys     0m1,605s

# ll linux-firmware_*
-rw-r--r-- 1 root root 462M set 13 14:36 linux-firmware_20240318.git3b128b60-0ubuntu2.4_amd64.deb
-rw-r--r-- 1 root root 479M nov  1 14:54 linux-firmware_20240318.git3b128b60-0ubuntu2.4_amd64.debc
```

Installing the package with CoW is six times faster than the stock dpkg:
```
# time dpkg -i linux-firmware_20240318.git3b128b60-0ubuntu2.4_amd64.deb
(Reading database ... 214246 files and directories currently installed.)
Preparing to unpack linux-firmware_20240318.git3b128b60-0ubuntu2.4_amd64.deb ...
Unpacking linux-firmware (20240318.git3b128b60-0ubuntu2.4) over (20240318.git3b128b60-0ubuntu2.4) ...
Setting up linux-firmware (20240318.git3b128b60-0ubuntu2.4) ...

real    0m39,264s
user    0m3,141s
sys     0m4,720s

# time dpkg -i linux-firmware_20240318.git3b128b60-0ubuntu2.4_amd64.debc
(Reading database ... 214246 files and directories currently installed.)
Preparing to unpack linux-firmware_20240318.git3b128b60-0ubuntu2.4_amd64.debc ...
Unpacking linux-firmware (20240318.git3b128b60-0ubuntu2.4) over (20240318.git3b128b60-0ubuntu2.4) ...
Setting up linux-firmware (20240318.git3b128b60-0ubuntu2.4) ...

real    0m6,531s
user    0m0,306s
sys     0m1,471s
```

To have a real world example, this is a dist-upgrade from Ubuntu 24.04 to 24.10.

Upgrade done with the stock dpkg takes 90 minutes, where 78 are needed by the package extraction:
[![asciicast](https://asciinema.org/a/686692.svg)](https://asciinema.org/a/686692)

With dpkg CoW the installation takes 44 minutes and the package extraction lowers to 31 minutes:
[![asciicast](https://asciinema.org/a/686707.svg)](https://asciinema.org/a/686707)
