package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"gopkg.in/yaml.v3"
)

type Node struct {
	Index        int
	Raw          string
	Scheme       string
	Name         string
	OriginalName string
	Host         string
	Port         int
	Security     string
	SNI          string
	Region       string
	URL          *url.URL

	// ✅ 关键修复：保存“不会把 + 变空格”的 Query 解析结果（避免 pbk 被破坏）
	Params map[string]string

	Vmess  *VmessConfig
	SS     *SSConfig
	Clash  map[string]interface{}
	IPInfo *IPInfo
}

type VmessConfig struct {
	Fields       map[string]interface{}
	HasPadding   bool
	OriginalName string
}

type SSConfig struct {
	Method   string
	Password string
	Plugin   string
}

type Result struct {
	Done       bool
	Pass       bool
	Err        string
	LatencyMs  []int64
	AvgMs      int64
	MaxMs      int64
	Attempts   int
	Successful int
}

type IPInfo struct {
	IP        string
	Country   string
	Region    string
	City      string
	ISP       string
	Org       string
	ASN       string
	Hosting   bool
	Proxy     bool
	Mobile    bool
	FromCache bool
}

type RegionRule struct {
	Pattern string
	Region  string
}

type TestSettings struct {
	Attempts         int
	Threshold        time.Duration
	Timeout          time.Duration
	Concurrency      int
	RequireAll       bool
	StopOnFail       bool
	Dedup            bool
	Rename           bool
	RenameFmt        string
	RegionRules      []RegionRule
	ExcludeEnabled   bool
	ExcludeKeywords  []string
	LatencyName      bool
	LatencyFmt       string
	IPRename         bool
	IPLookupURL      string
	IPLookupTimeout  time.Duration
	IPNameFmt        string
	UseCoreTest      bool
	CorePath         string
	CoreTestURL      string
	CoreStartTimeout time.Duration
}

type Output struct {
	GeneratedAt string        `yaml:"generated_at"`
	SourceFile  string        `yaml:"source_file"`
	Settings    OutputSetting `yaml:"settings"`
	Nodes       []OutputNode  `yaml:"nodes"`
}

type OutputSetting struct {
	Attempts    int   `yaml:"attempts"`
	ThresholdMs int64 `yaml:"threshold_ms"`
	TimeoutMs   int64 `yaml:"timeout_ms"`
}

type OutputNode struct {
	Name      string  `yaml:"name"`
	URL       string  `yaml:"url"`
	Region    string  `yaml:"region"`
	LatencyMs []int64 `yaml:"latency_ms"`
	AvgMs     int64   `yaml:"avg_ms"`
	MaxMs     int64   `yaml:"max_ms"`
}

const defaultTemplateFile = "demo.yaml"
const defaultIPLookupURL = "http://ip-api.com/json/{ip}?fields=status,message,country,regionName,city,isp,org,as,hosting,proxy,mobile,query"
const defaultIPNameFmt = "{country}-{region}-{city} {isp} {residential}"
const defaultLatencyFmt = "{avg}ms"
const defaultCoreTestURL = "https://www.gstatic.com/generate_204"

// ✅ 原代码写成了 6,000,000ms（100分钟），一旦内核卡住会等非常久。
//   这里改成更合理的 90s；并且你仍然可以在 UI 里改。
const defaultCoreStartTimeoutMs = 90000

type SettingsInputs struct {
	AttemptsEntry    *widget.Entry
	ThresholdEntry   *widget.Entry
	TimeoutEntry     *widget.Entry
	ConcurrencyEntry *widget.Entry
	RequireAllCheck  *widget.Check
	StopOnFailCheck  *widget.Check
	DedupCheck       *widget.Check
	RenameCheck      *widget.Check
	RenameFmtEntry   *widget.Entry
	RegionRulesEntry *widget.Entry

	ExcludeCheck *widget.Check
	ExcludeEntry *widget.Entry

	LatencyNameCheck *widget.Check
	LatencyFmtEntry  *widget.Entry

	IPRenameCheck      *widget.Check
	IPLookupURLEntry   *widget.Entry
	IPLookupTimeoutEnt *widget.Entry
	IPNameFmtEntry     *widget.Entry

	CoreTestCheck    *widget.Check
	CorePathEntry    *widget.Entry
	CoreTestURLEntry *widget.Entry
}

/* =========================
   ✅ Query/UUID/Reality 修复区
   ========================= */

// ✅ 自己解析 query：使用 PathUnescape（不会把 + 当空格），避免 pbk 被破坏
func parseQueryKeepPlus(rawQuery string) map[string]string {
	out := map[string]string{}
	rawQuery = strings.TrimSpace(rawQuery)
	if rawQuery == "" {
		return out
	}
	parts := strings.FieldsFunc(rawQuery, func(r rune) bool {
		return r == '&' || r == ';'
	})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		k := kv[0]
		v := ""
		if len(kv) == 2 {
			v = kv[1]
		}
		if uk, err := url.PathUnescape(k); err == nil {
			k = uk
		}
		if uv, err := url.PathUnescape(v); err == nil {
			v = uv
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(v)
		if k == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func paramGet(m map[string]string, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		k = strings.ToLower(strings.TrimSpace(k))
		if k == "" {
			continue
		}
		if v, ok := m[k]; ok {
			if strings.TrimSpace(v) != "" {
				return v
			}
		}
	}
	return ""
}

func cleanToken(s string) string {
	s = sanitizeString(s)
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'")
	s = strings.TrimSpace(s)
	return s
}

func isHexChar(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func isHexString(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !isHexChar(r) {
			return false
		}
	}
	return true
}

// ✅ 统一 UUID：允许 32hex 或 36（带 -），统一输出 36 小写
func normalizeUUID(raw string) (string, error) {
	s := cleanToken(raw)
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("empty uuid")
	}
	s = strings.TrimPrefix(strings.ToLower(s), "urn:uuid:")
	s = strings.TrimPrefix(strings.ToLower(s), "uuid:")
	s = strings.Trim(s, "{}")
	s = strings.TrimSpace(s)

	// userinfo 里可能有 %xx（极少数订阅会这样）；尝试解码 2 次
	for i := 0; i < 2 && strings.Contains(s, "%"); i++ {
		if dec, err := url.PathUnescape(s); err == nil && dec != s {
			s = dec
		} else {
			break
		}
	}

	s = strings.ToLower(strings.TrimSpace(s))

	if len(s) == 32 && isHexString(s) {
		return fmt.Sprintf("%s-%s-%s-%s-%s",
			s[0:8], s[8:12], s[12:16], s[16:20], s[20:32]), nil
	}
	if len(s) != 36 {
		return "", errors.New("invalid uuid length")
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return "", errors.New("invalid uuid hyphens")
	}
	for i, ch := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !isHexChar(ch) {
			return "", errors.New("invalid uuid char")
		}
	}
	return s, nil
}

// ✅ Reality public-key：必须能 base64 解码成 32 字节（X25519 公钥）
func normalizeRealityPublicKey(raw string) (string, error) {
	s := cleanToken(raw)
	if s == "" {
		return "", errors.New("empty public-key")
	}

	// 常见坑：某些解析把 + -> 空格；public-key 里不该有空白，直接去掉/修复
	if strings.ContainsAny(s, " \t\r\n") {
		// 先尝试把空白当成 +（更接近真实链接里没编码的 +）
		s2 := strings.ReplaceAll(s, " ", "+")
		s2 = strings.ReplaceAll(s2, "\t", "")
		s2 = strings.ReplaceAll(s2, "\r", "")
		s2 = strings.ReplaceAll(s2, "\n", "")
		s = s2
	}

	b, err := decodeBase64(s)
	if err != nil {
		return "", err
	}
	if len(b) != 32 {
		return "", fmt.Errorf("public-key bytes != 32 (got %d)", len(b))
	}

	// ✅ 归一化输出为 URL-safe Raw Base64（mihomo 文档示例风格）
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ✅ Reality short-id：hex 串，偶数长度，通常 <=16字节（这里放宽到 <=16字节=32 hex）
func normalizeRealityShortID(raw string) (string, error) {
	s := cleanToken(raw)
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return "", nil
	}
	if len(s)%2 == 1 {
		return "", errors.New("short-id length must be even")
	}
	if len(s) > 32 {
		return "", errors.New("short-id too long (>32 hex)")
	}
	for _, r := range s {
		if r < '0' || r > 'f' || (r > '9' && r < 'a') {
			return "", errors.New("short-id must be hex")
		}
	}
	return s, nil
}

func main() {
	a := app.New()
	applyChineseTheme(a)
	w := a.NewWindow("节点延迟测试")
	w.Resize(fyne.NewSize(1200, 800))

	var (
		nodes         []Node
		results       []Result
		filePath      string
		cancelRun     context.CancelFunc
		running       bool
		resultCh      chan resultUpdate
		logLines      []string
		logMu         sync.Mutex
		totalPassed   int
		totalDone     int
		activeSetting TestSettings
	)

	pathLabel := widget.NewLabel("未选择文件")
	countLabel := widget.NewLabel("节点数：0")
	passLabel := widget.NewLabel("通过：0")
	progressLabel := widget.NewLabel("空闲")

	attemptsEntry := widget.NewEntry()
	attemptsEntry.SetText("3")
	thresholdEntry := widget.NewEntry()
	thresholdEntry.SetText("300")
	timeoutEntry := widget.NewEntry()
	timeoutEntry.SetText("1500")
	concurrencyEntry := widget.NewEntry()
	concurrencyEntry.SetText("64")

	requireAllCheck := widget.NewCheck("所有次数均需低于阈值", nil)
	requireAllCheck.SetChecked(true)
	stopOnFailCheck := widget.NewCheck("失败则提前停止", nil)
	stopOnFailCheck.SetChecked(true)
	dedupCheck := widget.NewCheck("按协议+主机+端口去重", nil)
	dedupCheck.SetChecked(true)

	renameCheck := widget.NewCheck("按地区规则重命名", nil)
	renameCheck.SetChecked(true)
	renameFmtEntry := widget.NewEntry()
	renameFmtEntry.SetText("{region} {name}")
	renameFmtEntry.SetPlaceHolder("{region} {name} | {host} | {scheme} | {index}")
	renameFmtEntryWrap := wrapEntryMinWidth(renameFmtEntry, 420)

	regionRulesEntry := widget.NewMultiLineEntry()
	regionRulesEntry.SetPlaceHolder("规则=地区（每行一条），例：US=美国")
	regionRulesEntry.SetMinRowsVisible(3)

	excludeCheck := widget.NewCheck("启用关键词排除", nil)
	excludeEntry := widget.NewMultiLineEntry()
	excludeEntry.SetPlaceHolder("每行一个关键词，或用逗号分隔")
	excludeEntry.SetMinRowsVisible(2)

	latencyNameCheck := widget.NewCheck("名称追加延迟", nil)
	latencyFmtEntry := widget.NewEntry()
	latencyFmtEntry.SetText(defaultLatencyFmt)
	latencyFmtEntry.SetPlaceHolder("{avg}ms | {max}ms | {min}ms")
	latencyFmtEntryWrap := wrapEntryMinWidth(latencyFmtEntry, 260)

	ipRenameCheck := widget.NewCheck("根据IP信息重命名", nil)
	ipLookupURLEntry := widget.NewEntry()
	ipLookupURLEntry.SetText(defaultIPLookupURL)
	ipLookupTimeoutEnt := widget.NewEntry()
	ipLookupTimeoutEnt.SetText("3000")
	ipNameFmtEntry := widget.NewEntry()
	ipNameFmtEntry.SetText(defaultIPNameFmt)
	ipNameFmtEntry.SetPlaceHolder("{country}-{region}-{city} {isp} {residential}")

	coreTestCheck := widget.NewCheck("使用内核真实测试（更接近 Clash）", nil)
	corePathEntry := widget.NewEntry()
	corePathEntry.SetPlaceHolder("内核路径（mihomo/clash），留空自动查找")
	coreTestURLEntry := widget.NewEntry()
	coreTestURLEntry.SetText(defaultCoreTestURL)
	coreTestURLEntry.SetPlaceHolder("测试 URL（http/https）")

	importTextEntry := widget.NewMultiLineEntry()
	importTextEntry.SetPlaceHolder("粘贴节点/订阅内容或 YAML proxies")
	importTextEntry.SetMinRowsVisible(4)
	subURLEntry := widget.NewEntry()
	subURLEntry.SetPlaceHolder("订阅链接（http/https）")

	logEntry := widget.NewMultiLineEntry()
	logEntry.SetPlaceHolder("日志输出")
	logEntry.Wrapping = fyne.TextWrapWord
	logEntry.SetMinRowsVisible(16)
	logEntry.Disable()

	inputs := SettingsInputs{
		AttemptsEntry:      attemptsEntry,
		ThresholdEntry:     thresholdEntry,
		TimeoutEntry:       timeoutEntry,
		ConcurrencyEntry:   concurrencyEntry,
		RequireAllCheck:    requireAllCheck,
		StopOnFailCheck:    stopOnFailCheck,
		DedupCheck:         dedupCheck,
		RenameCheck:        renameCheck,
		RenameFmtEntry:     renameFmtEntry,
		RegionRulesEntry:   regionRulesEntry,
		ExcludeCheck:       excludeCheck,
		ExcludeEntry:       excludeEntry,
		LatencyNameCheck:   latencyNameCheck,
		LatencyFmtEntry:    latencyFmtEntry,
		IPRenameCheck:      ipRenameCheck,
		IPLookupURLEntry:   ipLookupURLEntry,
		IPLookupTimeoutEnt: ipLookupTimeoutEnt,
		IPNameFmtEntry:     ipNameFmtEntry,
		CoreTestCheck:      coreTestCheck,
		CorePathEntry:      corePathEntry,
		CoreTestURLEntry:   coreTestURLEntry,
	}

	appendLog := func(format string, args ...interface{}) {
		logMu.Lock()
		defer logMu.Unlock()
		line := fmt.Sprintf(format, args...)
		logLines = append(logLines, time.Now().Format("15:04:05")+" "+line)
		if len(logLines) > 400 {
			logLines = logLines[len(logLines)-400:]
		}
		logEntry.SetText(strings.Join(logLines, "\n"))
	}

	table := widget.NewTable(
		func() (int, int) {
			if len(nodes) == 0 {
				return 1, 7
			}
			return len(nodes) + 1, 7
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			if id.Row == 0 {
				switch id.Col {
				case 0:
					label.SetText("名称")
				case 1:
					label.SetText("主机")
				case 2:
					label.SetText("端口")
				case 3:
					label.SetText("地区")
				case 4:
					label.SetText("平均(ms)")
				case 5:
					label.SetText("最大(ms)")
				case 6:
					label.SetText("状态")
				default:
					label.SetText("")
				}
				return
			}
			idx := id.Row - 1
			if idx < 0 || idx >= len(nodes) {
				label.SetText("")
				return
			}
			node := nodes[idx]
			var res Result
			if idx < len(results) {
				res = results[idx]
			}
			switch id.Col {
			case 0:
				label.SetText(displayName(node))
			case 1:
				label.SetText(node.Host)
			case 2:
				label.SetText(strconv.Itoa(node.Port))
			case 3:
				label.SetText(node.Region)
			case 4:
				if res.Done {
					label.SetText(strconv.FormatInt(res.AvgMs, 10))
				} else {
					label.SetText("")
				}
			case 5:
				if res.Done {
					label.SetText(strconv.FormatInt(res.MaxMs, 10))
				} else {
					label.SetText("")
				}
			case 6:
				if !res.Done {
					label.SetText("等待")
				} else if res.Pass {
					label.SetText("通过")
				} else if res.Err != "" {
					label.SetText("错误")
				} else {
					label.SetText("失败")
				}
			default:
				label.SetText("")
			}
		},
	)

	table.SetColumnWidth(0, 320)
	table.SetColumnWidth(1, 200)
	table.SetColumnWidth(2, 60)
	table.SetColumnWidth(3, 140)
	table.SetColumnWidth(4, 80)
	table.SetColumnWidth(5, 80)
	table.SetColumnWidth(6, 80)

	applyImportedNodes := func(loaded []Node, warnings []string) {
		if inputs.ExcludeCheck.Checked && len(inputs.ExcludeEntry.Text) > 0 {
			before := len(loaded)
			loaded = filterNodesByKeywords(loaded, parseKeywords(inputs.ExcludeEntry.Text))
			after := len(loaded)
			if after < before {
				appendLog("已排除：%d -> %d", before, after)
			}
		}
		if inputs.DedupCheck.Checked {
			before := len(loaded)
			loaded = dedupNodes(loaded)
			after := len(loaded)
			if after < before {
				appendLog("已去重：%d -> %d", before, after)
			}
		} else {
			loaded = reindexNodes(loaded)
		}
		nodes = loaded
		results = make([]Result, len(nodes))
		totalPassed = 0
		totalDone = 0
		countLabel.SetText(fmt.Sprintf("节点数：%d", len(nodes)))
		passLabel.SetText("通过：0")
		progressLabel.SetText("就绪")
		table.Refresh()
		for _, warn := range warnings {
			appendLog("解析警告：%s", warn)
		}
		appendLog("已加载 %d 个节点", len(nodes))
	}

	loadNodesFromPath := func(path string) {
		loaded, warnings, err := readNodesFromFile(path)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		applyImportedNodes(loaded, warnings)
	}
	clearImported := func() {
		filePath = ""
		nodes = nil
		results = nil
		totalPassed = 0
		totalDone = 0
		pathLabel.SetText("未选择文件")
		countLabel.SetText("节点数：0")
		passLabel.SetText("通过：0")
		progressLabel.SetText("空闲")
		table.Refresh()
		appendLog("已清除导入数据")
	}

	openBtn := widget.NewButton("选择节点文件", func() {
		withSuppressedFyneLog(func() {
			fileOpen := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				if r == nil {
					return
				}
				defer r.Close()
				path := normalizeFilePath(r.URI().Path())
				path = filepath.FromSlash(path)
				filePath = path
				pathLabel.SetText(path)
				loadNodesFromPath(path)
			}, w)
			fileOpen.SetConfirmText("打开")
			fileOpen.SetDismissText("取消")
			fileOpen.Resize(fyne.NewSize(1000, 700))
			fileOpen.Show()
		})
	})

	importTextBtn := widget.NewButton("从文本导入", func() {
		text := strings.TrimSpace(importTextEntry.Text)
		if text == "" {
			dialog.ShowInformation("提示", "请输入要导入的内容。", w)
			return
		}
		loaded, warnings, err := parseNodesFromText(text)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		filePath = ""
		pathLabel.SetText("文本导入")
		applyImportedNodes(loaded, warnings)
	})

	importSubBtn := widget.NewButton("订阅导入", func() {
		subURL := strings.TrimSpace(subURLEntry.Text)
		if subURL == "" {
			dialog.ShowInformation("提示", "请输入订阅链接。", w)
			return
		}
		content, err := fetchSubscription(subURL, 20*time.Second)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		loaded, warnings, err := parseNodesFromText(content)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		filePath = ""
		pathLabel.SetText("订阅导入")
		applyImportedNodes(loaded, warnings)
	})

	reloadBtn := widget.NewButton("重新加载", func() {
		if filePath == "" {
			dialog.ShowInformation("提示", "未选择文件。", w)
			return
		}
		loadNodesFromPath(filePath)
	})
	clearBtn := widget.NewButton("清除导入", func() {
		if len(nodes) == 0 {
			clearImported()
			return
		}
		dialog.ShowConfirm("确认", "确定清除已导入的节点吗？", func(ok bool) {
			if ok {
				clearImported()
			}
		}, w)
	})

	startBtn := widget.NewButton("开始测试", func() {})
	stopBtn := widget.NewButton("停止", func() {
		if cancelRun != nil {
			cancelRun()
			appendLog("已请求停止")
		}
	})
	stopBtn.Disable()

	saveYamlBtn := widget.NewButton("保存YAML", func() {
		if len(nodes) == 0 {
			dialog.ShowInformation("提示", "未加载节点。", w)
			return
		}
		settings, ok := getSettingsOrWarn(w, inputs)
		if !ok {
			return
		}
		cfg, _, err := buildClashConfig(defaultTemplateFile, nodes, results, settings)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		withSuppressedFyneLog(func() {
			save := dialog.NewFileSave(func(wc fyne.URIWriteCloser, err error) {
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				if wc == nil {
					return
				}
				defer wc.Close()

				cfg = sanitizeYAMLMap(cfg)
				data, err := yaml.Marshal(cfg)
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				data = sanitizeYAMLOutput(data)
				_, err = wc.Write(data)
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				appendLog("已保存YAML：%s", wc.URI().Path())
			}, w)
			save.SetFileName("nodes.yaml")
			save.SetFilter(storage.NewExtensionFileFilter([]string{".yaml", ".yml"}))
			save.SetConfirmText("保存")
			save.SetDismissText("取消")
			save.Resize(fyne.NewSize(1000, 700))
			save.Show()
		})
	})

	copyYamlBtn := widget.NewButton("复制YAML", func() {
		if len(nodes) == 0 {
			dialog.ShowInformation("提示", "未加载节点。", w)
			return
		}
		settings, ok := getSettingsOrWarn(w, inputs)
		if !ok {
			return
		}
		cfg, _, err := buildClashConfig(defaultTemplateFile, nodes, results, settings)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		cfg = sanitizeYAMLMap(cfg)
		data, err := yaml.Marshal(cfg)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		data = sanitizeYAMLOutput(data)

		if err := setClipboardText(w, string(data)); err != nil {
			dialog.ShowError(err, w)
			appendLog("复制失败：%v", err)
			return
		}
		appendLog("已复制YAML到剪贴板")
	})

	copyUrlsBtn := widget.NewButton("复制链接", func() {
		settings, ok := getSettingsOrWarn(w, inputs)
		if !ok {
			return
		}
		lines := buildFilteredURLList(nodes, results, settings)
		if len(lines) == 0 {
			dialog.ShowInformation("提示", "没有通过的节点可复制。", w)
			return
		}
		if err := setClipboardText(w, strings.Join(lines, "\n")); err != nil {
			dialog.ShowError(err, w)
			appendLog("复制失败：%v", err)
			return
		}
		appendLog("已复制 %d 条链接到剪贴板", len(lines))
	})

	saveUrlsBtn := widget.NewButton("保存链接", func() {
		settings, ok := getSettingsOrWarn(w, inputs)
		if !ok {
			return
		}
		lines := buildFilteredURLList(nodes, results, settings)
		if len(lines) == 0 {
			dialog.ShowInformation("提示", "没有通过的节点可导出。", w)
			return
		}
		withSuppressedFyneLog(func() {
			save := dialog.NewFileSave(func(wc fyne.URIWriteCloser, err error) {
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				if wc == nil {
					return
				}
				defer wc.Close()
				_, err = wc.Write([]byte(strings.Join(lines, "\n")))
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				appendLog("已保存链接列表：%s", wc.URI().Path())
			}, w)
			save.SetFileName("nodes.txt")
			save.SetFilter(storage.NewExtensionFileFilter([]string{".txt"}))
			save.SetConfirmText("保存")
			save.SetDismissText("取消")
			save.Resize(fyne.NewSize(1000, 700))
			save.Show()
		})
	})

	startBtn.OnTapped = func() {
		if running {
			return
		}
		if len(nodes) == 0 {
			dialog.ShowInformation("提示", "未加载节点。", w)
			return
		}
		settings, ok := getSettingsOrWarn(w, inputs)
		if !ok {
			return
		}
		activeSetting = settings
		ctx, cancel := context.WithCancel(context.Background())
		cancelRun = cancel
		results = make([]Result, len(nodes))
		totalPassed = 0
		totalDone = 0
		table.Refresh()
		if settings.IPRename {
			progressLabel.SetText("获取IP信息...")
		} else {
			progressLabel.SetText("测试中")
		}
		running = true
		stopBtn.Enable()
		startBtn.Disable()
		openBtn.Disable()
		reloadBtn.Disable()
		if settings.UseCoreTest {
			appendLog("真实测试：内核=%s URL=%s", settings.CorePath, settings.CoreTestURL)
		}
		appendLog("测试开始：次数=%d 阈值=%dms 超时=%dms 并发=%d",
			settings.Attempts, settings.Threshold.Milliseconds(), settings.Timeout.Milliseconds(), settings.Concurrency)

		resultCh = make(chan resultUpdate, settings.Concurrency)

		go func() {
			for upd := range resultCh {
				u := upd
				if u.Index >= 0 && u.Index < len(results) {
					results[u.Index] = u.Result
					totalDone++
					if u.Result.Pass {
						totalPassed++
					}
					if activeSetting.LatencyName {
						name, region := buildOutputName(nodes[u.Index], activeSetting, u.Result)
						nodes[u.Index].Name = name
						nodes[u.Index].Region = region
					}
				}
				passLabel.SetText(fmt.Sprintf("通过：%d", totalPassed))
				progressLabel.SetText(fmt.Sprintf("完成 %d/%d", totalDone, len(nodes)))
				table.Refresh()
			}
		}()

		go func() {
			if settings.IPRename {
				appendLog("开始获取IP信息...")
				refreshIPInfo(ctx, nodes, settings, appendLog)
				appendLog("IP信息获取完成")
				applyRegionsAndNames(nodes, settings)
				table.Refresh()
			} else {
				applyRegionsAndNames(nodes, settings)
				table.Refresh()
			}
			progressLabel.SetText("测试中")
			runTests(ctx, nodes, settings, resultCh, appendLog)
			close(resultCh)
			running = false
			stopBtn.Disable()
			startBtn.Enable()
			openBtn.Enable()
			reloadBtn.Enable()
			if ctx.Err() == context.Canceled {
				progressLabel.SetText("已取消")
			} else {
				progressLabel.SetText("已完成")
			}
		}()
	}

	settingsGrid := container.NewGridWithColumns(4,
		widget.NewLabel("测试次数"), attemptsEntry,
		widget.NewLabel("阈值(ms)"), thresholdEntry,
		widget.NewLabel("超时(ms)"), timeoutEntry,
		widget.NewLabel("并发"), concurrencyEntry,
	)

	flagsRow := container.NewHBox(requireAllCheck, stopOnFailCheck, dedupCheck)
	renameRow := container.NewBorder(nil, nil, renameCheck, nil,
		container.NewHBox(widget.NewLabel("命名格式"), renameFmtEntryWrap))
	latencyRow := container.NewBorder(nil, nil, latencyNameCheck, nil,
		container.NewHBox(widget.NewLabel("延迟格式"), latencyFmtEntryWrap))

	excludeBox := widget.NewCard("关键词排除", "", container.NewVBox(excludeCheck, excludeEntry))
	regionBox := widget.NewCard("地区规则", "规则=地区（忽略大小写）", regionRulesEntry)

	coreTestForm := widget.NewForm(
		widget.NewFormItem("内核路径", corePathEntry),
		widget.NewFormItem("测试URL", coreTestURLEntry),
	)
	coreTestBox := widget.NewCard("真实可用性测试", "", container.NewVBox(coreTestCheck, coreTestForm))

	ipForm := widget.NewForm(
		widget.NewFormItem("查询URL", ipLookupURLEntry),
		widget.NewFormItem("超时(ms)", ipLookupTimeoutEnt),
		widget.NewFormItem("命名格式", ipNameFmtEntry),
	)
	ipBox := widget.NewCard("IP信息命名", "", container.NewVBox(ipRenameCheck, ipForm))

	importBox := widget.NewCard("导入", "", container.NewVBox(
		container.NewHBox(openBtn, reloadBtn, clearBtn, layout.NewSpacer(), importSubBtn),
		container.NewBorder(nil, nil, widget.NewLabel("订阅链接"), nil, subURLEntry),
		importTextEntry,
		importTextBtn,
	))

	logBox := widget.NewCard("日志", "", logEntry)

	buttonsRow := container.NewHBox(startBtn, stopBtn, layout.NewSpacer(), copyYamlBtn, saveYamlBtn, copyUrlsBtn, saveUrlsBtn)

	topRow := container.NewHBox(layout.NewSpacer(), countLabel, passLabel, progressLabel)
	pathRow := container.NewHBox(widget.NewLabel("来源："), pathLabel)

	mainControlBox := container.NewVBox(
		pathRow,
		topRow,
		importBox,
		buttonsRow,
		logBox,
	)
	mainLeft := container.NewVScroll(mainControlBox)
	mainRight := container.NewVScroll(table)
	mainSplit := container.NewHSplit(mainLeft, mainRight)
	mainSplit.SetOffset(0.38)

	settingsBox := container.NewVBox(
		settingsGrid,
		flagsRow,
		renameRow,
		latencyRow,
		regionBox,
		excludeBox,
		coreTestBox,
		ipBox,
	)
	settingsContent := container.NewVScroll(settingsBox)

	tabs := container.NewAppTabs(
		container.NewTabItem("测试", mainSplit),
		container.NewTabItem("配置", settingsContent),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	w.SetContent(tabs)
	w.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if len(uris) == 0 {
			return
		}
		path := normalizeFilePath(uris[0].Path())
		path = filepath.FromSlash(path)
		if path == "" {
			return
		}
		filePath = path
		pathLabel.SetText(path)
		loadNodesFromPath(path)
		appendLog("已拖入文件：%s", path)
	})

	w.ShowAndRun()
}

type resultUpdate struct {
	Index  int
	Result Result
}

func getSettingsOrWarn(w fyne.Window, inputs SettingsInputs) (TestSettings, bool) {
	settings, err := parseSettings(inputs)
	if err != nil {
		dialog.ShowError(err, w)
		return TestSettings{}, false
	}
	return settings, true
}

func parseSettings(inputs SettingsInputs) (TestSettings, error) {
	attempts, err := strconv.Atoi(strings.TrimSpace(inputs.AttemptsEntry.Text))
	if err != nil || attempts <= 0 {
		return TestSettings{}, errors.New("测试次数不合法")
	}
	thresholdMs, err := strconv.Atoi(strings.TrimSpace(inputs.ThresholdEntry.Text))
	if err != nil || thresholdMs <= 0 {
		return TestSettings{}, errors.New("阈值不合法")
	}
	timeoutMs, err := strconv.Atoi(strings.TrimSpace(inputs.TimeoutEntry.Text))
	if err != nil || timeoutMs <= 0 {
		return TestSettings{}, errors.New("超时不合法")
	}
	concurrency, err := strconv.Atoi(strings.TrimSpace(inputs.ConcurrencyEntry.Text))
	if err != nil || concurrency <= 0 {
		return TestSettings{}, errors.New("并发不合法")
	}
	ipTimeoutMs := 3000
	if inputs.IPLookupTimeoutEnt != nil {
		if v, err := strconv.Atoi(strings.TrimSpace(inputs.IPLookupTimeoutEnt.Text)); err == nil && v > 0 {
			ipTimeoutMs = v
		}
	}
	rules := parseRegionRules(inputs.RegionRulesEntry.Text)
	excludeKeywords := parseKeywords(inputs.ExcludeEntry.Text)
	coreTestURL := strings.TrimSpace(inputs.CoreTestURLEntry.Text)
	if coreTestURL == "" {
		coreTestURL = defaultCoreTestURL
	}
	useCoreTest := inputs.CoreTestCheck.Checked
	corePathInput := strings.TrimSpace(inputs.CorePathEntry.Text)
	corePath := ""
	if useCoreTest {
		if !isHTTPURL(coreTestURL) {
			return TestSettings{}, errors.New("测试URL不合法（需为 http/https）")
		}
		resolved, err := resolveCorePath(corePathInput)
		if err != nil {
			return TestSettings{}, err
		}
		corePath = resolved
	}
	return TestSettings{
		Attempts:         attempts,
		Threshold:        time.Duration(thresholdMs) * time.Millisecond,
		Timeout:          time.Duration(timeoutMs) * time.Millisecond,
		Concurrency:      concurrency,
		RequireAll:       inputs.RequireAllCheck.Checked,
		StopOnFail:       inputs.StopOnFailCheck.Checked,
		Dedup:            inputs.DedupCheck.Checked,
		Rename:           inputs.RenameCheck.Checked,
		RenameFmt:        strings.TrimSpace(inputs.RenameFmtEntry.Text),
		RegionRules:      rules,
		ExcludeEnabled:   inputs.ExcludeCheck.Checked,
		ExcludeKeywords:  excludeKeywords,
		LatencyName:      inputs.LatencyNameCheck.Checked,
		LatencyFmt:       strings.TrimSpace(inputs.LatencyFmtEntry.Text),
		IPRename:         inputs.IPRenameCheck.Checked,
		IPLookupURL:      strings.TrimSpace(inputs.IPLookupURLEntry.Text),
		IPLookupTimeout:  time.Duration(ipTimeoutMs) * time.Millisecond,
		IPNameFmt:        strings.TrimSpace(inputs.IPNameFmtEntry.Text),
		UseCoreTest:      useCoreTest,
		CorePath:         corePath,
		CoreTestURL:      coreTestURL,
		CoreStartTimeout: time.Duration(defaultCoreStartTimeoutMs) * time.Millisecond,
	}, nil
}

func applyRegionsAndNames(nodes []Node, settings TestSettings) {
	for i := range nodes {
		node := &nodes[i]
		name, region := computeNameAndRegion(*node, settings)
		node.Region = region
		node.Name = name
	}
}

func runTests(ctx context.Context, nodes []Node, settings TestSettings, resultCh chan<- resultUpdate, logf func(string, ...interface{})) {
	if settings.UseCoreTest {
		runTestsWithCore(ctx, nodes, settings, resultCh, logf)
		return
	}

	workers := settings.Concurrency
	if workers <= 0 {
		workers = 1
	}

	jobs := make(chan int)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for idx := range jobs {
			if ctx.Err() != nil {
				return
			}
			res := testNode(ctx, nodes[idx], settings)
			resultCh <- resultUpdate{Index: idx, Result: res}
		}
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker()
	}

	for i := range nodes {
		if ctx.Err() != nil {
			break
		}
		jobs <- i
	}

	close(jobs)
	wg.Wait()
}

// ✅ 核心修复：
// - 在生成代理时就校验 UUID / reality public-key / short-id 等，直接跳过坏节点，避免 mihomo 启动被一个坏节点炸掉
func runTestsWithCore(ctx context.Context, nodes []Node, settings TestSettings, resultCh chan<- resultUpdate, logf func(string, ...interface{})) {
	skipFlows := map[string]struct{}{
		"xtls-rprx-direct": {},
		"xtls-rprx-origin": {},
		"xtls-rprx-splice": {},
	}

	badNodeReasons := make(map[int]string)

	var (
		proxies          []interface{}
		proxyNames       []string
		proxyNameByIndex []string
		skipReasons      []string
		startErr         error
		tester           *coreTester
	)

	corePath := settings.CorePath
	if corePath == "" {
		resolved, err := resolveCorePath("")
		if err != nil {
			startErr = err
		} else {
			corePath = resolved
		}
	}

	// 兜底重试：如果还有别的未知字段导致启动失败，仍然能自动剔除
	maxRetries := 2000
	if len(nodes) < maxRetries {
		maxRetries = len(nodes)
	}
	if maxRetries < 10 {
		maxRetries = 10
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if startErr != nil {
			break
		}

		var warnings []string
		proxies, proxyNames, proxyNameByIndex, skipReasons, warnings = buildTestProxies(nodes, skipFlows, badNodeReasons)
		for _, warn := range warnings {
			if logf != nil {
				logf("跳过节点：%s", warn)
			}
		}

		if len(proxies) == 0 {
			startErr = errors.New("没有可测试的节点")
			break
		}

		if logf != nil {
			logf("启动内核：%s", corePath)
		}

		tester, startErr = startCoreTester(ctx, corePath, proxies, proxyNames, settings.CoreStartTimeout)
		if startErr == nil {
			break
		}

		if flow := extractUnsupportedXtlsFlow(startErr); flow != "" {
			key := strings.ToLower(strings.TrimSpace(flow))
			if _, exists := skipFlows[key]; !exists {
				skipFlows[key] = struct{}{}
				if logf != nil {
					logf("内核不支持 flow=%s，已跳过该类型后重试", flow)
				}
				continue
			}
		}

		if idx, reason := extractProxyParseError(startErr); idx > 0 && idx <= len(proxyNames) {
			badProxyName := proxyNames[idx-1]
			badNodeIdx := -1
			for i, name := range proxyNameByIndex {
				if name == badProxyName {
					badNodeIdx = i
					break
				}
			}
			if badNodeIdx >= 0 {
				r := strings.TrimSpace(strings.Trim(reason, "\"'"))
				if r == "" {
					r = "内核配置解析失败"
				}
				badNodeReasons[badNodeIdx] = fmt.Sprintf("内核拒绝：%s", r)
				if logf != nil {
					logf("内核拒绝 proxy %d（name=%s）：%s，已跳过该节点后重试", idx, badProxyName, r)
				}
				continue
			}
		}

		if logf != nil {
			logf("内核启动失败：%v", startErr)
		}
		break
	}

	if tester == nil {
		for i := range nodes {
			if i < len(skipReasons) && skipReasons[i] != "" {
				resultCh <- resultUpdate{
					Index: i,
					Result: Result{
						Done:     true,
						Pass:     false,
						Err:      skipReasons[i],
						Attempts: settings.Attempts,
					},
				}
				continue
			}
			if i < len(proxyNameByIndex) && proxyNameByIndex[i] != "" {
				errMsg := "内核启动失败"
				if startErr != nil {
					errMsg = startErr.Error()
				}
				resultCh <- resultUpdate{
					Index: i,
					Result: Result{
						Done:     true,
						Pass:     false,
						Err:      errMsg,
						Attempts: settings.Attempts,
					},
				}
			}
		}
		return
	}
	defer tester.Close()

	for i := range nodes {
		if i < len(skipReasons) && skipReasons[i] != "" {
			resultCh <- resultUpdate{
				Index: i,
				Result: Result{
					Done:     true,
					Pass:     false,
					Err:      skipReasons[i],
					Attempts: settings.Attempts,
				},
			}
		}
	}

	client := &http.Client{Timeout: settings.Timeout + 2*time.Second}
	jobs := make(chan int)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for idx := range jobs {
			if ctx.Err() != nil {
				return
			}
			name := proxyNameByIndex[idx]
			if name == "" {
				continue
			}
			res := testNodeWithMeasure(ctx, settings, func(timeout time.Duration) (time.Duration, error) {
				return coreDelay(client, tester.apiURL, name, settings.CoreTestURL, timeout)
			})
			resultCh <- resultUpdate{Index: idx, Result: res}
		}
	}

	workers := settings.Concurrency
	if workers <= 0 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker()
	}
	for i := range nodes {
		if ctx.Err() != nil {
			break
		}
		if proxyNameByIndex[i] == "" {
			continue
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

func testNode(ctx context.Context, node Node, settings TestSettings) Result {
	return testNodeWithMeasure(ctx, settings, func(timeout time.Duration) (time.Duration, error) {
		return measureOnce(node, timeout)
	})
}

func testNodeWithMeasure(ctx context.Context, settings TestSettings, measure func(timeout time.Duration) (time.Duration, error)) Result {
	var (
		latencies []int64
		sum       int64
		maxMs     int64
	)
	pass := settings.RequireAll
	anySuccess := false
	var errMsg string
	for i := 0; i < settings.Attempts; i++ {
		if ctx.Err() != nil {
			return Result{Done: true, Pass: false, Err: "canceled", Attempts: i}
		}
		d, err := measure(settings.Timeout)
		if err != nil {
			errMsg = err.Error()
			if settings.RequireAll || settings.StopOnFail {
				pass = false
				break
			}
			continue
		}
		ms := d.Milliseconds()
		anySuccess = true
		latencies = append(latencies, ms)
		sum += ms
		if ms > maxMs {
			maxMs = ms
		}
		if time.Duration(ms)*time.Millisecond > settings.Threshold {
			if settings.RequireAll {
				pass = false
				if settings.StopOnFail {
					break
				}
			}
		} else if !settings.RequireAll {
			pass = true
		}
	}
	avg := int64(0)
	if len(latencies) > 0 {
		avg = sum / int64(len(latencies))
	}
	if settings.RequireAll && len(latencies) < settings.Attempts {
		pass = false
	}
	if settings.RequireAll && maxMs > settings.Threshold.Milliseconds() {
		pass = false
	}
	if !settings.RequireAll && !anySuccess {
		pass = false
	}
	return Result{
		Done:       true,
		Pass:       pass,
		Err:        errMsg,
		LatencyMs:  latencies,
		AvgMs:      avg,
		MaxMs:      maxMs,
		Attempts:   settings.Attempts,
		Successful: len(latencies),
	}
}

func measureOnce(node Node, timeout time.Duration) (time.Duration, error) {
	addr := net.JoinHostPort(node.Host, strconv.Itoa(node.Port))
	dialer := &net.Dialer{Timeout: timeout}
	start := time.Now()

	if strings.EqualFold(node.Security, "tls") || strings.EqualFold(node.Security, "reality") || strings.EqualFold(node.Scheme, "trojan") {
		conn, err := dialer.Dial("tcp", addr)
		if err != nil {
			return 0, err
		}
		_ = conn.SetDeadline(time.Now().Add(timeout))
		cfg := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         node.SNI,
		}
		tlsConn := tls.Client(conn, cfg)
		if err := tlsConn.Handshake(); err != nil {
			_ = tlsConn.Close()
			return 0, err
		}
		_ = tlsConn.Close()
		return time.Since(start), nil
	}

	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return 0, err
	}
	_ = conn.Close()
	return time.Since(start), nil
}

type coreTester struct {
	cmd     *exec.Cmd
	apiURL  string
	tempDir string
	done    chan error
	logPath string
	logFile *os.File
}

func buildTestProxies(nodes []Node, skipFlows map[string]struct{}, badNodeReasons map[int]string) ([]interface{}, []string, []string, []string, []string) {
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

		base := strings.TrimSpace(displayName(node))
		if base == "" {
			base = fmt.Sprintf("节点_%d", i+1)
		}
		name := uniqueName(base, nameCount)

		proxy, err := nodeToClashProxy(node, name)
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

		// ✅ validateClashProxy 已增强：会校验 UUID / reality public-key / short-id
		if err := validateClashProxy(proxy); err != nil {
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

func resolveCorePath(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input != "" {
		if p, err := exec.LookPath(input); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("未找到内核：%s", input)
	}
	candidates := []string{"mihomo", "mihomo.exe", "clash-meta", "clash-meta.exe", "clash", "clash.exe"}
	for _, exe := range candidates {
		if p, err := exec.LookPath(exe); err == nil {
			return p, nil
		}
	}
	return "", errors.New("未找到 Clash/Mihomo 内核（请填写可执行文件路径）")
}

func isHTTPURL(raw string) bool {
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
	cfg = sanitizeYAMLMap(cfg)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}
	data = sanitizeYAMLOutput(data)
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		_ = logFile.Close()
		_ = os.RemoveAll(tempDir)
		return nil, err
	}
	cmd := exec.CommandContext(ctx, corePath, "-f", configPath, "-d", tempDir)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
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
	return &coreTester{cmd: cmd, apiURL: apiURL, tempDir: tempDir, done: done, logPath: logPath, logFile: logFile}, nil
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

func readNodesFromFile(path string) ([]Node, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	text := string(data)
	return parseNodesFromText(text)
}

func parseNodesFromText(text string) ([]Node, []string, error) {
	text = strings.TrimSpace(strings.TrimPrefix(text, "\uFEFF"))
	if text == "" {
		return nil, nil, errors.New("内容为空")
	}
	if nodes, warnings, ok := tryParseYAMLNodes(text); ok {
		return nodes, warnings, nil
	}
	if decoded, ok := tryDecodeSubscription(text); ok {
		text = decoded
		if nodes, warnings, ok := tryParseYAMLNodes(text); ok {
			return nodes, warnings, nil
		}
	}
	return parseNodesFromLines(text)
}

func tryParseYAMLNodes(text string) ([]Node, []string, bool) {
	var data interface{}
	clean := sanitizeTextForYAML(text)
	if err := yaml.Unmarshal([]byte(clean), &data); err != nil {
		return nil, nil, false
	}
	nodes, warnings, ok := parseNodesFromYAMLData(data)
	if !ok || len(nodes) == 0 {
		return nil, nil, false
	}
	return nodes, warnings, true
}

func parseNodesFromYAMLData(data interface{}) ([]Node, []string, bool) {
	switch v := data.(type) {
	case map[string]interface{}:
		if proxies, ok := toSlice(v["proxies"]); ok {
			nodes, warnings := parseProxyList(proxies)
			return nodes, warnings, true
		}
	case map[interface{}]interface{}:
		v2, _ := toStringMap(v)
		if proxies, ok := toSlice(v2["proxies"]); ok {
			nodes, warnings := parseProxyList(proxies)
			return nodes, warnings, true
		}
	case []interface{}:
		nodes, warnings := parseProxyList(v)
		return nodes, warnings, true
	default:
		return nil, nil, false
	}
	return nil, nil, false
}

func parseProxyList(list []interface{}) ([]Node, []string) {
	var nodes []Node
	var warnings []string
	for i, item := range list {
		m, ok := toStringMap(item)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("proxy %d: 不是有效对象", i+1))
			continue
		}
		node, err := parseNodeFromClashProxy(m)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("proxy %d: %v", i+1, err))
			continue
		}
		if cleaned, ok := sanitizeNode(node); ok {
			node = cleaned
		} else {
			warnings = append(warnings, fmt.Sprintf("proxy %d: 字段无效，已跳过", i+1))
			continue
		}
		node.Index = len(nodes) + 1
		nodes = append(nodes, node)
	}
	return nodes, warnings
}

func parseNodesFromLines(text string) ([]Node, []string, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
	var (
		nodes    []Node
		warnings []string
	)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(sanitizeLineText(scanner.Text()))
		if lineNum == 1 {
			line = strings.TrimPrefix(line, "\uFEFF")
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		node, err := parseNode(line)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d: %v", lineNum, err))
			continue
		}
		if cleaned, ok := sanitizeNode(node); ok {
			node = cleaned
		} else {
			warnings = append(warnings, fmt.Sprintf("line %d: 字段无效，已跳过", lineNum))
			continue
		}
		node.Index = len(nodes) + 1
		nodes = append(nodes, node)
	}
	if err := scanner.Err(); err != nil {
		return nodes, warnings, err
	}
	if len(nodes) == 0 {
		return nil, warnings, errors.New("未识别到节点")
	}
	return nodes, warnings, nil
}

func tryDecodeSubscription(text string) (string, bool) {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', ' ', '\t':
			return -1
		case '\u200b', '\u200c', '\u200d', '\ufeff':
			return -1
		default:
			if unicode.IsControl(r) || isUnicodeNoncharacter(r) {
				return -1
			}
			return r
		}
	}, text)
	if clean == "" {
		return "", false
	}
	b, err := decodeBase64(clean)
	if err != nil {
		return "", false
	}
	decoded := strings.TrimSpace(string(b))
	if decoded == "" {
		return "", false
	}
	if strings.Contains(decoded, "://") || strings.Contains(decoded, "proxies:") {
		return decoded, true
	}
	return "", false
}

func parseNodeFromClashProxy(m map[string]interface{}) (Node, error) {
	typ := strings.ToLower(strings.TrimSpace(getStringFromMap(m, "type")))
	if typ == "" {
		return Node{}, errors.New("代理缺少 type")
	}
	name := getStringFromMap(m, "name")
	server := getStringFromMap(m, "server")
	port, err := getIntFromMap(m, "port")
	if err != nil {
		return Node{}, err
	}
	node := Node{
		Scheme:       typ,
		Name:         name,
		OriginalName: name,
		Host:         server,
		Port:         port,
		Clash:        m,
	}
	if tlsVal, ok := getBoolFromMap(m, "tls"); ok && tlsVal {
		node.Security = "tls"
	}
	if sni := firstNonEmpty(getStringFromMap(m, "servername"), getStringFromMap(m, "sni"), getStringFromMap(m, "peer")); sni != "" {
		node.SNI = sni
	} else if node.Host != "" {
		node.SNI = node.Host
	}

	switch typ {
	case "ss", "shadowsocks":
		node.Scheme = "ss"
		method := firstNonEmpty(getStringFromMap(m, "cipher"), getStringFromMap(m, "method"))
		password := getStringFromMap(m, "password")
		if strings.TrimSpace(method) == "" {
			return Node{}, errors.New("ss 缺少 cipher/method")
		}
		if strings.TrimSpace(password) == "" {
			return Node{}, errors.New("ss 缺少 password")
		}
		m["cipher"] = method
		node.SS = &SSConfig{
			Method:   method,
			Password: password,
			Plugin:   getStringFromMap(m, "plugin"),
		}

	case "vmess":
		if strings.TrimSpace(getStringFromMap(m, "cipher")) == "" {
			m["cipher"] = "auto"
		}

	case "vless":
		// keep

	case "trojan":
		password := getStringFromMap(m, "password")
		if strings.TrimSpace(password) == "" {
			return Node{}, errors.New("trojan 缺少 password")
		}
		node.Scheme = "trojan"
		node.Security = "tls"
		if sni := firstNonEmpty(getStringFromMap(m, "sni"), getStringFromMap(m, "servername"), getStringFromMap(m, "peer")); sni != "" {
			node.SNI = sni
		} else if node.Host != "" {
			node.SNI = node.Host
		}

	default:
		return Node{}, fmt.Errorf("不支持的代理类型：%s", typ)
	}

	if node.Host == "" || node.Port == 0 {
		return Node{}, errors.New("代理缺少 server 或 port")
	}
	return node, nil
}

func parseNode(raw string) (Node, error) {
	if strings.HasPrefix(strings.ToLower(raw), "vmess://") {
		return parseVmess(raw)
	}
	if strings.HasPrefix(strings.ToLower(raw), "ss://") {
		return parseSS(raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Node{}, err
	}
	if u.Scheme == "" || u.Host == "" {
		return Node{}, errors.New("missing scheme or host")
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return Node{}, errors.New("missing or invalid port")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Node{}, errors.New("invalid port")
	}
	name := decodeFragment(u.Fragment)

	// ✅ 关键：Params 用 PathUnescape 解析，不会把 + 变空格
	params := parseQueryKeepPlus(u.RawQuery)

	security := strings.ToLower(strings.TrimSpace(paramGet(params, "security")))
	if strings.EqualFold(u.Scheme, "trojan") && security == "" {
		security = "tls"
	}
	sni := firstNonEmpty(paramGet(params, "sni"), paramGet(params, "servername"), paramGet(params, "peer"))
	if sni == "" {
		sni = host
	}

	return Node{
		Raw:          raw,
		Scheme:       u.Scheme,
		Name:         name,
		OriginalName: name,
		Host:         host,
		Port:         port,
		Security:     security,
		SNI:          sni,
		URL:          u,
		Params:       params,
	}, nil
}

func parseVmess(raw string) (Node, error) {
	payload := strings.TrimPrefix(raw, "vmess://")
	hasPadding := strings.Contains(payload, "=")
	data, err := decodeBase64(payload)
	if err != nil {
		return Node{}, err
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(data, &fields); err != nil {
		return Node{}, err
	}
	host := getString(fields, "add")
	portStr := getString(fields, "port")
	name := getString(fields, "ps")
	if host == "" || portStr == "" {
		return Node{}, errors.New("vmess missing host or port")
	}
	port, err := parsePort(portStr)
	if err != nil {
		return Node{}, err
	}
	security := strings.ToLower(getString(fields, "tls"))
	if security == "" {
		security = strings.ToLower(getString(fields, "security"))
	}
	if security != "tls" {
		security = ""
	}
	sni := getString(fields, "sni")
	if sni == "" {
		sni = host
	}
	if strings.TrimSpace(getString(fields, "scy")) == "" && strings.TrimSpace(getString(fields, "cipher")) == "" {
		fields["scy"] = "auto"
	}
	return Node{
		Raw:          raw,
		Scheme:       "vmess",
		Name:         name,
		OriginalName: name,
		Host:         host,
		Port:         port,
		Security:     security,
		SNI:          sni,
		Vmess: &VmessConfig{
			Fields:       fields,
			HasPadding:   hasPadding,
			OriginalName: name,
		},
	}, nil
}

func parseSS(raw string) (Node, error) {
	rest := strings.TrimPrefix(raw, "ss://")
	name := ""
	if idx := strings.Index(rest, "#"); idx >= 0 {
		name = decodeFragment(rest[idx+1:])
		rest = rest[:idx]
	}
	plugin := ""
	if idx := strings.Index(rest, "?"); idx >= 0 {
		plugin = rest[idx+1:]
		rest = rest[:idx]
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return Node{}, errors.New("ss empty")
	}

	parsePlain := func(plain string) (method, password, host string, port int, err error) {
		at := strings.LastIndex(plain, "@")
		if at < 0 {
			return "", "", "", 0, errors.New("ss missing @")
		}
		userinfo := plain[:at]
		hostport := plain[at+1:]
		colon := strings.Index(userinfo, ":")
		if colon < 0 {
			return "", "", "", 0, errors.New("ss missing method:password")
		}
		method = userinfo[:colon]
		password = userinfo[colon+1:]
		host, portStr, err2 := net.SplitHostPort(hostport)
		if err2 != nil {
			return "", "", "", 0, errors.New("ss missing or invalid port")
		}
		p, err3 := strconv.Atoi(portStr)
		if err3 != nil {
			return "", "", "", 0, errors.New("ss invalid port")
		}
		return method, password, host, p, nil
	}

	plain := rest

	if !strings.Contains(plain, "@") {
		decoded, err := decodeBase64(plain)
		if err != nil {
			return Node{}, err
		}
		plain = string(decoded)
		m, pw, host, port, err := parsePlain(plain)
		if err != nil {
			return Node{}, err
		}
		return Node{
			Raw:          raw,
			Scheme:       "ss",
			Name:         name,
			OriginalName: name,
			Host:         host,
			Port:         port,
			Security:     "",
			SNI:          host,
			SS: &SSConfig{
				Method:   m,
				Password: pw,
				Plugin:   plugin,
			},
		}, nil
	}

	at := strings.LastIndex(plain, "@")
	userPart := plain[:at]
	hostPart := plain[at+1:]

	if !strings.Contains(userPart, ":") {
		if decoded, err := decodeBase64(userPart); err == nil {
			decodedStr := string(decoded)
			if strings.Contains(decodedStr, ":") {
				userPart = decodedStr
			}
		}
	}

	plain2 := userPart + "@" + hostPart
	m, pw, host, port, err := parsePlain(plain2)
	if err != nil {
		return Node{}, err
	}

	return Node{
		Raw:          raw,
		Scheme:       "ss",
		Name:         name,
		OriginalName: name,
		Host:         host,
		Port:         port,
		Security:     "",
		SNI:          host,
		SS: &SSConfig{
			Method:   m,
			Password: pw,
			Plugin:   plugin,
		},
	}, nil
}

func buildFilteredURLList(nodes []Node, results []Result, settings TestSettings) []string {
	var lines []string
	for i, node := range nodes {
		if i >= len(results) {
			continue
		}
		res := results[i]
		if !res.Done || !res.Pass {
			continue
		}
		name, _ := buildOutputName(node, settings, res)
		urlStr, err := buildOutputURL(node, name)
		if err != nil {
			urlStr = node.Raw
		}
		if strings.TrimSpace(urlStr) == "" {
			continue
		}
		lines = append(lines, urlStr)
	}
	return lines
}

func buildClashConfig(templatePath string, nodes []Node, results []Result, settings TestSettings) (map[string]interface{}, []string, error) {
	data, err := os.ReadFile(templatePath)
	if err != nil {
		root := map[string]interface{}{
			"port":        7890,
			"socks-port":  7891,
			"mixed-port":  7892,
			"mode":        "rule",
			"log-level":   "warning",
			"allow-lan":   true,
			"external-ui": "",
			"dns": map[string]interface{}{
				"enable": false,
			},
			"proxy-groups": []interface{}{
				map[string]interface{}{
					"name":    "AUTO",
					"type":    "select",
					"proxies": []string{},
				},
			},
			"rules": []interface{}{
				"MATCH,AUTO",
			},
		}
		oldNames := extractProxyNames(root)
		proxies, names := buildClashProxies(nodes, results, settings)
		if len(names) == 0 {
			return nil, nil, errors.New("没有通过的节点可导出")
		}
		root["proxies"] = proxies
		updateProxyGroups(root, oldNames, names)
		return root, names, nil
	}

	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, nil, fmt.Errorf("模板解析失败：%v", err)
	}
	oldNames := extractProxyNames(root)
	proxies, names := buildClashProxies(nodes, results, settings)
	if len(names) == 0 {
		return nil, nil, errors.New("没有通过的节点可导出")
	}
	root["proxies"] = proxies
	updateProxyGroups(root, oldNames, names)
	return root, names, nil
}

func buildClashProxies(nodes []Node, results []Result, settings TestSettings) ([]interface{}, []string) {
	var proxies []interface{}
	var names []string
	nameCount := make(map[string]int)
	for i, node := range nodes {
		if i >= len(results) {
			continue
		}
		res := results[i]
		if !res.Done || !res.Pass {
			continue
		}
		baseName, _ := buildOutputName(node, settings, res)
		name := uniqueName(baseName, nameCount)
		proxy, err := nodeToClashProxy(node, name)
		if err != nil {
			continue
		}
		proxies = append(proxies, proxy)
		names = append(names, name)
	}
	return proxies, names
}

func uniqueName(name string, counts map[string]int) string {
	n := strings.TrimSpace(name)
	if n == "" {
		n = "节点"
	}
	if counts[n] == 0 {
		counts[n] = 1
		return n
	}
	counts[n]++
	return fmt.Sprintf("%s_%d", n, counts[n])
}

func nodeToClashProxy(node Node, name string) (map[string]interface{}, error) {
	if node.Clash != nil {
		cloned, ok := cloneStringMap(node.Clash)
		if !ok {
			return nil, errors.New("代理配置无效")
		}
		cloned["name"] = name
		p := normalizeClashProxy(cloned)
		if err := validateClashProxy(p); err != nil {
			return nil, err
		}
		return p, nil
	}
	switch strings.ToLower(node.Scheme) {
	case "vless":
		return buildVlessProxy(node, name)
	case "vmess":
		p, err := buildVmessProxy(node, name)
		if err != nil {
			return nil, err
		}
		return normalizeClashProxy(p), nil
	case "ss":
		p, err := buildSSProxy(node, name)
		if err != nil {
			return nil, err
		}
		return normalizeClashProxy(p), nil
	case "trojan":
		p, err := buildTrojanProxy(node, name)
		if err != nil {
			return nil, err
		}
		return normalizeClashProxy(p), nil
	default:
		return nil, fmt.Errorf("不支持的协议：%s", node.Scheme)
	}
}

func buildVlessProxy(node Node, name string) (map[string]interface{}, error) {
	u := node.URL
	if u == nil {
		parsed, err := url.Parse(node.Raw)
		if err != nil {
			return nil, err
		}
		u = parsed
	}
	if u == nil || u.User == nil {
		return nil, errors.New("无效的 vless 节点")
	}

	// ✅ UUID 预校验（大量垃圾订阅里经常是乱的）
	uuidRaw := u.User.Username()
	uuid, err := normalizeUUID(uuidRaw)
	if err != nil {
		return nil, fmt.Errorf("vless UUID 无效：%v", err)
	}

	params := node.Params
	if params == nil {
		params = parseQueryKeepPlus(u.RawQuery)
	}

	network := strings.ToLower(paramGet(params, "type"))
	if network == "" {
		network = "tcp"
	}

	sec := strings.ToLower(strings.TrimSpace(firstNonEmpty(paramGet(params, "security"), node.Security)))
	tlsEnabled := sec == "tls" || sec == "reality"

	proxy := map[string]interface{}{
		"name":    name,
		"type":    "vless",
		"server":  node.Host,
		"port":    node.Port,
		"uuid":    uuid,
		"udp":     true,
		"tls":     tlsEnabled,
		"network": network,
	}
	proxy["encryption"] = normalizeVlessEncryption(paramGet(params, "encryption"))

	if sni := firstNonEmpty(paramGet(params, "sni"), paramGet(params, "servername"), paramGet(params, "peer"), node.SNI); sni != "" {
		proxy["servername"] = sni
	}

	if fp := paramGet(params, "fp"); fp != "" {
		proxy["client-fingerprint"] = fp
	}
	if alpn := paramGet(params, "alpn"); alpn != "" {
		proxy["alpn"] = splitCSV(alpn)
	}
	if flow := paramGet(params, "flow"); flow != "" {
		proxy["flow"] = flow
	}
	if v, ok := parseBoolStr(firstNonEmpty(paramGet(params, "allowInsecure"), paramGet(params, "insecure"))); ok {
		proxy["skip-cert-verify"] = v
	}

	// ✅ Reality：必须 public-key 有效，否则直接跳过，避免炸内核
	if sec == "reality" {
		ro := map[string]interface{}{}

		pbkRaw := firstNonEmpty(
			paramGet(params, "pbk"),
			paramGet(params, "publickey"),
			paramGet(params, "public-key"),
		)
		pbk, err := normalizeRealityPublicKey(pbkRaw)
		if err != nil {
			return nil, fmt.Errorf("reality public-key 无效：%v", err)
		}
		ro["public-key"] = pbk

		sidRaw := firstNonEmpty(
			paramGet(params, "sid"),
			paramGet(params, "shortid"),
			paramGet(params, "short-id"),
		)
		if sidRaw != "" {
			sid, err := normalizeRealityShortID(sidRaw)
			if err != nil {
				return nil, fmt.Errorf("reality short-id 无效：%v", err)
			}
			if sid != "" {
				ro["short-id"] = sid
			}
		}

		spx := firstNonEmpty(
			paramGet(params, "spx"),
			paramGet(params, "spiderx"),
			paramGet(params, "spider-x"),
		)
		if spx != "" {
			ro["spider-x"] = spx
		}

		proxy["reality-opts"] = ro
		proxy["tls"] = true
	}

	switch network {
	case "ws":
		wsOpts := map[string]interface{}{}
		path := firstNonEmpty(paramGet(params, "path"), u.Path)
		if path == "" {
			path = "/"
		}
		wsOpts["path"] = path

		if host := paramGet(params, "host"); host != "" {
			wsOpts["headers"] = map[string]interface{}{"Host": host}
		}
		proxy["ws-opts"] = wsOpts

	case "grpc":
		if svc := firstNonEmpty(paramGet(params, "servicename"), paramGet(params, "service"), paramGet(params, "grpc-service-name")); svc != "" {
			proxy["grpc-opts"] = map[string]interface{}{"grpc-service-name": svc}
		}
	}

	return normalizeClashProxy(proxy), nil
}

func buildVmessProxy(node Node, name string) (map[string]interface{}, error) {
	if node.Vmess == nil {
		return nil, errors.New("无效的 vmess 节点")
	}
	fields := node.Vmess.Fields
	server := firstNonEmpty(getString(fields, "add"), node.Host)
	portStr := firstNonEmpty(getString(fields, "port"), strconv.Itoa(node.Port))
	port, err := parsePort(portStr)
	if err != nil {
		return nil, err
	}

	uuidRaw := getString(fields, "id")
	uuid, err := normalizeUUID(uuidRaw)
	if err != nil {
		return nil, fmt.Errorf("vmess UUID 无效：%v", err)
	}

	network := strings.ToLower(getString(fields, "net"))
	proxy := map[string]interface{}{
		"name":   name,
		"type":   "vmess",
		"server": server,
		"port":   port,
		"uuid":   uuid,
		"udp":    true,
	}

	cipher := firstNonEmpty(getString(fields, "scy"), getString(fields, "cipher"))
	if strings.TrimSpace(cipher) == "" {
		cipher = "auto"
	}
	proxy["cipher"] = cipher

	if aidStr := getString(fields, "aid"); aidStr != "" {
		if aid, err := strconv.Atoi(aidStr); err == nil {
			proxy["alterId"] = aid
		}
	}
	if network != "" {
		proxy["network"] = network
	}
	if tlsVal := strings.ToLower(getString(fields, "tls")); tlsVal != "" {
		proxy["tls"] = tlsVal == "tls" || tlsVal == "true"
	}
	if sni := getString(fields, "sni"); sni != "" {
		proxy["servername"] = sni
	}
	if fp := getString(fields, "fp"); fp != "" {
		proxy["client-fingerprint"] = fp
	}
	if alpn := getString(fields, "alpn"); alpn != "" {
		proxy["alpn"] = splitCSV(alpn)
	}
	if v, ok := parseBoolStr(firstNonEmpty(getString(fields, "allowInsecure"), getString(fields, "allowinsecure"))); ok {
		proxy["skip-cert-verify"] = v
	}

	switch network {
	case "ws":
		wsOpts := map[string]interface{}{}
		path := getString(fields, "path")
		if path == "" {
			path = "/"
		}
		wsOpts["path"] = path
		if host := getString(fields, "host"); host != "" {
			wsOpts["headers"] = map[string]interface{}{"Host": host}
		}
		proxy["ws-opts"] = wsOpts

	case "grpc":
		if svc := firstNonEmpty(getString(fields, "serviceName"), getString(fields, "servicename")); svc != "" {
			proxy["grpc-opts"] = map[string]interface{}{"grpc-service-name": svc}
		}
	}

	return proxy, nil
}

func buildSSProxy(node Node, name string) (map[string]interface{}, error) {
	if node.SS == nil {
		return nil, errors.New("无效的 ss 节点")
	}
	proxy := map[string]interface{}{
		"name":     name,
		"type":     "ss",
		"server":   node.Host,
		"port":     node.Port,
		"cipher":   node.SS.Method,
		"password": node.SS.Password,
		"udp":      true,
	}
	if plugin, opts := parseSSPlugin(node.SS.Plugin); plugin != "" {
		proxy["plugin"] = plugin
		if len(opts) > 0 {
			proxy["plugin-opts"] = opts
		}
	}
	return proxy, nil
}

// ✅ trojan：强制 tls=true；同时支持 security=reality（pbk/sid/spx）
func buildTrojanProxy(node Node, name string) (map[string]interface{}, error) {
	u := node.URL
	if u == nil {
		parsed, err := url.Parse(node.Raw)
		if err != nil {
			return nil, err
		}
		u = parsed
	}
	if u == nil || u.Host == "" {
		return nil, errors.New("无效的 trojan 节点")
	}
	password := ""
	if u.User != nil {
		if pw, ok := u.User.Password(); ok && pw != "" {
			password = pw
		} else {
			password = u.User.Username()
		}
	}
	if strings.TrimSpace(password) == "" {
		return nil, errors.New("trojan 缺少 password")
	}

	params := node.Params
	if params == nil {
		params = parseQueryKeepPlus(u.RawQuery)
	}

	network := strings.ToLower(paramGet(params, "type"))
	if network == "" {
		network = "tcp"
	}

	sec := strings.ToLower(strings.TrimSpace(firstNonEmpty(paramGet(params, "security"), node.Security)))
	if sec == "" {
		sec = "tls"
	}

	p := map[string]interface{}{
		"name":     name,
		"type":     "trojan",
		"server":   node.Host,
		"port":     node.Port,
		"password": password,
		"udp":      true,
		"tls":      true, // ✅ trojan 强制 tls
	}

	if sni := firstNonEmpty(paramGet(params, "sni"), paramGet(params, "peer"), paramGet(params, "servername"), node.SNI); sni != "" {
		p["sni"] = sni
	}
	if fp := paramGet(params, "fp"); fp != "" {
		p["client-fingerprint"] = fp
	}
	if alpn := paramGet(params, "alpn"); alpn != "" {
		p["alpn"] = splitCSV(alpn)
	}
	if v, ok := parseBoolStr(firstNonEmpty(paramGet(params, "allowInsecure"), paramGet(params, "insecure"))); ok {
		p["skip-cert-verify"] = v
	}

	if sec == "reality" {
		ro := map[string]interface{}{}
		pbkRaw := firstNonEmpty(paramGet(params, "pbk"), paramGet(params, "publickey"), paramGet(params, "public-key"))
		pbk, err := normalizeRealityPublicKey(pbkRaw)
		if err != nil {
			return nil, fmt.Errorf("reality public-key 无效：%v", err)
		}
		ro["public-key"] = pbk

		sidRaw := firstNonEmpty(paramGet(params, "sid"), paramGet(params, "shortid"), paramGet(params, "short-id"))
		if sidRaw != "" {
			sid, err := normalizeRealityShortID(sidRaw)
			if err != nil {
				return nil, fmt.Errorf("reality short-id 无效：%v", err)
			}
			if sid != "" {
				ro["short-id"] = sid
			}
		}

		spx := firstNonEmpty(paramGet(params, "spx"), paramGet(params, "spiderx"), paramGet(params, "spider-x"))
		if spx != "" {
			ro["spider-x"] = spx
		}

		p["reality-opts"] = ro
		p["tls"] = true
	}

	if network != "" && network != "tcp" {
		p["network"] = network
	}
	switch network {
	case "ws":
		wsOpts := map[string]interface{}{}
		path := firstNonEmpty(paramGet(params, "path"), u.Path)
		if path == "" {
			path = "/"
		}
		wsOpts["path"] = path
		if host := paramGet(params, "host"); host != "" {
			wsOpts["headers"] = map[string]interface{}{"Host": host}
		}
		p["ws-opts"] = wsOpts

	case "grpc":
		if svc := firstNonEmpty(paramGet(params, "servicename"), paramGet(params, "service"), paramGet(params, "grpc-service-name")); svc != "" {
			p["grpc-opts"] = map[string]interface{}{"grpc-service-name": svc}
		}
	}

	return p, nil
}

func parseSSPlugin(raw string) (string, map[string]interface{}) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	spec := ""
	if values, err := url.ParseQuery(raw); err == nil {
		spec = values.Get("plugin")
	}
	if spec == "" {
		spec = raw
	}
	parts := strings.Split(spec, ";")
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return "", nil
	}
	opts := map[string]interface{}{}
	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			opts[kv[0]] = kv[1]
		} else {
			opts[part] = true
		}
	}
	return name, opts
}

func extractProxyNames(root map[string]interface{}) []string {
	var names []string
	list, ok := toSlice(root["proxies"])
	if !ok {
		return names
	}
	for _, item := range list {
		m, ok := toStringMap(item)
		if !ok {
			continue
		}
		if name, ok := m["name"].(string); ok && name != "" {
			names = append(names, name)
		}
	}
	return names
}

func updateProxyGroups(root map[string]interface{}, oldNames, newNames []string) {
	groups, ok := toSlice(root["proxy-groups"])
	if !ok {
		return
	}
	oldSet := make(map[string]struct{})
	for _, n := range oldNames {
		oldSet[n] = struct{}{}
	}
	for i, item := range groups {
		groupMap, ok := toStringMap(item)
		if !ok {
			continue
		}
		proxyList, ok := toStringSlice(groupMap["proxies"])
		if !ok {
			continue
		}
		hasOld := false
		for _, p := range proxyList {
			if _, exists := oldSet[p]; exists {
				hasOld = true
				break
			}
		}
		if !hasOld {
			continue
		}
		var keep []string
		for _, p := range proxyList {
			if _, exists := oldSet[p]; !exists {
				keep = append(keep, p)
			}
		}
		keep = append(keep, newNames...)
		groupMap["proxies"] = keep
		groups[i] = groupMap
	}
	root["proxy-groups"] = groups
}

func toSlice(v interface{}) ([]interface{}, bool) {
	if v == nil {
		return nil, false
	}
	switch t := v.(type) {
	case []interface{}:
		return t, true
	default:
		return nil, false
	}
}

func toStringMap(v interface{}) (map[string]interface{}, bool) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, true
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out, true
	default:
		return nil, false
	}
}

func cloneStringMap(v map[string]interface{}) (map[string]interface{}, bool) {
	cloned := cloneValue(v)
	out, ok := cloned.(map[string]interface{})
	return out, ok
}

func cloneValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			out[k] = cloneValue(val)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			out[fmt.Sprint(k)] = cloneValue(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(t))
		for _, val := range t {
			out = append(out, cloneValue(val))
		}
		return out
	case []byte:
		copied := make([]byte, len(t))
		copy(copied, t)
		return copied
	default:
		return t
	}
}

func sanitizeYAMLMap(root map[string]interface{}) map[string]interface{} {
	clean := sanitizeYAMLValue(root)
	if m, ok := clean.(map[string]interface{}); ok {
		return m
	}
	return root
}

func sanitizeYAMLValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch t := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			out[sanitizeString(k)] = sanitizeYAMLValue(val)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			out[sanitizeString(fmt.Sprint(k))] = sanitizeYAMLValue(val)
		}
		return out
	case map[string]string:
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			out[sanitizeString(k)] = sanitizeString(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(t))
		for _, val := range t {
			out = append(out, sanitizeYAMLValue(val))
		}
		return out
	case []string:
		out := make([]interface{}, 0, len(t))
		for _, s := range t {
			out = append(out, sanitizeString(s))
		}
		return out
	case []byte:
		return sanitizeString(string(t))
	case string:
		return sanitizeString(t)
	default:
		rv := reflect.ValueOf(v)
		if !rv.IsValid() {
			return v
		}
		for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
			if rv.IsNil() {
				return v
			}
			rv = rv.Elem()
		}

		switch rv.Kind() {
		case reflect.Slice, reflect.Array:
			n := rv.Len()
			out := make([]interface{}, 0, n)
			for i := 0; i < n; i++ {
				out = append(out, sanitizeYAMLValue(rv.Index(i).Interface()))
			}
			return out
		case reflect.Map:
			out := make(map[string]interface{}, rv.Len())
			for _, k := range rv.MapKeys() {
				ks := sanitizeString(fmt.Sprint(k.Interface()))
				out[ks] = sanitizeYAMLValue(rv.MapIndex(k).Interface())
			}
			return out
		default:
			return v
		}
	}
}

func isUnicodeNoncharacter(r rune) bool {
	if r >= 0xFDD0 && r <= 0xFDEF {
		return true
	}
	if r&0xFFFF == 0xFFFE || r&0xFFFF == 0xFFFF {
		return true
	}
	return false
}

func sanitizeString(s string) string {
	if s == "" {
		return s
	}
	s = strings.ToValidUTF8(s, " ")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\u00a0':
			b.WriteRune(' ')
		case r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\ufeff':
		case r >= 0xD800 && r <= 0xDFFF:
			b.WriteRune(' ')
		case isUnicodeNoncharacter(r):
			b.WriteRune(' ')
		case unicode.IsControl(r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sanitizeTextForYAML(text string) string {
	if text == "" {
		return text
	}
	text = strings.ToValidUTF8(text, " ")
	var b strings.Builder
	b.Grow(len(text))

	skipNextLF := false
	for _, r := range text {
		if skipNextLF {
			skipNextLF = false
			if r == '\n' {
				continue
			}
		}
		switch {
		case r == '\u00a0':
			b.WriteRune(' ')
		case r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\ufeff':
		case r == '\r':
			b.WriteRune('\n')
			skipNextLF = true
		case r == '\n':
			b.WriteRune('\n')
		case r == '\t':
			b.WriteRune(' ')
		case r >= 0xD800 && r <= 0xDFFF:
			b.WriteRune(' ')
		case isUnicodeNoncharacter(r):
			b.WriteRune(' ')
		case unicode.IsControl(r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sanitizeYAMLOutput(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	clean := bytes.ToValidUTF8(data, []byte(" "))
	return []byte(sanitizeTextForYAML(string(clean)))
}

func sanitizeLineText(text string) string {
	if text == "" {
		return text
	}
	text = strings.ToValidUTF8(text, " ")
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch {
		case r == '\u00a0':
			b.WriteRune(' ')
		case r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\ufeff':
		case r >= 0xD800 && r <= 0xDFFF:
			b.WriteRune(' ')
		case isUnicodeNoncharacter(r):
			b.WriteRune(' ')
		case unicode.IsControl(r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sanitizeNode(node Node) (Node, bool) {
	node.Name = sanitizeString(node.Name)
	node.OriginalName = sanitizeString(node.OriginalName)
	node.Host = sanitizeString(node.Host)
	node.SNI = sanitizeString(node.SNI)
	node.Region = sanitizeString(node.Region)

	if node.Params != nil {
		out := make(map[string]string, len(node.Params))
		for k, v := range node.Params {
			kk := strings.ToLower(sanitizeString(k))
			out[kk] = sanitizeString(v)
		}
		node.Params = out
	}

	if node.SS != nil {
		node.SS.Method = sanitizeString(node.SS.Method)
		node.SS.Password = sanitizeString(node.SS.Password)
		node.SS.Plugin = sanitizeString(node.SS.Plugin)
	}
	if node.Vmess != nil {
		node.Vmess.OriginalName = sanitizeString(node.Vmess.OriginalName)
		if cleaned, ok := sanitizeYAMLValue(node.Vmess.Fields).(map[string]interface{}); ok {
			node.Vmess.Fields = cleaned
		}
	}
	if node.Clash != nil {
		node.Clash = sanitizeYAMLMap(node.Clash)
	}
	if node.URL != nil {
		u := *node.URL
		u.Path = sanitizeString(u.Path)
		u.RawQuery = sanitizeString(u.RawQuery)
		u.Fragment = sanitizeString(u.Fragment)
		node.URL = &u
	}
	if strings.TrimSpace(node.Host) == "" || node.Port <= 0 {
		return node, false
	}
	return node, true
}

func normalizeClashProxy(proxy map[string]interface{}) map[string]interface{} {
	if proxy == nil {
		return proxy
	}
	proxy = sanitizeYAMLMap(proxy)
	typ := strings.ToLower(strings.TrimSpace(getStringFromMap(proxy, "type")))

	switch typ {
	case "vless":
		proxy["encryption"] = normalizeVlessEncryption(getStringFromMap(proxy, "encryption"))
	case "vmess":
		if strings.TrimSpace(getStringFromMap(proxy, "cipher")) == "" {
			proxy["cipher"] = "auto"
		}
	case "ss", "shadowsocks":
		if strings.TrimSpace(getStringFromMap(proxy, "cipher")) == "" {
			if v := strings.TrimSpace(getStringFromMap(proxy, "method")); v != "" {
				proxy["cipher"] = v
			}
		}
	case "trojan":
		// ✅ trojan 强制 tls
		proxy["tls"] = true
		if strings.TrimSpace(getStringFromMap(proxy, "sni")) == "" {
			if v := strings.TrimSpace(getStringFromMap(proxy, "servername")); v != "" {
				proxy["sni"] = v
			}
		}
	}

	// ✅ reality-opts 归一化（支持 publicKey/sid 等别名）
	if roRaw, ok := proxy["reality-opts"]; ok && roRaw != nil {
		ro, ok2 := toStringMap(roRaw)
		if ok2 && len(ro) > 0 {
			pub := firstNonEmpty(getStringFromMap(ro, "public-key"), getStringFromMap(ro, "publicKey"), getStringFromMap(ro, "publickey"), getStringFromMap(ro, "pbk"))
			if pub != "" {
				if norm, err := normalizeRealityPublicKey(pub); err == nil {
					ro["public-key"] = norm
				}
			}
			sid := firstNonEmpty(getStringFromMap(ro, "short-id"), getStringFromMap(ro, "shortId"), getStringFromMap(ro, "shortid"), getStringFromMap(ro, "sid"))
			if sid != "" {
				if norm, err := normalizeRealityShortID(sid); err == nil && norm != "" {
					ro["short-id"] = norm
				}
			}
			proxy["reality-opts"] = ro
			proxy["tls"] = true
		}
	}

	return proxy
}

func validateClashProxy(p map[string]interface{}) error {
	typ := strings.ToLower(strings.TrimSpace(getStringFromMap(p, "type")))

	// ✅ 基础字段校验
	switch typ {
	case "ss", "shadowsocks":
		if strings.TrimSpace(getStringFromMap(p, "cipher")) == "" {
			return errors.New("key 'cipher' missing")
		}
		if strings.TrimSpace(getStringFromMap(p, "password")) == "" {
			return errors.New("key 'password' missing")
		}
		if strings.TrimSpace(getStringFromMap(p, "server")) == "" {
			return errors.New("key 'server' missing")
		}
		if _, err := getIntFromMap(p, "port"); err != nil {
			return errors.New("key 'port' missing")
		}
	case "vmess":
		if strings.TrimSpace(getStringFromMap(p, "uuid")) == "" {
			return errors.New("key 'uuid' missing")
		}
		if strings.TrimSpace(getStringFromMap(p, "server")) == "" {
			return errors.New("key 'server' missing")
		}
		if _, err := getIntFromMap(p, "port"); err != nil {
			return errors.New("key 'port' missing")
		}
	case "vless":
		if strings.TrimSpace(getStringFromMap(p, "uuid")) == "" {
			return errors.New("key 'uuid' missing")
		}
		if strings.TrimSpace(getStringFromMap(p, "server")) == "" {
			return errors.New("key 'server' missing")
		}
		if _, err := getIntFromMap(p, "port"); err != nil {
			return errors.New("key 'port' missing")
		}
	case "trojan":
		if strings.TrimSpace(getStringFromMap(p, "password")) == "" {
			return errors.New("key 'password' missing")
		}
		if strings.TrimSpace(getStringFromMap(p, "server")) == "" {
			return errors.New("key 'server' missing")
		}
		if _, err := getIntFromMap(p, "port"); err != nil {
			return errors.New("key 'port' missing")
		}
		// trojan 强制 tls
		p["tls"] = true
	}

	// ✅ UUID 预校验，避免 mihomo 启动直接炸
	if typ == "vmess" || typ == "vless" {
		uuidRaw := getStringFromMap(p, "uuid")
		uuid, err := normalizeUUID(uuidRaw)
		if err != nil {
			return fmt.Errorf("invalid uuid: %v", err)
		}
		p["uuid"] = uuid
	}

	// ✅ Reality 预校验：只要 reality-opts 非空，必须有合法 public-key（并可选 short-id）
	if roRaw, ok := p["reality-opts"]; ok && roRaw != nil {
		ro, ok2 := toStringMap(roRaw)
		if !ok2 {
			return errors.New("reality-opts invalid")
		}
		if len(ro) > 0 {
			pub := firstNonEmpty(getStringFromMap(ro, "public-key"), getStringFromMap(ro, "publicKey"), getStringFromMap(ro, "publickey"), getStringFromMap(ro, "pbk"))
			if strings.TrimSpace(pub) == "" {
				return errors.New("reality-opts missing public-key")
			}
			pubNorm, err := normalizeRealityPublicKey(pub)
			if err != nil {
				return fmt.Errorf("invalid REALITY public key: %v", err)
			}
			ro["public-key"] = pubNorm

			sid := firstNonEmpty(getStringFromMap(ro, "short-id"), getStringFromMap(ro, "shortId"), getStringFromMap(ro, "shortid"), getStringFromMap(ro, "sid"))
			if sid != "" {
				sidNorm, err := normalizeRealityShortID(sid)
				if err != nil {
					return fmt.Errorf("invalid REALITY short ID: %v", err)
				}
				if sidNorm != "" {
					ro["short-id"] = sidNorm
				}
			}

			p["reality-opts"] = ro
			p["tls"] = true
		}
	}

	return nil
}

func normalizeVlessEncryption(raw string) string {
	token := normalizeTokenValue(raw)
	switch token {
	case "", "none":
		return "none"
	case "auto":
		return "auto"
	default:
		return "none"
	}
}

func normalizeTokenValue(raw string) string {
	v := strings.ToLower(strings.TrimSpace(sanitizeString(raw)))
	if v == "" {
		return ""
	}
	for i, r := range v {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			return strings.TrimSpace(v[:i])
		}
	}
	return v
}

func toStringSlice(v interface{}) ([]string, bool) {
	if v == nil {
		return nil, false
	}
	switch t := v.(type) {
	case []string:
		return t, true
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out, true
	default:
		return nil, false
	}
}

func splitCSV(v string) []string {
	parts := strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseBoolStr(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func fetchSubscription(urlStr string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "node-latency-gui")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("订阅请求失败：%s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func refreshIPInfo(ctx context.Context, nodes []Node, settings TestSettings, logf func(string, ...interface{})) {
	if !settings.IPRename {
		return
	}
	urlTemplate := strings.TrimSpace(settings.IPLookupURL)
	if urlTemplate == "" {
		logf("IP查询URL为空，跳过")
		return
	}
	client := &http.Client{Timeout: settings.IPLookupTimeout}
	cache := make(map[string]*IPInfo)
	var cacheMu sync.Mutex
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	for i := range nodes {
		wg.Add(1)
		idx := i
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			ip := resolveHostIP(ctx, nodes[idx].Host, settings.IPLookupTimeout)
			if ip == "" {
				return
			}
			cacheMu.Lock()
			if info, ok := cache[ip]; ok {
				cacheMu.Unlock()
				infoCopy := *info
				infoCopy.FromCache = true
				nodes[idx].IPInfo = &infoCopy
				return
			}
			cacheMu.Unlock()

			info, err := queryIPInfo(client, urlTemplate, ip)
			if err != nil {
				return
			}
			cacheMu.Lock()
			cache[ip] = info
			cacheMu.Unlock()
			nodes[idx].IPInfo = info
		}()
	}
	wg.Wait()
}

func resolveHostIP(parent context.Context, host string, timeout time.Duration) string {
	if net.ParseIP(host) != nil {
		return host
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	base := parent
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, timeout)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return ""
	}
	for _, addr := range addrs {
		if addr.IP.To4() != nil {
			return addr.IP.String()
		}
	}
	return addrs[0].IP.String()
}

func queryIPInfo(client *http.Client, urlTemplate, ip string) (*IPInfo, error) {
	urlStr := strings.ReplaceAll(urlTemplate, "{ip}", ip)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "node-latency-gui")
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
	if status := getStringFromMap(raw, "status"); status != "" && status != "success" {
		return nil, fmt.Errorf("IP查询失败：%s", getStringFromMap(raw, "message"))
	}
	info := &IPInfo{
		IP:      firstNonEmpty(getStringFromMap(raw, "query"), getStringFromMap(raw, "ip"), ip),
		Country: firstNonEmpty(getStringFromMap(raw, "country"), getStringFromMap(raw, "country_name")),
		Region:  firstNonEmpty(getStringFromMap(raw, "regionName"), getStringFromMap(raw, "region")),
		City:    firstNonEmpty(getStringFromMap(raw, "city")),
		ISP:     firstNonEmpty(getStringFromMap(raw, "isp")),
		Org:     firstNonEmpty(getStringFromMap(raw, "org")),
		ASN:     firstNonEmpty(getStringFromMap(raw, "as")),
	}
	if v, ok := getBoolFromMap(raw, "hosting"); ok {
		info.Hosting = v
	}
	if v, ok := getBoolFromMap(raw, "proxy"); ok {
		info.Proxy = v
	}
	if v, ok := getBoolFromMap(raw, "mobile"); ok {
		info.Mobile = v
	}
	if privacy, ok := raw["privacy"].(map[string]interface{}); ok {
		if v, ok := getBoolFromMap(privacy, "hosting"); ok {
			info.Hosting = v
		}
		if v, ok := getBoolFromMap(privacy, "proxy"); ok {
			info.Proxy = v
		}
		if v, ok := getBoolFromMap(privacy, "vpn"); ok && v {
			info.Proxy = true
		}
	}
	return info, nil
}

func isResidential(info *IPInfo) bool {
	if info == nil {
		return false
	}
	return !info.Hosting && !info.Proxy
}

func wrapEntryMinWidth(entry *widget.Entry, width float32) fyne.CanvasObject {
	if entry == nil {
		return widget.NewLabel("")
	}
	min := entry.MinSize()
	if width < min.Width {
		width = min.Width
	}
	return container.NewGridWrap(fyne.NewSize(width, min.Height), entry)
}

func withSuppressedFyneLog(fn func()) {
	if fn == nil {
		return
	}
	prev := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(prev)
	fn()
}

func setClipboardText(w fyne.Window, text string) error {
	if w != nil {
		withSuppressedFyneLog(func() {
			w.Clipboard().SetContent(text)
		})
	}

	switch runtime.GOOS {
	case "windows":
		psScript := `[Console]::InputEncoding=[System.Text.Encoding]::UTF8; $t=[Console]::In.ReadToEnd(); Set-Clipboard -Value $t`
		if err := pipeToCommandIfExists([]string{"powershell", "powershell.exe", "pwsh"},
			[]string{"-NoProfile", "-NonInteractive", "-Command", psScript}, text); err == nil {
			return nil
		}
		if err := pipeToCommandIfExists([]string{"cmd", "cmd.exe"}, []string{"/c", "chcp 65001 >NUL & clip"}, text); err == nil {
			return nil
		}
		if err := pipeToCommandIfExists([]string{"cmd", "cmd.exe"}, []string{"/c", "clip"}, text); err == nil {
			return nil
		}
		return errors.New("复制失败：无法调用 PowerShell/clip 写入剪贴板（请检查系统权限或安全软件拦截）")
	case "darwin":
		if err := pipeToCommandIfExists([]string{"pbcopy"}, nil, text); err == nil {
			return nil
		}
		return nil
	default:
		if err := pipeToCommandIfExists([]string{"wl-copy"}, nil, text); err == nil {
			return nil
		}
		if err := pipeToCommandIfExists([]string{"xclip"}, []string{"-selection", "clipboard"}, text); err == nil {
			return nil
		}
		if err := pipeToCommandIfExists([]string{"xsel"}, []string{"--clipboard", "--input"}, text); err == nil {
			return nil
		}
		return nil
	}
}

func pipeToCommandIfExists(candidates []string, args []string, text string) error {
	for _, exe := range candidates {
		if exe == "" {
			continue
		}
		if _, err := exec.LookPath(exe); err != nil {
			continue
		}
		cmd := exec.Command(exe, args...)
		cmd.Stdin = strings.NewReader(text)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%s: %s", exe, msg)
		}
		return fmt.Errorf("%s: %v", exe, err)
	}
	return errors.New("command not found")
}

func buildOutputURL(node Node, newName string) (string, error) {
	switch node.Scheme {
	case "vmess":
		if node.Vmess == nil {
			return node.Raw, nil
		}
		node.Vmess.Fields["ps"] = newName
		data, err := json.Marshal(node.Vmess.Fields)
		if err != nil {
			return "", err
		}
		enc := base64.RawStdEncoding
		if node.Vmess.HasPadding {
			enc = base64.StdEncoding
		}
		return "vmess://" + enc.EncodeToString(data), nil
	case "ss":
		if node.SS == nil {
			return node.Raw, nil
		}
		u := &url.URL{
			Scheme: "ss",
			User:   url.UserPassword(node.SS.Method, node.SS.Password),
			Host:   net.JoinHostPort(node.Host, strconv.Itoa(node.Port)),
		}
		if node.SS.Plugin != "" {
			u.RawQuery = node.SS.Plugin
		}
		if newName != "" {
			u.Fragment = url.PathEscape(newName)
		}
		return u.String(), nil
	default:
		if node.URL == nil {
			return node.Raw, nil
		}
		u := *node.URL
		if newName != "" {
			u.Fragment = url.PathEscape(newName)
		} else {
			u.Fragment = ""
		}
		return u.String(), nil
	}
}

func parseRegionRules(text string) []RegionRule {
	var rules []RegionRule
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		pattern := strings.TrimSpace(parts[0])
		region := strings.TrimSpace(parts[1])
		if pattern == "" || region == "" {
			continue
		}
		rules = append(rules, RegionRule{Pattern: pattern, Region: region})
	}
	return rules
}

func parseKeywords(text string) []string {
	var out []string
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == ',' || r == ';'
	})
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		out = append(out, strings.ToLower(f))
	}
	return out
}

func filterNodesByKeywords(nodes []Node, keywords []string) []Node {
	if len(keywords) == 0 {
		return nodes
	}
	var out []Node
	for _, n := range nodes {
		if shouldExclude(n, keywords) {
			continue
		}
		out = append(out, n)
	}
	return out
}

func shouldExclude(node Node, keywords []string) bool {
	target := strings.ToLower(strings.Join([]string{node.Name, node.OriginalName, node.Host, node.Raw}, " "))
	for _, kw := range keywords {
		if kw != "" && strings.Contains(target, kw) {
			return true
		}
	}
	return false
}

func matchRegion(name, host string, rules []RegionRule) string {
	target := strings.ToLower(name + " " + host)
	for _, rule := range rules {
		if strings.Contains(target, strings.ToLower(rule.Pattern)) {
			return rule.Region
		}
	}
	return ""
}

func formatName(fmtStr, region, name, host, scheme string, index int) string {
	if fmtStr == "" {
		fmtStr = "{region} {name}"
	}
	out := fmtStr
	out = strings.ReplaceAll(out, "{region}", region)
	out = strings.ReplaceAll(out, "{name}", name)
	out = strings.ReplaceAll(out, "{host}", host)
	out = strings.ReplaceAll(out, "{scheme}", scheme)
	out = strings.ReplaceAll(out, "{index}", strconv.Itoa(index))
	out = strings.TrimSpace(out)
	if out == "" {
		if name != "" {
			return name
		}
		return host
	}
	return out
}

func computeNameAndRegion(node Node, settings TestSettings) (string, string) {
	baseName := baseNameFromNode(node, settings)
	region := matchRegion(baseName, node.Host, settings.RegionRules)
	if region == "" && node.IPInfo != nil {
		region = formatIPRegion(node.IPInfo)
	}
	if settings.Rename {
		return formatName(settings.RenameFmt, region, baseName, node.Host, node.Scheme, node.Index), region
	}
	return baseName, region
}

func buildOutputName(node Node, settings TestSettings, res Result) (string, string) {
	name, region := computeNameAndRegion(node, settings)
	if settings.LatencyName && res.Done && len(res.LatencyMs) > 0 {
		name = applyLatencyName(name, settings.LatencyFmt, res)
	}
	return name, region
}

func baseNameFromNode(node Node, settings TestSettings) string {
	if settings.IPRename && node.IPInfo != nil {
		ipName := formatIPName(settings.IPNameFmt, node.IPInfo)
		if ipName != "" {
			return ipName
		}
	}
	if node.OriginalName != "" {
		return node.OriginalName
	}
	if node.Host != "" {
		return node.Host
	}
	return node.Raw
}

func formatIPRegion(info *IPInfo) string {
	if info == nil {
		return ""
	}
	if info.Region != "" {
		return info.Region
	}
	return info.Country
}

func formatIPName(fmtStr string, info *IPInfo) string {
	if info == nil {
		return ""
	}
	if fmtStr == "" {
		fmtStr = defaultIPNameFmt
	}
	residential := "机房"
	if isResidential(info) {
		residential = "家宽"
	}
	out := fmtStr
	out = strings.ReplaceAll(out, "{country}", info.Country)
	out = strings.ReplaceAll(out, "{region}", info.Region)
	out = strings.ReplaceAll(out, "{city}", info.City)
	out = strings.ReplaceAll(out, "{isp}", info.ISP)
	out = strings.ReplaceAll(out, "{org}", info.Org)
	out = strings.ReplaceAll(out, "{asn}", info.ASN)
	out = strings.ReplaceAll(out, "{ip}", info.IP)
	out = strings.ReplaceAll(out, "{residential}", residential)
	out = strings.TrimSpace(out)
	return out
}

func applyLatencyName(name string, fmtStr string, res Result) string {
	if fmtStr == "" {
		fmtStr = defaultLatencyFmt
	}
	minMs := res.AvgMs
	if len(res.LatencyMs) > 0 {
		minMs = res.LatencyMs[0]
		for _, v := range res.LatencyMs {
			if v < minMs {
				minMs = v
			}
		}
	}
	out := fmtStr
	out = strings.ReplaceAll(out, "{avg}", strconv.FormatInt(res.AvgMs, 10))
	out = strings.ReplaceAll(out, "{max}", strconv.FormatInt(res.MaxMs, 10))
	out = strings.ReplaceAll(out, "{min}", strconv.FormatInt(minMs, 10))
	out = strings.ReplaceAll(out, "{name}", name)
	out = strings.TrimSpace(out)
	if strings.Contains(fmtStr, "{name}") {
		return out
	}
	if out == "" {
		return name
	}
	return strings.TrimSpace(name + " " + out)
}

func displayName(node Node) string {
	if node.Name != "" {
		return node.Name
	}
	if node.Host != "" {
		return node.Host
	}
	return node.Raw
}

func decodeFragment(fragment string) string {
	if fragment == "" {
		return ""
	}
	s, err := url.PathUnescape(fragment)
	if err != nil {
		return fragment
	}
	return s
}

func parsePort(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty port")
	}
	if p, err := strconv.Atoi(s); err == nil {
		return p, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int(f), nil
	}
	return 0, errors.New("invalid port")
}

func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func getStringFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

func getIntFromMap(m map[string]interface{}, key string) (int, error) {
	if m == nil {
		return 0, errors.New("缺少字段")
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0, errors.New("缺少端口")
	}
	switch t := v.(type) {
	case int:
		return t, nil
	case int64:
		return int(t), nil
	case float64:
		return int(t), nil
	case string:
		return parsePort(t)
	default:
		return 0, errors.New("端口格式错误")
	}
}

func getBoolFromMap(m map[string]interface{}, key string) (bool, bool) {
	if m == nil {
		return false, false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false, false
	}
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		return parseBoolStr(t)
	default:
		return false, false
	}
}

func decodeBase64(s string) ([]byte, error) {
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return nil, errors.New("invalid base64")
}

func dedupNodes(nodes []Node) []Node {
	seen := make(map[string]struct{})
	var out []Node
	for _, n := range nodes {
		key := fmt.Sprintf("%s|%s|%d", strings.ToLower(n.Scheme), strings.ToLower(n.Host), n.Port)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, n)
	}
	for i := range out {
		out[i].Index = i + 1
	}
	return out
}

func reindexNodes(nodes []Node) []Node {
	for i := range nodes {
		nodes[i].Index = i + 1
	}
	return nodes
}

type appTheme struct {
	base fyne.Theme
	font fyne.Resource
}

func (t *appTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return t.base.Color(name, variant)
}
func (t *appTheme) Font(style fyne.TextStyle) fyne.Resource {
	if t.font != nil {
		return t.font
	}
	return t.base.Font(style)
}
func (t *appTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}
func (t *appTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}

func applyChineseTheme(a fyne.App) {
	res := loadChineseFontResource()
	if res == nil {
		return
	}
	a.Settings().SetTheme(&appTheme{base: theme.DefaultTheme(), font: res})
}

func loadChineseFontResource() fyne.Resource {
	candidates := []string{
		`C:\Windows\Fonts\simhei.ttf`,
		`C:\Windows\Fonts\simsunb.ttf`,
		`C:\Windows\Fonts\msyh.ttf`,
		`C:\Windows\Fonts\msyhbd.ttf`,
	}
	for _, path := range candidates {
		if !fileExists(path) || !isSupportedFontFile(path) {
			continue
		}
		res, err := fyne.LoadResourceFromPath(path)
		if err == nil {
			return res
		}
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isSupportedFontFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".ttf" || ext == ".otf"
}

func normalizeFilePath(uriPath string) string {
	if unescaped, err := url.PathUnescape(uriPath); err == nil {
		uriPath = unescaped
	}
	if len(uriPath) >= 3 && uriPath[0] == '/' && uriPath[2] == ':' {
		return uriPath[1:]
	}
	return uriPath
}
