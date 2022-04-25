package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func getGitURIForFile(dir string) (string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = filepath.Dir(dir)
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

func defaultProjectName() string {
	uri, err := getGitURIForFile(".")
	if err != nil {
		absDir, err := filepath.Abs(".")
		if err != nil {
			return ""
		}
		return absDir
	}
	return uri
}
