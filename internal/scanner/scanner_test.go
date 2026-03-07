package scanner

import (
	"reflect"
	"testing"
)

func TestParsePorts(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
		wantErr  bool
	}{
		{"80", []int{80}, false},
		{"80,443", []int{80, 443}, false},
		{"80-82", []int{80, 81, 82}, false},
		{"22, 80-81, 443", []int{22, 80, 81, 443}, false},
		{"65536", nil, true},                      // out of bounds
		{"abc", nil, true},                        // invalid number
		{"10-5", []int{5, 6, 7, 8, 9, 10}, false}, // reverse range
	}

	for _, tt := range tests {
		got, err := ParsePorts(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParsePorts(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && !reflect.DeepEqual(got, tt.expected) {
			t.Errorf("ParsePorts(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestGetServiceName(t *testing.T) {
	if getServiceName(80) != "HTTP" {
		t.Errorf("expected HTTP for 80")
	}
	if getServiceName(443) != "HTTPS" {
		t.Errorf("expected HTTPS for 443")
	}
	if getServiceName(99999) != "Unknown" {
		t.Errorf("expected Unknown for 99999")
	}
}
