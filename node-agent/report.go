package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *NodeAgent) ReportToController(ctx context.Context, metrics *MetricsData) (*ControllerResponse, error) {
	if err := a.flushPending(ctx); err != nil {
		// 不阻塞本次上报：历史补报失败只做记录
		a.logger.Printf("补报失败（将重试）：%v", err)
	}

	resp, err := a.postMetrics(ctx, metrics)
	if err != nil {
		_ = a.appendPending(metrics)
		return nil, err
	}
	return resp, nil
}

func (a *NodeAgent) postMetrics(ctx context.Context, metrics *MetricsData) (*ControllerResponse, error) {
	body, err := json.Marshal(metrics)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(a.controllerURL, "/") + "/api/metrics"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", a.agentToken)

	res, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 16*1024))
		return nil, fmt.Errorf("控制器返回非 2xx：code=%d body=%s", res.StatusCode, strings.TrimSpace(string(b)))
	}

	var cr ControllerResponse
	if err := json.NewDecoder(res.Body).Decode(&cr); err != nil {
		return nil, err
	}
	return &cr, nil
}

func (a *NodeAgent) appendPending(metrics *MetricsData) error {
	if err := os.MkdirAll(a.stateDir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(a.stateDir, "pending.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.Marshal(metrics)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func (a *NodeAgent) flushPending(ctx context.Context) error {
	path := filepath.Join(a.stateDir, "pending.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	var remaining [][]byte
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var m MetricsData
		if err := json.Unmarshal(line, &m); err != nil {
			// 坏行丢弃，避免卡死队列
			continue
		}
		if _, err := a.postMetrics(ctx, &m); err != nil {
			remaining = append(remaining, append([]byte(nil), line...))
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}

	if len(remaining) == 0 {
		_ = os.Remove(path)
		return nil
	}

	// 保留失败的行（最多 500 条），避免磁盘无限增长
	if len(remaining) > 500 {
		remaining = remaining[len(remaining)-500:]
	}
	var buf bytes.Buffer
	for _, line := range remaining {
		buf.Write(line)
		buf.WriteByte('\n')
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (a *NodeAgent) defaultClient() *http.Client {
	return &http.Client{Timeout: 8 * time.Second}
}
