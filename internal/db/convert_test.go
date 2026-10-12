package db

import (
	"reflect"
	"testing"
)

func TestConvertValue(t *testing.T) {
	tests := []struct {
		dbType string
		raw    []byte
		want   any
	}{
		{"INT", []byte("42"), int64(42)},
		{"BIGINT", []byte("-9"), int64(-9)},
		{"TINYINT", []byte("1"), int64(1)},
		{"YEAR", []byte("2026"), int64(2026)},
		{"UNSIGNED BIGINT", []byte("18446744073709551615"), uint64(18446744073709551615)},
		{"UNSIGNED INT", []byte("7"), uint64(7)},
		{"DOUBLE", []byte("1.5"), float64(1.5)},
		{"FLOAT", []byte("-0.25"), float64(-0.25)},
		// DECIMAL stays a string so no precision is lost.
		{"DECIMAL", []byte("12.30"), "12.30"},
		{"DATE", []byte("2026-10-12"), "2026-10-12"},
		{"DATETIME", []byte("2026-10-12 08:00:00"), "2026-10-12 08:00:00"},
		{"VARCHAR", []byte("hello"), "hello"},
		{"ENUM", []byte("done"), "done"},
		{"JSON", []byte(`{"a":1}`), `{"a":1}`},
		{"BLOB", []byte{0x68, 0x69}, "hi"},
		{"INT", nil, nil},
		{"VARCHAR", nil, nil},
		// Values that do not parse fall back to the original text.
		{"INT", []byte("x"), "x"},
		{"DOUBLE", []byte("x"), "x"},
	}
	for _, tt := range tests {
		got := convertValue(tt.dbType, tt.raw)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("convertValue(%q, %q) = %#v, want %#v", tt.dbType, tt.raw, got, tt.want)
		}
	}
}
