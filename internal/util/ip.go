package util

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"node-latency/internal/model"
)

func RefreshIPInfo(ctx context.Context, nodes []model.Node, settings model.TestSettings, logf func(string, ...interface{})) {
	if !settings.IPRename {
		return
	}
	urlTemplate := strings.TrimSpace(settings.IPLookupURL)
	if urlTemplate == "" {
		if logf != nil {
			logf("IP查询URL为空，跳过")
		}
		return
	}

	// 统计需要查询的节点数
	totalNodes := len(nodes)
	if totalNodes == 0 {
		return
	}

	if logf != nil {
		logf("开始查询 IP 地理位置（共 %d 个节点）...", totalNodes)
	}

	timeout := settings.IPLookupTimeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 50,
			IdleConnTimeout:    30 * time.Second,
		},
	}

	cache := make(map[string]*model.IPInfo)
	var cacheMu sync.Mutex

	// 提高并发数到 64
	concurrency := 64
	if concurrency > totalNodes {
		concurrency = totalNodes
	}
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	var completed int32

	startTime := time.Now()

	for i := range nodes {
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}

			if ctx.Err() != nil {
				return
			}

			host := nodes[idx].Host
			if host == "" {
				return
			}

			// 解析 IP
			ip := ResolveHostIP(ctx, host, timeout)
			if ip == "" {
				return
			}

			// 检查缓存
			cacheMu.Lock()
			if info, ok := cache[ip]; ok {
				cacheMu.Unlock()
				infoCopy := *info
				infoCopy.FromCache = true
				nodes[idx].IPInfo = &infoCopy
				atomic.AddInt32(&completed, 1)
				return
			}
			cacheMu.Unlock()

			// 查询 IP 信息
			info, err := QueryIPInfo(client, urlTemplate, ip)
			if err != nil {
				atomic.AddInt32(&completed, 1)
				return
			}

			cacheMu.Lock()
			cache[ip] = info
			cacheMu.Unlock()

			nodes[idx].IPInfo = info
			atomic.AddInt32(&completed, 1)
		}(i)
	}

	// 进度报告协程
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		lastCount := int32(0)

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				current := atomic.LoadInt32(&completed)
				if current > lastCount {
					rate := float64(current-lastCount) / 2.0
					if logf != nil && rate > 0 {
						logf("IP 查询进度: %d/%d (%.1f%%)  %.0f 个/秒", current, totalNodes, float64(current)/float64(totalNodes)*100, rate)
					}
					lastCount = current
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
	close(done)

	elapsed := time.Since(startTime)
	if logf != nil {
		finalCount := atomic.LoadInt32(&completed)
		cacheHits := 0
		cacheMu.Lock()
		cacheHits = len(cache)
		cacheMu.Unlock()
		logf("IP 查询完成: %d/%d 成功, 缓存 %d 个 IP, 耗时 %.1f 秒", finalCount, totalNodes, cacheHits, elapsed.Seconds())
	}
}

func ResolveHostIP(parent context.Context, host string, timeout time.Duration) string {
	// 如果已经是 IP 地址，直接返回
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}

	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return ""
	}

	// 优先返回 IPv4
	for _, addr := range addrs {
		if addr.IP.To4() != nil {
			return addr.IP.String()
		}
	}

	return addrs[0].IP.String()
}

func QueryIPInfo(client *http.Client, urlTemplate, ip string) (*model.IPInfo, error) {
	urlStr := strings.ReplaceAll(urlTemplate, "{ip}", ip)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "node-latency-gui/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("IP查询失败：%s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	// 检查错误状态
	if status := GetStringFromMap(raw, "status"); status != "" && status != "success" {
		msg := GetStringFromMap(raw, "message")
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("IP查询失败：%s", msg)
	}

	info := &model.IPInfo{
		IP:      FirstNonEmpty(GetStringFromMap(raw, "query"), GetStringFromMap(raw, "ip"), ip),
		Country: FirstNonEmpty(GetStringFromMap(raw, "country"), GetStringFromMap(raw, "country_name")),
		Region:  FirstNonEmpty(GetStringFromMap(raw, "regionName"), GetStringFromMap(raw, "region")),
		City:    FirstNonEmpty(GetStringFromMap(raw, "city")),
		ISP:     FirstNonEmpty(GetStringFromMap(raw, "isp")),
		Org:     FirstNonEmpty(GetStringFromMap(raw, "org")),
		ASN:     FirstNonEmpty(GetStringFromMap(raw, "as")),
	}

	if v, ok := GetBoolFromMap(raw, "hosting"); ok {
		info.Hosting = v
	}
	if v, ok := GetBoolFromMap(raw, "proxy"); ok {
		info.Proxy = v
	}
	if v, ok := GetBoolFromMap(raw, "mobile"); ok {
		info.Mobile = v
	}

	// 处理 privacy 字段（ip-api.com 格式）
	if privacy, ok := raw["privacy"].(map[string]interface{}); ok {
		if v, ok := GetBoolFromMap(privacy, "hosting"); ok {
			info.Hosting = v
		}
		if v, ok := GetBoolFromMap(privacy, "proxy"); ok {
			info.Proxy = v
		}
		if v, ok := GetBoolFromMap(privacy, "vpn"); ok && v {
			info.Proxy = true
		}
	}

	return info, nil
}
