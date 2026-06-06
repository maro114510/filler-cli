package amivoice

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const issueAuthorizationEndpoint = "https://acp-api.amivoice.com/issue_service_authorization"

// OneTimeKeyOptions configures optional parameters for key issuance.
type OneTimeKeyOptions struct {
	// ValidFor sets the key lifetime. 0 uses the API default (30 s).
	// The keystore TTL is 2 h, so callers typically pass 2*time.Hour.
	ValidFor time.Duration
	// AllowedCIDRs restricts the issued key to the given IPv4 CIDR ranges.
	AllowedCIDRs []string
}

// IssueOneTimeKey obtains a short-lived API key from AmiVoice using
// a service ID and password.
func IssueOneTimeKey(serviceID, servicePassword string, opts OneTimeKeyOptions) (string, error) {
	return issueOneTimeKeyURL(issueAuthorizationEndpoint, serviceID, servicePassword, opts)
}

func issueOneTimeKeyURL(endpoint, serviceID, servicePassword string, opts OneTimeKeyOptions) (string, error) {
	if serviceID == "" {
		return "", fmt.Errorf("amivoice: serviceID must not be empty")
	}
	if servicePassword == "" {
		return "", fmt.Errorf("amivoice: servicePassword must not be empty")
	}

	form := url.Values{}
	form.Set("sid", serviceID)
	form.Set("spw", servicePassword)
	if opts.ValidFor > 0 {
		form.Set("epi", fmt.Sprintf("%d", opts.ValidFor.Milliseconds()))
	}
	if len(opts.AllowedCIDRs) > 0 {
		form.Set("ipa", strings.Join(opts.AllowedCIDRs, ","))
	}

	client := &http.Client{Timeout: defaultTimeout}
	resp, err := client.PostForm(endpoint, form)
	if err != nil {
		return "", fmt.Errorf("amivoice: issue authorization: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("amivoice: issue authorization: read response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("amivoice: issue authorization: %s: %s", resp.Status, body)
	}

	return string(body), nil
}
