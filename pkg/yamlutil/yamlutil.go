package yamlutil

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ReadFile reads a YAML file and unmarshals it into the target struct.
func ReadFile(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, target)
}

// WriteFile marshals a struct to YAML and writes it to the given path.
// Creates parent directories if needed.
func WriteFile(path string, source interface{}) error {
	data, err := yaml.Marshal(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// WriteFileSafe marshals and atomically writes to the given path by writing a
// temporary file in the target directory and renaming it into place. This
// avoids leaving a partially-written file behind on failure. Parent
// directories are created if needed.
func WriteFileSafe(path string, source interface{}) error {
	data, err := yaml.Marshal(source)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*.yaml")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName) // no-op if the rename below succeeded
	}()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(0644); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
