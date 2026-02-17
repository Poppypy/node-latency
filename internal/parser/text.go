package parser

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"node-latency/internal/model"
	"node-latency/internal/util"

	"gopkg.in/yaml.v3"
)

func ParseNodesFromText(text string) ([]model.Node, []string, error) {
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

func tryParseYAMLNodes(text string) ([]model.Node, []string, bool) {
	var data interface{}
	clean := util.SanitizeTextForYAML(text)
	if err := yaml.Unmarshal([]byte(clean), &data); err != nil {
		return nil, nil, false
	}
	nodes, warnings, ok := parseNodesFromYAMLData(data)
	if !ok || len(nodes) == 0 {
		return nil, nil, false
	}
	return nodes, warnings, true
}

func parseNodesFromYAMLData(data interface{}) ([]model.Node, []string, bool) {
	var allNodes []model.Node
	var allWarnings []string
	hasAny := false

	switch v := data.(type) {
	case map[string]interface{}:
		// 解析 proxies 字段
		if proxies, ok := util.ToSlice(v["proxies"]); ok && len(proxies) > 0 {
			nodes, warnings := parseProxyList(proxies)
			allNodes = append(allNodes, nodes...)
			allWarnings = append(allWarnings, warnings...)
			hasAny = true
		}
		// 解析 proxy-providers 字段
		if providers, ok := util.ToStringMap(v["proxy-providers"]); ok && len(providers) > 0 {
			nodes, warnings := parseProxyProviders(providers)
			allNodes = append(allNodes, nodes...)
			allWarnings = append(allWarnings, warnings...)
			hasAny = true
		}
	case map[interface{}]interface{}:
		v2, _ := util.ToStringMap(v)
		// 解析 proxies 字段
		if proxies, ok := util.ToSlice(v2["proxies"]); ok && len(proxies) > 0 {
			nodes, warnings := parseProxyList(proxies)
			allNodes = append(allNodes, nodes...)
			allWarnings = append(allWarnings, warnings...)
			hasAny = true
		}
		// 解析 proxy-providers 字段
		if providers, ok := util.ToStringMap(v2["proxy-providers"]); ok && len(providers) > 0 {
			nodes, warnings := parseProxyProviders(providers)
			allNodes = append(allNodes, nodes...)
			allWarnings = append(allWarnings, warnings...)
			hasAny = true
		}
	case []interface{}:
		nodes, warnings := parseProxyList(v)
		return nodes, warnings, true
	}

	if !hasAny || len(allNodes) == 0 {
		return nil, allWarnings, false
	}
	return allNodes, allWarnings, true
}

func parseProxyProviders(providers map[string]interface{}) ([]model.Node, []string) {
	var allNodes []model.Node
	var warnings []string

	for name, provider := range providers {
		p, ok := util.ToStringMap(provider)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("proxy-provider %s: 格式无效", name))
			continue
		}

		providerType := strings.ToLower(util.GetStringFromMap(p, "type"))
		if providerType != "http" {
			warnings = append(warnings, fmt.Sprintf("proxy-provider %s: 只支持 http 类型", name))
			continue
		}

		url := util.GetStringFromMap(p, "url")
		if url == "" {
			warnings = append(warnings, fmt.Sprintf("proxy-provider %s: 缺少 url", name))
			continue
		}

		// 获取 provider 内容
		text, err := util.FetchSubscription(url, 30*time.Second)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("proxy-provider %s: 获取失败: %v", name, err))
			continue
		}

		// 解析 provider 内容
		nodes, parseWarnings, ok := tryParseYAMLNodes(text)
		if !ok || len(nodes) == 0 {
			// 尝试 base64 解码
			if decoded, ok2 := tryDecodeSubscription(text); ok2 {
				nodes, parseWarnings, ok = tryParseYAMLNodes(decoded)
			}
		}

		if !ok || len(nodes) == 0 {
			warnings = append(warnings, fmt.Sprintf("proxy-provider %s: 未解析到节点", name))
			continue
		}

		// 添加前缀
		override, _ := util.ToStringMap(p["override"])
		prefix := util.GetStringFromMap(override, "additional-prefix")
		if prefix != "" {
			for i := range nodes {
				nodes[i].Name = prefix + nodes[i].Name
				nodes[i].OriginalName = prefix + nodes[i].OriginalName
			}
		}

		for _, w := range parseWarnings {
			warnings = append(warnings, fmt.Sprintf("proxy-provider %s: %s", name, w))
		}
		allNodes = append(allNodes, nodes...)
	}

	return allNodes, warnings
}

func parseProxyList(list []interface{}) ([]model.Node, []string) {
	var nodes []model.Node
	var warnings []string
	for i, item := range list {
		m, ok := util.ToStringMap(item)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("proxy %d: 不是有效对象", i+1))
			continue
		}
		node, err := ParseNodeFromClashProxy(m)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("proxy %d: %v", i+1, err))
			continue
		}
		if cleaned, ok := SanitizeNode(node); ok {
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

func parseNodesFromLines(text string) ([]model.Node, []string, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
	var (
		nodes    []model.Node
		warnings []string
	)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(util.SanitizeLineText(scanner.Text()))
		if lineNum == 1 {
			line = strings.TrimPrefix(line, "\uFEFF")
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		node, err := ParseNode(line)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d: %v", lineNum, err))
			continue
		}
		if cleaned, ok := SanitizeNode(node); ok {
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
			if unicode.IsControl(r) || util.IsUnicodeNoncharacter(r) {
				return -1
			}
			return r
		}
	}, text)
	if clean == "" {
		return "", false
	}
	b, err := util.DecodeBase64(clean)
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

func ReadNodesFromFile(path string) ([]model.Node, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	text := string(data)
	return ParseNodesFromText(text)
}
