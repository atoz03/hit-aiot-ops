package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type PriceIndex struct {
	// models 按长度倒序，避免 "RTX 30" 抢先匹配 "RTX 3090"
	models []string
	price  map[string]float64
}

func NewPriceIndex(rows []PriceRow) PriceIndex {
	price := make(map[string]float64, len(rows))
	models := make([]string, 0, len(rows))
	for _, r := range rows {
		m := strings.TrimSpace(r.Model)
		if m == "" {
			continue
		}
		price[m] = r.Price
		models = append(models, m)
	}
	sort.Slice(models, func(i, j int) bool {
		if len(models[i]) == len(models[j]) {
			return models[i] > models[j]
		}
		return len(models[i]) > len(models[j])
	})
	return PriceIndex{models: models, price: price}
}

func (pi PriceIndex) MatchPrice(gpuModel string) (float64, bool) {
	for _, m := range pi.models {
		if strings.Contains(gpuModel, m) {
			return pi.price[m], true
		}
	}
	return 0, false
}

// CalculateProcessCost 计算单个进程在一个采样周期（默认 1 分钟）内的费用。
func CalculateProcessCost(proc UserProcess, prices PriceIndex, defaultPricePerMinute float64) float64 {
	cost := 0.0
	for _, g := range proc.GPUUsage {
		if p, ok := prices.MatchPrice(g.GPUModel); ok {
			cost += p
		} else {
			cost += defaultPricePerMinute
		}
	}
	// 金额保留 4 位小数，便于后续聚合与对账
	return math.Round(cost*10000) / 10000
}

func StatusForBalance(balance, warningThreshold, limitedThreshold float64) string {
	if balance < 0 {
		return "blocked"
	}
	if balance < limitedThreshold {
		return "limited"
	}
	if balance < warningThreshold {
		return "warning"
	}
	return "normal"
}

// DecideActions 根据余额状态决定下发动作。
// 为避免刷屏：warning/limited/blocked 的 notify/block 仅在状态变化时下发；
// blocked 的 kill 在超过宽限期且仍存在 GPU 进程时下发（可重复下发，Agent 应幂等执行）。
func DecideActions(now time.Time, prevStatus string, user User, warningThreshold, limitedThreshold float64, grace time.Duration, pids []int32) []Action {
	newStatus := user.Status
	if newStatus == "" {
		newStatus = StatusForBalance(user.Balance, warningThreshold, limitedThreshold)
	}

	var actions []Action

	// 解除限制：余额恢复后允许继续启动任务
	if (prevStatus == "limited" || prevStatus == "blocked") && (newStatus == "normal" || newStatus == "warning") {
		actions = append(actions, Action{
			Type:     "unblock_user",
			Username: user.Username,
			Reason:   "余额已恢复，解除限制",
		})
	}

	switch newStatus {
	case "warning":
		if prevStatus != "warning" {
			actions = append(actions, Action{
				Type:     "notify",
				Username: user.Username,
				Message:  formatBalanceMessage("余额预警", user.Balance),
			})
		}
	case "limited":
		if prevStatus != "limited" {
			actions = append(actions, Action{
				Type:     "block_user",
				Username: user.Username,
				Reason:   formatBalanceMessage("余额不足，限制新 GPU 任务", user.Balance),
			})
		}
	case "blocked":
		// 首次进入 blocked 先提醒，超过宽限期再 kill
		if prevStatus != "blocked" {
			actions = append(actions, Action{
				Type:     "notify",
				Username: user.Username,
				Message:  formatBalanceMessage("已欠费，宽限期后将终止 GPU 任务", user.Balance),
			})
		}

		if user.BlockedAt != nil && grace > 0 && now.Sub(*user.BlockedAt) >= grace && len(pids) > 0 {
			actions = append(actions, Action{
				Type:     "kill_process",
				Username: user.Username,
				PIDs:     pids,
				Reason:   formatBalanceMessage("欠费超过宽限期，终止 GPU 进程", user.Balance),
			})
		}
	}

	return actions
}

func formatBalanceMessage(prefix string, balance float64) string {
	return strings.TrimSpace(prefix) + "（当前余额：" + formatMoney(balance) + " 元）"
}

func formatMoney(v float64) string {
	// 统一输出两位小数，便于脚本解析与前端展示
	return fmt.Sprintf("%.2f", v)
}
