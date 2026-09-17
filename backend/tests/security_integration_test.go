//go:build integration

package tests

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAuthOwnershipCSRFAndValidationBoundaries(t *testing.T) {
	baseURL := os.Getenv("PULSEPOLL_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost"
	}

	unauthenticated := newClient()
	response := doJSON(t, unauthenticated, http.MethodPost, baseURL+"/api/polls", map[string]any{
		"title": "No session", "options": []string{"A", "B"},
	}, "", http.StatusUnauthorized)
	response.Body.Close()

	owner := signupClient(t, baseURL, "Security Owner")
	attacker := signupClient(t, baseURL, "Other Account")
	ownerCSRF := cookieValue(t, owner, baseURL, "pp_csrf")

	response = doJSON(t, owner, http.MethodPost, baseURL+"/api/polls", map[string]any{
		"title": "Missing CSRF", "options": []string{"A", "B"},
	}, "", http.StatusForbidden)
	response.Body.Close()

	response = doJSON(t, owner, http.MethodPost, baseURL+"/api/polls", map[string]any{
		"title": "Unknown field", "options": []string{"A", "B"}, "admin": true,
	}, ownerCSRF, http.StatusBadRequest)
	response.Body.Close()

	response = doJSON(t, owner, http.MethodPost, baseURL+"/api/polls", map[string]any{
		"title": "Duplicate options", "options": []string{"Same", " same "},
	}, ownerCSRF, http.StatusBadRequest)
	response.Body.Close()

	oversized, err := http.NewRequest(http.MethodPost, baseURL+"/api/polls", strings.NewReader(`{"title":"`+strings.Repeat("x", 40<<10)+`","options":["A","B"]}`))
	if err != nil {
		t.Fatal(err)
	}
	oversized.Header.Set("Content-Type", "application/json")
	oversized.Header.Set("X-CSRF-Token", ownerCSRF)
	oversizedResponse, err := owner.Do(oversized)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, oversizedResponse.Body)
	oversizedResponse.Body.Close()
	if oversizedResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("oversized request status = %d, want 400", oversizedResponse.StatusCode)
	}

	response = doJSON(t, owner, http.MethodPost, baseURL+"/api/polls", map[string]any{
		"title": "Ownership boundary", "options": []string{"Allowed", "Denied"},
		"showResultsBeforeVote": true,
	}, ownerCSRF, http.StatusCreated)
	var poll pollResponse
	decodeJSON(t, response.Body, &poll)
	response.Body.Close()

	attackerCSRF := cookieValue(t, attacker, baseURL, "pp_csrf")
	response = doJSON(t, attacker, http.MethodPatch, baseURL+"/api/polls/"+poll.Slug+"/close", nil, attackerCSRF, http.StatusForbidden)
	response.Body.Close()
	response = doJSON(t, attacker, http.MethodGet, baseURL+"/api/polls/"+poll.Slug+"/export.csv", nil, "", http.StatusNotFound)
	response.Body.Close()

	corsRequest, err := http.NewRequest(http.MethodGet, baseURL+"/api/polls/"+poll.Slug, nil)
	if err != nil {
		t.Fatal(err)
	}
	corsRequest.Header.Set("Origin", "https://attacker.example")
	corsResponse, err := unauthenticated.Do(corsRequest)
	if err != nil {
		t.Fatal(err)
	}
	corsResponse.Body.Close()
	if corsResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("disallowed origin status = %d, want 403", corsResponse.StatusCode)
	}

	voter := newClient()
	status, err := vote(voter, baseURL, poll.Slug, poll.Options[0].ID, 501)
	if err != nil || status != http.StatusAccepted {
		t.Fatalf("first vote status=%d err=%v", status, err)
	}
	status, err = vote(voter, baseURL, poll.Slug, poll.Options[0].ID, 501)
	if err != nil || status != http.StatusConflict {
		t.Fatalf("duplicate vote status=%d err=%v", status, err)
	}

	response = doJSON(t, owner, http.MethodPatch, baseURL+"/api/polls/"+poll.Slug+"/close", nil, ownerCSRF, http.StatusOK)
	response.Body.Close()
	lateVoter := newClient()
	status, err = vote(lateVoter, baseURL, poll.Slug, poll.Options[1].ID, 502)
	if err != nil || status != http.StatusConflict {
		t.Fatalf("closed-poll vote status=%d err=%v", status, err)
	}
}

func TestLoginRotatesPresentedSession(t *testing.T) {
	baseURL := os.Getenv("PULSEPOLL_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost"
	}
	email := fmt.Sprintf("rotation-%d@example.test", time.Now().UnixNano())
	password := "Rotation!2026"
	client := newClient()
	response := doJSON(t, client, http.MethodPost, baseURL+"/api/auth/signup", map[string]string{
		"name": "Rotation Owner", "email": email, "password": password,
	}, "", http.StatusOK)
	response.Body.Close()

	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	oldJar, _ := cookiejar.New(nil)
	for _, cookie := range client.Jar.Cookies(u) {
		clone := *cookie
		oldJar.SetCookies(u, []*http.Cookie{&clone})
	}
	oldClient := &http.Client{Jar: oldJar, Timeout: 15 * time.Second}

	response = doJSON(t, client, http.MethodPost, baseURL+"/api/auth/login", map[string]string{
		"email": email, "password": password,
	}, "", http.StatusOK)
	response.Body.Close()

	request, _ := http.NewRequest(http.MethodGet, baseURL+"/api/auth/me", bytes.NewReader(nil))
	oldResponse, err := oldClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	oldResponse.Body.Close()
	if oldResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old session status = %d, want 401 after rotation", oldResponse.StatusCode)
	}

	response = doJSON(t, client, http.MethodGet, baseURL+"/api/auth/me", nil, "", http.StatusOK)
	response.Body.Close()
}

func signupClient(t *testing.T, baseURL, name string) *http.Client {
	t.Helper()
	client := newClient()
	email := fmt.Sprintf("security-%d-%s@example.test", time.Now().UnixNano(), strings.ToLower(strings.ReplaceAll(name, " ", "-")))
	response := doJSON(t, client, http.MethodPost, baseURL+"/api/auth/signup", map[string]string{
		"name": name, "email": email, "password": "Security!2026",
	}, "", http.StatusOK)
	response.Body.Close()
	return client
}
