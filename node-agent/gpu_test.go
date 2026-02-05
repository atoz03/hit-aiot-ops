package main

import (
	"testing"
)

func TestSplitCSVLine(t *testing.T) {
	parts := splitCSVLine("123, NVIDIA A100, 00000000:3B:00.0, 4096")
	if len(parts) != 4 {
		t.Fatalf("len(parts)=%d", len(parts))
	}
	if parts[0] != "123" || parts[3] != "4096" {
		t.Fatalf("unexpected parts=%v", parts)
	}
}
