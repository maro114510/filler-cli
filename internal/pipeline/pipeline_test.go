package pipeline_test

import (
	"errors"
	"testing"

	"github.com/maro114510/filler-cli/internal/amivoice"
	"github.com/maro114510/filler-cli/internal/pipeline"
)

// stubSender implements pipeline.Sender for testing.
type stubSender struct {
	resp *amivoice.Response
	err  error
}

func (s *stubSender) Send(_ string, _ amivoice.Options) (*amivoice.Response, error) {
	return s.resp, s.err
}

func makeResponse(tokens ...amivoice.Token) *amivoice.Response {
	return &amivoice.Response{
		Results: []amivoice.Result{{Tokens: tokens}},
	}
}

// Run returns error when sender returns error.
func TestRun_SenderError(t *testing.T) {
	s := &stubSender{err: errors.New("api error")}
	_, err := pipeline.Run(s, "sample.wav", pipeline.Options{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Run with no tokens → durationSec = 0.0, TotalFillers = 0.
func TestRun_NoTokens(t *testing.T) {
	s := &stubSender{resp: makeResponse()}
	result, err := pipeline.Run(s, "sample.wav", pipeline.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DurationSec != 0.0 {
		t.Errorf("DurationSec: got %f, want 0.0", result.DurationSec)
	}
	if result.Metrics.TotalFillers != 0 {
		t.Errorf("TotalFillers: got %d, want 0", result.Metrics.TotalFillers)
	}
}

// DurationSec is the max EndTime across all tokens.
func TestRun_DurationSecFromMaxEndTime(t *testing.T) {
	s := &stubSender{resp: makeResponse(
		amivoice.Token{Written: "こんにちは", StartTime: 0.0, EndTime: 2.5},
		amivoice.Token{Written: "%えーと%", Spoken: "えーと", Confidence: 0.9, StartTime: 3.0, EndTime: 4.0},
		amivoice.Token{Written: "今日は", StartTime: 4.5, EndTime: 6.0},
	)}
	result, err := pipeline.Run(s, "sample.wav", pipeline.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DurationSec != 6.0 {
		t.Errorf("DurationSec: got %f, want 6.0", result.DurationSec)
	}
}

// Metrics are populated correctly from the response.
func TestRun_MetricsPopulated(t *testing.T) {
	s := &stubSender{resp: makeResponse(
		amivoice.Token{Written: "%えーと%", Spoken: "えーと", Confidence: 0.9, StartTime: 1.0, EndTime: 1.5},
		amivoice.Token{Written: "%あー%", Spoken: "あー", Confidence: 0.8, StartTime: 2.0, EndTime: 2.5},
	)}
	result, err := pipeline.Run(s, "sample.wav", pipeline.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Metrics.TotalFillers != 2 {
		t.Errorf("TotalFillers: got %d, want 2", result.Metrics.TotalFillers)
	}
	if result.AudioFile != "sample.wav" {
		t.Errorf("AudioFile: got %s, want sample.wav", result.AudioFile)
	}
}

// KeepFillerToken option is forwarded to the sender.
func TestRun_KeepFillerTokenForwarded(t *testing.T) {
	var captured amivoice.Options
	s := &captureOptsSender{resp: makeResponse()}
	s.capture = &captured
	_, err := pipeline.Run(s, "sample.wav", pipeline.Options{KeepFillerToken: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.KeepFillerToken != 1 {
		t.Errorf("KeepFillerToken not forwarded: got %d, want 1", captured.KeepFillerToken)
	}
}

type captureOptsSender struct {
	resp    *amivoice.Response
	capture *amivoice.Options
}

func (s *captureOptsSender) Send(_ string, opts amivoice.Options) (*amivoice.Response, error) {
	*s.capture = opts
	return s.resp, nil
}

// DurationSec uses max across multiple results.
func TestRun_DurationSecAcrossMultipleResults(t *testing.T) {
	resp := &amivoice.Response{
		Results: []amivoice.Result{
			{Tokens: []amivoice.Token{
				{Written: "hello", StartTime: 0.0, EndTime: 3.0},
			}},
			{Tokens: []amivoice.Token{
				{Written: "%えー%", Spoken: "えー", Confidence: 0.9, StartTime: 5.0, EndTime: 7.5},
			}},
		},
	}
	s := &stubSender{resp: resp}
	result, err := pipeline.Run(s, "sample.wav", pipeline.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DurationSec != 7.5 {
		t.Errorf("DurationSec: got %f, want 7.5", result.DurationSec)
	}
}
