package git

import (
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func URIForFile(dir string) (string, error) {
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

func RevParse(dir string, abbrefRef bool, rev string) (string, error) {
	var cmd *exec.Cmd
	if abbrefRef {
		cmd = exec.Command("git", "rev-parse", "--abbrev-ref", rev)
	} else {
		cmd = exec.Command("git", "rev-parse", rev)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get absolute path for %s", dir)
	}
	cmd.Dir = absDir
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "git rev-parse ... (pwd=%s)", cmd.Dir)
	}
	return string(out), nil
}
