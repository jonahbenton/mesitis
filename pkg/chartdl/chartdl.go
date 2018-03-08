// This utilizes ideas/code from
//
// https://medium.com/@skdomino/taring-untaring-files-in-go-6b07cf56bc07
// https://stackoverflow.com/questions/11692860/how-can-i-efficiently-download-a-large-file-using-go
// https://gist.github.com/indraniel/1a91458984179ab4cf80
//
// However, none of the above were sufficiently correct.
//
package chartdl

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/golang/glog"
)

func DownloadChart(destdir, url string) (string, error) {
	tmpfile, err := downloadFile(destdir, url)
	if err != nil {
		e := fmt.Errorf("Failed to retrieve chartUrl and write to scratchPath: %s", err)
		glog.Errorln(e)
		return "", e
	}
	tarroot, err := untar(destdir, tmpfile)
	if err != nil {
		e := fmt.Errorf("Failed to untar %s: %s", tmpfile, err)
		glog.Errorln(e)
		return "", e
	}
	os.Remove(tmpfile)
	return tarroot, nil
}

func downloadFile(destdir string, url string) (string, error) {

	// get the data first
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// check server response
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	// TODO check on mime type...?

	// if we have the data, create and write
	tmpfile, err := ioutil.TempFile(destdir, "chart-downloader")
	if err != nil {
		return "", err
	}
	defer tmpfile.Close()

	_, err = io.Copy(tmpfile, resp.Body)
	if err != nil {
		return "", err
	}

	return tmpfile.Name(), nil
}

// untar takes a dest and src path, untargzs src into dest, returning the created root dir
func untar(destdir string, srcpath string) (string, error) {

	r, err := os.Open(srcpath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	gzr, err := gzip.NewReader(r)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	// keep track of implicitly created directories
	// one of them may be the tarroot
	shortestDir := ""
	shortestFileParent := ""

	for {
		header, err := tr.Next()

		// if no more files are found, we are done
		if err == io.EOF {
			break
		}

		// return any other error
		if err != nil {
			return "", err
		}

		// if the header is nil, just skip it (not sure how this happens)
		if header == nil {
			continue
		}

		// the target location where the dir/file should be created
		// TODO do we need to check that header.Name doesn't begin with or have .. ?
		target := filepath.Join(destdir, header.Name)

		switch header.Typeflag {

		// if dir and it doesn't exist create it
		// TODO go pkg has tar.TypeDir == char int 5, e.g. int 53
		case 5:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return "", err
				}
			}
			// capture the shortest created directory, could be tarroot
			if shortestDir == "" || len(target) < len(shortestDir) {
				shortestDir = target
			}

		// if it's a file create it. make sure parent dir exists first
		// TODO go pkg has tar.TypeDir == char int 0, e.g. int 48
		case 0:
			parent := filepath.Dir(target)
			if _, err := os.Stat(parent); err != nil {
				if err := os.MkdirAll(parent, 0755); err != nil {
					return "", err
				}
			}

			// if the file exists, bail
			if _, err := os.Stat(target); err == nil {
				return "", fmt.Errorf("File already exists in destination, bailing: %s\n", target)
			}

			// capture the shortest parent directory, could be tarroot
			if shortestFileParent == "" || len(parent) < len(shortestFileParent) {
				shortestFileParent = parent
			}

			// os.OpenFile(O_CREATE) succeeds even while a file exists
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return "", err
			}
			defer f.Close()

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return "", err
			}

		default:
			return "", fmt.Errorf("Target %s has unknown type. %v, %v\n", target, header.Typeflag)
		}
	}
	// calculate tarroot
	if shortestDir == "" && shortestFileParent == "" {
		return "", fmt.Errorf("Cannot determine tar root directory")
	}
	if shortestDir != "" && len(shortestDir) < len(shortestFileParent) {
		return shortestDir, nil
	}
	return shortestFileParent, nil

}
