package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"node-latency/internal/clash"
	"node-latency/internal/model"
	"node-latency/internal/parser"
	"node-latency/internal/tester"
	"node-latency/internal/util"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"
)

// App is the Wails bridge between Go backend and Vue frontend
type App struct {
	ctx       context.Context
	mu        sync.Mutex
	nodes     []model.Node
	results   []model.Result
	settings  model.TestSettings
	cancelRun context.CancelFunc
	running   bool
}

func NewApp() *App {
	return &App{
		settings: model.TestSettings{
			Attempts:         3,
			Threshold:        1500 * time.Millisecond,
			Timeout:          1500 * time.Millisecond,
			Concurrency:      32,
			RequireAll:       true,
			StopOnFail:       true,
			Dedup:            true,
			CoreTestURL:      model.DefaultCoreTestURL,
			CoreStartTimeout: time.Duration(model.DefaultCoreStartTimeoutMs) * time.Millisecond,
		},
	}
}

// startup is called by Wails on app start
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ===== Methods exposed to frontend =====

// ImportFromText parses text/subscription content and returns node list
func (a *App) ImportFromText(text string) ([]model.NodeDTO, error) {
	nodes, warnings, err := parser.ParseNodesFromText(text)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	settings := a.settings
	a.mu.Unlock()

	if settings.ExcludeEnabled && len(settings.ExcludeKeywords) > 0 {
		nodes = util.FilterNodesByKeywords(nodes, settings.ExcludeKeywords)
	}
	if settings.Dedup {
		nodes = util.DedupNodes(nodes)
	}
	nodes = util.ReindexNodes(nodes)

	a.mu.Lock()
	a.nodes = nodes
	a.results = make([]model.Result, len(nodes))
	a.mu.Unlock()

	for _, w := range warnings {
		wailsRuntime.EventsEmit(a.ctx, "log", w)
	}
	return model.ToNodeDTOs(nodes), nil
}

// ImportFromFile opens a file dialog and imports nodes
func (a *App) ImportFromFile() ([]model.NodeDTO, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择节点文件",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "节点文件", Pattern: "*.txt;*.yaml;*.yml;*.conf"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return a.ImportFromText(string(data))
}

// ImportFromSubscription fetches a subscription URL and imports nodes
func (a *App) ImportFromSubscription(urlStr string) ([]model.NodeDTO, error) {
	text, err := util.FetchSubscription(urlStr, 30*time.Second)
	if err != nil {
		return nil, err
	}
	return a.ImportFromText(text)
}

// ImportMultipleSubscriptions fetches multiple subscription URLs and imports nodes
func (a *App) ImportMultipleSubscriptions(urls []string) ([]model.NodeDTO, error) {
	var allText strings.Builder
	var failedUrls []string

	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		text, err := util.FetchSubscription(u, 30*time.Second)
		if err != nil {
			failedUrls = append(failedUrls, fmt.Sprintf("%s: %v", u, err))
			continue
		}
		allText.WriteString(text)
		allText.WriteString("\n")
	}

	if allText.Len() == 0 {
		if len(failedUrls) > 0 {
			return nil, fmt.Errorf("所有订阅获取失败:\n%s", strings.Join(failedUrls, "\n"))
		}
		return nil, errors.New("没有有效的订阅链接")
	}

	// 报告错误但继续导入
	for _, e := range failedUrls {
		wailsRuntime.EventsEmit(a.ctx, "log", e)
	}

	return a.ImportFromText(allText.String())
}

// ImportMultipleFiles opens a multiple file dialog and imports nodes
func (a *App) ImportMultipleFiles() ([]model.NodeDTO, error) {
	paths, err := wailsRuntime.OpenMultipleFilesDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择节点文件（可多选）",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "节点文件", Pattern: "*.txt;*.yaml;*.yml;*.conf"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}

	var allText strings.Builder
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "log", fmt.Sprintf("读取文件失败 %s: %v", path, err))
			continue
		}
		allText.WriteString(string(data))
		allText.WriteString("\n")
	}

	if allText.Len() == 0 {
		return nil, errors.New("没有读取到任何文件内容")
	}

	return a.ImportFromText(allText.String())
}

// ClearNodes clears all nodes
func (a *App) ClearNodes() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nodes = nil
	a.results = nil
}

// DeleteNodes deletes nodes by indices
func (a *App) DeleteNodes(indices []int) ([]model.NodeDTO, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.nodes) == 0 {
		return nil, nil
	}

	// 创建要删除的索引集合
	deleteSet := make(map[int]struct{})
	for _, idx := range indices {
		if idx >= 0 && idx < len(a.nodes) {
			deleteSet[idx] = struct{}{}
		}
	}

	// 保留未删除的节点
	var newNodes []model.Node
	for i, node := range a.nodes {
		if _, deleted := deleteSet[i]; !deleted {
			newNodes = append(newNodes, node)
		}
	}

	// 重新索引
	newNodes = util.ReindexNodes(newNodes)
	a.nodes = newNodes
	a.results = make([]model.Result, len(newNodes))

	return model.ToNodeDTOs(newNodes), nil
}

// GetNodes returns current nodes
func (a *App) GetNodes() []model.NodeDTO {
	a.mu.Lock()
	defer a.mu.Unlock()
	return model.ToNodeDTOs(a.nodes)
}

// UpdateSettings updates test settings from frontend
func (a *App) UpdateSettings(settings model.TestSettings) {
	a.mu.Lock()
	a.settings = settings
	a.mu.Unlock()
}

// GetSettings returns current settings
func (a *App) GetSettings() model.TestSettings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings
}

// StartTest launches the test, pushing results via events
func (a *App) StartTest() error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return errors.New("测试正在进行中")
	}
	a.running = true
	a.results = make([]model.Result, len(a.nodes))
	settings := a.settings
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelRun = cancel
	nodes := make([]model.Node, len(a.nodes))
	copy(nodes, a.nodes)
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.running = false
			a.cancelRun = nil
			a.mu.Unlock()
			wailsRuntime.EventsEmit(a.ctx, "test-complete", nil)
		}()

		done, passed := 0, 0
		total := len(nodes)

		logf := func(format string, args ...interface{}) {
			wailsRuntime.EventsEmit(a.ctx, "log", fmt.Sprintf(format, args...))
		}

		tester.RunTests(ctx, nodes, settings, func(idx int, res model.Result) {
			a.mu.Lock()
			if idx < len(a.results) {
				a.results[idx] = res
			}
			a.mu.Unlock()
			done++
			if res.Pass {
				passed++
			}

			wailsRuntime.EventsEmit(a.ctx, "test-result", model.TestResultEvent{
				Index:      idx,
				Done:       res.Done,
				Pass:       res.Pass,
				Err:        res.Err,
				AvgMs:      res.AvgMs,
				MaxMs:      res.MaxMs,
				LatencyMs:  res.LatencyMs,
				Attempts:   res.Attempts,
				Successful: res.Successful,
			})

			wailsRuntime.EventsEmit(a.ctx, "test-progress", model.TestProgressEvent{
				Total:   total,
				Done:    done,
				Passed:  passed,
				Running: true,
			})
		}, logf)

		// 测试完成后，对通过的节点查询出口 IP 并重命名
		if settings.IPRename && passed > 0 {
			a.mu.Lock()
			currentNodes := a.nodes
			currentResults := a.results
			a.mu.Unlock()

			// 收集通过的节点
			var passingNodes []model.Node
			var passingIndices []int // 记录在原数组中的索引
			for i, node := range currentNodes {
				if i < len(currentResults) && currentResults[i].Pass {
					passingNodes = append(passingNodes, node)
					passingIndices = append(passingIndices, i)
				}
			}

			if len(passingNodes) > 0 {
				logf("开始查询 %d 个通过节点的出口 IP...", len(passingNodes))

				// 查询出口 IP
				ipInfoMap := tester.QueryExitIPInfo(ctx, settings.CorePath, passingNodes, settings,
					func(done, total int) {
						wailsRuntime.EventsEmit(a.ctx, "ip-lookup-progress", map[string]int{
							"done":  done,
							"total": total,
						})
					}, logf)

				// 更新节点的 IPInfo 并重命名
				a.mu.Lock()
				for i, nodeIdx := range passingIndices {
					if info, ok := ipInfoMap[i]; ok && info != nil {
						a.nodes[nodeIdx].IPInfo = info
					}
				}

				// 应用重命名
				util.ApplyRegionsAndNames(a.nodes, settings)
				a.mu.Unlock()

				// 通知前端更新节点列表
				wailsRuntime.EventsEmit(a.ctx, "nodes-updated", model.ToNodeDTOs(a.nodes))
			}
		}
	}()
	return nil
}

// StopTest cancels all running tests and external processes via context
func (a *App) StopTest() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancelRun != nil {
		a.cancelRun()
	}
}

// IsRunning returns whether a test is in progress
func (a *App) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// ExportClashYAML exports passing nodes as Clash YAML config string
func (a *App) ExportClashYAML() (string, error) {
	a.mu.Lock()
	nodes := a.nodes
	results := a.results
	settings := a.settings
	a.mu.Unlock()

	root, _, err := clash.BuildClashConfig(model.DefaultTemplateFile, nodes, results, settings)
	if err != nil {
		return "", err
	}
	root = util.SanitizeYAMLMap(root)
	data, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	data = util.SanitizeYAMLOutput(data)
	return string(data), nil
}

// ExportNodeLinks exports passing nodes as URL link list
func (a *App) ExportNodeLinks() (string, error) {
	a.mu.Lock()
	nodes := a.nodes
	results := a.results
	settings := a.settings
	a.mu.Unlock()

	lines := clash.BuildFilteredURLList(nodes, results, settings)
	if len(lines) == 0 {
		return "", errors.New("没有节点可导出")
	}
	return strings.Join(lines, "\n"), nil
}

// SaveClashYAML saves Clash config to a file chosen by user
func (a *App) SaveClashYAML() error {
	yamlStr, err := a.ExportClashYAML()
	if err != nil {
		return err
	}
	path, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		Title:           "保存 Clash 配置",
		DefaultFilename: "nodes.yaml",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "YAML 文件", Pattern: "*.yaml;*.yml"},
		},
	})
	if err != nil || path == "" {
		return err
	}
	return os.WriteFile(path, []byte(yamlStr), 0o644)
}

// CopyToClipboard copies text to system clipboard
func (a *App) CopyToClipboard(text string) error {
	return util.SetClipboardText(text)
}

// ExportYAMLFlow exports passing nodes as YAML flow format (inline style)
func (a *App) ExportYAMLFlow() (string, error) {
	a.mu.Lock()
	nodes := a.nodes
	results := a.results
	settings := a.settings
	a.mu.Unlock()

	lines := clash.BuildYAMLFlowList(nodes, results, settings, nil)
	if len(lines) == 0 {
		return "", errors.New("没有节点可导出")
	}
	return strings.Join(lines, "\n"), nil
}

// ExportYAMLFlowFiltered exports passing nodes as YAML flow format with type filter
// typeFilter is a comma-separated list of proxy types to include (e.g., "vless,vmess,trojan")
func (a *App) ExportYAMLFlowFiltered(typeFilter string) (string, error) {
	a.mu.Lock()
	nodes := a.nodes
	results := a.results
	settings := a.settings
	a.mu.Unlock()

	var types []string
	if typeFilter != "" {
		for _, t := range strings.Split(typeFilter, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				types = append(types, t)
			}
		}
	}

	lines := clash.BuildYAMLFlowList(nodes, results, settings, types)
	if len(lines) == 0 {
		return "", errors.New("没有符合条件的节点可导出")
	}
	return strings.Join(lines, "\n"), nil
}

// ExportNodeLinksFiltered exports passing nodes as URL links with type filter
// typeFilter is a comma-separated list of proxy types to include (e.g., "vless,vmess,trojan")
func (a *App) ExportNodeLinksFiltered(typeFilter string) (string, error) {
	a.mu.Lock()
	nodes := a.nodes
	results := a.results
	settings := a.settings
	a.mu.Unlock()

	// Filter nodes by type
	typeSet := make(map[string]struct{})
	if typeFilter != "" {
		for _, t := range strings.Split(typeFilter, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				typeSet[strings.ToLower(t)] = struct{}{}
			}
		}
	}

	var filteredNodes []model.Node
	var filteredResults []model.Result

	for i, node := range nodes {
		if i >= len(results) {
			continue
		}
		res := results[i]
		if !res.Done || !res.Pass {
			continue
		}

		// Type filtering
		if len(typeSet) > 0 {
			if _, ok := typeSet[strings.ToLower(node.Scheme)]; !ok {
				continue
			}
		}

		filteredNodes = append(filteredNodes, node)
		filteredResults = append(filteredResults, res)
	}

	if len(filteredNodes) == 0 {
		return "", errors.New("没有符合条件的节点可导出")
	}

	lines := clash.BuildFilteredURLList(filteredNodes, filteredResults, settings)
	return strings.Join(lines, "\n"), nil
}

// GetAvailableProxyTypes returns a list of available proxy types in current nodes
func (a *App) GetAvailableProxyTypes() []string {
	a.mu.Lock()
	nodes := a.nodes
	results := a.results
	a.mu.Unlock()

	typeMap := make(map[string]struct{})
	for i, node := range nodes {
		if i < len(results) && results[i].Pass {
			typeMap[strings.ToLower(node.Scheme)] = struct{}{}
		}
	}

	var types []string
	for t := range typeMap {
		types = append(types, t)
	}
	return types
}
