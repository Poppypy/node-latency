package tester

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"node-latency/internal/clash"
	"node-latency/internal/model"
	"node-latency/internal/util"
)

type coreTester struct {
	cmd       *exec.Cmd
	apiURL    string
	mixedPort int
	tempDir   string
	done      chan error
	logPath   string
	logFile   *os.File
}

const batchSize = 200 // 每批处理的节点数量

// RunTestsWithCore 分批测��节点，避免一次性加载过多节点导致 mihomo 启动过慢
func RunTestsWithCore(ctx context.Context, nodes []model.Node, settings model.TestSettings, onResult func(idx int, res model.Result), logf func(string, ...interface{})) {
	skipFlows := map[string]struct{}{
		"xtls-rprx-direct": {},
		"xtls-rprx-origin": {},
		"xtls-rprx-splice": {},
	}

	corePath := settings.CorePath
	if corePath == "" {
		resolved, err := resolveCorePath("")
		if err != nil {
			if logf != nil {
				logf("未找到内核：%v", err)
			}
			// 所有节点标记失败
			for i := range nodes {
				onResult(i, model.Result{Done: true, Pass: false, Err: err.Error(), Attempts: settings.Attempts})
			}
			return
		}
		corePath = resolved
	}

	// 预处理：过滤和构建代理
	badNodeReasons := make(map[int]string)
	allProxies, allProxyNames, proxyNameByIndex, skipReasons, warnings := buildTestProxies(nodes, skipFlows, badNodeReasons)

	for _, warn := range warnings {
		if logf != nil {
			logf("跳过节点：%s", warn)
		}
	}

	// 先报告被跳过的节点
	for i, reason := range skipReasons {
		if reason != "" {
			onResult(i, model.Result{Done: true, Pass: false, Err: reason, Attempts: settings.Attempts})
		}
	}

	if len(allProxies) == 0 {
		if logf != nil {
			logf("没有可测试的节点")
		}
		return
	}

	// 收集需要测试的节点索引
	var testIndices []int
	for i, name := range proxyNameByIndex {
		if name != "" && skipReasons[i] == "" {
			testIndices = append(testIndices, i)
		}
	}

	// 如果不使用批次模式，一次性加载所有节点
	if !settings.UseBatchMode {
		if logf != nil {
			logf("共 %d 个节点需要测试，启动内核：%s", len(testIndices), corePath)
		}

		// 构建索引映射
		indexMapping := make(map[int]int)
		for i, globalIdx := range testIndices {
			indexMapping[i] = globalIdx
		}

		// 启动测试
		tester, err := startCoreTester(ctx, corePath, allProxies, allProxyNames, settings.CoreStartTimeout)
		if err != nil {
			if logf != nil {
				logf("内核启动失败：%v", err)
			}
			// 全部标记失败
			for _, globalIdx := range testIndices {
				onResult(globalIdx, model.Result{Done: true, Pass: false, Err: "内核启动失败: " + err.Error(), Attempts: settings.Attempts})
			}
			return
		}
		defer tester.Close()

		// 测试所有节点
		testBatch(ctx, tester, allProxies, allProxyNames, indexMapping, settings, onResult, logf)

		if logf != nil {
			logf("测试完成")
		}
		return
	}

	// 批次模式
	if logf != nil {
		logf("共 %d 个节点需要测试，分批处理（每批 %d 个）", len(testIndices), batchSize)
	}

	// 分批处理
	for batchStart := 0; batchStart < len(testIndices); batchStart += batchSize {
		if ctx.Err() != nil {
			break
		}

		batchEnd := batchStart + batchSize
		if batchEnd > len(testIndices) {
			batchEnd = len(testIndices)
		}

		batchIndices := testIndices[batchStart:batchEnd]

		// 构建这一批的代理配置
		var batchProxies []interface{}
		var batchProxyNames []string
		indexMapping := make(map[int]int) // 批次内索引 -> 原始索引

		for _, globalIdx := range batchIndices {
			proxyName := proxyNameByIndex[globalIdx]
			if proxyName == "" {
				continue
			}

			// 找到对应的代理配置
			proxyIdx := -1
			for i, name := range allProxyNames {
				if name == proxyName {
					proxyIdx = i
					break
				}
			}
			if proxyIdx >= 0 && proxyIdx < len(allProxies) {
				batchProxies = append(batchProxies, allProxies[proxyIdx])
				batchProxyNames = append(batchProxyNames, proxyName)
				indexMapping[len(batchProxies)-1] = globalIdx
			}
		}

		if len(batchProxies) == 0 {
			continue
		}

		if logf != nil {
			logf("启动内核（批次 %d-%d/%d）：%s", batchStart+1, batchEnd, len(testIndices), corePath)
		}

		// 启动这一批的测试
		tester, err := startCoreTester(ctx, corePath, batchProxies, batchProxyNames, settings.CoreStartTimeout)
		if err != nil {
			if logf != nil {
				logf("内核启动失败：%v", err)
			}
			// 这一批全部标记失败
			for _, globalIdx := range batchIndices {
				onResult(globalIdx, model.Result{Done: true, Pass: false, Err: "内核启动失败: " + err.Error(), Attempts: settings.Attempts})
			}
			continue
		}

		// 测试这一批
		testBatch(ctx, tester, batchProxies, batchProxyNames, indexMapping, settings, onResult, logf)

		// 关闭内核
		tester.Close()

		if logf != nil {
			logf("批次 %d-%d 测试完成", batchStart+1, batchEnd)
		}
	}
}

func testBatch(ctx context.Context, tester *coreTester, proxies []interface{}, proxyNames []string, indexMapping map[int]int, settings model.TestSettings, onResult func(idx int, res model.Result), logf func(string, ...interface{})) {
	// 配置更大的连接池
	transport := &http.Transport{
		MaxIdleConns:        256,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true,
	}
	client := &http.Client{
		Timeout:   settings.Timeout + 2*time.Second,
		Transport: transport,
	}
	jobs := make(chan int)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for localIdx := range jobs {
			if ctx.Err() != nil {
				return
			}
			name := proxyNames[localIdx]
			if name == "" {
				continue
			}

			globalIdx, ok := indexMapping[localIdx]
			if !ok {
				continue
			}

			res := TestNodeWithMeasure(ctx, settings, func(timeout time.Duration) (time.Duration, error) {
				return coreDelay(client, tester.apiURL, name, settings.CoreTestURL, timeout)
			})
			onResult(globalIdx, res)
		}
	}

	workers := settings.Concurrency
	if workers <= 0 {
		workers = 1
	}
	// 限制最大并发数为 64，避免资源耗尽
	if workers > 64 {
		workers = 64
	}
	if workers > len(proxies) {
		workers = len(proxies)
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker()
	}

	for i := range proxies {
		if ctx.Err() != nil {
			break
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

func buildTestProxies(nodes []model.Node, skipFlows map[string]struct{}, badNodeReasons map[int]string) ([]interface{}, []string, []string, []string, []string) {
	var proxies []interface{}
	var proxyNames []string
	proxyNameByIndex := make([]string, len(nodes))
	skipReasons := make([]string, len(nodes))
	nameCount := make(map[string]int)
	var warnings []string

	for i, node := range nodes {
		if badNodeReasons != nil {
			if msg, banned := badNodeReasons[i]; banned {
				if strings.TrimSpace(msg) == "" {
					msg = "已被内核拒绝"
				}
				skipReasons[i] = msg
				warnings = append(warnings, fmt.Sprintf("第%d个节点：%s", i+1, msg))
				continue
			}
		}

		base := strings.TrimSpace(node.Name)
		if base == "" {
			base = fmt.Sprintf("节点_%d", i+1)
		}
		name := uniqueName(base, nameCount)

		proxy, err := clash.NodeToClashProxy(node, name)
		if err != nil {
			msg := err.Error()
			skipReasons[i] = msg
			warnings = append(warnings, fmt.Sprintf("第%d个节点：%s", i+1, msg))
			continue
		}

		if flow := extractProxyFlow(proxy); flow != "" {
			flow = strings.ToLower(strings.TrimSpace(flow))
			if strings.Contains(flow, "xtls-rprx-direct") || strings.Contains(flow, "xtls-rprx-origin") {
				msg := fmt.Sprintf("flow=%s（Legacy XTLS 已废弃，mihomo 不支持）", flow)
				skipReasons[i] = msg
				warnings = append(warnings, fmt.Sprintf("第%d个节点：%s", i+1, msg))
				continue
			}
			if shouldSkipFlow(flow, skipFlows) {
				msg := fmt.Sprintf("flow=%s（内核不支持）", flow)
				skipReasons[i] = msg
				warnings = append(warnings, fmt.Sprintf("第%d个节点：%s", i+1, msg))
				continue
			}
		}

		if err := clash.ValidateClashProxy(proxy); err != nil {
			msg := err.Error()
			skipReasons[i] = msg
			warnings = append(warnings, fmt.Sprintf("第%d个节点：%s", i+1, msg))
			continue
		}

		proxyNameByIndex[i] = name
		proxies = append(proxies, proxy)
		proxyNames = append(proxyNames, name)
	}

	return proxies, proxyNames, proxyNameByIndex, skipReasons, warnings
}

func uniqueName(base string, count map[string]int) string {
	name := strings.TrimSpace(base)
	if name == "" {
		name = "节点"
	}
	if count[name] == 0 {
		count[name] = 1
		return name
	}
	count[name]++
	return fmt.Sprintf("%s_%d", name, count[name])
}

func resolveCorePath(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input != "" {
		if p, err := exec.LookPath(input); err == nil {
			return p, nil
		}
		if _, err := os.Stat(input); err == nil {
			abs, _ := filepath.Abs(input)
			return abs, nil
		}
		return "", fmt.Errorf("未找到内核：%s", input)
	}

	candidates := []string{
		"mihomo-windows-amd64-v3.exe",
		"mihomo-windows-amd64.exe",
		"mihomo.exe",
		"mihomo",
		"clash-meta.exe",
		"clash-meta",
		"clash.exe",
		"clash",
	}

	// 1. 检查可执行文件所在目录
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		for _, exe := range candidates {
			candidate := filepath.Join(exeDir, exe)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}

	// 2. 检查当前工作目录
	if wd, err := os.Getwd(); err == nil {
		for _, exe := range candidates {
			candidate := filepath.Join(wd, exe)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}

	// 3. 检查 PATH
	for _, exe := range candidates {
		if p, err := exec.LookPath(exe); err == nil {
			return p, nil
		}
	}

	return "", errors.New("未找到 Mihomo 内核，请将 mihomo.exe 放到程序同目录")
}

func extractProxyFlow(proxy map[string]interface{}) string {
	if proxy == nil {
		return ""
	}
	v, ok := proxy["flow"]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	}
}

func shouldSkipFlow(flow string, skipFlows map[string]struct{}) bool {
	flow = strings.TrimSpace(flow)
	if flow == "" || skipFlows == nil {
		return false
	}
	_, ok := skipFlows[strings.ToLower(flow)]
	return ok
}

func extractUnsupportedXtlsFlow(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	key := "unsupported xtls flow type:"
	idx := strings.Index(msg, key)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(msg[idx+len(key):])
	if rest == "" {
		return ""
	}
	if cut := strings.IndexAny(rest, "\r\n"); cut >= 0 {
		rest = rest[:cut]
	}
	rest = strings.TrimSpace(strings.Trim(rest, "\"'"))
	return rest
}

func extractProxyParseError(err error) (int, string) {
	if err == nil {
		return 0, ""
	}
	msg := err.Error()
	if msg == "" {
		return 0, ""
	}
	lines := strings.Split(msg, "\n")
	key := "parse config error: proxy "
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		low := strings.ToLower(l)
		pos := strings.Index(low, key)
		if pos < 0 {
			continue
		}
		rest := l[pos+len(key):]
		i := 0
		for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
			i++
		}
		if i == 0 {
			continue
		}
		n, err2 := strconv.Atoi(rest[:i])
		if err2 != nil || n <= 0 {
			continue
		}
		reason := strings.TrimSpace(rest[i:])
		reason = strings.TrimPrefix(reason, ":")
		reason = strings.TrimSpace(reason)
		return n, reason
	}
	return 0, ""
}

func pickFreePort() (int, error) {
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

func startCoreTester(ctx context.Context, corePath string, proxies []interface{}, proxyNames []string, startTimeout time.Duration) (*coreTester, error) {
	if len(proxies) == 0 {
		return nil, errors.New("没有可测试的节点")
	}

	apiPort, err := pickFreePort()
	if err != nil {
		return nil, err
	}
	mixedPort, err := pickFreePort()
	if err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "node-latency-core-")
	if err != nil {
		return nil, err
	}
	logPath := filepath.Join(tempDir, "core.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	cfg := map[string]interface{}{
		"mixed-port":          mixedPort,
		"external-controller": fmt.Sprintf("127.0.0.1:%d", apiPort),
		"mode":                "rule",
		"log-level":           "error",
		"proxies":             proxies,
		"proxy-groups": []interface{}{
			map[string]interface{}{
				"name":    "AUTO",
				"type":    "select",
				"proxies": proxyNames,
			},
		},
		"rules": []interface{}{
			"MATCH,AUTO",
		},
	}
	cfg = util.SanitizeYAMLMap(cfg)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}
	data = util.SanitizeYAMLOutput(data)
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		_ = logFile.Close()
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	// 使用独立的上下文启动内核，避免被父上下文取消影响
	coreCtx, coreCancel := context.WithTimeout(context.Background(), startTimeout+30*time.Second)
	_ = coreCancel // cancel 在 defer 中调用，但这里不需要因为 cmd 结束后会自动清理

	cmd := exec.CommandContext(coreCtx, corePath, "-f", configPath, "-d", tempDir)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	// 隐藏 cmd 窗口
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		_ = os.RemoveAll(tempDir)
		return nil, err
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	apiURL := fmt.Sprintf("http://127.0.0.1:%d", apiPort)
	if err := waitForCore(apiURL, startTimeout, done); err != nil {
		if tail := readLogTail(logPath, 120); tail != "" {
			err = fmt.Errorf("%v\n日志:\n%s", err, tail)
		}
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		_ = logFile.Close()
		_ = os.RemoveAll(tempDir)
		return nil, err
	}
	return &coreTester{cmd: cmd, apiURL: apiURL, mixedPort: mixedPort, tempDir: tempDir, done: done, logPath: logPath, logFile: logFile}, nil
}

func waitForCore(apiURL string, timeout time.Duration, done <-chan error) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			if err == nil {
				return errors.New("内核已退出")
			}
			return fmt.Errorf("内核已退出：%v", err)
		default:
		}
		req, _ := http.NewRequest("GET", apiURL+"/version", nil)
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("内核启动超时")
}

func (c *coreTester) Close() {
	if c == nil {
		return
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		if c.done != nil {
			select {
			case <-c.done:
			case <-time.After(2 * time.Second):
			}
		}
	}
	if c.logFile != nil {
		_ = c.logFile.Close()
	}
	if c.tempDir != "" {
		_ = os.RemoveAll(c.tempDir)
	}
}

func readLogTail(path string, maxLines int) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	if len(data) > 65536 {
		data = data[len(data)-65536:]
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// QueryExitIPInfo 查询通过指定代理的出口 IP 信息
func QueryExitIPInfo(ctx context.Context, corePath string, nodes []model.Node, settings model.TestSettings, onProgress func(done, total int), logf func(string, ...interface{})) map[int]*model.IPInfo {
	results := make(map[int]*model.IPInfo)
	if len(nodes) == 0 {
		return results
	}

	// 构建代理配置
	var proxies []interface{}
	var proxyNames []string
	proxyNameByIndex := make(map[int]string) // node index -> proxy name
	nameCount := make(map[string]int)

	for i, node := range nodes {
		base := strings.TrimSpace(node.Name)
		if base == "" {
			base = fmt.Sprintf("节点_%d", i+1)
		}
		name := uniqueName(base, nameCount)

		proxy, err := clash.NodeToClashProxy(node, name)
		if err != nil {
			continue
		}

		if err := clash.ValidateClashProxy(proxy); err != nil {
			continue
		}

		proxyNameByIndex[i] = name
		proxies = append(proxies, proxy)
		proxyNames = append(proxyNames, name)
	}

	if len(proxies) == 0 {
		if logf != nil {
			logf("没有可用于查询出口 IP 的节点")
		}
		return results
	}

	if logf != nil {
		logf("开始查询 %d 个节点的出口 IP...", len(proxies))
	}

	// 解析内核路径
	resolvedCorePath := corePath
	if resolvedCorePath == "" {
		var err error
		resolvedCorePath, err = resolveCorePath("")
		if err != nil {
			if logf != nil {
				logf("未找到内核：%v", err)
			}
			return results
		}
	}

	// 启动 Mihomo 内核
	tester, err := startCoreTester(ctx, resolvedCorePath, proxies, proxyNames, settings.CoreStartTimeout)
	if err != nil {
		if logf != nil {
			logf("启动内核查询出口 IP 失败：%v", err)
		}
		return results
	}
	defer tester.Close()

	client := &http.Client{Timeout: 10 * time.Second}
	ipURL := settings.IPLookupURL
	if ipURL == "" {
		ipURL = model.DefaultIPLookupURL
	}
	// 将 {ip} 替换为空，因为我们要查的是出口 IP
	ipURL = strings.ReplaceAll(ipURL, "{ip}", "")

	var mu sync.Mutex           // 保护 results 和 completed
	var proxyMu sync.Mutex       // 保护代理切换 + 请求的原子性
	var wg sync.WaitGroup
	sem := make(chan struct{}, 20) // 并发限制
	completed := 0

	for i, proxyName := range proxyNames {
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)
		go func(nodeIdx int, name string) {
			defer wg.Done()
			defer func() { <-sem }()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}

			// 使用锁保证"切换代理 + 发请求"是原子操作
			proxyMu.Lock()
			info := queryProxyExitIP(ctx, client, tester.apiURL, tester.mixedPort, name, ipURL)
			proxyMu.Unlock()

			if info != nil {
				mu.Lock()
				results[nodeIdx] = info
				mu.Unlock()
			}

			mu.Lock()
			completed++
			if onProgress != nil {
				onProgress(completed, len(proxyNames))
			}
			mu.Unlock()
		}(i, proxyName)
	}

	wg.Wait()

	if logf != nil {
		logf("出口 IP 查询完成：%d/%d 成功", len(results), len(proxies))
	}

	return results
}

// queryProxyExitIP 通过指定代理查询出口 IP
func queryProxyExitIP(ctx context.Context, client *http.Client, apiURL string, mixedPort int, proxyName, ipURL string) *model.IPInfo {
	// 1. 先切换 AUTO 组到指定代理
	selectURL := fmt.Sprintf("%s/proxies/AUTO", apiURL)
	selectBody := fmt.Sprintf(`{"name":"%s"}`, proxyName)
	req, err := http.NewRequestWithContext(ctx, "PUT", selectURL, strings.NewReader(selectBody))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	resp.Body.Close()

	// 2. 通过 mixed-port 发送请求到 IP API
	// 使用 HTTP 代理
	proxyURL := fmt.Sprintf("http://127.0.0.1:%d", mixedPort)
	proxy, err := url.Parse(proxyURL)
	if err != nil {
		return nil
	}

	proxyClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxy),
		},
	}

	req, err = http.NewRequestWithContext(ctx, "GET", ipURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "node-latency/1.0")

	resp, err = proxyClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}

	return parseIPInfoFromResponse(body)
}

// parseIPInfoFromResponse 从响应中解析 IP 信息
func parseIPInfoFromResponse(body []byte) *model.IPInfo {
	// 先尝试解析 Mihomo delay 响应格式
	var delayResp struct {
		Delay   int64  `json:"delay"`
		Message string `json:"message"`
	}

	// 如果是 delay 格式，message 中可能包含 IP API 的响应
	if err := json.Unmarshal(body, &delayResp); err == nil && delayResp.Delay > 0 {
		// delay 响应，无法获取 IP 信息
		return nil
	}

	// 尝试直接解析为 IP API 响应
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}

	// 检查是否是有效的 IP API 响应
	status := ""
	if s, ok := raw["status"].(string); ok {
		status = s
	}
	if status != "" && status != "success" {
		return nil
	}

	info := &model.IPInfo{
		IP:      firstString(raw, "query", "ip"),
		Country: firstString(raw, "country", "country_name"),
		Region:  firstString(raw, "regionName", "region"),
		City:    firstString(raw, "city"),
		ISP:     firstString(raw, "isp"),
		Org:     firstString(raw, "org"),
		ASN:     firstString(raw, "as"),
	}

	if v, ok := raw["hosting"].(bool); ok {
		info.Hosting = v
	}
	if v, ok := raw["proxy"].(bool); ok {
		info.Proxy = v
	}
	if v, ok := raw["mobile"].(bool); ok {
		info.Mobile = v
	}

	// 验证至少有 IP
	if info.IP == "" {
		return nil
	}

	return info
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func coreDelay(client *http.Client, apiURL, proxyName, testURL string, timeout time.Duration) (time.Duration, error) {
	if client == nil {
		return 0, errors.New("http client is nil")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	reqURL := fmt.Sprintf("%s/proxies/%s/delay?timeout=%d&url=%s",
		apiURL,
		url.PathEscape(proxyName),
		timeout.Milliseconds(),
		url.QueryEscape(testURL),
	)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return 0, fmt.Errorf("delay接口错误：%s", msg)
	}
	var data struct {
		Delay   int64  `json:"delay"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, err
	}
	if data.Delay <= 0 {
		if strings.TrimSpace(data.Message) != "" {
			return 0, errors.New(strings.TrimSpace(data.Message))
		}
		return 0, errors.New("delay返回无效")
	}
	return time.Duration(data.Delay) * time.Millisecond, nil
}
