package filler

import (
	"math"
	"regexp"
	"strings"

	"github.com/maro114510/filler-cli/internal/amivoice"
)

// fillerPattern matches tokens like %えーと% but not bare %.
var fillerPattern = regexp.MustCompile(`^%[^%]+%$`)

const (
	msPerSecond   = 1000.0 // seconds → milliseconds
	secsPerMinute = 60.0   // seconds → minutes
)

type FillerEvent struct {
	DisplayName string
	StartMs     int
	EndMs       int
	Confidence  float64
}

type Metrics struct {
	TotalFillers      int
	FillersPerMinute  float64
	Breakdown         map[string]int
	FirstFillerTimeMs int
	FillerEvents      []FillerEvent
	AverageConfidence float64
}

// Extract traverses all tokens in response, identifies fillers, and computes metrics.
// durationSec is the audio duration in seconds; passing 0 yields +Inf for FillersPerMinute.
func Extract(response *amivoice.Response, durationSec float64) *Metrics {
	m := &Metrics{
		FirstFillerTimeMs: -1,
		Breakdown:         make(map[string]int),
		FillerEvents:      []FillerEvent{},
	}

	var totalConfidence float64

	for _, result := range response.Results {
		for _, token := range result.Tokens {
			if !fillerPattern.MatchString(token.Written) {
				continue
			}

			displayName := token.Spoken
			if displayName == "" {
				displayName = strings.Trim(token.Written, "%")
			}

			startMs := int(math.Round(token.StartTime * msPerSecond))
			endMs := int(math.Round(token.EndTime * msPerSecond))

			if m.TotalFillers == 0 {
				m.FirstFillerTimeMs = startMs
			}

			m.FillerEvents = append(m.FillerEvents, FillerEvent{
				DisplayName: displayName,
				StartMs:     startMs,
				EndMs:       endMs,
				Confidence:  token.Confidence,
			})
			m.Breakdown[displayName]++
			totalConfidence += token.Confidence
			m.TotalFillers++
		}
	}

	m.FillersPerMinute = float64(m.TotalFillers) / (durationSec / secsPerMinute)

	if m.TotalFillers > 0 {
		m.AverageConfidence = totalConfidence / float64(m.TotalFillers)
	}

	return m
}
