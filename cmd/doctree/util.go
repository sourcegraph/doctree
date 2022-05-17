package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
)

func getGitURIForFile(dir string) (string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get absolute path for %s", dir)
	}
	cmd.Dir = absDir
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "git config --get remote.origin.url (pwd=%s)", cmd.Dir)
	}
	gitURL, err := normalizeGitURL(strings.TrimSpace(string(out)))
	if err != nil {
		return "", errors.Wrap(err, "normalizeGitURL")
	}
	return gitURL, nil
}

func normalizeGitURL(gitURL string) (string, error) {
	s := gitURL
	if strings.HasPrefix(gitURL, "git@") {
		s = strings.Replace(s, ":", "/", 1)
		s = strings.Replace(s, "git@", "git://", 1) // dummy scheme
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	p := u.Path
	p = strings.TrimSuffix(p, ".git")
	p = strings.TrimPrefix(p, "/")
	return fmt.Sprintf("%s/%s", u.Hostname(), p), nil
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not get home directory: %s", err)
		home = "."
	}
	return filepath.Join(home, ".doctree")
}

func defaultProjectName(defaultDir string) string {
	uri, err := getGitURIForFile(defaultDir)
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
