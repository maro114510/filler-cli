package report_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/maro114510/filler-cli/internal/filler"
	"github.com/maro114510/filler-cli/internal/report"
)

var fixedTime = time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)

func makeMetrics(events []filler.FillerEvent, breakdown map[string]int) *filler.Metrics {
	total := 0
	var totalConf float64
	firstMs := -1
	for _, e := range events {
		total++
		totalConf += e.Confidence
		if firstMs == -1 {
			firstMs = e.StartMs
		}
	}
	avgConf := 0.0
	if total > 0 {
		avgConf = totalConf / float64(total)
	}
	fpm := 0.0
	if total > 0 {
		fpm = float64(total) // 1-minute default audio: total/1min = total
	}
	return &filler.Metrics{
		TotalFillers:      total,
		FillersPerMinute:  fpm,
		Breakdown:         breakdown,
		FirstFillerTimeMs: firstMs,
		FillerEvents:      events,
		AverageConfidence: avgConf,
	}
}

// extractHeadings returns the text of all ## headings from a Markdown string, in order.
func extractHeadings(md string) []string {
	var headings []string
	for _, line := range strings.Split(md, "\n") {
		if heading, ok := strings.CutPrefix(line, "## "); ok {
			headings = append(headings, heading)
		}
	}
	return headings
}

// BuildJSON tests

func TestBuildJSON_AllFields(t *testing.T) {
	m := makeMetrics(
		[]filler.FillerEvent{
			{DisplayName: "えーと", StartMs: 1500, EndMs: 2000, Confidence: 0.9},
		},
		map[string]int{"えーと": 1},
	)
	data, err := report.BuildJSON("/path/to/sample.wav", 60.0, m, fixedTime)
	if err != nil {
		t.Fatalf("BuildJSON error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	requiredFields := []string{
		"audioFile", "durationSec", "generatedAt",
		"totalFillers", "fillersPerMinute", "breakdown",
		"firstFillerTimeMs", "fillerEvents", "averageConfidence",
	}
	var missing []string
	for _, f := range requiredFields {
		if _, ok := out[f]; !ok {
			missing = append(missing, f)
		}
	}
	if diff := cmp.Diff([]string(nil), missing); diff != "" {
		t.Errorf("missing JSON fields (-want none +got missing):\n%s", diff)
	}
}

func TestBuildJSON_AudioFileIsBasename(t *testing.T) {
	m := makeMetrics(nil, map[string]int{})
	data, err := report.BuildJSON("/path/to/sample.wav", 0.0, m, fixedTime)
	if err != nil {
		t.Fatalf("BuildJSON error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if diff := cmp.Diff("sample.wav", out["audioFile"]); diff != "" {
		t.Errorf("audioFile mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildJSON_GeneratedAtRFC3339(t *testing.T) {
	m := makeMetrics(nil, map[string]int{})
	data, err := report.BuildJSON("sample.wav", 0.0, m, fixedTime)
	if err != nil {
		t.Fatalf("BuildJSON error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	ts, ok := out["generatedAt"].(string)
	if !ok {
		t.Fatal("generatedAt is not a string")
	}
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("generatedAt is not RFC3339: %s", ts)
	}
	if diff := cmp.Diff("2026-06-06T12:00:00Z", ts); diff != "" {
		t.Errorf("generatedAt mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildJSON_FillerEventsPresent(t *testing.T) {
	wantEvents := []filler.FillerEvent{
		{DisplayName: "えーと", StartMs: 1500, EndMs: 2000, Confidence: 0.9},
		{DisplayName: "あー", StartMs: 3000, EndMs: 3500, Confidence: 0.8},
	}
	m := makeMetrics(wantEvents, map[string]int{"えーと": 1, "あー": 1})
	data, err := report.BuildJSON("sample.wav", 60.0, m, fixedTime)
	if err != nil {
		t.Fatalf("BuildJSON error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	events, ok := out["fillerEvents"].([]any)
	if !ok {
		t.Fatal("fillerEvents is not an array")
	}
	if diff := cmp.Diff(len(wantEvents), len(events)); diff != "" {
		t.Errorf("fillerEvents length mismatch (-want +got):\n%s", diff)
	}
}

// BuildMarkdown tests

func TestBuildMarkdown_Has8Sections(t *testing.T) {
	m := makeMetrics(nil, map[string]int{})
	md := report.BuildMarkdown("sample.wav", 60.0, m)

	// Section 1: title must contain the audio filename.
	if !strings.Contains(md, "sample.wav") {
		t.Errorf("title missing audio filename")
	}

	// Sections 2–8: verify the 7 ## headings in order.
	want := []string{
		report.SectionDuration,
		report.SectionTotalFillers,
		report.SectionFillersPerMin,
		report.SectionBreakdown,
		report.SectionTimeline,
		report.SectionNotes,
		report.SectionFurtherAnalysis,
	}
	got := extractHeadings(md)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("section headings mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildMarkdown_AudioFileInTitle(t *testing.T) {
	m := makeMetrics(nil, map[string]int{})
	md := report.BuildMarkdown("/path/to/my_speech.wav", 60.0, m)
	if !strings.Contains(md, "my_speech.wav") {
		t.Errorf("audio filename not found in markdown output")
	}
}

func TestBuildMarkdown_BreakdownTableContainsFillers(t *testing.T) {
	m := makeMetrics(
		[]filler.FillerEvent{
			{DisplayName: "えーと", StartMs: 1000, EndMs: 1500, Confidence: 0.9},
			{DisplayName: "あー", StartMs: 2000, EndMs: 2500, Confidence: 0.8},
		},
		map[string]int{"えーと": 1, "あー": 1},
	)
	md := report.BuildMarkdown("sample.wav", 60.0, m)

	wantEntries := []string{"えーと", "あー"}
	var missing []string
	for _, entry := range wantEntries {
		if !strings.Contains(md, entry) {
			missing = append(missing, entry)
		}
	}
	if diff := cmp.Diff([]string(nil), missing); diff != "" {
		t.Errorf("breakdown table missing entries (-want none +got missing):\n%s", diff)
	}
}

func TestBuildMarkdown_TimelineContainsEvents(t *testing.T) {
	m := makeMetrics(
		[]filler.FillerEvent{
			{DisplayName: "えーと", StartMs: 1500, EndMs: 2000, Confidence: 0.9},
		},
		map[string]int{"えーと": 1},
	)
	md := report.BuildMarkdown("sample.wav", 60.0, m)

	wantTokens := []string{"1500", "2000"}
	var missing []string
	for _, tok := range wantTokens {
		if !strings.Contains(md, tok) {
			missing = append(missing, tok)
		}
	}
	if diff := cmp.Diff([]string(nil), missing); diff != "" {
		t.Errorf("timeline missing timestamps (-want none +got missing):\n%s", diff)
	}
}

func TestBuildMarkdown_FurtherAnalysisHas5Rows(t *testing.T) {
	m := makeMetrics(nil, map[string]int{})
	md := report.BuildMarkdown("sample.wav", 60.0, m)

	wantMetrics := []string{
		"Speech Rate",
		"Pause Detection",
		"Filler Density",
		"Vocabulary Diversity",
		"Low-Confidence Token Rate",
	}
	var missing []string
	for _, metric := range wantMetrics {
		if !strings.Contains(md, metric) {
			missing = append(missing, metric)
		}
	}
	if diff := cmp.Diff([]string(nil), missing); diff != "" {
		t.Errorf("Further Analysis missing rows (-want none +got missing):\n%s", diff)
	}
}

func TestBuildMarkdown_NoFillers_SectionsPresent(t *testing.T) {
	m := &filler.Metrics{
		TotalFillers:      0,
		FillersPerMinute:  0.0,
		Breakdown:         map[string]int{},
		FirstFillerTimeMs: -1,
		FillerEvents:      []filler.FillerEvent{},
		AverageConfidence: 0.0,
	}
	md := report.BuildMarkdown("sample.wav", 0.0, m)

	// All 7 ## headings must still appear when there are no fillers.
	want := []string{
		report.SectionDuration,
		report.SectionTotalFillers,
		report.SectionFillersPerMin,
		report.SectionBreakdown,
		report.SectionTimeline,
		report.SectionNotes,
		report.SectionFurtherAnalysis,
	}
	got := extractHeadings(md)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("section headings mismatch with no fillers (-want +got):\n%s", diff)
	}
}

// ParseJSON tests

func TestParseJSON_RoundTrip(t *testing.T) {
	events := []filler.FillerEvent{
		{DisplayName: "えーと", StartMs: 1500, EndMs: 2000, Confidence: 0.9},
		{DisplayName: "あー", StartMs: 3000, EndMs: 3500, Confidence: 0.8},
	}
	m := makeMetrics(events, map[string]int{"えーと": 1, "あー": 1})
	const audioPath = "/path/to/test.wav"
	const durationSec = 60.0

	data, err := report.BuildJSON(audioPath, durationSec, m, fixedTime)
	if err != nil {
		t.Fatalf("BuildJSON: %v", err)
	}

	parsed, err := report.ParseJSON(data)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	if parsed.AudioFile != "test.wav" {
		t.Errorf("AudioFile = %q, want %q", parsed.AudioFile, "test.wav")
	}
	if parsed.DurationSec != durationSec {
		t.Errorf("DurationSec = %v, want %v", parsed.DurationSec, durationSec)
	}
	if parsed.Metrics.TotalFillers != m.TotalFillers {
		t.Errorf("TotalFillers = %d, want %d", parsed.Metrics.TotalFillers, m.TotalFillers)
	}
	if diff := cmp.Diff(m.Breakdown, parsed.Metrics.Breakdown); diff != "" {
		t.Errorf("Breakdown mismatch (-want +got):\n%s", diff)
	}
	if len(parsed.Metrics.FillerEvents) != len(m.FillerEvents) {
		t.Errorf("FillerEvents len = %d, want %d", len(parsed.Metrics.FillerEvents), len(m.FillerEvents))
	}
}

func TestParseJSON_EmptyFillerEvents(t *testing.T) {
	m := &filler.Metrics{
		TotalFillers:      0,
		FillersPerMinute:  0.0,
		Breakdown:         map[string]int{},
		FirstFillerTimeMs: -1,
		FillerEvents:      []filler.FillerEvent{},
	}
	data, err := report.BuildJSON("audio.wav", 30.0, m, fixedTime)
	if err != nil {
		t.Fatalf("BuildJSON: %v", err)
	}
	parsed, err := report.ParseJSON(data)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if parsed.Metrics.TotalFillers != 0 {
		t.Errorf("TotalFillers = %d, want 0", parsed.Metrics.TotalFillers)
	}
}

func TestParseJSON_InvalidJSON(t *testing.T) {
	_, err := report.ParseJSON([]byte("not-json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// BuildCoachMarkdown / BuildCoachJSON tests

func TestBuildCoachMarkdown_ContainsAllSections(t *testing.T) {
	m := makeMetrics(
		[]filler.FillerEvent{{DisplayName: "えーと", StartMs: 1000, EndMs: 1500, Confidence: 0.9}},
		map[string]int{"えーと": 1},
	)
	coach := &report.CoachData{
		ImprovementComments: "発話の冒頭でのフィラー使用を減らしましょう。",
		PatternAnalysis:     "フィラーはスピーチの最初の30%に集中しています。",
		QualityScore:        72,
		ScoreDelta:          nil,
	}
	md := report.BuildCoachMarkdown("sample.wav", 60.0, m, coach)

	wantSections := []string{
		report.SectionImprovementComments,
		report.SectionPatternAnalysis,
		report.SectionQualityScore,
	}
	for _, sec := range wantSections {
		if !strings.Contains(md, sec) {
			t.Errorf("coach markdown missing section: %q", sec)
		}
	}
	if !strings.Contains(md, "72") {
		t.Errorf("coach markdown should contain quality score 72")
	}
}

func TestBuildCoachJSON_ContainsCoachResult(t *testing.T) {
	m := makeMetrics(nil, map[string]int{})
	coach := &report.CoachData{
		ImprovementComments: "Good job.",
		PatternAnalysis:     "No pattern detected.",
		QualityScore:        85,
		ScoreDelta:          nil,
	}
	data, err := report.BuildCoachJSON("sample.wav", 60.0, m, fixedTime, coach)
	if err != nil {
		t.Fatalf("BuildCoachJSON: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Metrics fields must be present at top level (AC-3 compatibility).
	for _, field := range []string{"totalFillers", "fillersPerMinute", "breakdown"} {
		if _, ok := out[field]; !ok {
			t.Errorf("missing top-level field: %q", field)
		}
	}

	// Coach result must be nested under coachResult.
	cr, ok := out["coachResult"].(map[string]any)
	if !ok {
		t.Fatal("coachResult field missing or wrong type")
	}
	if cr["improvementComments"] != "Good job." {
		t.Errorf("improvementComments = %v, want %q", cr["improvementComments"], "Good job.")
	}
	if cr["qualityScore"] != float64(85) {
		t.Errorf("qualityScore = %v, want 85", cr["qualityScore"])
	}
}
