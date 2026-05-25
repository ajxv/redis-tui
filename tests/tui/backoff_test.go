package tui_test

import (
	"testing"
	"time"

	"github.com/ajxv/redis-tui/internal/tui"
)

func TestBackoffDuration_Attempt0(t *testing.T) {
	got := tui.BackoffDuration(0)
	want := 200 * time.Millisecond
	if got != want {
		t.Errorf("attempt 0: got %v, want %v", got, want)
	}
}

func TestBackoffDuration_Attempt1(t *testing.T) {
	got := tui.BackoffDuration(1)
	want := 400 * time.Millisecond
	if got != want {
		t.Errorf("attempt 1: got %v, want %v", got, want)
	}
}

func TestBackoffDuration_Attempt2(t *testing.T) {
	got := tui.BackoffDuration(2)
	want := 800 * time.Millisecond
	if got != want {
		t.Errorf("attempt 2: got %v, want %v", got, want)
	}
}

func TestBackoffDuration_Attempt7(t *testing.T) {
	got := tui.BackoffDuration(7)
	// 1<<7 * 200ms = 128 * 200ms = 25600ms = 25.6s
	want := 25600 * time.Millisecond
	if got != want {
		t.Errorf("attempt 7: got %v, want %v", got, want)
	}
}

func TestBackoffDuration_LargeAttempt_NeverExceeds30s(t *testing.T) {
	const maxWait = 30 * time.Second
	for _, attempt := range []int{8, 10, 50, 100, 1000} {
		got := tui.BackoffDuration(attempt)
		if got > maxWait {
			t.Errorf("attempt %d: got %v, exceeds max %v", attempt, got, maxWait)
		}
		if got <= 0 {
			t.Errorf("attempt %d: got non-positive duration %v", attempt, got)
		}
	}
}

func TestBackoffDuration_Monotonic_UpToCap(t *testing.T) {
	prev := tui.BackoffDuration(0)
	for i := 1; i <= 7; i++ {
		curr := tui.BackoffDuration(i)
		if curr <= prev {
			t.Errorf("attempt %d: duration %v not greater than attempt %d: %v", i, curr, i-1, prev)
		}
		prev = curr
	}
}
