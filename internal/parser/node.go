package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"node-latency/internal/model"
	"node-latency/internal/util"
)

func DecodeFragment(fragment string) string {
	if fragment == "" {
		return ""
	}
	// 使用统一的完全解码函数
	return util.FullyDecodeURL(fragment)
}

func ParseNode(raw string) (model.Node, error) {
	if strings.HasPrefix(strings.ToLower(raw), "vmess://") {
		return ParseVmess(raw)
	}
	if strings.HasPrefix(strings.ToLower(raw), "ss://") {
		return ParseSS(raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return model.Node{}, err
	}
	if u.Scheme == "" || u.Host == "" {
		return model.Node{}, errors.New("missing scheme or host")
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return model.Node{}, errors.New("missing or invalid port")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return model.Node{}, errors.New("invalid port")
	}
	name := DecodeFragment(u.Fragment)

	// ✅ 关键：Params 用 PathUnescape 解析，不会把 + 变空格
	params := ParseQueryKeepPlus(u.RawQuery)

	security := strings.ToLower(strings.TrimSpace(ParamGet(params, "security")))
	if strings.EqualFold(u.Scheme, "trojan") && security == "" {
		security = "tls"
	}
	sni := util.FirstNonEmpty(ParamGet(params, "sni"), ParamGet(params, "servername"), ParamGet(params, "peer"))
	if sni == "" {
		sni = host
	}

	return model.Node{
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

func ParseVmess(raw string) (model.Node, error) {
	payload := strings.TrimPrefix(raw, "vmess://")
	hasPadding := strings.Contains(payload, "=")
	data, err := util.DecodeBase64(payload)
	if err != nil {
		return model.Node{}, err
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(data, &fields); err != nil {
		return model.Node{}, err
	}
	host := util.GetString(fields, "add")
	portStr := util.GetString(fields, "port")
	name := util.GetString(fields, "ps")
	if host == "" || portStr == "" {
		return model.Node{}, errors.New("vmess missing host or port")
	}
	port, err := util.ParsePort(portStr)
	if err != nil {
		return model.Node{}, err
	}
	security := strings.ToLower(util.GetString(fields, "tls"))
	if security == "" {
		security = strings.ToLower(util.GetString(fields, "security"))
	}
	if security != "tls" {
		security = ""
	}
	sni := util.GetString(fields, "sni")
	if sni == "" {
		sni = host
	}
	if strings.TrimSpace(util.GetString(fields, "scy")) == "" && strings.TrimSpace(util.GetString(fields, "cipher")) == "" {
		fields["scy"] = "auto"
	}
	return model.Node{
		Raw:          raw,
		Scheme:       "vmess",
		Name:         name,
		OriginalName: name,
		Host:         host,
		Port:         port,
		Security:     security,
		SNI:          sni,
		Vmess: &model.VmessConfig{
			Fields:       fields,
			HasPadding:   hasPadding,
			OriginalName: name,
		},
	}, nil
}

func ParseSS(raw string) (model.Node, error) {
	rest := strings.TrimPrefix(raw, "ss://")
	name := ""
	if idx := strings.Index(rest, "#"); idx >= 0 {
		name = DecodeFragment(rest[idx+1:])
		rest = rest[:idx]
	}
	plugin := ""
	if idx := strings.Index(rest, "?"); idx >= 0 {
		plugin = rest[idx+1:]
		rest = rest[:idx]
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return model.Node{}, errors.New("ss empty")
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
		decoded, err := util.DecodeBase64(plain)
		if err != nil {
			return model.Node{}, err
		}
		plain = string(decoded)
		m, pw, host, port, err := parsePlain(plain)
		if err != nil {
			return model.Node{}, err
		}
		return model.Node{
			Raw:          raw,
			Scheme:       "ss",
			Name:         name,
			OriginalName: name,
			Host:         host,
			Port:         port,
			Security:     "",
			SNI:          host,
			SS: &model.SSConfig{
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
		if decoded, err := util.DecodeBase64(userPart); err == nil {
			decodedStr := string(decoded)
			if strings.Contains(decodedStr, ":") {
				userPart = decodedStr
			}
		}
	}

	plain2 := userPart + "@" + hostPart
	m, pw, host, port, err := parsePlain(plain2)
	if err != nil {
		return model.Node{}, err
	}

	return model.Node{
		Raw:          raw,
		Scheme:       "ss",
		Name:         name,
		OriginalName: name,
		Host:         host,
		Port:         port,
		Security:     "",
		SNI:          host,
		SS: &model.SSConfig{
			Method:   m,
			Password: pw,
			Plugin:   plugin,
		},
	}, nil
}

func ParseNodeFromClashProxy(m map[string]interface{}) (model.Node, error) {
	typ := strings.ToLower(strings.TrimSpace(util.GetStringFromMap(m, "type")))
	if typ == "" {
		return model.Node{}, errors.New("代理缺少 type")
	}
	name := util.GetStringFromMap(m, "name")
	server := util.GetStringFromMap(m, "server")
	port, err := util.GetIntFromMap(m, "port")
	if err != nil {
		return model.Node{}, err
	}
	node := model.Node{
		Scheme:       typ,
		Name:         name,
		OriginalName: name,
		Host:         server,
		Port:         port,
		Clash:        m,
	}
	if tlsVal, ok := util.GetBoolFromMap(m, "tls"); ok && tlsVal {
		node.Security = "tls"
	}
	if sni := util.FirstNonEmpty(util.GetStringFromMap(m, "servername"), util.GetStringFromMap(m, "sni"), util.GetStringFromMap(m, "peer")); sni != "" {
		node.SNI = sni
	} else if node.Host != "" {
		node.SNI = node.Host
	}

	switch typ {
	case "ss", "shadowsocks":
		node.Scheme = "ss"
		method := util.FirstNonEmpty(util.GetStringFromMap(m, "cipher"), util.GetStringFromMap(m, "method"))
		password := util.GetStringFromMap(m, "password")
		if strings.TrimSpace(method) == "" {
			return model.Node{}, errors.New("ss 缺少 cipher/method")
		}
		if strings.TrimSpace(password) == "" {
			return model.Node{}, errors.New("ss 缺少 password")
		}
		m["cipher"] = method
		node.SS = &model.SSConfig{
			Method:   method,
			Password: password,
			Plugin:   util.GetStringFromMap(m, "plugin"),
		}

	case "vmess":
		if strings.TrimSpace(util.GetStringFromMap(m, "cipher")) == "" {
			m["cipher"] = "auto"
		}

	case "vless":
		// keep

	case "trojan":
		password := util.GetStringFromMap(m, "password")
		if strings.TrimSpace(password) == "" {
			return model.Node{}, errors.New("trojan 缺少 password")
		}
		node.Scheme = "trojan"
		node.Security = "tls"
		if sni := util.FirstNonEmpty(util.GetStringFromMap(m, "sni"), util.GetStringFromMap(m, "servername"), util.GetStringFromMap(m, "peer")); sni != "" {
			node.SNI = sni
		} else if node.Host != "" {
			node.SNI = node.Host
		}

	case "hysteria2", "hysteria":
		node.Scheme = "hysteria2"
		password := util.GetStringFromMap(m, "password")
		if password == "" {
			password = util.GetStringFromMap(m, "auth")
		}
		if password != "" {
			m["password"] = password
		}
		if obfs := util.GetStringFromMap(m, "obfs"); obfs != "" {
			m["obfs"] = obfs
		}
		if obfsPassword := util.GetStringFromMap(m, "obfs-password"); obfsPassword != "" {
			m["obfs-password"] = obfsPassword
		}
		if tlsVal, ok := util.GetBoolFromMap(m, "tls"); ok && tlsVal {
			node.Security = "tls"
		}

	case "tuic":
		node.Scheme = "tuic"
		uuid := util.GetStringFromMap(m, "uuid")
		if uuid == "" {
			uuid = util.GetStringFromMap(m, "token")
		}
		if uuid != "" {
			m["uuid"] = uuid
		}
		if token := util.GetStringFromMap(m, "password"); token != "" {
			m["password"] = token
		}
		if congestionController := util.GetStringFromMap(m, "congestion-controller"); congestionController != "" {
			m["congestion-controller"] = congestionController
		}
		if alpn, ok := util.ToStringSlice(util.GetStringFromMap(m, "alpn")); ok {
			m["alpn"] = alpn
		}
		if tlsVal, ok := util.GetBoolFromMap(m, "tls"); ok && tlsVal {
			node.Security = "tls"
		}

	case "socks5", "socks":
		node.Scheme = "socks5"
		username := util.GetStringFromMap(m, "username")
		password := util.GetStringFromMap(m, "password")
		if username != "" {
			m["username"] = username
		}
		if password != "" {
			m["password"] = password
		}
		if tlsVal, ok := util.GetBoolFromMap(m, "tls"); ok && tlsVal {
			node.Security = "tls"
		}

	case "http", "https":
		node.Scheme = "http"
		username := util.GetStringFromMap(m, "username")
		password := util.GetStringFromMap(m, "password")
		if username != "" {
			m["username"] = username
		}
		if password != "" {
			m["password"] = password
		}
		if typ == "https" {
			node.Security = "tls"
		} else if tlsVal, ok := util.GetBoolFromMap(m, "tls"); ok && tlsVal {
			node.Security = "tls"
		}

	default:
		return model.Node{}, fmt.Errorf("不支持的代理类型：%s", typ)
	}

	if node.Host == "" || node.Port == 0 {
		return model.Node{}, errors.New("代理缺少 server 或 port")
	}
	return node, nil
}

func SanitizeNode(node model.Node) (model.Node, bool) {
	node.Name = util.SanitizeString(node.Name)
	node.OriginalName = util.SanitizeString(node.OriginalName)
	node.Host = util.SanitizeString(node.Host)
	node.SNI = util.SanitizeString(node.SNI)
	node.Region = util.SanitizeString(node.Region)

	if node.Params != nil {
		out := make(map[string]string, len(node.Params))
		for k, v := range node.Params {
			kk := strings.ToLower(util.SanitizeString(k))
			out[kk] = util.SanitizeString(v)
		}
		node.Params = out
	}

	if node.SS != nil {
		node.SS.Method = util.SanitizeString(node.SS.Method)
		node.SS.Password = util.SanitizeString(node.SS.Password)
		node.SS.Plugin = util.SanitizeString(node.SS.Plugin)
	}
	if node.Vmess != nil {
		node.Vmess.OriginalName = util.SanitizeString(node.Vmess.OriginalName)
		if cleaned, ok := util.SanitizeYAMLValue(node.Vmess.Fields).(map[string]interface{}); ok {
			node.Vmess.Fields = cleaned
		}
	}
	if node.Clash != nil {
		node.Clash = util.SanitizeYAMLMap(node.Clash)
	}
	if node.URL != nil {
		u := *node.URL
		u.Path = util.SanitizeString(u.Path)
		u.RawQuery = util.SanitizeString(u.RawQuery)
		u.Fragment = util.SanitizeString(u.Fragment)
		node.URL = &u
	}
	if strings.TrimSpace(node.Host) == "" || node.Port <= 0 {
		return node, false
	}
	return node, true
}
