package main

import (
	"testing"
	"time"
)

func TestCalculateProcessCost_MatchAndDefault(t *testing.T) {
	pi := NewPriceIndex([]PriceRow{
		{Model: "A100", Price: 0.5},
		{Model: "RTX 3090", Price: 0.2},
	})
	proc := UserProcess{
		Username: "u",
		PID:      1,
		GPUUsage: []GPUUsage{
			{GPUModel: "NVIDIA A100-SXM4-80GB"},
			{GPUModel: "Unknown GPU"},
		},
	}

	got := CalculateProcessCost(proc, pi, 0.1)
	want := 0.6
	if got != want {
		t.Fatalf("cost=%v want=%v", got, want)
	}
}

func TestStatusAndActions_GraceKill(t *testing.T) {
	now := time.Date(2026, 2, 5, 16, 0, 0, 0, time.UTC)
	blockedAt := now.Add(-11 * time.Minute)
	u := User{Username: "alice", Balance: -1, Status: "blocked", BlockedAt: &blockedAt}
	acts := DecideActions(now, "blocked", u, 100, 10, 10*time.Minute, []int32{123})
	if len(acts) == 0 {
		t.Fatalf("expected actions")
	}
	foundKill := false
	for _, a := range acts {
		if a.Type == "kill_process" {
			foundKill = true
		}
	}
	if !foundKill {
		t.Fatalf("expected kill_process action")
	}
}
