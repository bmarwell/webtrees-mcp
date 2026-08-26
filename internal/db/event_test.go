package db

import "testing"

func TestParseEventDateAcceptsPreferredAndCommonForms(t *testing.T) {
	tests := []struct {
		input string
		end   bool
		year  int
		month int
		day   int
	}{
		{"1815-12-10", false, 1815, 12, 10},
		{"1815", true, 1815, 12, 31},
		{"10 DEC 1815", false, 1815, 12, 10},
		{"BET 1815 AND 1817", true, 1817, 12, 31},
	}
	for _, test := range tests {
		got, err := parseEventDate(test.input, test.end)
		if err != nil || got.year != test.year || got.month != test.month || got.day != test.day {
			t.Errorf("parseEventDate(%q, %v) = %+v, %v", test.input, test.end, got, err)
		}
	}
}
