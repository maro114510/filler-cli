package filler_test

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/maro114510/filler-cli/internal/amivoice"
	"github.com/maro114510/filler-cli/internal/filler"
)

func makeResponse(tokens ...amivoice.Token) *amivoice.Response {
	return &amivoice.Response{
		Results: []amivoice.Result{
			{Tokens: tokens},
		},
	}
}

// AC-1: %えー% with spoken="" → filler, DisplayName="えー" (strip % from written)
func TestExtract_FillerDisplayNameFromWritten(t *testing.T) {
	resp := makeResponse(amivoice.Token{
		Written: "%えー%", Spoken: "", Confidence: 0.9,
	})
	m := filler.Extract(resp, 60.0)
	if m.TotalFillers != 1 {
		t.Fatalf("TotalFillers: got %d, want 1", m.TotalFillers)
	}
	want := filler.FillerEvent{DisplayName: "えー", StartMs: 0, EndMs: 0, Confidence: 0.9}
	if diff := cmp.Diff(want, m.FillerEvents[0]); diff != "" {
		t.Errorf("FillerEvent mismatch (-want +got):\n%s", diff)
	}
}

// AC-1 variant: spoken non-empty → use spoken as DisplayName
func TestExtract_FillerDisplayNameFromSpoken(t *testing.T) {
	resp := makeResponse(amivoice.Token{
		Written: "%えーと%", Spoken: "えーと", Confidence: 0.9,
	})
	m := filler.Extract(resp, 60.0)
	if m.TotalFillers != 1 {
		t.Fatalf("TotalFillers: got %d, want 1", m.TotalFillers)
	}
	want := filler.FillerEvent{DisplayName: "えーと", StartMs: 0, EndMs: 0, Confidence: 0.9}
	if diff := cmp.Diff(want, m.FillerEvents[0]); diff != "" {
		t.Errorf("FillerEvent mismatch (-want +got):\n%s", diff)
	}
}

// AC-2: written="%" → NOT a filler (bare % is not matched by ^%[^%]+%$)
func TestExtract_BarePercentNotFiller(t *testing.T) {
	resp := makeResponse(amivoice.Token{Written: "%"})
	m := filler.Extract(resp, 60.0)
	if m.TotalFillers != 0 {
		t.Errorf("bare %% must not be a filler, TotalFillers: %d", m.TotalFillers)
	}
	if len(m.FillerEvents) != 0 {
		t.Errorf("FillerEvents must be empty for bare %%, got: %v", m.FillerEvents)
	}
}

// AC-3: regular token → NOT a filler
func TestExtract_RegularTokenNotFiller(t *testing.T) {
	resp := makeResponse(amivoice.Token{Written: "こんにちは"})
	m := filler.Extract(resp, 60.0)
	if m.TotalFillers != 0 {
		t.Errorf("regular token must not be a filler, TotalFillers: %d", m.TotalFillers)
	}
}

// AC-4: FillersPerMinute correct — 6 fillers in 60s → 6.0
func TestExtract_FillersPerMinute(t *testing.T) {
	tokens := make([]amivoice.Token, 6)
	for i := range tokens {
		tokens[i] = amivoice.Token{Written: "%えー%", Spoken: "えー", Confidence: 0.9}
	}
	resp := makeResponse(tokens...)
	m := filler.Extract(resp, 60.0)
	if math.Abs(m.FillersPerMinute-6.0) > 1e-9 {
		t.Errorf("FillersPerMinute: got %f, want 6.0", m.FillersPerMinute)
	}
}

// AC-5: AverageConfidence correct — (0.8 + 1.0) / 2 = 0.9
func TestExtract_AverageConfidence(t *testing.T) {
	resp := makeResponse(
		amivoice.Token{Written: "%えー%", Spoken: "えー", Confidence: 0.8},
		amivoice.Token{Written: "%あー%", Spoken: "あー", Confidence: 1.0},
	)
	m := filler.Extract(resp, 60.0)
	if math.Abs(m.AverageConfidence-0.9) > 1e-9 {
		t.Errorf("AverageConfidence: got %f, want 0.9", m.AverageConfidence)
	}
}

// No fillers → FirstFillerTimeMs=-1, AverageConfidence=0.0, FillerEvents empty
func TestExtract_NoFillers(t *testing.T) {
	resp := makeResponse(amivoice.Token{Written: "こんにちは"})
	m := filler.Extract(resp, 60.0)
	if m.FirstFillerTimeMs != -1 {
		t.Errorf("FirstFillerTimeMs: got %d, want -1", m.FirstFillerTimeMs)
	}
	if m.AverageConfidence != 0.0 {
		t.Errorf("AverageConfidence: got %f, want 0.0", m.AverageConfidence)
	}
	if len(m.FillerEvents) != 0 {
		t.Errorf("FillerEvents: expected empty, got %v", m.FillerEvents)
	}
}

// FirstFillerTimeMs is set to the StartMs of the first filler in traversal order
func TestExtract_FirstFillerTimeMs(t *testing.T) {
	resp := makeResponse(
		amivoice.Token{Written: "こんにちは"},
		amivoice.Token{Written: "%えー%", Spoken: "えー", Confidence: 0.9, StartTime: 1.5, EndTime: 2.0},
		amivoice.Token{Written: "%あー%", Spoken: "あー", Confidence: 0.8, StartTime: 3.0, EndTime: 3.5},
	)
	m := filler.Extract(resp, 60.0)
	if m.FirstFillerTimeMs != 1500 {
		t.Errorf("FirstFillerTimeMs: got %d, want 1500", m.FirstFillerTimeMs)
	}
}

// durationSec=0 with fillers → FillersPerMinute=+Inf
func TestExtract_ZeroDuration(t *testing.T) {
	resp := makeResponse(amivoice.Token{Written: "%えー%", Spoken: "えー", Confidence: 0.9})
	m := filler.Extract(resp, 0.0)
	if !math.IsInf(m.FillersPerMinute, 1) {
		t.Errorf("FillersPerMinute with durationSec=0: got %v, want +Inf", m.FillersPerMinute)
	}
}

// Breakdown counts each display name correctly
func TestExtract_Breakdown(t *testing.T) {
	resp := makeResponse(
		amivoice.Token{Written: "%えー%", Spoken: "えー", Confidence: 0.9},
		amivoice.Token{Written: "%えー%", Spoken: "えー", Confidence: 0.8},
		amivoice.Token{Written: "%あー%", Spoken: "あー", Confidence: 0.7},
	)
	m := filler.Extract(resp, 60.0)
	wantBreakdown := map[string]int{"えー": 2, "あー": 1}
	if diff := cmp.Diff(wantBreakdown, m.Breakdown); diff != "" {
		t.Errorf("Breakdown mismatch (-want +got):\n%s", diff)
	}
}

// FillerEvents traversal order across multiple results
func TestExtract_MultipleResults(t *testing.T) {
	resp := &amivoice.Response{
		Results: []amivoice.Result{
			{Tokens: []amivoice.Token{
				{Written: "%えー%", Spoken: "えー", Confidence: 0.9, StartTime: 0.5, EndTime: 1.0},
			}},
			{Tokens: []amivoice.Token{
				{Written: "%あー%", Spoken: "あー", Confidence: 0.8, StartTime: 2.0, EndTime: 2.5},
			}},
		},
	}
	m := filler.Extract(resp, 120.0)
	wantEvents := []filler.FillerEvent{
		{DisplayName: "えー", StartMs: 500, EndMs: 1000, Confidence: 0.9},
		{DisplayName: "あー", StartMs: 2000, EndMs: 2500, Confidence: 0.8},
	}
	if diff := cmp.Diff(wantEvents, m.FillerEvents); diff != "" {
		t.Errorf("FillerEvents mismatch (-want +got):\n%s", diff)
	}
}

// math.Round for ms conversion: startTime=0.8 (float64) must round to 800ms, not truncate to 799ms
func TestExtract_TimeRounding(t *testing.T) {
	resp := makeResponse(amivoice.Token{
		Written: "%えー%", Spoken: "えー", Confidence: 0.9,
		StartTime: 0.8,
		EndTime:   1.6,
	})
	m := filler.Extract(resp, 60.0)
	want := filler.FillerEvent{DisplayName: "えー", StartMs: 800, EndMs: 1600, Confidence: 0.9}
	if diff := cmp.Diff(want, m.FillerEvents[0]); diff != "" {
		t.Errorf("FillerEvent mismatch (-want +got):\n%s", diff)
	}
}

// Mixed tokens: only %...% pattern extracted, others ignored
func TestExtract_MixedTokens(t *testing.T) {
	resp := makeResponse(
		amivoice.Token{Written: "%えーと%", Spoken: "えーと", Confidence: 0.95, StartTime: 0.1, EndTime: 0.8},
		amivoice.Token{Written: "今日は", Spoken: "きょうは", Confidence: 0.99, StartTime: 0.9, EndTime: 1.5},
		amivoice.Token{Written: "%", Confidence: 0.5, StartTime: 1.6, EndTime: 1.7},
	)
	m := filler.Extract(resp, 60.0)
	wantEvents := []filler.FillerEvent{
		{DisplayName: "えーと", StartMs: 100, EndMs: 800, Confidence: 0.95},
	}
	if diff := cmp.Diff(wantEvents, m.FillerEvents); diff != "" {
		t.Errorf("FillerEvents mismatch (-want +got):\n%s", diff)
	}
}
