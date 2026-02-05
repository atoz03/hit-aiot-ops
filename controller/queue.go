package main

import (
	"sync"
	"time"
)

// QueueItem 表示一次 GPU 申请请求。
// 当前实现仅支持排队记录（不做真实分配），用于后续扩展 findAvailableGPU 等策略。
type QueueItem struct {
	Username  string    `json:"username"`
	GPUType   string    `json:"gpu_type"`
	Count     int       `json:"count"`
	Timestamp time.Time `json:"timestamp"`
}

type Queue struct {
	mu    sync.Mutex
	items []QueueItem
}

func NewQueue() *Queue {
	return &Queue{items: make([]QueueItem, 0)}
}

func (q *Queue) Enqueue(item QueueItem) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, item)
	return len(q.items)
}

func (q *Queue) Snapshot() []QueueItem {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]QueueItem, len(q.items))
	copy(out, q.items)
	return out
}

func estimateWaitMinutes(position int) int {
	// 简单估算：每个排队项按 10 分钟计算。后续可改成基于历史分配/作业时长的预测。
	if position <= 0 {
		return 0
	}
	return position * 10
}
