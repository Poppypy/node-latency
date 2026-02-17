package util

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func FetchSubscription(urlStr string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}

	// 多种 User-Agent 尝试
	userAgents := []string{
		"ClashMeta/1.14.4",
		"ClashforWindows/0.20.39",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Clash/1.18.0",
	}

	var lastErr error
	for i, ua := range userAgents {
		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Connection", "keep-alive")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			// EOF 错误时尝试下一个 User-Agent
			if strings.Contains(err.Error(), "EOF") && i < len(userAgents)-1 {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return "", fmt.Errorf("订阅请求失败：%v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("订阅请求失败：%s", resp.Status)
			if i < len(userAgents)-1 {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return "", lastErr
		}

		var reader io.Reader = resp.Body
		// 处理 gzip 压缩
		if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
			gzReader, err := gzip.NewReader(resp.Body)
			if err != nil {
				return "", fmt.Errorf("解压失败：%v", err)
			}
			defer gzReader.Close()
			reader = gzReader
		}

		body, err := io.ReadAll(reader)
		if err != nil {
			return "", fmt.Errorf("读取响应失败：%v", err)
		}
		return string(body), nil
	}

	return "", lastErr
}

func PickFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("无法获取本地端口")
	}
	return addr.Port, nil
}

func IsHTTPURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Host == "" {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}
