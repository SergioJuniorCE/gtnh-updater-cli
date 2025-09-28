package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()
	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer df.Close()
	_, err = io.Copy(df, sf)
	return err
}

func copyDir(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(srcDir, e.Name())
		dstPath := filepath.Join(dstDir, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func firstExistingPath(rel string, roots []string) (string, bool) {
	for _, root := range roots {
		candidate := filepath.Join(root, rel)
		if pathExists(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func preferredDestinationPath(rel string, roots []string) string {
	for _, root := range roots {
		if pathExists(root) {
			return filepath.Join(root, rel)
		}
	}
	if len(roots) == 0 {
		return rel
	}
	return filepath.Join(roots[len(roots)-1], rel)
}

// backupInstanceDir creates a timestamped folder copy of the destination instance
// e.g. C:\instances\GTNH -> C:\instances\GTNH.backup-20250101-120102
func backupInstanceDir(destInstancePath string) (string, error) {
	info, err := os.Stat(destInstancePath)
	if err != nil {
		return "", fmt.Errorf("destination not found: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("destination is not a directory")
	}
	parent := filepath.Dir(destInstancePath)
	base := filepath.Base(destInstancePath)
	ts := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(parent, fmt.Sprintf("%s.backup-%s", base, ts))
	if err := copyDir(destInstancePath, backupPath); err != nil {
		return "", err
	}
	return backupPath, nil
}

// migrateInstance copies selected folders/files from source instance into destination instance
func migrateInstance(sourceInstancePath, destinationInstancePath string) error {
	toCopyDirs := []string{
		"saves",
		"backups",
		"journeymap",
		"visualprospecting",
		"TCNodeTracker",
		"schematics",
		"resourcepacks",
		"shaderpacks",
		"screenshots",
	}
	toCopyFiles := []string{
		"localconfig.cfg",
		"BotaniaVars.dat",
		"options.txt",
		filepath.Join("serverutilities", "serverutilities.cfg"),
	}

	sourceRoots := []string{
		filepath.Join(sourceInstancePath, ".minecraft"),
		filepath.Join(sourceInstancePath, "minecraft"),
		sourceInstancePath,
	}
	destRoots := []string{
		filepath.Join(destinationInstancePath, ".minecraft"),
		filepath.Join(destinationInstancePath, "minecraft"),
		destinationInstancePath,
	}

	for _, d := range toCopyDirs {
		src, ok := firstExistingPath(d, sourceRoots)
		if !ok {
			continue
		}
		dst := preferredDestinationPath(d, destRoots)
		if err := copyDir(src, dst); err != nil {
			return fmt.Errorf("copy dir %s: %w", d, err)
		}
	}
	for _, f := range toCopyFiles {
		src, ok := firstExistingPath(f, sourceRoots)
		if !ok {
			continue
		}
		dst := preferredDestinationPath(f, destRoots)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy file %s: %w", f, err)
		}
	}
	return nil
}
