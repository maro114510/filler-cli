package amivoice

import (
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// --- helpers ---

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	c := &Client{
		apiKey:     "test-api-key",
		endpoint:   ts.URL,
		httpClient: ts.Client(),
	}
	return c, ts
}

// writeTempAudio creates a temp file with the given extension and dummy content.
func writeTempAudio(t *testing.T, ext string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "audio*"+ext)
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	_, _ = f.WriteString("dummy audio data")
	f.Close()
	return f.Name()
}

const validResponseJSON = `{
	"results": [
		{
			"text": "えーと今日は",
			"tokens": [
				{
					"written": "%えーと%",
					"spoken":  "えーと",
					"confidence": 0.95,
					"starttime": 0.1,
					"endtime":   0.8
				},
				{
					"written": "今日は",
					"spoken":  "きょうは",
					"confidence": 0.99,
					"starttime": 0.9,
					"endtime":   1.5
				}
			]
		}
	],
	"text": "えーと今日は",
	"code": "",
	"message": ""
}`

// --- AC-2: non-existent file ---

func TestSend_FileNotFound(t *testing.T) {
	c := &Client{apiKey: "k", endpoint: "http://unused", httpClient: &http.Client{}}
	_, err := c.Send(filepath.Join(t.TempDir(), "nosuch.wav"), Options{GrammarFileNames: "-a-general", KeepFillerToken: 1})
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

// --- AC-3: unsupported extension ---

func TestSend_UnsupportedExtension(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := &Client{apiKey: "k", endpoint: ts.URL, httpClient: ts.Client()}
	path := writeTempAudio(t, ".ogg")
	_, err := c.Send(path, Options{GrammarFileNames: "-a-general", KeepFillerToken: 1})
	if err == nil {
		t.Fatal("expected error for unsupported extension, got nil")
	}
	if called {
		t.Error("HTTP server must not be called for unsupported extension")
	}
}

func TestSend_WavExtensionAccepted(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validResponseJSON))
	})
	path := writeTempAudio(t, ".wav")
	resp, err := c.Send(path, Options{GrammarFileNames: "-a-general", KeepFillerToken: 1})
	if err != nil {
		t.Fatalf("unexpected error for .wav: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestSend_Mp3ExtensionAccepted(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validResponseJSON))
	})
	path := writeTempAudio(t, ".mp3")
	resp, err := c.Send(path, Options{GrammarFileNames: "-a-general", KeepFillerToken: 1})
	if err != nil {
		t.Fatalf("unexpected error for .mp3: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// --- AC-1: valid WAV returns populated Response ---

func TestSend_ValidResponse(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validResponseJSON))
	})
	want := &Response{
		Text: "えーと今日は",
		Results: []Result{
			{
				Text: "えーと今日は",
				Tokens: []Token{
					{Written: "%えーと%", Spoken: "えーと", Confidence: 0.95, StartTime: 0.1, EndTime: 0.8},
					{Written: "今日は", Spoken: "きょうは", Confidence: 0.99, StartTime: 0.9, EndTime: 1.5},
				},
			},
		},
	}
	path := writeTempAudio(t, ".wav")
	got, err := c.Send(path, Options{GrammarFileNames: "-a-general", KeepFillerToken: 1})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Send() mismatch (-want +got):\n%s", diff)
	}
}

// --- token field mapping including float times ---

func TestSend_TokenFieldMapping(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validResponseJSON))
	})
	want := Token{
		Written:    "%えーと%",
		Spoken:     "えーと",
		Confidence: 0.95,
		StartTime:  0.1,
		EndTime:    0.8,
	}
	path := writeTempAudio(t, ".wav")
	resp, err := c.Send(path, Options{GrammarFileNames: "-a-general", KeepFillerToken: 1})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	got := resp.Results[0].Tokens[0]
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("token[0] mismatch (-want +got):\n%s", diff)
	}
}

// --- AC-4: non-2xx response returns error with status and body ---

func TestSend_Non2xxReturnsErrorWithStatusAndBody(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid api key"))
	})
	path := writeTempAudio(t, ".wav")
	_, err := c.Send(path, Options{GrammarFileNames: "-a-general", KeepFillerToken: 1})
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "401") {
		t.Errorf("error message should contain status code 401, got: %q", msg)
	}
	if !strings.Contains(msg, "invalid api key") {
		t.Errorf("error message should contain response body, got: %q", msg)
	}
}

func TestSend_500ReturnsErrorWithBody(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	})
	path := writeTempAudio(t, ".wav")
	_, err := c.Send(path, Options{GrammarFileNames: "-a-general", KeepFillerToken: 1})
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should contain 500, got: %q", err.Error())
	}
}

// --- AmiVoice application-level error (HTTP 200, code != "") ---

func TestSend_AppLevelErrorCode(t *testing.T) {
	body := `{"results":[],"text":"","code":"-","message":"received illegal service authorization"}`
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	path := writeTempAudio(t, ".wav")
	_, err := c.Send(path, Options{GrammarFileNames: "-a-general", KeepFillerToken: 1})
	if err == nil {
		t.Fatal("expected error for AmiVoice error code, got nil")
	}
}

// --- multipart structure: u contains api key, a is last part ---

func TestSend_MultipartStructure(t *testing.T) {
	var partNames []string
	var apiKeyField string

	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			http.Error(w, "expected multipart", http.StatusBadRequest)
			return
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			name := part.FormName()
			partNames = append(partNames, name)
			if name == "u" {
				buf := make([]byte, 256)
				n, _ := part.Read(buf)
				apiKeyField = string(buf[:n])
			}
			part.Close()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validResponseJSON))
	})

	path := writeTempAudio(t, ".wav")
	if _, err := c.Send(path, Options{GrammarFileNames: "-a-general", KeepFillerToken: 1}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if apiKeyField != "test-api-key" {
		t.Errorf("u field: got %q, want %q", apiKeyField, "test-api-key")
	}
	if len(partNames) == 0 {
		t.Fatal("no multipart parts found")
	}
	if last := partNames[len(partNames)-1]; last != "a" {
		t.Errorf("last multipart part must be 'a', got %q", last)
	}
}

// --- AC-2: New() constructs a client with a non-zero Timeout ---

func TestNew_ClientHasNonZeroTimeout(t *testing.T) {
	c := New("test-key")
	if c.httpClient.Timeout <= 0 {
		t.Errorf("New() httpClient.Timeout = %v, want > 0", c.httpClient.Timeout)
	}
}

// --- AC-3: Send() returns an error when the server does not respond within the timeout ---

func TestSend_TimeoutReturnsError(t *testing.T) {
	// Use a very short timeout so the test completes quickly.
	shortTimeout := 50 * time.Millisecond
	var called bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		time.Sleep(shortTimeout * 10) // sleep far beyond the client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := &Client{
		apiKey:   "test-key",
		endpoint: ts.URL,
		httpClient: &http.Client{
			Timeout: shortTimeout,
		},
	}
	path := writeTempAudio(t, ".wav")
	_, err := c.Send(path, Options{GrammarFileNames: "-a-general", KeepFillerToken: 1})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	_ = called // server may or may not have been reached before timeout
	msg := err.Error()
	if !strings.Contains(msg, "timeout") && !strings.Contains(msg, "context deadline exceeded") {
		t.Errorf("error should mention timeout, got: %q", msg)
	}
}
