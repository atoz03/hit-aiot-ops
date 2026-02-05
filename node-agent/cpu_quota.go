package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func (a *NodeAgent) setUserCPUQuota(ctx context.Context, username string, percent float64, reason string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username 不能为空")
	}
	if percent < 0 || percent > 100 {
		return fmt.Errorf("cpu_quota_percent 必须在 [0,100]（0 表示解除限制），实际=%v", percent)
	}

	uid, err := lookupUID(ctx, username)
	if err != nil {
		return err
	}

	// 优先使用 systemd（最符合运维习惯）
	if isSystemd() && hasCommand("systemctl") {
		if err := setCPUQuotaBySystemd(ctx, uid, percent); err == nil {
			return a.writeCPUQuotaState(username, percent, reason)
		} else {
			a.logger.Printf("systemd CPUQuota 设置失败，将尝试 cgroup v2：user=%s uid=%d err=%v", username, uid, err)
		}
	}

	// cgroup v2 兜底（直接写 cpu.max）
	if err := setCPUQuotaByCgroupV2(uid, percent); err != nil {
		return err
	}
	return a.writeCPUQuotaState(username, percent, reason)
}

func (a *NodeAgent) writeCPUQuotaState(username string, percent float64, reason string) error {
	homeDir := filepath.Join("/home", username)
	path := filepath.Join(homeDir, ".cpu_quota")
	content := fmt.Sprintf("cpu_quota_percent=%.2f\nreason=%s\n", percent, strings.TrimSpace(reason))
	_ = os.MkdirAll(homeDir, 0755)
	return os.WriteFile(path, []byte(content), 0644)
}

func lookupUID(ctx context.Context, username string) (int, error) {
	cmd := exec.CommandContext(ctx, "id", "-u", username)
	b, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("获取 uid 失败：%w", err)
	}
	uidStr := strings.TrimSpace(string(b))
	uid, err := strconv.Atoi(uidStr)
	if err != nil {
		return 0, fmt.Errorf("解析 uid 失败：%w", err)
	}
	return uid, nil
}

func isSystemd() bool {
	_, err := os.Stat("/run/systemd/system")
	return err == nil
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func setCPUQuotaBySystemd(ctx context.Context, uid int, percent float64) error {
	slice := fmt.Sprintf("user-%d.slice", uid)
	var cmd *exec.Cmd
	if percent <= 0 {
		// 解除限制：把属性清空即可恢复默认（无限制）
		cmd = exec.CommandContext(ctx, "systemctl", "set-property", "--runtime", slice, "CPUQuota=")
	} else {
		value := fmt.Sprintf("CPUQuota=%.2f%%", percent)
		cmd = exec.CommandContext(ctx, "systemctl", "set-property", "--runtime", slice, value)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl set-property 失败：%w（out=%s）", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func setCPUQuotaByCgroupV2(uid int, percent float64) error {
	period := int64(100000) // 100ms
	value := ""
	if percent <= 0 {
		value = fmt.Sprintf("max %d", period)
	} else {
		quota := int64(float64(period) * (percent / 100.0))
		if quota < 1000 {
			quota = 1000
		}
		value = fmt.Sprintf("%d %d", quota, period)
	}

	paths := []string{
		fmt.Sprintf("/sys/fs/cgroup/user.slice/user-%d.slice/cpu.max", uid),
		fmt.Sprintf("/sys/fs/cgroup/user-%d.slice/cpu.max", uid),
	}
	for _, p := range paths {
		if err := os.WriteFile(p, []byte(value), 0644); err == nil {
			return nil
		}
	}
	return fmt.Errorf("未找到可写 cpu.max（uid=%d），请确认系统启用 cgroup v2 且 Agent 以 root 运行", uid)
}
