package play

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	// saveFileMode keeps the save to its owner.
	saveFileMode = 0o600
	// saveDirMode is the mode of the directory of the save when the store has to create it.
	saveDirMode = 0o700
)

// Store keeps the [Snapshot] of a game between runs.
type Store interface {
	// Load returns the saved snapshot, and false when nothing was saved yet, which is a new game.
	// A save that cannot be read is an error and never a new game.
	Load() (Snapshot, bool, error)
	// Save replaces the saved snapshot. It leaves the previous one in place when it fails.
	Save(Snapshot) error
}

// FileStore is a [Store] that keeps the snapshot in one JSON file.
type FileStore struct {
	path string
}

// NewFileStore returns the store of the file at path. The file and its directory are created by
// the first Save.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// Load reads the file. A file that is missing is not an error. A file that is not a snapshot of
// this version, whether it is damaged, truncated, from a newer game or has a field this game does
// not know, is an error that names the path, and the file is left as it is.
func (s *FileStore) Load() (Snapshot, bool, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, fmt.Errorf("play: read save file %q: %w", s.path, err)
	}
	snap, err := decodeSnapshot(data)
	if err != nil {
		return Snapshot{}, false, fmt.Errorf("play: save file %q is not readable, move it away to start a new game: %w", s.path, err)
	}
	return snap, true, nil
}

// decodeSnapshot reads one snapshot of the current version, and nothing after it.
func decodeSnapshot(data []byte) (Snapshot, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var snap Snapshot
	if err := dec.Decode(&snap); err != nil {
		return Snapshot{}, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Snapshot{}, errors.New("data after the snapshot")
	}
	if snap.Version != SaveVersion {
		return Snapshot{}, fmt.Errorf("version %d, this game reads version %d", snap.Version, SaveVersion)
	}
	return snap, nil
}

// Save writes the snapshot to a temporary file next to the save, then renames it over the save, so
// that a crash or a full disk never leaves half a save.
func (s *FileStore) Save(snap Snapshot) (err error) {
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("play: encode save: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, saveDirMode); err != nil {
		return fmt.Errorf("play: create save directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("play: create temporary save: %w", err)
	}
	defer func() {
		if err != nil {
			// The save failed already: a temporary file that cannot be removed adds nothing to say.
			_ = os.Remove(tmp.Name())
		}
	}()
	if err := tmp.Chmod(saveFileMode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("play: protect temporary save: %w", err)
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("play: write temporary save: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("play: flush temporary save: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("play: close temporary save: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("play: replace save file %q: %w", s.path, err)
	}
	return nil
}
