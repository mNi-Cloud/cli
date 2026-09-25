package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeSource hands back whatever outcome the test puts into it.
type fakeSource chan callbackOutcome

func (s fakeSource) Outcomes() <-chan callbackOutcome {
	return s
}

func newFakeSource() fakeSource {
	return make(fakeSource, 1)
}

func TestFirstOutcomeTakesTheSourceThatAnswers(t *testing.T) {
	silent, answering := newFakeSource(), newFakeSource()
	answering <- callbackOutcome{result: callbackResult{Code: "the-code", State: "the-state"}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := firstOutcome(ctx, silent, answering)
	if err != nil {
		t.Fatalf("firstOutcome() error = %v", err)
	}
	if result.Code != "the-code" || result.State != "the-state" {
		t.Errorf("firstOutcome() = %+v, want the response of the source that answered", result)
	}
}

func TestFirstOutcomeCarriesTheErrorOfASource(t *testing.T) {
	refusal := &AuthorizationError{Code: "access_denied"}
	source := newFakeSource()
	source <- callbackOutcome{err: refusal}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := firstOutcome(ctx, newFakeSource(), source); !errors.Is(err, refusal) {
		t.Errorf("firstOutcome() error = %v, want the error the source handed back", err)
	}
}

func TestFirstOutcomeStopsWhenTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := firstOutcome(ctx, newFakeSource(), newFakeSource()); !errors.Is(err, context.Canceled) {
		t.Errorf("firstOutcome() error = %v, want context.Canceled", err)
	}
}
