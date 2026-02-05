package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

func (a *NodeAgent) ExecuteAction(ctx context.Context, action Action) error {
	switch action.Type {
	case "notify":
		return a.writeNotice(action.Username, action.Message)
	case "block_user":
		return a.blockUserGPUAccess(action.Username, action.Reason)
	case "unblock_user":
		return a.unblockUserGPUAccess(action.Username)
	case "set_cpu_quota":
		return a.setUserCPUQuota(ctx, action.Username, action.CPUQuotaPercent, action.Reason)
	case "kill_process":
		return a.killProcesses(ctx, action.Username, action.PIDs, action.Reason)
	default:
		return fmt.Errorf("未知 action.type：%s", action.Type)
	}
}

func (a *NodeAgent) writeNotice(username string, message string) error {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(message) == "" {
		return nil
	}
	homeDir := filepath.Join("/home", username)
	noticeFile := filepath.Join(homeDir, ".gpu_notice")
	content := fmt.Sprintf("%s\n%s\n", time.Now().Format(time.RFC3339), message)
	return os.WriteFile(noticeFile, []byte(content), 0644)
}

func (a *NodeAgent) blockUserGPUAccess(username string, reason string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username 不能为空")
	}
	homeDir := filepath.Join("/home", username)
	flagFile := filepath.Join(homeDir, ".gpu_blocked")
	if strings.TrimSpace(reason) == "" {
		reason = "余额不足，限制新 GPU 任务"
	}
	return os.WriteFile(flagFile, []byte(reason+"\n"), 0644)
}

func (a *NodeAgent) unblockUserGPUAccess(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username 不能为空")
	}
	flagFile := filepath.Join("/home", username, ".gpu_blocked")
	if err := os.Remove(flagFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (a *NodeAgent) killProcesses(ctx context.Context, username string, pids []int32, reason string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username 不能为空")
	}
	if len(pids) == 0 {
		return nil
	}

	log.Printf("执行 kill_process：user=%s pids=%v reason=%s", username, pids, strings.TrimSpace(reason))

	for _, pid := range pids {
		proc, err := process.NewProcess(pid)
		if err != nil {
			continue
		}
		procUser, err := proc.Username()
		if err != nil || procUser != username {
			continue
		}

		_ = syscall.Kill(int(pid), syscall.SIGTERM)
	}

	// 给进程一点退出时间，避免直接 SIGKILL 造成数据损坏
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
	}

	for _, pid := range pids {
		proc, err := process.NewProcess(pid)
		if err != nil {
			continue
		}
		procUser, err := proc.Username()
		if err != nil || procUser != username {
			continue
		}
		if err := syscall.Kill(int(pid), syscall.SIGKILL); err != nil {
			// 可能已经退出，不视为硬错误
			log.Printf("SIGKILL 失败 pid=%d err=%v", pid, err)
		}
	}

	return nil
}
