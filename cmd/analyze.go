package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/maro114510/filler-cli/internal/amivoice"
	"github.com/maro114510/filler-cli/internal/keystore"
	"github.com/maro114510/filler-cli/internal/pipeline"
	"github.com/maro114510/filler-cli/internal/report"
	"github.com/spf13/cobra"
)

var (
	analyzeFormat          string
	analyzeOutput          string
	analyzeKeepFillerToken int
)

var supportedExts = map[string]bool{".wav": true, ".mp3": true}

var analyzeCmd = &cobra.Command{
	Use:   "analyze <audio-file>",
	Short: "Analyze filler words in the given audio file",
	Args:  cobra.ExactArgs(1),
	RunE:  runAnalyze,
}

func runAnalyze(_ *cobra.Command, args []string) error {
	audioPath := args[0]

	if err := validateAudioFile(audioPath); err != nil {
		return err
	}

	key, err := loadOrPromptKey()
	if err != nil {
		return err
	}

	client := amivoice.New(key)
	result, err := pipeline.Run(client, audioPath, pipeline.Options{
		KeepFillerToken: analyzeKeepFillerToken,
	})
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	output, err := buildOutput(audioPath, result)
	if err != nil {
		return err
	}

	return writeOutput(output)
}

func validateAudioFile(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	if !supportedExts[ext] {
		return fmt.Errorf("unsupported file type %q: must be .wav or .mp3 (file: %s)", ext, path)
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("file not found: %s", path)
	} else if err != nil {
		return fmt.Errorf("cannot access file %s: %w", path, err)
	}
	return nil
}

func buildOutput(audioPath string, result *pipeline.Result) (string, error) {
	switch strings.ToLower(analyzeFormat) {
	case "json":
		data, err := report.BuildJSON(audioPath, result.DurationSec, result.Metrics, time.Now())
		if err != nil {
			return "", fmt.Errorf("failed to build JSON report: %w", err)
		}
		return string(data) + "\n", nil
	case "markdown", "md", "":
		return report.BuildMarkdown(audioPath, result.DurationSec, result.Metrics), nil
	default:
		return "", fmt.Errorf("unknown format %q: must be json or markdown", analyzeFormat)
	}
}

func writeOutput(output string) error {
	if analyzeOutput == "" {
		fmt.Print(output)
		return nil
	}
	if err := os.WriteFile(analyzeOutput, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write output to %s: %w", analyzeOutput, err)
	}
	return nil
}

func loadOrPromptKey() (string, error) {
	ks := keystore.New()

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
	analyzeCmd.Flags().StringVar(&analyzeFormat, "format", "markdown", "output format: json or markdown")
	analyzeCmd.Flags().StringVar(&analyzeOutput, "output", "", "write output to file instead of stdout")
	analyzeCmd.Flags().IntVar(&analyzeKeepFillerToken, "keep-filler-token", 1, "pass keepFillerToken to AmiVoice (0 or 1)")
}
