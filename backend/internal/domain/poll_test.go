package domain_test

import (
	"testing"
	"time"

	"pulsepoll/backend/internal/domain"
)

func TestNewPollInputValidateNormalizesAValidPoll(t *testing.T) {
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	closesAt := now.Add(30 * time.Minute)

	got, err := (domain.NewPollInput{
		Question: "  Which demo should we ship?  ",
		Options:  []string{"  Live voting ", "Failure recovery  "},
		ClosesAt: &closesAt,
	}).Validate(now)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got.Question != "Which demo should we ship?" {
		t.Fatalf("Question = %q", got.Question)
	}
	if got.Options[0] != "Live voting" || got.Options[1] != "Failure recovery" {
		t.Fatalf("Options = %#v", got.Options)
	}
}

func TestNewPollInputValidateRejectsDuplicateOptions(t *testing.T) {
	_, err := (domain.NewPollInput{
		Question: "Pick one",
		Options:  []string{"Go", " go "},
	}).Validate(time.Now())
	if err == nil || err.Code != domain.CodeDuplicateOption {
		t.Fatalf("Validate() error = %#v, want code %q", err, domain.CodeDuplicateOption)
	}
}

func TestNewPollInputValidateRejectsPastCloseTime(t *testing.T) {
	now := time.Now()
	closesAt := now.Add(-time.Second)
	_, err := (domain.NewPollInput{
		Question: "Pick one",
		Options:  []string{"A", "B"},
		ClosesAt: &closesAt,
	}).Validate(now)
	if err == nil || err.Code != domain.CodeInvalidCloseTime {
		t.Fatalf("Validate() error = %#v, want code %q", err, domain.CodeInvalidCloseTime)
	}
}
