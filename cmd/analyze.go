package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/maro114510/filler-cli/internal/amivoice"
	"github.com/maro114510/filler-cli/internal/keystore"
	"github.com/maro114510/filler-cli/internal/pipeline"
	"github.com/maro114510/filler-cli/internal/report"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	if err := os.WriteFile(analyzeOutput, []byte(output), 0600); err != nil {
		return fmt.Errorf("failed to write output to %s: %w", analyzeOutput, err)
	}
	return nil
}

type oneTimeKeyIssuer func(serviceID, servicePassword string, opts amivoice.OneTimeKeyOptions) (string, error)

func loadOrPromptKey() (string, error) {
	ks, err := keystore.New()
	if err != nil {
		return "", fmt.Errorf("load keystore: %w", err)
	}
	return loadOrPromptKeyInternal(ks, amivoice.IssueOneTimeKey)
}

func loadOrPromptKeyInternal(ks *keystore.Store, issuer oneTimeKeyIssuer) (string, error) {
	if key := os.Getenv("AMIVOICE_API_KEY"); key != "" {
		return key, nil
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
		return "", fmt.Errorf("load credentials: %w", err)
	}

	sid := os.Getenv("AMIVOICE_SERVICE_ID")
	spw := os.Getenv("AMIVOICE_SERVICE_PASSWORD")
	if sid != "" || spw != "" {
		if sid == "" {
			return "", errors.New("AMIVOICE_SERVICE_PASSWORD is set but AMIVOICE_SERVICE_ID is missing")
		}
		if spw == "" {
			return "", errors.New("AMIVOICE_SERVICE_ID is set but AMIVOICE_SERVICE_PASSWORD is missing")
		}
		key, err = issuer(sid, spw, amivoice.OneTimeKeyOptions{ValidFor: 2 * time.Hour})
		if err != nil {
			return "", fmt.Errorf("issue one-time key: %w", err)
		}
		if err := ks.Save(key); err != nil {
			return "", fmt.Errorf("failed to save credentials: %w", err)
		}
		return key, nil
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
	key, err := readMasked(os.Stderr, os.Stdin)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("failed to read API key: %w", err)
	}
	if key == "" {
		return "", errors.New("API key must not be empty")
	}
	return key, nil
}

// readMasked reads a line from in, printing '*' for each character to out.
// When in is not a terminal (e.g. piped input), it falls back to plain line reading.
func readMasked(out io.Writer, in *os.File) (string, error) {
	fd := int(in.Fd())
	if !term.IsTerminal(fd) {
		r := bufio.NewReader(in)
		line, err := r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	old, err := term.MakeRaw(fd)
	if err != nil {
		return "", fmt.Errorf("set raw mode: %w", err)
	}
	defer term.Restore(fd, old) //nolint:errcheck

	var buf []byte
	b := make([]byte, 1)
	for {
		if _, err := in.Read(b); err != nil {
			if errors.Is(err, io.EOF) && len(buf) > 0 {
				return string(buf), nil
			}
			return "", err
		}
		switch b[0] {
		case '\r', '\n':
			return string(buf), nil
		case 3: // Ctrl+C
			return "", errors.New("interrupted")
		case 4: // Ctrl+D
			if len(buf) == 0 {
				return "", io.EOF
			}
			return string(buf), nil
		case 127, '\b': // DEL / Backspace
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				fmt.Fprint(out, "\b \b")
			}
		default:
			if b[0] >= 32 {
				buf = append(buf, b[0])
				fmt.Fprint(out, "*")
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.Flags().StringVar(&analyzeFormat, "format", "markdown", "output format: json or markdown")
	analyzeCmd.Flags().StringVar(&analyzeOutput, "output", "", "write output to file instead of stdout")
	analyzeCmd.Flags().IntVar(&analyzeKeepFillerToken, "keep-filler-token", 1, "pass keepFillerToken to AmiVoice (0 or 1)")
}
