# debcow

`debcow` is a tool to adapt a Debian package in a way which is suitable for copy-on-write instalation.

## How it works
Copy-on-write via filesystem reflinks requires the data to be aligned on the filesystem boundary, typically 4k. This is not the case for Debian packages, which are typically AR archives which contains a `data.tar.xz` archive with the actual files. This archive is compressed and not aligned.
To have the data aligned, `debcow` do the following operations:
1. Insert a dummy file named `_data-pad` before the `data.tar` archive, to have the data.tar archive aligned.
1. Uncompress `data.tar.xz` to have the data uncompressed on disk.
1. Filter the `data.tar`, convert the tar archive from the GNU format to the POSIX format, and insert a comment in every file header to align the files on the filesystem boundary.

## How to use it
To extract the data with copy-on-write the requirements are:
1. a filesystem that supports copy-on-write, like btrfs or xfs.
2. a custom `tar` binary which supports archives embedded in another file, and extraction with copy-on-write. Such tool can be found here:
https://github.com/teknoraver/tar
3. a custom `dpkg` binary which calls `tar` with the `--reflink` and `--offset` options, so it can extract the deb package without file copy. This dpkg version can be found here:
https://github.com/teknoraver/dpkg

## Sample usage and performances
Ideally during the download phase, `apt` will pipe the downloaded packages to `debcow`, so the package is transcoded with no extra delay.
The transcoded package is still compatible with standard dpkg, so it can be installed even with Debian stock binaries.
If the system has the custom `tar` and `dpkg` binaries, the alignd package will be extracted by using filesystem reflinks, which is much faster than the standard extraction:
```
$ debcow <linux-image-6.11.2-arm64_6.11.2-1_arm64.deb >linux-image-6.11.2-arm64_6.11.2-1_arm64_cow.deb
oldpos: 135108, endpos: 185692160
Decompressed size changed from 90266468 to 185556992

$ ll linux-image-*
-rw-r--r--. 1 teknoraver teknoraver 178M Oct 14 01:41 linux-image-6.11.2-arm64_6.11.2-1_arm64_cow.deb
-rw-r--r--. 1 teknoraver teknoraver  87M Oct  7 20:52 linux-image-6.11.2-arm64_6.11.2-1_arm64.deb

$ time dpkg -x linux-image-6.11.2-arm64_6.11.2-1_arm64.deb root1/

real	0m1.568s
user	0m3.829s
sys	0m0.483s

$ time dpkg -x linux-image-6.11.2-arm64_6.11.2-1_arm64_cow.deb root2/

real	0m0.174s
user	0m0.017s
sys	0m0.126s

$ diff -urN root1/ root2/
$
```
The difference is much more noticeable with larger packages, like 0ad-data which is more than 1GB:
```
$ time dpkg -x 0ad-data_0.0.26-1_all.deb root1/

real	0m13.564s
user	1m15.945s
sys	0m4.938s

$ time dpkg -x 0ad-data_0.0.26-1_all_cow.deb root2/

real	0m1.411s
user	0m0.012s
sys	0m0.665s
```
