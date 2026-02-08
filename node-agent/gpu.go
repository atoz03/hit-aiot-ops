package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

var errNoNvidiaSMI = errors.New("未检测到 nvidia-smi")

func (a *NodeAgent) getGPUUsageMap(ctx context.Context) (map[int32][]GPUUsage, error) {
	out := make(map[int32][]GPUUsage)

	lines, err := a.runNvidiaSMI(ctx,
		"--query-compute-apps=pid,gpu_name,gpu_bus_id,used_memory",
		"--format=csv,noheader,nounits",
	)
	if err != nil {
		// 无 GPU/无驱动时允许降级
		if errors.Is(err, errNoNvidiaSMI) {
			return out, nil
		}
		return nil, err
	}

	busIDToIndex, _ := a.getBusIDToIndexMap(ctx)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := splitCSVLine(line)
		if len(parts) < 4 {
			continue
		}

		pid, err := parseInt32(parts[0])
		if err != nil || pid <= 0 {
			continue
		}

		gpuName := strings.TrimSpace(parts[1])
		busID := strings.TrimSpace(parts[2])
		memMB, _ := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)

		gpuID := int32(-1)
		if idx, ok := busIDToIndex[normalizeBusID(busID)]; ok {
			gpuID = idx
		}

		out[pid] = append(out[pid], GPUUsage{
			GPUID:    gpuID,
			GPUModel: gpuName,
			GPUBusID: busID,
			MemoryMB: memMB,
		})
	}

	return out, nil
}

func (a *NodeAgent) getBusIDToIndexMap(ctx context.Context) (map[string]int32, error) {
	lines, err := a.runNvidiaSMI(ctx,
		"--query-gpu=index,pci.bus_id",
		"--format=csv,noheader",
	)
	if err != nil {
		if errors.Is(err, errNoNvidiaSMI) {
			return map[string]int32{}, nil
		}
		return nil, err
	}

	out := make(map[string]int32)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := splitCSVLine(line)
		if len(parts) < 2 {
			continue
		}
		idx64, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 32)
		if err != nil {
			continue
		}
		busID := normalizeBusID(parts[1])
		out[busID] = int32(idx64)
	}
	return out, nil
}

func (a *NodeAgent) getGPUInventory(ctx context.Context) (string, int, error) {
	lines, err := a.runNvidiaSMI(ctx,
		"--query-gpu=name",
		"--format=csv,noheader",
	)
	if err != nil {
		if errors.Is(err, errNoNvidiaSMI) {
			return "", 0, nil
		}
		return "", 0, err
	}
	count := 0
	model := ""
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		count++
		if model == "" {
			model = name
		}
	}
	return model, count, nil
}

func (a *NodeAgent) runNvidiaSMI(ctx context.Context, args ...string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "nvidia-smi", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		// Linux 上找不到命令一般是 *exec.Error
		var ee *exec.Error
		if errors.As(err, &ee) {
			return nil, errNoNvidiaSMI
		}
		return nil, fmt.Errorf("nvidia-smi 执行失败：%w（stderr=%s）", err, strings.TrimSpace(stderr.String()))
	}

	text := strings.TrimSpace(string(b))
	if text == "" {
		return nil, nil
	}
	lines := strings.Split(text, "\n")
	return lines, nil
}

func splitCSVLine(line string) []string {
	// nvidia-smi 的 csv 输出本身不包含复杂转义，这里做最小实现即可
	parts := strings.Split(line, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func parseInt32(s string) (int32, error) {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 32)
	return int32(v), err
}

func normalizeBusID(busID string) string {
	// 统一为大写，去掉多余空格
	return strings.ToUpper(strings.TrimSpace(busID))
}
