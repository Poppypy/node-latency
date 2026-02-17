package clash

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"node-latency/internal/model"
	"node-latency/internal/util"

	"gopkg.in/yaml.v3"
)

func BuildClashConfig(templatePath string, nodes []model.Node, results []model.Result, settings model.TestSettings) (map[string]interface{}, []string, error) {
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
			"dns":         map[string]interface{}{"enable": false},
			"proxy-groups": []interface{}{
				map[string]interface{}{
					"name":    "AUTO",
					"type":    "select",
					"proxies": []string{},
				},
			},
			"rules": []interface{}{"MATCH,AUTO"},
		}
		oldNames := extractProxyNames(root)
		proxies, names := BuildClashProxies(nodes, results, settings)
		if len(names) == 0 {
			return nil, nil, errors.New("没有节点可导出")
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
	proxies, names := BuildClashProxies(nodes, results, settings)
	if len(names) == 0 {
		return nil, nil, errors.New("没有节点可导出")
	}
	root["proxies"] = proxies
	updateProxyGroups(root, oldNames, names)
	return root, names, nil
}

func BuildClashProxies(nodes []model.Node, results []model.Result, settings model.TestSettings) ([]interface{}, []string) {
	var proxies []interface{}
	var names []string
	nameCount := make(map[string]int)
	hasResults := len(results) > 0

	for i, node := range nodes {
		// 如果有测试结果，只导出通过的节点
		if hasResults {
			if i >= len(results) {
				continue
			}
			res := results[i]
			if !res.Done || !res.Pass {
				continue
			}
		}

		var res model.Result
		if i < len(results) {
			res = results[i]
		}
		baseName, _ := util.BuildOutputName(node, settings, res)
		// 对名称进行地区缩写和 URL 解码处理
		baseName = util.AbbreviateRegionInName(baseName)
		name := util.UniqueName(baseName, nameCount)
		proxy, err := NodeToClashProxy(node, name)
		if err != nil {
			continue
		}
		proxies = append(proxies, proxy)
		names = append(names, name)
	}
	return proxies, names
}

func BuildTestProxies(nodes []model.Node, skipFlows map[string]struct{}, badNodeReasons map[int]string) ([]interface{}, []string, []string, []string, []string) {
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

		base := strings.TrimSpace(util.DisplayName(node))
		if base == "" {
			base = fmt.Sprintf("节点_%d", i+1)
		}
		name := util.UniqueName(base, nameCount)

		proxy, err := NodeToClashProxy(node, name)
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
		if err := ValidateClashProxy(proxy); err != nil {
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

func extractProxyNames(root map[string]interface{}) []string {
	var names []string
	list, ok := util.ToSlice(root["proxies"])
	if !ok {
		return names
	}
	for _, item := range list {
		m, ok := util.ToStringMap(item)
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
	groups, ok := util.ToSlice(root["proxy-groups"])
	if !ok {
		return
	}
	oldSet := make(map[string]struct{})
	for _, n := range oldNames {
		oldSet[n] = struct{}{}
	}
	for i, item := range groups {
		groupMap, ok := util.ToStringMap(item)
		if !ok {
			continue
		}
		proxyList, ok := util.ToStringSlice(groupMap["proxies"])
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
