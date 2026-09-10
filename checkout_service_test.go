package main

import "testing"

func TestShouldRetryDeliveryFailures(t *testing.T) {
	cases := []struct {
		status int
		retry  bool
	}{{200, false}, {500, true}, {503, true}, {429, true}, {0, true}, {400, false}}
	for _, tc := range cases {
		if got := shouldRetry(tc.status); got != tc.retry {
			t.Errorf("status %d: got %v, want %v", tc.status, got, tc.retry)
		}
	}
}
