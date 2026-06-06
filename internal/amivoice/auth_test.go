package amivoice

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestIssueOneTimeKey_ValidCredentials(t *testing.T) {
	const fixtureKey = "issued-key-abc123"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "parse form", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, fixtureKey)
	}))
	defer ts.Close()

	key, err := issueOneTimeKeyURL(ts.URL, "my-sid", "my-spw", OneTimeKeyOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key == "" {
		t.Fatal("expected non-empty key")
	}
	if key != fixtureKey {
		t.Errorf("got key %q, want %q", key, fixtureKey)
	}
}

func TestIssueOneTimeKey_ValidFor_ForwardedAsEpi(t *testing.T) {
	var capturedEpi string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "parse form", http.StatusBadRequest)
			return
		}
		capturedEpi = r.Form.Get("epi")
		_, _ = fmt.Fprint(w, "issued-key")
	}))
	defer ts.Close()

	validFor := 2 * time.Hour
	_, err := issueOneTimeKeyURL(ts.URL, "sid", "spw", OneTimeKeyOptions{ValidFor: validFor})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantEpi := fmt.Sprintf("%d", validFor.Milliseconds())
	if capturedEpi != wantEpi {
		t.Errorf("epi = %q, want %q", capturedEpi, wantEpi)
	}
}

// ValidFor == 0: epi must be omitted.
func TestIssueOneTimeKey_ValidForZero_EpiOmitted(t *testing.T) {
	var capturedForm url.Values
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "parse form", http.StatusBadRequest)
			return
		}
		capturedForm = r.PostForm
		_, _ = fmt.Fprint(w, "issued-key")
	}))
	defer ts.Close()

	_, err := issueOneTimeKeyURL(ts.URL, "sid", "spw", OneTimeKeyOptions{ValidFor: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := capturedForm["epi"]; ok {
		t.Error("epi must not be present when ValidFor == 0")
	}
}

// AllowedCIDRs non-empty: ipa is forwarded as comma-joined string.
func TestIssueOneTimeKey_AllowedCIDRs_ForwardedAsIpa(t *testing.T) {
	var capturedIpa string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "parse form", http.StatusBadRequest)
			return
		}
		capturedIpa = r.Form.Get("ipa")
		_, _ = fmt.Fprint(w, "issued-key")
	}))
	defer ts.Close()

	cidrs := []string{"192.168.1.0/24", "10.0.0.0/8"}
	_, err := issueOneTimeKeyURL(ts.URL, "sid", "spw", OneTimeKeyOptions{AllowedCIDRs: cidrs})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantIpa := strings.Join(cidrs, ",")
	if capturedIpa != wantIpa {
		t.Errorf("ipa = %q, want %q", capturedIpa, wantIpa)
	}
}

// AllowedCIDRs empty: ipa must be omitted.
func TestIssueOneTimeKey_AllowedCIDRsEmpty_IpaOmitted(t *testing.T) {
	var capturedForm url.Values
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "parse form", http.StatusBadRequest)
			return
		}
		capturedForm = r.PostForm
		_, _ = fmt.Fprint(w, "issued-key")
	}))
	defer ts.Close()

	_, err := issueOneTimeKeyURL(ts.URL, "sid", "spw", OneTimeKeyOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := capturedForm["ipa"]; ok {
		t.Error("ipa must not be present when AllowedCIDRs is empty")
	}
}

func TestIssueOneTimeKey_Non2xx_ReturnsErrorWithStatusAndBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, "invalid credentials")
	}))
	defer ts.Close()

	_, err := issueOneTimeKeyURL(ts.URL, "sid", "spw", OneTimeKeyOptions{})
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "401") {
		t.Errorf("error should contain 401, got: %q", msg)
	}
	if !strings.Contains(msg, "invalid credentials") {
		t.Errorf("error should contain response body, got: %q", msg)
	}
}

func TestIssueOneTimeKey_EmptyServiceID_ErrorBeforeHTTP(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, err := issueOneTimeKeyURL(ts.URL, "", "spw", OneTimeKeyOptions{})
	if err == nil {
		t.Fatal("expected error for empty serviceID, got nil")
	}
	if called {
		t.Error("HTTP server must not be called when serviceID is empty")
	}
}

func TestIssueOneTimeKey_EmptyServicePassword_ErrorBeforeHTTP(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, err := issueOneTimeKeyURL(ts.URL, "sid", "", OneTimeKeyOptions{})
	if err == nil {
		t.Fatal("expected error for empty servicePassword, got nil")
	}
	if called {
		t.Error("HTTP server must not be called when servicePassword is empty")
	}
}

// Verify form fields sid and spw are sent correctly.
func TestIssueOneTimeKey_FormFields_SidAndSpw(t *testing.T) {
	var capturedSid, capturedSpw string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "parse form", http.StatusBadRequest)
			return
		}
		capturedSid = r.Form.Get("sid")
		capturedSpw = r.Form.Get("spw")
		_, _ = fmt.Fprint(w, "issued-key")
	}))
	defer ts.Close()

	_, err := issueOneTimeKeyURL(ts.URL, "my-service-id", "my-service-pw", OneTimeKeyOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedSid != "my-service-id" {
		t.Errorf("sid = %q, want %q", capturedSid, "my-service-id")
	}
	if capturedSpw != "my-service-pw" {
		t.Errorf("spw = %q, want %q", capturedSpw, "my-service-pw")
	}
}
