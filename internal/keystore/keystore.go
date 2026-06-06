package keystore

import (
	"encoding/json"
	"errors"
	"fmt"
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
	LLMKey      string    `json:"llm_key,omitempty"`
	LLMProvider string    `json:"llm_provider,omitempty"`
	SavedAt     time.Time `json:"saved_at"`
}

type Store struct {
	path string
}

// New returns a Store using the default path ~/.config/filler-cli/credentials.json.
func New() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("keystore: resolve home directory: %w", err)
	}
	return &Store{path: filepath.Join(home, ".config", "filler-cli", "credentials.json")}, nil
}

// NewWithPath returns a Store using the given path. Intended for testing.
func NewWithPath(path string) *Store {
	return &Store{path: path}
}

// Path returns the credentials file path. Exposed for testing.
func (s *Store) Path() string {
	return s.path
}

// Load returns the stored AmiVoice API key if it is within the TTL.
// Returns ErrNotFound if the file is absent and ErrExpired if the TTL has passed.
func (s *Store) Load() (string, error) {
	cred, err := s.read()
	if err != nil {
		return "", err
	}
	return cred.AmiVoiceKey, nil
}

// LoadLLM returns the stored LLM key and provider if present and within the TTL.
// Returns ErrNotFound if the file is absent or no LLM key is stored.
// Returns ErrExpired if the TTL has passed.
func (s *Store) LoadLLM() (key, provider string, err error) {
	cred, err := s.read()
	if err != nil {
		return "", "", err
	}
	if cred.LLMKey == "" {
		return "", "", ErrNotFound
	}
	return cred.LLMKey, cred.LLMProvider, nil
}

// Save writes the AmiVoice key with time.Now() as saved_at and sets file mode 0600.
// Preserves any existing LLM key and provider stored in the file.
func (s *Store) Save(key string) error {
	return s.SaveWithTime(key, time.Now())
}

// SaveWithTime writes the AmiVoice key with the given timestamp.
// Preserves any existing LLM key and provider. Exposed for testing TTL boundaries.
func (s *Store) SaveWithTime(key string, savedAt time.Time) error {
	cred := s.readExisting()
	cred.AmiVoiceKey = key
	cred.SavedAt = savedAt
	return s.write(cred)
}

// SaveLLM writes the LLM key and provider, preserves the AmiVoice key, and resets the shared TTL.
func (s *Store) SaveLLM(key, provider string) error {
	return s.SaveLLMWithTime(key, provider, time.Now())
}

// SaveLLMWithTime is the testable version of SaveLLM.
func (s *Store) SaveLLMWithTime(key, provider string, savedAt time.Time) error {
	cred := s.readExisting()
	cred.LLMKey = key
	cred.LLMProvider = provider
	cred.SavedAt = savedAt
	return s.write(cred)
}

// Delete removes the credentials file. It is a no-op if the file does not exist.
func (s *Store) Delete() error {
	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("keystore: remove credentials: %w", err)
	}
	return nil
}

// read reads the credentials file, checks TTL, and returns the result.
func (s *Store) read() (*credentials, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("keystore: read credentials: %w", err)
	}

	var cred credentials
	if err := json.Unmarshal(data, &cred); err != nil {
		return nil, fmt.Errorf("keystore: unmarshal credentials: %w", err)
	}

	if time.Since(cred.SavedAt) >= ttl {
		return nil, ErrExpired
	}

	return &cred, nil
}

// readExisting reads the current credentials from disk without TTL check.
// Returns empty credentials if the file is absent or unreadable.
func (s *Store) readExisting() credentials {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return credentials{}
	}
	var c credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return credentials{}
	}
	return c
}

// write serializes and writes credentials to disk with mode 0600.
func (s *Store) write(cred credentials) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("keystore: create directory: %w", err)
	}
	data, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("keystore: marshal credentials: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("keystore: write credentials: %w", err)
	}
	return nil
}
