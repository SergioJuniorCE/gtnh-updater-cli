package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
)

const (
	gtnhDownloadsBaseURL       = "https://downloads.gtnewhorizons.com"
	gtnhDownloadsDownloadsPath = "/Multi_mc_downloads"
	gtnhDownloadsListingURL    = gtnhDownloadsBaseURL + gtnhDownloadsDownloadsPath + "/?raw"
)

type progressReader struct {
	reader    io.Reader
	total     int64
	readBytes int64
	callback  func(downloaded, total int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.reader.Read(b)
	if n > 0 {
		read := atomic.AddInt64(&p.readBytes, int64(n))
		if p.callback != nil {
			p.callback(read, p.total)
		}
	}
	return n, err
}

func (p *progressReader) read() int64 {
	return atomic.LoadInt64(&p.readBytes)
}

// downloadVersionZip downloads the selected version zip into destDir and returns the file path.
// If progress is non-nil it will receive the number of bytes downloaded and the total size (when known).
func downloadVersionZip(versionRef, destDir string, progress func(downloaded, total int64)) (string, error) {
	versionRef = strings.TrimSpace(versionRef)
	if versionRef == "" {
		return "", fmt.Errorf("empty version file name")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}

	var (
		downloadURL string
		fileName    string
	)

	if parsed, err := url.Parse(versionRef); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		downloadURL = parsed.String()
		fileName = path.Base(parsed.Path)
	} else {
		cleaned := strings.TrimLeft(versionRef, "/")
		if !strings.HasPrefix(cleaned, strings.TrimPrefix(gtnhDownloadsDownloadsPath, "/")) {
			cleaned = strings.TrimPrefix(gtnhDownloadsDownloadsPath, "/") + "/" + cleaned
		}
		downloadURL = fmt.Sprintf("%s/%s", strings.TrimRight(gtnhDownloadsBaseURL, "/"), cleaned)
		fileName = path.Base(cleaned)
	}

	if fileName == "" || fileName == "." || fileName == "/" {
		return "", fmt.Errorf("cannot determine filename from %q", versionRef)
	}

	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	zipPath := filepath.Join(destDir, fileName)
	f, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if progress != nil {
		reader := &progressReader{reader: resp.Body, total: resp.ContentLength, callback: progress}
		if _, err := io.Copy(f, reader); err != nil {
			return "", err
		}
		progress(reader.read(), reader.total)
	} else {
		if _, err := io.Copy(f, resp.Body); err != nil {
			return "", err
		}
	}
	return zipPath, nil
}

// extractZip extracts the zip archive into destDir. It prevents path traversal and
// attempts to preserve directory/file structure. If progress is non-nil it receives
// the number of entries processed out of the total and the current entry name.
func extractZip(zipPath, destDir string, progress func(processed, total int, name string)) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	totalEntries := len(r.File)
	processed := 0
	for _, f := range r.File {
		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") {
			continue
		}
		targetPath := filepath.Join(destDir, cleanName)
		// ensure targetPath is within destDir
		if !strings.HasPrefix(filepath.Clean(targetPath)+string(os.PathSeparator), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			processed++
			if progress != nil {
				progress(processed, totalEntries, f.Name)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(targetPath)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
		processed++
		if progress != nil {
			progress(processed, totalEntries, f.Name)
		}
	}
	return nil
}

// maybeFlattenSingleDir moves the contents of a single subdirectory up to destDir
// if the zip extracted into a nested folder. This helps when archives contain a top-level folder.
func maybeFlattenSingleDir(destDir string) error {
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return err
	}
	var onlyDir string
	for _, e := range entries {
		if e.IsDir() {
			if onlyDir != "" {
				// more than one dir
				return nil
			}
			onlyDir = filepath.Join(destDir, e.Name())
		} else {
			// has files at root; do not flatten
			return nil
		}
	}
	if onlyDir == "" {
		return nil
	}
	// move children up
	children, err := os.ReadDir(onlyDir)
	if err != nil {
		return err
	}
	for _, c := range children {
		src := filepath.Join(onlyDir, c.Name())
		dst := filepath.Join(destDir, c.Name())
		if err := os.Rename(src, dst); err != nil {
			// fallback to copy if rename fails (e.g., across devices)
			if c.IsDir() {
				if err2 := copyDir(src, dst); err2 != nil {
					return err
				}
				_ = os.RemoveAll(src)
			} else {
				if err2 := copyFile(src, dst); err2 != nil {
					return err
				}
				_ = os.Remove(src)
			}
		}
	}
	_ = os.Remove(onlyDir)
	return nil
}
