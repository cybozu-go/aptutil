package mirror

import (
	"os"
	"path/filepath"
)

// DirSync calls fsync(2) on the directory to save changes in the directory.
//
// This should be called after os.Create, os.Rename and so on.
func DirSync(d string) error {
	f, err := os.OpenFile(d, os.O_RDONLY, 0755)
	if err != nil {
		return err
	}
	err = f.Sync()
	if err != nil {
		return err
	}
	return f.Close()
}

func dirSyncFunc(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if !info.Mode().IsDir() {
		return nil
	}

	return DirSync(path)
}

// DirSyncTree calls DirSync recursively on a directory tree
// rooted from d.
func DirSyncTree(d string) error {
	// filepath.Walk includes d.
	return filepath.Walk(d, dirSyncFunc)
}
