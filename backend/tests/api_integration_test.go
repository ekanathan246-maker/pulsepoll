//go:build integration

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

type pollResponse struct {
	Slug       string `json:"slug"`
	Version    int64  `json:"version"`
	TotalVotes int64  `json:"totalVotes"`
	Options    []struct {
		ID string `json:"id"`
	} `json:"options"`
}

func TestConcurrentVotesAreDurableAndDuplicateSafe(t *testing.T) {
	baseURL := os.Getenv("PULSEPOLL_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost"
	}
	host := newClient()
	email := fmt.Sprintf("concurrency-%d@example.test", time.Now().UnixNano())
	response := doJSON(t, host, http.MethodPost, baseURL+"/api/auth/signup", map[string]any{
		"name": "Concurrency Host", "email": email, "password": "Concurrency!2026",
	}, "", http.StatusOK)
	response.Body.Close()

	csrf := cookieValue(t, host, baseURL, "pp_csrf")
	response = doJSON(t, host, http.MethodPost, baseURL+"/api/polls", map[string]any{
		"title": "Can one hundred voters agree?", "options": []string{"Yes", "Not yet"},
		"showResultsBeforeVote": true,
	}, csrf, http.StatusCreated)
	var poll pollResponse
	decodeJSON(t, response.Body, &poll)
	response.Body.Close()
	if len(poll.Options) != 2 {
		t.Fatalf("created poll options = %d", len(poll.Options))
	}

	const voters = 100
	errCh := make(chan error, voters)
	var wg sync.WaitGroup
	for i := 0; i < voters; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			client := newClient()
			status, err := vote(client, baseURL, poll.Slug, poll.Options[0].ID, index)
			if err != nil || status != http.StatusAccepted {
				errCh <- fmt.Errorf("voter %d first request: status=%d err=%v", index, status, err)
				return
			}
			status, err = vote(client, baseURL, poll.Slug, poll.Options[0].ID, index)
			if err != nil || status != http.StatusConflict {
				errCh <- fmt.Errorf("voter %d duplicate: status=%d err=%v", index, status, err)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
	if t.Failed() {
		return
	}

	response = doJSON(t, host, http.MethodGet, baseURL+"/api/polls/"+poll.Slug, nil, "", http.StatusOK)
	var snapshot pollResponse
	decodeJSON(t, response.Body, &snapshot)
	response.Body.Close()
	if snapshot.TotalVotes != voters {
		t.Fatalf("durable total = %d, want %d", snapshot.TotalVotes, voters)
	}
	if snapshot.Version != int64(voters+1) {
		t.Fatalf("durable version = %d, want %d", snapshot.Version, voters+1)
	}
}

func vote(client *http.Client, baseURL, slug, optionID string, index int) (int, error) {
	body, _ := json.Marshal(map[string]string{"optionId": optionID})
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/api/polls/"+slug+"/vote", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", fmt.Sprintf("pulsepoll-integration-voter-%d", index))
	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	return response.StatusCode, nil
}

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 15 * time.Second}
}

func doJSON(t *testing.T, client *http.Client, method, endpoint string, value any, csrf string, wantStatus int) *http.Response {
	t.Helper()
	var body io.Reader
	if value != nil {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		t.Fatal(err)
	}
	if value != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != wantStatus {
		payload, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("%s %s status = %d, want %d: %s", method, endpoint, response.StatusCode, wantStatus, payload)
	}
	return response
}

func cookieValue(t *testing.T, client *http.Client, baseURL, name string) string {
	t.Helper()
	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range client.Jar.Cookies(u) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	t.Fatalf("cookie %q not found", name)
	return ""
}

func decodeJSON(t *testing.T, reader io.Reader, target any) {
	t.Helper()
	if err := json.NewDecoder(reader).Decode(target); err != nil {
		t.Fatal(err)
	}
}
