package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/maro114510/filler-cli/internal/keystore"
	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze <audio-file>",
	Short: "Analyze filler words in the given audio file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := loadOrPromptKey()
		if err != nil {
			return err
		}
		_ = key
		return errors.New("not yet implemented")
	},
}

func loadOrPromptKey() (string, error) {
	ks, err := keystore.New()
	if err != nil {
		return "", err
	}

	key, err := ks.Load()
	if err == nil {
		return key, nil
	}

	if errors.Is(err, keystore.ErrExpired) {
		if delErr := ks.Delete(); delErr != nil {
			return "", fmt.Errorf("failed to delete expired credentials: %w", delErr)
		}
	} else if !errors.Is(err, keystore.ErrNotFound) {
		return "", err
	}

	key, err = promptKey()
	if err != nil {
		return "", err
	}
	if err := ks.Save(key); err != nil {
		return "", fmt.Errorf("failed to save credentials: %w", err)
	}
	return key, nil
}

func promptKey() (string, error) {
	fmt.Fprint(os.Stderr, "Enter AmiVoice API key: ")
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read API key: %w", err)
	}
	key := strings.TrimSpace(line)
	if key == "" {
		return "", errors.New("API key must not be empty")
	}
	return key, nil
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
}
