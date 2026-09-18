package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	CodeInvalidQuestion  = "invalid_question"
	CodeInvalidOptions   = "invalid_options"
	CodeDuplicateOption  = "duplicate_option"
	CodeInvalidCloseTime = "invalid_close_time"
)

// ValidationError is safe to return to API clients. Code is stable for
// clients and tests; Message is deliberately human readable.
type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string { return e.Message }

// NewPollInput is the complete public interface for validating and
// normalizing a poll before persistence.
type NewPollInput struct {
	Question          string
	Description       string
	Options           []string
	ClosesAt          *time.Time
	ShowResultsBefore bool
}

// Validate returns a normalized copy. It never mutates the caller's slices.
func (in NewPollInput) Validate(now time.Time) (NewPollInput, *ValidationError) {
	in.Question = strings.TrimSpace(in.Question)
	in.Description = strings.TrimSpace(in.Description)

	if n := utf8.RuneCountInString(in.Question); n < 1 || n > 160 {
		return NewPollInput{}, validationError(CodeInvalidQuestion, "Question must contain 1 to 160 characters.")
	}
	if utf8.RuneCountInString(in.Description) > 500 {
		return NewPollInput{}, validationError(CodeInvalidQuestion, "Description must contain at most 500 characters.")
	}
	if len(in.Options) < 2 || len(in.Options) > 10 {
		return NewPollInput{}, validationError(CodeInvalidOptions, "A poll needs between 2 and 10 options.")
	}

	options := make([]string, len(in.Options))
	seen := make(map[string]struct{}, len(in.Options))
	for i, raw := range in.Options {
		option := strings.TrimSpace(raw)
		if n := utf8.RuneCountInString(option); n < 1 || n > 80 {
			return NewPollInput{}, validationError(CodeInvalidOptions, fmt.Sprintf("Option %d must contain 1 to 80 characters.", i+1))
		}
		key := strings.ToLower(option)
		if _, exists := seen[key]; exists {
			return NewPollInput{}, validationError(CodeDuplicateOption, "Poll options must be unique.")
		}
		seen[key] = struct{}{}
		options[i] = option
	}
	in.Options = options

	if in.ClosesAt != nil {
		closeTime := in.ClosesAt.UTC()
		if !closeTime.After(now.UTC()) {
			return NewPollInput{}, validationError(CodeInvalidCloseTime, "Close time must be in the future.")
		}
		in.ClosesAt = &closeTime
	}
	return in, nil
}

func validationError(code, message string) *ValidationError {
	return &ValidationError{Code: code, Message: message}
}
