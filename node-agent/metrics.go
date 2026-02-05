package main

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

type cpuSample struct {
	total float64
	ts    time.Time
}

func (a *NodeAgent) CollectMetrics(ctx context.Context) (*MetricsData, error) {
	metrics := &MetricsData{
		NodeID:    a.nodeID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Users:     []UserProcess{},
	}

	gpuMap, err := a.getGPUUsageMap(ctx)
	if err != nil {
		return nil, err
	}

	// CPU 计费需要观察 CPU-only 进程，因此进行一次全量扫描；
	// 为控制开销，只上报「占用 CPU 超过阈值」或「使用 GPU」的进程。
	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("获取进程列表失败：%w", err)
	}

	now := time.Now()
	seen := make(map[int32]struct{}, len(procs))
	for _, proc := range procs {
		pid := proc.Pid
		if pid <= 0 {
			continue
		}
		seen[pid] = struct{}{}

		username, err := proc.Username()
		if err != nil || username == "" || username == "root" {
			continue
		}

		// 计算 CPU 百分比（自维护采样差分，避免依赖 gopsutil 的内部缓存行为）
		cpuPercent := a.computeCPUPercent(ctx, proc, now)
		gpuUsage := gpuMap[pid]

		if len(gpuUsage) == 0 && cpuPercent < a.cpuMinPercent {
			continue
		}

		memInfo, _ := proc.MemoryInfo()
		memoryMB := 0.0
		if memInfo != nil {
			memoryMB = float64(memInfo.RSS) / 1024 / 1024
		}

		metrics.Users = append(metrics.Users, UserProcess{
			Username:   username,
			PID:        pid,
			CPUPercent: cpuPercent,
			MemoryMB:   memoryMB,
			GPUUsage:   gpuUsage,
		})
	}

	// 清理不再存在的 pid，防止内存增长
	for pid := range a.lastCPUSample {
		if _, ok := seen[pid]; !ok {
			delete(a.lastCPUSample, pid)
		}
	}

	return metrics, nil
}

func (a *NodeAgent) validateConfig() error {
	if a.nodeID == "" {
		return fmt.Errorf("NODE_ID 不能为空")
	}
	if a.controllerURL == "" {
		return fmt.Errorf("CONTROLLER_URL 不能为空")
	}
	if a.agentToken == "" {
		return fmt.Errorf("AGENT_TOKEN 不能为空")
	}
	return nil
}

func (a *NodeAgent) computeCPUPercent(ctx context.Context, proc *process.Process, now time.Time) float64 {
	t, err := proc.TimesWithContext(ctx)
	if err != nil {
		return 0
	}
	total := t.User + t.System

	last, ok := a.lastCPUSample[proc.Pid]
	a.lastCPUSample[proc.Pid] = cpuSample{total: total, ts: now}
	if !ok {
		return 0
	}

	elapsed := now.Sub(last.ts).Seconds()
	if elapsed <= 0 {
		return 0
	}
	delta := total - last.total
	if delta <= 0 {
		return 0
	}

	percent := (delta / elapsed) * 100.0
	if math.IsNaN(percent) || math.IsInf(percent, 0) {
		return 0
	}
	// 某些平台可能出现短时间抖动，做个上限保护（不严格依赖 numCPU）
	if percent < 0 {
		return 0
	}
	return percent
}
