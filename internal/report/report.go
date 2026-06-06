// Package report builds JSON and Markdown reports from filler analysis results.
// BuildJSON produces a machine-readable representation for downstream processing.
// BuildMarkdown produces an 8-section human-readable document for publication.
package report

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/maro114510/filler-cli/internal/filler"
)

// fillerEvent is the JSON representation of a single filler occurrence,
// mirroring filler.FillerEvent with camelCase JSON keys for the report schema.
type fillerEvent struct {
	DisplayName string  `json:"displayName"`
	StartMs     int     `json:"startMs"`
	EndMs       int     `json:"endMs"`
	Confidence  float64 `json:"confidence"`
}

// jsonReport is the top-level JSON structure emitted by BuildJSON.
// Field names follow the report schema defined in Issue #6.
type jsonReport struct {
	AudioFile         string         `json:"audioFile"`
	DurationSec       float64        `json:"durationSec"`
	GeneratedAt       time.Time      `json:"generatedAt"`
	TotalFillers      int            `json:"totalFillers"`
	FillersPerMinute  float64        `json:"fillersPerMinute"`
	Breakdown         map[string]int `json:"breakdown"`
	FirstFillerTimeMs int            `json:"firstFillerTimeMs"`
	FillerEvents      []fillerEvent  `json:"fillerEvents"`
	AverageConfidence float64        `json:"averageConfidence"`
}

// Markdown section heading text for all 7 ## sections of the report.
// These constants are exported so tests and callers can reference the same strings.
const (
	SectionDuration        = "Estimated Speech Duration"
	SectionTotalFillers    = "Total Fillers"
	SectionFillersPerMin   = "Fillers per Minute"
	SectionBreakdown       = "Filler Breakdown"
	SectionTimeline        = "Filler Event Timeline"
	SectionNotes           = "Notes"
	SectionFurtherAnalysis = "Further Analysis Opportunities"
)

// furtherAnalysisBody is the fixed body of the Further Analysis Opportunities section,
// listing metrics derivable from existing AmiVoice token data without extra API calls.
const furtherAnalysisBody = `The following metrics can be derived from the AmiVoice token data already returned
in this response. They are not yet implemented but require no additional API calls.

| Metric | Formula | AmiVoice Fields Used |
|--------|---------|----------------------|
| Speech Rate (WPM) | non-filler token count / (durationSec / 60) | ` + "`tokens[].written`, `tokens[].starttime`" + ` |
| Pause Detection | gaps where ` + "`starttime[n+1] - endtime[n] > threshold`" + ` | ` + "`tokens[].starttime`, `tokens[].endtime`" + ` |
| Filler Density by Segment | filler count in first/middle/last third of duration | ` + "`tokens[].starttime`" + `, duration |
| Vocabulary Diversity (TTR) | unique written tokens / total non-filler tokens | ` + "`tokens[].written`" + ` |
| Low-Confidence Token Rate | tokens with confidence < 0.7 / total tokens | ` + "`tokens[].confidence`" + ` |
`

// BuildJSON serializes filler metrics to the canonical JSON report format.
// audioPath may be a full path; only the base filename is stored in the output.
// generatedAt is embedded as-is so callers control the timestamp (useful for tests).
func BuildJSON(audioPath string, durationSec float64, m *filler.Metrics, generatedAt time.Time) ([]byte, error) {
	events := make([]fillerEvent, len(m.FillerEvents))
	for i, e := range m.FillerEvents {
		events[i] = fillerEvent{
			DisplayName: e.DisplayName,
			StartMs:     e.StartMs,
			EndMs:       e.EndMs,
			Confidence:  e.Confidence,
		}
	}

	r := jsonReport{
		AudioFile:         filepath.Base(audioPath),
		DurationSec:       durationSec,
		GeneratedAt:       generatedAt,
		TotalFillers:      m.TotalFillers,
		FillersPerMinute:  m.FillersPerMinute,
		Breakdown:         m.Breakdown,
		FirstFillerTimeMs: m.FirstFillerTimeMs,
		FillerEvents:      events,
		AverageConfidence: m.AverageConfidence,
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("report: marshal JSON: %w", err)
	}
	return data, nil
}

// BuildMarkdown produces a human-readable 8-section Markdown report.
// Sections: title (filename), duration, total fillers, fillers/min,
// filler breakdown table, event timeline, notes, further analysis opportunities.
func BuildMarkdown(audioPath string, durationSec float64, m *filler.Metrics) string {
	audioFile := filepath.Base(audioPath)
	var b strings.Builder

	// Section 1: Audio file name (# title)
	fmt.Fprintf(&b, "# Filler Analysis: %s\n\n", audioFile)

	// Section 2: Estimated Speech Duration
	fmt.Fprintf(&b, "## %s\n\n%.1f s\n\n", SectionDuration, durationSec)

	// Section 3: Total Fillers
	fmt.Fprintf(&b, "## %s\n\n%d\n\n", SectionTotalFillers, m.TotalFillers)

	// Section 4: Fillers per Minute
	fmt.Fprintf(&b, "## %s\n\n%.2f\n\n", SectionFillersPerMin, m.FillersPerMinute)

	// Section 5: Filler Breakdown
	fmt.Fprintf(&b, "## %s\n\n", SectionBreakdown)
	if len(m.Breakdown) == 0 {
		b.WriteString("_(no fillers detected)_\n\n")
	} else {
		b.WriteString("| Filler | Count |\n|--------|-------|\n")
		for name, count := range m.Breakdown {
			fmt.Fprintf(&b, "| %s | %d |\n", name, count)
		}
		b.WriteString("\n")
	}

	// Section 6: Filler Event Timeline
	fmt.Fprintf(&b, "## %s\n\n", SectionTimeline)
	if len(m.FillerEvents) == 0 {
		b.WriteString("_(no filler events detected)_\n\n")
	} else {
		b.WriteString("| Filler | Start (ms) | End (ms) | Confidence |\n|--------|-----------|---------|------------|\n")
		for _, e := range m.FillerEvents {
			fmt.Fprintf(&b, "| %s | %d | %d | %.2f |\n", e.DisplayName, e.StartMs, e.EndMs, e.Confidence)
		}
		b.WriteString("\n")
	}

	// Section 7: Notes
	fmt.Fprintf(&b, "## %s\n\n", SectionNotes)
	b.WriteString("Confidence values reflect AmiVoice ASR recognition certainty, not filler detection certainty.\n\n")

	// Section 8: Further Analysis Opportunities
	fmt.Fprintf(&b, "## %s\n\n", SectionFurtherAnalysis)
	b.WriteString(furtherAnalysisBody)

	return b.String()
}
