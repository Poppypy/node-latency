package clash

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"node-latency/internal/model"
	"node-latency/internal/parser"
	"node-latency/internal/util"
)

func NodeToClashProxy(node model.Node, name string) (map[string]interface{}, error) {
	if node.Clash != nil {
		cloned, ok := util.CloneStringMap(node.Clash)
		if !ok {
			return nil, errors.New("代理配置无效")
		}
		cloned["name"] = name
		p := NormalizeClashProxy(cloned)
		if err := ValidateClashProxy(p); err != nil {
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
		return NormalizeClashProxy(p), nil
	case "ss":
		p, err := buildSSProxy(node, name)
		if err != nil {
			return nil, err
		}
		return NormalizeClashProxy(p), nil
	case "trojan":
		p, err := buildTrojanProxy(node, name)
		if err != nil {
			return nil, err
		}
		return NormalizeClashProxy(p), nil
	case "hysteria2", "hysteria":
		return buildHysteria2Proxy(node, name)
	case "tuic":
		return buildTuicProxy(node, name)
	case "socks5", "socks":
		return buildSocks5Proxy(node, name)
	case "http", "https":
		return buildHTTPProxy(node, name)
	default:
		return nil, fmt.Errorf("不支持的协议：%s", node.Scheme)
	}
}

func buildVlessProxy(node model.Node, name string) (map[string]interface{}, error) {
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

	// ✅ UUID 预校验
	uuidRaw := u.User.Username()
	uuid, err := parser.NormalizeUUID(uuidRaw)
	if err != nil {
		return nil, fmt.Errorf("vless UUID 无效：%v", err)
	}

	params := node.Params
	if params == nil {
		params = parser.ParseQueryKeepPlus(u.RawQuery)
	}

	network := strings.ToLower(parser.ParamGet(params, "type"))
	if network == "" {
		network = "tcp"
	}

	sec := strings.ToLower(strings.TrimSpace(util.FirstNonEmpty(parser.ParamGet(params, "security"), node.Security)))
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
	proxy["encryption"] = NormalizeVlessEncryption(parser.ParamGet(params, "encryption"))

	if sni := util.FirstNonEmpty(parser.ParamGet(params, "sni"), parser.ParamGet(params, "servername"), parser.ParamGet(params, "peer"), node.SNI); sni != "" {
		proxy["servername"] = sni
	}
	if fp := parser.ParamGet(params, "fp"); fp != "" {
		proxy["client-fingerprint"] = fp
	}
	if alpn := parser.ParamGet(params, "alpn"); alpn != "" {
		proxy["alpn"] = util.SplitCSV(alpn)
	}
	if flow := parser.ParamGet(params, "flow"); flow != "" {
		proxy["flow"] = flow
	}
	if v, ok := util.ParseBoolStr(util.FirstNonEmpty(parser.ParamGet(params, "allowInsecure"), parser.ParamGet(params, "insecure"))); ok {
		proxy["skip-cert-verify"] = v
	}

	// ✅ Reality：必须 public-key 有效
	if sec == "reality" {
		ro := map[string]interface{}{}
		pbkRaw := util.FirstNonEmpty(
			parser.ParamGet(params, "pbk"),
			parser.ParamGet(params, "publickey"),
			parser.ParamGet(params, "public-key"),
		)
		pbk, err := parser.NormalizeRealityPublicKey(pbkRaw)
		if err != nil {
			return nil, fmt.Errorf("reality public-key 无效：%v", err)
		}
		ro["public-key"] = pbk

		sidRaw := util.FirstNonEmpty(
			parser.ParamGet(params, "sid"),
			parser.ParamGet(params, "shortid"),
			parser.ParamGet(params, "short-id"),
		)
		if sidRaw != "" {
			sid, err := parser.NormalizeRealityShortID(sidRaw)
			if err != nil {
				return nil, fmt.Errorf("reality short-id 无效：%v", err)
			}
			if sid != "" {
				ro["short-id"] = sid
			}
		}

		spx := util.FirstNonEmpty(
			parser.ParamGet(params, "spx"),
			parser.ParamGet(params, "spiderx"),
			parser.ParamGet(params, "spider-x"),
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
		path := util.FirstNonEmpty(parser.ParamGet(params, "path"), u.Path)
		if path == "" {
			path = "/"
		}
		wsOpts["path"] = path
		if host := parser.ParamGet(params, "host"); host != "" {
			wsOpts["headers"] = map[string]interface{}{"Host": host}
		}
		proxy["ws-opts"] = wsOpts
	case "grpc":
		if svc := util.FirstNonEmpty(parser.ParamGet(params, "servicename"), parser.ParamGet(params, "service"), parser.ParamGet(params, "grpc-service-name")); svc != "" {
			proxy["grpc-opts"] = map[string]interface{}{"grpc-service-name": svc}
		}
	}

	return NormalizeClashProxy(proxy), nil
}

// PLACEHOLDER_VMESS_SS_TROJAN

func buildVmessProxy(node model.Node, name string) (map[string]interface{}, error) {
	if node.Vmess == nil {
		return nil, errors.New("无效的 vmess 节点")
	}
	fields := node.Vmess.Fields
	server := util.FirstNonEmpty(util.GetString(fields, "add"), node.Host)
	portStr := util.FirstNonEmpty(util.GetString(fields, "port"), strconv.Itoa(node.Port))
	port, err := util.ParsePort(portStr)
	if err != nil {
		return nil, err
	}
	uuidRaw := util.GetString(fields, "id")
	uuid, err := parser.NormalizeUUID(uuidRaw)
	if err != nil {
		return nil, fmt.Errorf("vmess UUID 无效：%v", err)
	}
	network := strings.ToLower(util.GetString(fields, "net"))
	proxy := map[string]interface{}{
		"name":   name,
		"type":   "vmess",
		"server": server,
		"port":   port,
		"uuid":   uuid,
		"udp":    true,
	}
	cipher := util.FirstNonEmpty(util.GetString(fields, "scy"), util.GetString(fields, "cipher"))
	if strings.TrimSpace(cipher) == "" {
		cipher = "auto"
	}
	proxy["cipher"] = cipher
	if aidStr := util.GetString(fields, "aid"); aidStr != "" {
		if aid, err := strconv.Atoi(aidStr); err == nil {
			proxy["alterId"] = aid
		}
	}
	if network != "" {
		proxy["network"] = network
	}
	if tlsVal := strings.ToLower(util.GetString(fields, "tls")); tlsVal != "" {
		proxy["tls"] = tlsVal == "tls" || tlsVal == "true"
	}
	if sni := util.GetString(fields, "sni"); sni != "" {
		proxy["servername"] = sni
	}
	if fp := util.GetString(fields, "fp"); fp != "" {
		proxy["client-fingerprint"] = fp
	}
	if alpn := util.GetString(fields, "alpn"); alpn != "" {
		proxy["alpn"] = util.SplitCSV(alpn)
	}
	if v, ok := util.ParseBoolStr(util.FirstNonEmpty(util.GetString(fields, "allowInsecure"), util.GetString(fields, "allowinsecure"))); ok {
		proxy["skip-cert-verify"] = v
	}
	switch network {
	case "ws":
		wsOpts := map[string]interface{}{}
		path := util.GetString(fields, "path")
		if path == "" {
			path = "/"
		}
		wsOpts["path"] = path
		if host := util.GetString(fields, "host"); host != "" {
			wsOpts["headers"] = map[string]interface{}{"Host": host}
		}
		proxy["ws-opts"] = wsOpts
	case "grpc":
		if svc := util.FirstNonEmpty(util.GetString(fields, "serviceName"), util.GetString(fields, "servicename")); svc != "" {
			proxy["grpc-opts"] = map[string]interface{}{"grpc-service-name": svc}
		}
	}
	return proxy, nil
}

func buildSSProxy(node model.Node, name string) (map[string]interface{}, error) {
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

// ✅ trojan：强制 tls=true；同时支持 security=reality（pbk/sid/spx）
func buildTrojanProxy(node model.Node, name string) (map[string]interface{}, error) {
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
		params = parser.ParseQueryKeepPlus(u.RawQuery)
	}

	network := strings.ToLower(parser.ParamGet(params, "type"))
	if network == "" {
		network = "tcp"
	}

	sec := strings.ToLower(strings.TrimSpace(util.FirstNonEmpty(parser.ParamGet(params, "security"), node.Security)))
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

	if sni := util.FirstNonEmpty(parser.ParamGet(params, "sni"), parser.ParamGet(params, "peer"), parser.ParamGet(params, "servername"), node.SNI); sni != "" {
		p["sni"] = sni
	}
	if fp := parser.ParamGet(params, "fp"); fp != "" {
		p["client-fingerprint"] = fp
	}
	if alpn := parser.ParamGet(params, "alpn"); alpn != "" {
		p["alpn"] = util.SplitCSV(alpn)
	}
	if v, ok := util.ParseBoolStr(util.FirstNonEmpty(parser.ParamGet(params, "allowInsecure"), parser.ParamGet(params, "insecure"))); ok {
		p["skip-cert-verify"] = v
	}

	if sec == "reality" {
		ro := map[string]interface{}{}
		pbkRaw := util.FirstNonEmpty(parser.ParamGet(params, "pbk"), parser.ParamGet(params, "publickey"), parser.ParamGet(params, "public-key"))
		pbk, err := parser.NormalizeRealityPublicKey(pbkRaw)
		if err != nil {
			return nil, fmt.Errorf("reality public-key 无效：%v", err)
		}
		ro["public-key"] = pbk

		sidRaw := util.FirstNonEmpty(parser.ParamGet(params, "sid"), parser.ParamGet(params, "shortid"), parser.ParamGet(params, "short-id"))
		if sidRaw != "" {
			sid, err := parser.NormalizeRealityShortID(sidRaw)
			if err != nil {
				return nil, fmt.Errorf("reality short-id 无效：%v", err)
			}
			if sid != "" {
				ro["short-id"] = sid
			}
		}

		spx := util.FirstNonEmpty(parser.ParamGet(params, "spx"), parser.ParamGet(params, "spiderx"), parser.ParamGet(params, "spider-x"))
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
		path := util.FirstNonEmpty(parser.ParamGet(params, "path"), u.Path)
		if path == "" {
			path = "/"
		}
		wsOpts["path"] = path
		if host := parser.ParamGet(params, "host"); host != "" {
			wsOpts["headers"] = map[string]interface{}{"Host": host}
		}
		p["ws-opts"] = wsOpts
	case "grpc":
		if svc := util.FirstNonEmpty(parser.ParamGet(params, "servicename"), parser.ParamGet(params, "service"), parser.ParamGet(params, "grpc-service-name")); svc != "" {
			p["grpc-opts"] = map[string]interface{}{"grpc-service-name": svc}
		}
	}

	return p, nil
}

func buildHysteria2Proxy(node model.Node, name string) (map[string]interface{}, error) {
	// 如果来自 Clash 配置，直接克隆并更新名称
	if node.Clash != nil {
		cloned, ok := util.CloneStringMap(node.Clash)
		if !ok {
			return nil, errors.New("hysteria2 配置无效")
		}
		cloned["name"] = name
		cloned["type"] = "hysteria2"
		return NormalizeClashProxy(cloned), nil
	}

	// 从 URL 构建（hysteria2://password@host:port?sni=xxx）
	proxy := map[string]interface{}{
		"name":   name,
		"type":   "hysteria2",
		"server": node.Host,
		"port":   node.Port,
	}

	if node.URL != nil && node.URL.User != nil {
		if pw, ok := node.URL.User.Password(); ok && pw != "" {
			proxy["password"] = pw
		} else {
			proxy["password"] = node.URL.User.Username()
		}
	}

	params := node.Params
	if params == nil && node.URL != nil {
		params = parser.ParseQueryKeepPlus(node.URL.RawQuery)
	}
	if params != nil {
		if sni := parser.ParamGet(params, "sni"); sni != "" {
			proxy["sni"] = sni
		}
		if obfs := parser.ParamGet(params, "obfs"); obfs != "" {
			proxy["obfs"] = obfs
		}
		if obfsPw := parser.ParamGet(params, "obfs-password"); obfsPw != "" {
			proxy["obfs-password"] = obfsPw
		}
	}

	if node.Security == "tls" {
		proxy["tls"] = true
	}

	return NormalizeClashProxy(proxy), nil
}

func buildTuicProxy(node model.Node, name string) (map[string]interface{}, error) {
	// 如果来自 Clash 配置，直接克隆并更新名称
	if node.Clash != nil {
		cloned, ok := util.CloneStringMap(node.Clash)
		if !ok {
			return nil, errors.New("tuic 配置无效")
		}
		cloned["name"] = name
		cloned["type"] = "tuic"
		return NormalizeClashProxy(cloned), nil
	}

	// 从 URL 构建（tuic://uuid:password@host:port?sni=xxx）
	proxy := map[string]interface{}{
		"name":   name,
		"type":   "tuic",
		"server": node.Host,
		"port":   node.Port,
		"udp":    true,
	}

	if node.URL != nil && node.URL.User != nil {
		proxy["uuid"] = node.URL.User.Username()
		if pw, ok := node.URL.User.Password(); ok && pw != "" {
			proxy["password"] = pw
		}
	}

	params := node.Params
	if params == nil && node.URL != nil {
		params = parser.ParseQueryKeepPlus(node.URL.RawQuery)
	}
	if params != nil {
		if sni := parser.ParamGet(params, "sni"); sni != "" {
			proxy["sni"] = sni
		}
		if cc := parser.ParamGet(params, "congestion-controller"); cc != "" {
			proxy["congestion-controller"] = cc
		}
		if alpn := parser.ParamGet(params, "alpn"); alpn != "" {
			proxy["alpn"] = util.SplitCSV(alpn)
		}
	}

	if node.Security == "tls" {
		proxy["tls"] = true
	}

	return NormalizeClashProxy(proxy), nil
}

func buildSocks5Proxy(node model.Node, name string) (map[string]interface{}, error) {
	// 如果来自 Clash 配置，直接克隆并更新名称
	if node.Clash != nil {
		cloned, ok := util.CloneStringMap(node.Clash)
		if !ok {
			return nil, errors.New("socks5 配置无效")
		}
		cloned["name"] = name
		cloned["type"] = "socks5"
		return NormalizeClashProxy(cloned), nil
	}

	// 从 URL 构建（socks5://user:pass@host:port）
	proxy := map[string]interface{}{
		"name":   name,
		"type":   "socks5",
		"server": node.Host,
		"port":   node.Port,
		"udp":    true,
	}

	if node.URL != nil && node.URL.User != nil {
		proxy["username"] = node.URL.User.Username()
		if pw, ok := node.URL.User.Password(); ok && pw != "" {
			proxy["password"] = pw
		}
	}

	if node.Security == "tls" {
		proxy["tls"] = true
	}

	return NormalizeClashProxy(proxy), nil
}

func buildHTTPProxy(node model.Node, name string) (map[string]interface{}, error) {
	// 如果来自 Clash 配置，直接克隆并更新名称
	if node.Clash != nil {
		cloned, ok := util.CloneStringMap(node.Clash)
		if !ok {
			return nil, errors.New("http 配置无效")
		}
		cloned["name"] = name
		cloned["type"] = "http"
		return NormalizeClashProxy(cloned), nil
	}

	// 从 URL 构建（http://user:pass@host:port）
	proxy := map[string]interface{}{
		"name":   name,
		"type":   "http",
		"server": node.Host,
		"port":   node.Port,
	}

	if node.URL != nil && node.URL.User != nil {
		proxy["username"] = node.URL.User.Username()
		if pw, ok := node.URL.User.Password(); ok && pw != "" {
			proxy["password"] = pw
		}
	}

	if node.Security == "tls" {
		proxy["tls"] = true
	}

	return NormalizeClashProxy(proxy), nil
}
