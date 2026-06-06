package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/maro114510/filler-cli/internal/amivoice"
	"github.com/maro114510/filler-cli/internal/keystore"
	"github.com/maro114510/filler-cli/internal/llm"
	"github.com/maro114510/filler-cli/internal/pipeline"
	"github.com/maro114510/filler-cli/internal/report"
	"github.com/spf13/cobra"
)

var (
	coachFromJSON string
	coachFormat   string
	coachOutput   string
)

var coachCmd = &cobra.Command{
	Use:   "coach [audio-file]",
	Short: "Analyze filler words and generate LLM coaching feedback",
	Long: `coach runs the static analysis pipeline then calls an LLM to generate
improvement comments, pattern analysis, and a speech quality score.

Use --from-json to skip the AmiVoice call and use a pre-computed result:
  filler-cli coach --from-json result.json

Requires an LLM API key via LLM_API_KEY environment variable or stored in the keystore.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCoach,
}

func runCoach(_ *cobra.Command, args []string) error {
	if coachFromJSON == "" && len(args) == 0 {
		return errors.New("provide an audio file or use --from-json")
	}

	var pipelineResult *pipeline.Result

	if coachFromJSON != "" {
		data, err := os.ReadFile(coachFromJSON)
		if err != nil {
			return fmt.Errorf("read --from-json file: %w", err)
		}
		parsed, err := report.ParseJSON(data)
		if err != nil {
			return fmt.Errorf("parse --from-json: %w", err)
		}
		pipelineResult = &pipeline.Result{
			AudioFile:   parsed.AudioFile,
			DurationSec: parsed.DurationSec,
			Metrics:     parsed.Metrics,
		}
	} else {
		audioPath := args[0]
		if err := validateAudioFile(audioPath); err != nil {
			return err
		}
		amiKey, err := loadOrPromptKey()
		if err != nil {
			return err
		}
		client := amivoice.New(amiKey)
		result, err := pipeline.Run(client, audioPath, pipeline.Options{KeepFillerToken: 1})
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}
		pipelineResult = result
	}

	ks, err := keystore.New()
	if err != nil {
		return fmt.Errorf("load keystore: %w", err)
	}
	llmKey, provider, err := loadOrPromptLLMKeyInternal(ks)
	if err != nil {
		if errors.Is(err, keystore.ErrExpired) {
			return fmt.Errorf("LLM API key has expired — please re-set LLM_API_KEY: %w", err)
		}
		return err
	}

	commenter, err := newCommenter(llmKey, provider)
	if err != nil {
		return err
	}

	coachResult, err := commenter.Coach(pipelineResult.Metrics)
	if err != nil {
		return fmt.Errorf("LLM coaching failed: %w", err)
	}

	audioPath := pipelineResult.AudioFile
	if len(args) > 0 {
		audioPath = args[0]
	}

	output, err := buildCoachOutput(audioPath, pipelineResult, coachResult)
	if err != nil {
		return err
	}
	return writeCoachOutput(output)
}

// loadOrPromptLLMKeyInternal resolves the LLM API key and provider.
// Priority: LLM_API_KEY env var → keystore.
// Returns an error with a message referencing the LLM key if not found.
func loadOrPromptLLMKeyInternal(ks *keystore.Store) (key, provider string, err error) {
	if k := os.Getenv("LLM_API_KEY"); k != "" {
		return k, "anthropic", nil
	}

	key, provider, err = ks.LoadLLM()
	if errors.Is(err, keystore.ErrNotFound) {
		return "", "", errors.New("LLM API key not found: set LLM_API_KEY environment variable")
	}
	if err != nil {
		return "", "", fmt.Errorf("load LLM credentials: %w", err)
	}
	return key, provider, nil
}

// newCommenter creates a Commenter for the given provider.
func newCommenter(key, provider string) (llm.Commenter, error) {
	switch strings.ToLower(provider) {
	case "anthropic":
		return llm.NewAnthropicCommenter(key), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider %q: supported providers are: anthropic", provider)
	}
}

func buildCoachOutput(audioPath string, result *pipeline.Result, cr *llm.CoachResult) (string, error) {
	cd := &report.CoachData{
		ImprovementComments: cr.ImprovementComments,
		PatternAnalysis:     cr.PatternAnalysis,
		QualityScore:        cr.QualityScore,
		ScoreDelta:          cr.ScoreDelta,
	}

	switch strings.ToLower(coachFormat) {
	case "json":
		data, err := report.BuildCoachJSON(audioPath, result.DurationSec, result.Metrics, time.Now(), cd)
		if err != nil {
			return "", fmt.Errorf("failed to build coach JSON report: %w", err)
		}
		return string(data) + "\n", nil
	case "markdown", "md", "":
		return report.BuildCoachMarkdown(audioPath, result.DurationSec, result.Metrics, cd), nil
	default:
		return "", fmt.Errorf("unknown format %q: must be json or markdown", coachFormat)
	}
}

func writeCoachOutput(output string) error {
	if coachOutput == "" {
		fmt.Print(output)
		return nil
	}
	path := filepath.Clean(coachOutput)
	if err := os.WriteFile(path, []byte(output), 0600); err != nil {
		return fmt.Errorf("failed to write output to %s: %w", path, err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(coachCmd)
	coachCmd.Flags().StringVar(&coachFromJSON, "from-json", "", "use pre-computed result JSON instead of calling AmiVoice")
	coachCmd.Flags().StringVar(&coachFormat, "format", "markdown", "output format: json or markdown")
	coachCmd.Flags().StringVar(&coachOutput, "output", "", "write output to file instead of stdout")
}
