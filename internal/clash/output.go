package clash

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"node-latency/internal/model"
	"node-latency/internal/util"
)

func BuildFilteredURLList(nodes []model.Node, results []model.Result, settings model.TestSettings) []string {
	var lines []string
	for i, node := range nodes {
		if i >= len(results) {
			continue
		}
		res := results[i]
		if !res.Done || !res.Pass {
			continue
		}
		name, _ := util.BuildOutputName(node, settings, res)
		// 对名称进行地区缩写处理
		name = util.AbbreviateRegionInName(name)
		urlStr, err := BuildOutputURL(node, name)
		if err != nil {
			// 尝试使用原始 URL
			urlStr = node.Raw
		}
		if strings.TrimSpace(urlStr) == "" {
			continue
		}
		lines = append(lines, urlStr)
	}
	return lines
}

func BuildOutputURL(node model.Node, newName string) (string, error) {
	// 先完全解码名称（可能来自已编码的原始名称）
	newName = util.FullyDecodeURL(newName)

	switch strings.ToLower(node.Scheme) {
	case "vmess":
		return buildVmessURL(node, newName)
	case "ss":
		return buildSSURL(node, newName)
	case "vless", "trojan":
		return buildVlessOrTrojanURL(node, newName)
	case "hysteria2", "hysteria", "tuic", "socks5", "socks", "http", "https":
		// 这些协议主要来自 Clash 配置，尝试从 Clash 构建或使用原始 URL
		if node.Clash != nil {
			return buildURLFromClash(node, newName)
		}
		// 如果有原始 URL，使用它
		if node.Raw != "" {
			if node.URL != nil {
				u := *node.URL
				u.Fragment = newName
				return u.String(), nil
			}
			return node.Raw, nil
		}
		// 尝试从 Clash 配置构建
		return buildURLFromClash(node, newName)
	default:
		// 尝试使用原始 URL
		if node.Raw != "" {
			if node.URL != nil {
				u := *node.URL
				u.Fragment = newName
				return u.String(), nil
			}
			return node.Raw, nil
		}
		// 最后尝试从 Clash 配置构建
		if node.Clash != nil {
			return buildURLFromClash(node, newName)
		}
		return "", nil
	}
}

func buildVmessURL(node model.Node, newName string) (string, error) {
	if node.Vmess == nil {
		return node.Raw, nil
	}
	// 复制 fields 避免修改原始数据
	fields := make(map[string]interface{})
	for k, v := range node.Vmess.Fields {
		fields[k] = v
	}
	fields["ps"] = newName
	data, err := json.Marshal(fields)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	if node.Vmess.HasPadding {
		enc = base64.StdEncoding
	}
	return "vmess://" + enc.EncodeToString(data), nil
}

func buildSSURL(node model.Node, newName string) (string, error) {
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
	u.Fragment = newName
	return u.String(), nil
}

func buildVlessOrTrojanURL(node model.Node, newName string) (string, error) {
	// 如果有原始 URL，使用它
	if node.URL != nil {
		u := *node.URL
		u.Fragment = newName
		return u.String(), nil
	}

	// 如果有 Raw，尝试解析
	if node.Raw != "" {
		u, err := url.Parse(node.Raw)
		if err == nil {
			u.Fragment = newName
			return u.String(), nil
		}
		return node.Raw, nil
	}

	// 从 Clash 配置构建 URL
	if node.Clash == nil {
		return "", nil
	}

	return buildURLFromClash(node, newName)
}

func buildURLFromClash(node model.Node, newName string) (string, error) {
	m := node.Clash
	scheme := strings.ToLower(node.Scheme)

	switch scheme {
	case "vless":
		return buildVlessURLFromClash(m, node, newName)
	case "trojan":
		return buildTrojanURLFromClash(m, node, newName)
	case "vmess":
		return buildVmessURLFromClash(m, node, newName)
	case "ss":
		return buildSSURLFromClash(m, node, newName)
	case "hysteria2", "hysteria":
		return buildHysteria2URLFromClash(m, node, newName)
	case "tuic":
		return buildTuicURLFromClash(m, node, newName)
	case "socks5", "socks":
		return buildSocks5URLFromClash(m, node, newName)
	case "http", "https":
		return buildHTTPURLFromClash(m, node, newName)
	default:
		return "", nil
	}
}

func buildVlessURLFromClash(m map[string]interface{}, node model.Node, newName string) (string, error) {
	uuid := util.GetStringFromMap(m, "uuid")
	if uuid == "" {
		return "", nil
	}

	u := &url.URL{
		Scheme: "vless",
		User:   url.User(uuid),
		Host:   net.JoinHostPort(node.Host, strconv.Itoa(node.Port)),
	}

	// 构建查询参数
	params := url.Values{}
	if network := util.GetStringFromMap(m, "network"); network != "" {
		params.Set("type", network)
	}
	if security := util.GetStringFromMap(m, "tls"); security == "true" {
		params.Set("security", "tls")
	}
	if sni := util.GetStringFromMap(m, "servername"); sni != "" {
		params.Set("sni", sni)
	}
	if flow := util.GetStringFromMap(m, "flow"); flow != "" {
		params.Set("flow", flow)
	}

	// 处理 ws
	if network := util.GetStringFromMap(m, "network"); network == "ws" {
		if wsPath := util.GetStringFromMap(m, "ws-path"); wsPath != "" {
			params.Set("path", wsPath)
		}
		if wsHeaders, ok := util.GetStringMap(m, "ws-headers"); ok {
			if host := wsHeaders["Host"]; host != "" {
				params.Set("host", host)
			}
		}
	}

	u.RawQuery = params.Encode()
	u.Fragment = newName
	return u.String(), nil
}

func buildTrojanURLFromClash(m map[string]interface{}, node model.Node, newName string) (string, error) {
	password := util.GetStringFromMap(m, "password")
	if password == "" {
		return "", nil
	}

	u := &url.URL{
		Scheme: "trojan",
		User:   url.User(password),
		Host:   net.JoinHostPort(node.Host, strconv.Itoa(node.Port)),
	}

	// 构建查询参数
	params := url.Values{}
	if sni := util.GetStringFromMap(m, "sni"); sni != "" {
		params.Set("sni", sni)
	}
	network := util.GetStringFromMap(m, "network")
	if network != "" {
		params.Set("type", network)
	}
	if network == "ws" {
		if wsPath := util.GetStringFromMap(m, "ws-path"); wsPath != "" {
			params.Set("path", wsPath)
		}
		if wsHeaders, ok := util.GetStringMap(m, "ws-headers"); ok {
			if host := wsHeaders["Host"]; host != "" {
				params.Set("host", host)
			}
		}
	}

	u.RawQuery = params.Encode()
	u.Fragment = newName
	return u.String(), nil
}

func buildVmessURLFromClash(m map[string]interface{}, node model.Node, newName string) (string, error) {
	// 构建 VMess JSON
	fields := map[string]interface{}{
		"add":   node.Host,
		"port":  node.Port,
		"ps":    newName,
		"scy":   util.GetStringFromMap(m, "cipher"),
		"net":   util.GetStringFromMap(m, "network"),
		"type":  util.GetStringFromMap(m, "network"),
		"host":  "",
		"path":  "",
		"tls":   "",
		"sni":   "",
	}

	if tls, _ := util.GetBoolFromMap(m, "tls"); tls {
		fields["tls"] = "tls"
	}
	if sni := util.GetStringFromMap(m, "servername"); sni != "" {
		fields["sni"] = sni
	}
	if network := util.GetStringFromMap(m, "network"); network == "ws" {
		if wsPath := util.GetStringFromMap(m, "ws-path"); wsPath != "" {
			fields["path"] = wsPath
		}
		if wsHeaders, ok := util.GetStringMap(m, "ws-headers"); ok {
			if host := wsHeaders["Host"]; host != "" {
				fields["host"] = host
			}
		}
	}
	if uuid := util.GetStringFromMap(m, "uuid"); uuid != "" {
		fields["id"] = uuid
	}
	if alterId := util.GetIntFromMapDefault(m, "alterId", 0); alterId > 0 {
		fields["aid"] = alterId
	}
	if v := util.GetStringFromMap(m, "v"); v != "" {
		fields["v"] = v
	} else {
		fields["v"] = "2"
	}

	data, err := json.Marshal(fields)
	if err != nil {
		return "", err
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(data), nil
}

func buildSSURLFromClash(m map[string]interface{}, node model.Node, newName string) (string, error) {
	method := util.FirstNonEmpty(util.GetStringFromMap(m, "cipher"), util.GetStringFromMap(m, "method"))
	password := util.GetStringFromMap(m, "password")
	if method == "" || password == "" {
		return "", nil
	}

	u := &url.URL{
		Scheme: "ss",
		User:   url.UserPassword(method, password),
		Host:   net.JoinHostPort(node.Host, strconv.Itoa(node.Port)),
	}

	// 处理 plugin
	if plugin := util.GetStringFromMap(m, "plugin"); plugin != "" {
		pluginOpts := util.GetStringFromMap(m, "plugin-opts")
		if pluginOpts != "" {
			u.RawQuery = "plugin=" + plugin + "&" + pluginOpts
		}
	}

	u.Fragment = newName
	return u.String(), nil
}

func buildHysteria2URLFromClash(m map[string]interface{}, node model.Node, newName string) (string, error) {
	u := &url.URL{
		Scheme: "hysteria2",
		Host:   net.JoinHostPort(node.Host, strconv.Itoa(node.Port)),
	}

	// password 在 userinfo 位置
	if password := util.GetStringFromMap(m, "password"); password != "" {
		u.User = url.User(password)
	}

	// 构建查询参数
	params := url.Values{}
	if sni := util.GetStringFromMap(m, "sni"); sni != "" {
		params.Set("sni", sni)
	}
	if obfs := util.GetStringFromMap(m, "obfs"); obfs != "" {
		params.Set("obfs", obfs)
	}
	if obfsPw := util.GetStringFromMap(m, "obfs-password"); obfsPw != "" {
		params.Set("obfs-password", obfsPw)
	}
	if insecure, _ := util.GetBoolFromMap(m, "skip-cert-verify"); insecure {
		params.Set("insecure", "1")
	}

	u.RawQuery = params.Encode()
	u.Fragment = newName
	return u.String(), nil
}

func buildTuicURLFromClash(m map[string]interface{}, node model.Node, newName string) (string, error) {
	uuid := util.GetStringFromMap(m, "uuid")
	if uuid == "" {
		return "", nil
	}

	password := util.GetStringFromMap(m, "password")
	if password == "" {
		password = util.GetStringFromMap(m, "token")
	}

	u := &url.URL{
		Scheme: "tuic",
		Host:   net.JoinHostPort(node.Host, strconv.Itoa(node.Port)),
	}

	if password != "" {
		u.User = url.UserPassword(uuid, password)
	} else {
		u.User = url.User(uuid)
	}

	// 构建查询参数
	params := url.Values{}
	if sni := util.GetStringFromMap(m, "sni"); sni != "" {
		params.Set("sni", sni)
	}
	if cc := util.GetStringFromMap(m, "congestion-controller"); cc != "" {
		params.Set("congestion-controller", cc)
	}
	if alpn := util.GetStringFromMap(m, "alpn"); alpn != "" {
		params.Set("alpn", alpn)
	}
	if insecure, _ := util.GetBoolFromMap(m, "skip-cert-verify"); insecure {
		params.Set("insecure", "1")
	}

	u.RawQuery = params.Encode()
	u.Fragment = newName
	return u.String(), nil
}

func buildSocks5URLFromClash(m map[string]interface{}, node model.Node, newName string) (string, error) {
	u := &url.URL{
		Scheme: "socks5",
		Host:   net.JoinHostPort(node.Host, strconv.Itoa(node.Port)),
	}

	username := util.GetStringFromMap(m, "username")
	password := util.GetStringFromMap(m, "password")
	if username != "" && password != "" {
		u.User = url.UserPassword(username, password)
	} else if username != "" {
		u.User = url.User(username)
	}

	// 构建查询参数
	params := url.Values{}
	if tls, _ := util.GetBoolFromMap(m, "tls"); tls {
		params.Set("tls", "1")
	}

	if len(params) > 0 {
		u.RawQuery = params.Encode()
	}
	u.Fragment = newName
	return u.String(), nil
}

func buildHTTPURLFromClash(m map[string]interface{}, node model.Node, newName string) (string, error) {
	scheme := "http"
	if tls, _ := util.GetBoolFromMap(m, "tls"); tls {
		scheme = "https"
	}

	u := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(node.Host, strconv.Itoa(node.Port)),
	}

	username := util.GetStringFromMap(m, "username")
	password := util.GetStringFromMap(m, "password")
	if username != "" && password != "" {
		u.User = url.UserPassword(username, password)
	} else if username != "" {
		u.User = url.User(username)
	}

	u.Fragment = newName
	return u.String(), nil
}

// BuildFilteredProxyList builds a list of Clash proxy maps with type filtering
func BuildFilteredProxyList(nodes []model.Node, results []model.Result, settings model.TestSettings, typeFilter []string) []map[string]interface{} {
	typeSet := make(map[string]struct{})
	for _, t := range typeFilter {
		typeSet[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
	}

	var proxies []map[string]interface{}
	nameCount := make(map[string]int)

	for i, node := range nodes {
		if i >= len(results) {
			continue
		}
		res := results[i]
		if !res.Done || !res.Pass {
			continue
		}

		// Type filtering
		scheme := strings.ToLower(node.Scheme)
		if len(typeSet) > 0 {
			if _, ok := typeSet[scheme]; !ok {
				continue
			}
		}

		baseName, _ := util.BuildOutputName(node, settings, res)
		baseName = util.AbbreviateRegionInName(baseName)
		name := util.UniqueName(baseName, nameCount)

		proxy, err := NodeToClashProxy(node, name)
		if err != nil {
			continue
		}
		proxies = append(proxies, proxy)
	}
	return proxies
}

// FormatYAMLFlow formats a proxy map as YAML flow style (inline)
// Example: - {name: xxx, server: xxx, port: 443, type: vless, ...}
func FormatYAMLFlow(proxy map[string]interface{}) string {
	if proxy == nil {
		return ""
	}

	// Define the preferred key order for readability
	keyOrder := []string{
		"name", "server", "port", "type",
		"uuid", "password", "method", "cipher",
		"tls", "sni", "servername", "skip-cert-verify",
		"network", "ws-opts", "grpc-opts",
		"reality-opts", "client-fingerprint", "alpn",
		"flow", "encryption", "alterId",
		"username", "obfs", "obfs-password",
		"udp", "congestion-controller",
	}

	// Build ordered map
	ordered := make([]struct {
		k string
		v interface{}
	}, 0, len(proxy))

	// Add keys in preferred order
	addedKeys := make(map[string]struct{})
	for _, k := range keyOrder {
		if v, ok := proxy[k]; ok {
			ordered = append(ordered, struct {
				k string
				v interface{}
			}{k, v})
			addedKeys[k] = struct{}{}
		}
	}

	// Add remaining keys
	for k, v := range proxy {
		if _, added := addedKeys[k]; !added {
			ordered = append(ordered, struct {
				k string
				v interface{}
			}{k, v})
		}
	}

	// Build flow format
	var parts []string
	for _, item := range ordered {
		vStr := formatYAMLValue(item.v)
		parts = append(parts, fmt.Sprintf("%s: %s", item.k, vStr))
	}

	return "{ " + strings.Join(parts, ", ") + " }"
}

// formatYAMLValue formats a value for YAML flow style
func formatYAMLValue(v interface{}) string {
	if v == nil {
		return "null"
	}

	switch val := v.(type) {
	case string:
		// Check if string needs quoting
		if strings.ContainsAny(val, ":{}[]'\"#,|>\\n") ||
			strings.HasPrefix(val, " ") ||
			strings.HasSuffix(val, " ") ||
			val == "" {
			// Escape single quotes and use single quotes
			escaped := strings.ReplaceAll(val, "'", "''")
			return "'" + escaped + "'"
		}
		// Check if it looks like a number or boolean
		if _, err := strconv.ParseInt(val, 10, 64); err == nil {
			return "'" + val + "'"
		}
		if _, err := strconv.ParseFloat(val, 64); err == nil {
			return "'" + val + "'"
		}
		if val == "true" || val == "false" || val == "null" {
			return "'" + val + "'"
		}
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%v", val)
	case []string:
		if len(val) == 0 {
			return "[]"
		}
		var items []string
		for _, s := range val {
			items = append(items, formatYAMLValue(s))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case []interface{}:
		if len(val) == 0 {
			return "[]"
		}
		var items []string
		for _, item := range val {
			items = append(items, formatYAMLValue(item))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case map[string]interface{}:
		if len(val) == 0 {
			return "{}"
		}
		var items []string
		for k, vv := range val {
			items = append(items, k+": "+formatYAMLValue(vv))
		}
		return "{" + strings.Join(items, ", ") + "}"
	default:
		// Fallback to string representation
		s := fmt.Sprintf("%v", val)
		return formatYAMLValue(s)
	}
}

// BuildYAMLFlowList builds a list of YAML flow format proxy strings
func BuildYAMLFlowList(nodes []model.Node, results []model.Result, settings model.TestSettings, typeFilter []string) []string {
	proxies := BuildFilteredProxyList(nodes, results, settings, typeFilter)
	var lines []string
	for _, proxy := range proxies {
		line := "  - " + FormatYAMLFlow(proxy)
		lines = append(lines, line)
	}
	return lines
}
