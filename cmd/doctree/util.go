package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/sourcegraph/doctree/doctree/git"
)

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not get home directory: %s", err)
		home = "."
	}
	return filepath.Join(home, ".doctree")
}

func defaultProjectName(defaultDir string) string {
	uri, err := git.URIForFile(defaultDir)
	if err != nil {
		absDir, err := filepath.Abs(defaultDir)
		if err != nil {
			return ""
		}
		return absDir
	}
	return uri
}

func isParentDir(parent, child string) (bool, error) {
	relativePath, err := filepath.Rel(parent, child)
	if err != nil {
		return false, err
	}
	return !strings.Contains(relativePath, ".."), nil
}

// Recursively watch a directory
func recursiveWatch(watcher *fsnotify.Watcher, dir string) error {
	err := filepath.Walk(dir, func(walkPath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() && !strings.HasPrefix(fi.Name(), ".") { // file is directory and isn't hidden
			if err = watcher.Add(walkPath); err != nil {
				return errors.Wrap(err, "watcher.Add")
			}
		}
		return nil
	})
	return err
}
