package indexer

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// Write autoindexedProjects as JSON in the provided filepath.
func WriteAutoIndex(path string, autoindexedProjects map[string]AutoIndexedProject) error {
	f, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "Create")
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(autoindexedProjects); err != nil {
		return errors.Wrap(err, "Encode")
	}

	return nil
}

// Read autoindexedProjects array from the provided filepath.
func ReadAutoIndex(path string) (map[string]AutoIndexedProject, error) {
	autoIndexedProjects := make(map[string]AutoIndexedProject)
	data, err := os.ReadFile(path)
	if err != nil {
		if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
			if err := os.Mkdir(filepath.Dir(path), os.ModePerm); err != nil {
				return nil, errors.Wrap(err, "CreateAutoIndexDirectory")
			}
		}
		if os.IsNotExist(err) {
			_, err := os.Create(path)
			if err != nil {
				return nil, errors.Wrap(err, "CreateAutoIndexFile")
			}
			return autoIndexedProjects, nil
		}
		return nil, errors.Wrap(err, "ReadAutoIndexFile")
	}
	err = json.Unmarshal(data, &autoIndexedProjects)
	if err != nil {
		return nil, errors.Wrap(err, "ParseAutoIndexFile")
	}

	return autoIndexedProjects, nil
}
