package pipeline

import (
	"fmt"

	"github.com/maro114510/filler-cli/internal/amivoice"
	"github.com/maro114510/filler-cli/internal/filler"
)

// Sender is satisfied by *amivoice.Client.
// Defined here so that pipeline is the sole caller of Send() in production code.
type Sender interface {
	Send(audioPath string, opts amivoice.Options) (*amivoice.Response, error)
}

// Options controls pipeline behaviour.
type Options struct {
	// KeepFillerToken is forwarded to the AmiVoice API (0 or 1, default 1).
	KeepFillerToken int
}

// Result holds everything produced by the pipeline for a single audio file.
type Result struct {
	AudioFile   string
	DurationSec float64
	Metrics     *filler.Metrics
}

// Run sends the audio through AmiVoice, extracts filler metrics, and returns a Result.
// durationSec is derived from the maximum token EndTime across all results.
func Run(sender Sender, audioPath string, opts Options) (*Result, error) {
	resp, err := sender.Send(audioPath, amivoice.Options{
		GrammarFileNames: "-a-general",
		KeepFillerToken:  opts.KeepFillerToken,
	})
	if err != nil {
		return nil, fmt.Errorf("pipeline: send audio: %w", err)
	}

	durationSec := maxEndTime(resp)
	metrics := filler.Extract(resp, durationSec)

	return &Result{
		AudioFile:   audioPath,
		DurationSec: durationSec,
		Metrics:     metrics,
	}, nil
}

func maxEndTime(resp *amivoice.Response) float64 {
	var max float64
	for _, r := range resp.Results {
		for _, t := range r.Tokens {
			if t.EndTime > max {
				max = t.EndTime
			}
		}
	}
	return max
}
