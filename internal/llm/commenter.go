// Package llm provides LLM-powered coaching on filler analysis results.
package llm

import "github.com/maro114510/filler-cli/internal/filler"

// CoachResult holds the LLM coaching output for a single analysis.
type CoachResult struct {
	ImprovementComments string
	PatternAnalysis     string
	QualityScore        int
	ScoreDelta          *int // nil — no prior-run tracking in this implementation
}

// Commenter produces coaching feedback from filler metrics.
type Commenter interface {
	Coach(metrics *filler.Metrics) (*CoachResult, error)
}
