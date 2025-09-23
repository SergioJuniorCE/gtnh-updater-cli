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
)

const (
	gtnhDownloadsBaseURL       = "https://downloads.gtnewhorizons.com"
	gtnhDownloadsDownloadsPath = "/Multi_mc_downloads"
	gtnhDownloadsListingURL    = gtnhDownloadsBaseURL + gtnhDownloadsDownloadsPath + "/?raw"
)

// downloadVersionZip downloads the selected version zip into destDir and returns the file path.
func downloadVersionZip(versionRef, destDir string) (string, error) {
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
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return zipPath, nil
}

// extractZip extracts the zip archive into destDir. It prevents path traversal and
// attempts to preserve directory/file structure.
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
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
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		func() {
			defer rc.Close()
			out, err := os.Create(targetPath)
			if err != nil {
				rc.Close()
				panic(err)
			}
			defer out.Close()
			_, err = io.Copy(out, rc)
			if err != nil {
				panic(err)
			}
		}()
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
