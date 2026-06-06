package keystore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var (
	ErrNotFound = errors.New("credentials not found")
	ErrExpired  = errors.New("credentials expired")
)

const ttl = 2 * time.Hour

type credentials struct {
	AmiVoiceKey string    `json:"amivoice_key"`
	SavedAt     time.Time `json:"saved_at"`
}

type Store struct {
	path string
}

// New returns a Store using the default path ~/.config/filler-cli/credentials.json.
func New() *Store {
	home, _ := os.UserHomeDir()
	return &Store{path: filepath.Join(home, ".config", "filler-cli", "credentials.json")}
}

// NewWithPath returns a Store using the given path. Intended for testing.
func NewWithPath(path string) *Store {
	return &Store{path: path}
}

// Path returns the credentials file path. Exposed for testing.
func (s *Store) Path() string {
	return s.path
}

// Load returns the stored API key if it is within the TTL.
// Returns ErrNotFound if the file is absent and ErrExpired if the TTL has passed.
func (s *Store) Load() (string, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}

	var cred credentials
	if err := json.Unmarshal(data, &cred); err != nil {
		return "", err
	}

	if time.Since(cred.SavedAt) >= ttl {
		return "", ErrExpired
	}

	return cred.AmiVoiceKey, nil
}

// Save writes the key with time.Now() as saved_at and sets file mode 0600.
func (s *Store) Save(key string) error {
	return s.SaveWithTime(key, time.Now())
}

// SaveWithTime writes the key with the given timestamp. Exposed for testing TTL boundaries.
func (s *Store) SaveWithTime(key string, savedAt time.Time) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}

	cred := credentials{AmiVoiceKey: key, SavedAt: savedAt}
	data, err := json.Marshal(cred)
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0600)
}

// Delete removes the credentials file. It is a no-op if the file does not exist.
func (s *Store) Delete() error {
	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
