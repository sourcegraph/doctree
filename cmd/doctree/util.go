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

func isParentDir(parent, child string) (bool, error) {
	relativePath, err := filepath.Rel(parent, child)
	if err != nil {
		return false, err
	}
	return !strings.Contains(relativePath, ".."), nil
}

// Get hash of the directory.
// It detects changes in content as well as metadata of all the files and subdirectories.
//
// Reference: https://unix.stackexchange.com/questions/35832/how-do-i-get-the-md5-sum-of-a-directorys-contents-as-one-sum
func GetDirHash(dir string) string {
	tarCmd := exec.Command("tar", "-cf", "-", dir)
	md5sumCmd := exec.Command("md5sum")

	pipe, _ := tarCmd.StdoutPipe()
	defer pipe.Close()

	md5sumCmd.Stdin = pipe

	// Run the tar command
	err := tarCmd.Start()
	if err != nil {
		fmt.Printf("tar command failed with '%s'\n", err)
		return "0"
	}
	// Run and get the output of md5sum command
	res, _ := md5sumCmd.Output()

	return string(res)
}
