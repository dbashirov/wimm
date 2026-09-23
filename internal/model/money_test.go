package model

import (
	"encoding/json"
	"testing"
)

func TestParseMoney(t *testing.T) {
	tests := []struct {
		in      string
		want    Money
		wantErr bool
	}{
		{in: "0", want: 0},
		{in: "10", want: 1000},
		{in: "10.5", want: 1050},
		{in: "10,05", want: 1005},
		{in: "1 234,56", want: 123456},
		{in: "1 234.56", want: 123456},
		{in: ".5", want: 50},
		{in: "7.", want: 700},
		{in: "-12.30", want: -1230},
		{in: "9999999999999.99", want: 999999999999999},
		{in: "", wantErr: true},
		{in: "-", wantErr: true},
		{in: ".", wantErr: true},
		{in: "1.234", wantErr: true},
		{in: "1.2.3", wantErr: true},
		{in: "1e3", wantErr: true},
		{in: "+5", wantErr: true},
		{in: "abc", wantErr: true},
		{in: "10000000000000", wantErr: true},
	}
	for _, tc := range tests {
		got, err := ParseMoney(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseMoney(%q) = %d, ожидалась ошибка", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("ParseMoney(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}
}

func TestMoneyDecimalAndJSON(t *testing.T) {
	tests := []struct {
		in   Money
		want string
	}{
		{0, "0.00"},
		{7, "0.07"},
		{123456, "1234.56"},
		{-1050, "-10.50"},
	}
	for _, tc := range tests {
		if got := tc.in.Decimal(); got != tc.want {
			t.Errorf("Decimal(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}

	data, err := json.Marshal(struct {
		Amounts []Money `json:"amounts"`
	}{Amounts: []Money{123456, -5}})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if got := string(data); got != `{"amounts":[1234.56,-0.05]}` {
		t.Errorf("json = %s", got)
	}
}
