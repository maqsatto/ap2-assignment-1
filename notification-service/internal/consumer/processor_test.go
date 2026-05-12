package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	processed map[string]bool
	marked    []string
}

func (s *fakeStore) AlreadyProcessed(ctx context.Context, eventID string) (bool, error) {
	return s.processed[eventID], nil
}

func (s *fakeStore) MarkProcessed(ctx context.Context, eventID string) error {
	s.processed[eventID] = true
	s.marked = append(s.marked, eventID)
	return nil
}

type fakeSender struct {
	calls int
	err   error
}

func (s *fakeSender) Send(ctx context.Context, msg EmailMessage) error {
	s.calls++
	return s.err
}

func paymentEventBody(t *testing.T, eventID string) []byte {
	t.Helper()
	body, err := json.Marshal(PaymentCompletedEvent{
		EventID:       eventID,
		OrderID:       "ORD-1",
		Amount:        12345,
		CustomerEmail: "student@example.com",
		Status:        "Authorized",
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestProcessorSkipsAlreadyProcessedEvent(t *testing.T) {
	store := &fakeStore{processed: map[string]bool{"PAY-1": true}}
	sender := &fakeSender{}
	processor := NewProcessor(store, sender)

	if err := processor.Process(context.Background(), paymentEventBody(t, "PAY-1")); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if sender.calls != 0 {
		t.Fatalf("expected sender not to be called for duplicate event, got %d", sender.calls)
	}
}

func TestProcessorMarksEventAfterSending(t *testing.T) {
	store := &fakeStore{processed: map[string]bool{}}
	sender := &fakeSender{}
	processor := NewProcessor(store, sender)

	if err := processor.Process(context.Background(), paymentEventBody(t, "PAY-2")); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if sender.calls != 1 {
		t.Fatalf("expected one send call, got %d", sender.calls)
	}
	if !store.processed["PAY-2"] {
		t.Fatalf("expected event to be marked processed after send")
	}
}

func TestProcessorDoesNotMarkEventWhenProviderFails(t *testing.T) {
	store := &fakeStore{processed: map[string]bool{}}
	sender := &fakeSender{err: errors.New("provider timeout")}
	processor := NewProcessor(store, sender)

	if err := processor.Process(context.Background(), paymentEventBody(t, "PAY-3")); err == nil {
		t.Fatalf("expected provider error")
	}
	if store.processed["PAY-3"] {
		t.Fatalf("event should not be marked processed when sending fails")
	}
}

func TestRetryBackoffDoublesByAttempt(t *testing.T) {
	tests := []struct {
		attempt int32
		want    time.Duration
	}{
		{attempt: 1, want: 2 * time.Second},
		{attempt: 2, want: 4 * time.Second},
		{attempt: 3, want: 8 * time.Second},
	}

	for _, tt := range tests {
		if got := RetryBackoff(tt.attempt); got != tt.want {
			t.Fatalf("RetryBackoff(%d) = %s, want %s", tt.attempt, got, tt.want)
		}
	}
}
