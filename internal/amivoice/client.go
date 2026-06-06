package amivoice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const defaultEndpoint = "https://acp-api.amivoice.com/v1/recognize"

var supportedExtensions = map[string]bool{
	".wav": true,
	".mp3": true,
}

// Options controls the recognition parameters sent to the AmiVoice API.
type Options struct {
	// GrammarFileNames specifies the engine name (e.g. "-a-general").
	// End-to-End engines suppress filler tokens and must not be used.
	GrammarFileNames string
	// KeepFillerToken enables filler token output when set to 1.
	KeepFillerToken int // 0 or 1
}

// Token represents a single morpheme returned by the AmiVoice API.
// StartTime and EndTime are in seconds relative to the start of the audio.
type Token struct {
	Written    string
	Spoken     string
	Confidence float64
	StartTime  float64
	EndTime    float64
}

// Result holds the recognition output for one utterance segment.
type Result struct {
	Text   string
	Tokens []Token
}

// Response is the top-level recognition result returned by Send.
type Response struct {
	// Text is the concatenation of all Result.Text values.
	Text    string
	Results []Result
}

// Client sends audio to the AmiVoice synchronous HTTP API.
type Client struct {
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		endpoint:   defaultEndpoint,
		httpClient: http.DefaultClient,
	}
}

func (c *Client) Send(audioPath string, opts Options) (*Response, error) {
	ext := strings.ToLower(filepath.Ext(audioPath))
	if !supportedExtensions[ext] {
		return nil, fmt.Errorf("amivoice: unsupported audio format %q", ext)
	}

	f, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("amivoice: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if err := w.WriteField("u", c.apiKey); err != nil {
		return nil, fmt.Errorf("amivoice: write field u: %w", err)
	}

	d := fmt.Sprintf("grammarFileNames=%s keepFillerToken=%d", opts.GrammarFileNames, opts.KeepFillerToken)
	if err := w.WriteField("d", d); err != nil {
		return nil, fmt.Errorf("amivoice: write field d: %w", err)
	}

	// 'a' must be the last multipart part per AmiVoice spec.
	part, err := w.CreateFormFile("a", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("amivoice: create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("amivoice: copy audio: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("amivoice: close writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.endpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("amivoice: new request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("amivoice: do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("amivoice: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("amivoice: %s: %s", resp.Status, body)
	}

	return parseResponse(body)
}

// internal JSON types matching AmiVoice's snake_case field names.
type apiToken struct {
	Written    string  `json:"written"`
	Spoken     string  `json:"spoken"`
	Confidence float64 `json:"confidence"`
	StartTime  float64 `json:"starttime"`
	EndTime    float64 `json:"endtime"`
}

type apiResult struct {
	Text   string     `json:"text"`
	Tokens []apiToken `json:"tokens"`
}

type apiResponse struct {
	Results []apiResult `json:"results"`
	Text    string      `json:"text"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
}

func parseResponse(body []byte) (*Response, error) {
	var raw apiResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("amivoice: parse response: %w", err)
	}

	// AmiVoice signals application-level errors via code != "" even on HTTP 200.
	if raw.Code != "" {
		return nil, fmt.Errorf("amivoice: %s: %s", raw.Code, raw.Message)
	}

	out := &Response{
		Text:    raw.Text,
		Results: make([]Result, len(raw.Results)),
	}
	for i, r := range raw.Results {
		tokens := make([]Token, len(r.Tokens))
		for j, t := range r.Tokens {
			tokens[j] = Token{
				Written:    t.Written,
				Spoken:     t.Spoken,
				Confidence: t.Confidence,
				StartTime:  t.StartTime,
				EndTime:    t.EndTime,
			}
		}
		out.Results[i] = Result{Text: r.Text, Tokens: tokens}
	}
	return out, nil
}
