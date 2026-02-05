package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/process"
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
	if err := setCPUQuotaByCgroupV2(uid, percent); err == nil {
		// 尝试把该用户进程归入目标 cgroup（仅当目录存在且可写时）
		_ = moveUserProcsToCgroupV2(uid, username)
		return a.writeCPUQuotaState(username, percent, reason)
	}

	// cgroup v1 兜底（cpu.cfs_*）：兼容无法升级到 cgroup v2 的老系统
	if err := setCPUQuotaByCgroupV1(uid, username, percent); err == nil {
		return a.writeCPUQuotaState(username, percent, reason)
	}

	return fmt.Errorf("CPU 限流失败：需要 systemd CPUQuota 或 cgroup v2/cgroup v1（uid=%d）", uid)
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

func setCPUQuotaByCgroupV1(uid int, username string, percent float64) error {
	mount, err := findCgroupV1MountPoint("cpu")
	if err != nil {
		return err
	}

	groupDir := filepath.Join(mount, "gpuops", fmt.Sprintf("user-%d", uid))
	if err := os.MkdirAll(groupDir, 0755); err != nil {
		return fmt.Errorf("创建 cgroup v1 目录失败：%w", err)
	}

	period := int64(100000) // 100ms
	quota := int64(-1)      // -1 表示不限制
	if percent > 0 {
		quota = int64(float64(period) * (percent / 100.0))
		if quota < 1000 {
			quota = 1000
		}
	}

	if err := os.WriteFile(filepath.Join(groupDir, "cpu.cfs_period_us"), []byte(fmt.Sprintf("%d", period)), 0644); err != nil {
		return fmt.Errorf("写 cpu.cfs_period_us 失败：%w", err)
	}
	if err := os.WriteFile(filepath.Join(groupDir, "cpu.cfs_quota_us"), []byte(fmt.Sprintf("%d", quota)), 0644); err != nil {
		return fmt.Errorf("写 cpu.cfs_quota_us 失败：%w", err)
	}

	// 把用户进程移动进该 cgroup，才能生效
	return moveUserProcsToTasks(username, filepath.Join(groupDir, "tasks"))
}

func findCgroupV1MountPoint(controller string) (string, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return "", fmt.Errorf("无法读取 /proc/mounts：%w", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		// 典型格式：cgroup /sys/fs/cgroup/cpu cgroup rw,nosuid,nodev,noexec,relatime,cpu 0 0
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 {
			continue
		}
		mountPoint := fields[1]
		fsType := fields[2]
		options := fields[3]
		if fsType != "cgroup" {
			continue
		}
		if optionHasController(options, controller) {
			return mountPoint, nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("扫描 /proc/mounts 失败：%w", err)
	}
	return "", fmt.Errorf("未找到 cgroup v1 %s 控制器挂载点", controller)
}

func optionHasController(options string, controller string) bool {
	for _, opt := range strings.Split(options, ",") {
		if opt == controller {
			return true
		}
	}
	return false
}

func moveUserProcsToTasks(username string, tasksPath string) error {
	if _, err := os.Stat(tasksPath); err != nil {
		return fmt.Errorf("tasks 文件不存在：%s", tasksPath)
	}

	procs, err := process.Processes()
	if err != nil {
		return fmt.Errorf("获取进程列表失败：%w", err)
	}

	f, err := os.OpenFile(tasksPath, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开 tasks 失败：%w", err)
	}
	defer f.Close()

	for _, p := range procs {
		u, err := p.Username()
		if err != nil || u != username {
			continue
		}
		// 向 tasks 写入 PID 即可迁移（失败不致命）
		_, _ = f.WriteString(fmt.Sprintf("%d\n", p.Pid))
	}
	return nil
}

func moveUserProcsToCgroupV2(uid int, username string) error {
	paths := []string{
		fmt.Sprintf("/sys/fs/cgroup/user.slice/user-%d.slice/cgroup.procs", uid),
		fmt.Sprintf("/sys/fs/cgroup/user-%d.slice/cgroup.procs", uid),
	}
	var target string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			target = p
			break
		}
	}
	if target == "" {
		return nil
	}

	procs, err := process.Processes()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, p := range procs {
		u, err := p.Username()
		if err != nil || u != username {
			continue
		}
		_, _ = f.WriteString(fmt.Sprintf("%d\n", p.Pid))
	}
	return nil
}
