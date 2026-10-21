package handler

import (
	"strconv"
	"testing"
)

func TestParsePage(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 1},
		{"abc", 1},
		{"0", 1},
		{"-5", 1},
		{"3", 3},
		{strconv.Itoa(maxOperationsPage), maxOperationsPage},
		{strconv.Itoa(maxOperationsPage + 1), maxOperationsPage},
		{"9223372036854775807", maxOperationsPage},
		{"99999999999999999999", 1}, // не влезает в int - Atoi ошибка
	}
	for _, tc := range tests {
		if got := parsePage(tc.in); got != tc.want {
			t.Errorf("parsePage(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
