package clash

import (
	"errors"
	"fmt"
	"strings"

	"node-latency/internal/parser"
	"node-latency/internal/util"
)

// NormalizeVlessEncryption normalizes vless encryption field
func NormalizeVlessEncryption(raw string) string {
	token := util.NormalizeTokenValue(raw)
	switch token {
	case "", "none":
		return "none"
	case "auto":
		return "auto"
	default:
		return "none"
	}
}

// NormalizeClashProxy normalizes all proxy types
func NormalizeClashProxy(proxy map[string]interface{}) map[string]interface{} {
	if proxy == nil {
		return proxy
	}
	proxy = util.SanitizeYAMLMap(proxy)
	typ := strings.ToLower(strings.TrimSpace(util.GetStringFromMap(proxy, "type")))

	switch typ {
	case "vless":
		proxy["encryption"] = NormalizeVlessEncryption(util.GetStringFromMap(proxy, "encryption"))
	case "vmess":
		if strings.TrimSpace(util.GetStringFromMap(proxy, "cipher")) == "" {
			proxy["cipher"] = "auto"
		}
	case "ss", "shadowsocks":
		if strings.TrimSpace(util.GetStringFromMap(proxy, "cipher")) == "" {
			if v := strings.TrimSpace(util.GetStringFromMap(proxy, "method")); v != "" {
				proxy["cipher"] = v
			}
		}
	case "trojan":
		// ✅ trojan 强制 tls
		proxy["tls"] = true
		if strings.TrimSpace(util.GetStringFromMap(proxy, "sni")) == "" {
			if v := strings.TrimSpace(util.GetStringFromMap(proxy, "servername")); v != "" {
				proxy["sni"] = v
			}
		}
	case "https":
		// ✅ https 类型转换为 http + tls
		proxy["type"] = "http"
		proxy["tls"] = true
	case "socks":
		// ✅ socks 转换为 socks5
		proxy["type"] = "socks5"
	}

	// ✅ reality-opts 归一化（支持 publicKey/sid 等别名）
	if roRaw, ok := proxy["reality-opts"]; ok && roRaw != nil {
		ro, ok2 := util.ToStringMap(roRaw)
		if ok2 && len(ro) > 0 {
			pub := util.FirstNonEmpty(util.GetStringFromMap(ro, "public-key"), util.GetStringFromMap(ro, "publicKey"), util.GetStringFromMap(ro, "publickey"), util.GetStringFromMap(ro, "pbk"))
			if pub != "" {
				if norm, err := parser.NormalizeRealityPublicKey(pub); err == nil {
					ro["public-key"] = norm
				}
			}
			sid := util.FirstNonEmpty(util.GetStringFromMap(ro, "short-id"), util.GetStringFromMap(ro, "shortId"), util.GetStringFromMap(ro, "shortid"), util.GetStringFromMap(ro, "sid"))
			if sid != "" {
				if norm, err := parser.NormalizeRealityShortID(sid); err == nil && norm != "" {
					ro["short-id"] = norm
				}
			}
			proxy["reality-opts"] = ro
			proxy["tls"] = true
		}
	}

	return proxy
}

// ValidateClashProxy validates proxy config before sending to mihomo
func ValidateClashProxy(p map[string]interface{}) error {
	typ := strings.ToLower(strings.TrimSpace(util.GetStringFromMap(p, "type")))

	// ✅ 基础字段校验
	switch typ {
	case "ss", "shadowsocks":
		if strings.TrimSpace(util.GetStringFromMap(p, "cipher")) == "" {
			return errors.New("key 'cipher' missing")
		}
		if strings.TrimSpace(util.GetStringFromMap(p, "password")) == "" {
			return errors.New("key 'password' missing")
		}
		if strings.TrimSpace(util.GetStringFromMap(p, "server")) == "" {
			return errors.New("key 'server' missing")
		}
		if _, err := util.GetIntFromMap(p, "port"); err != nil {
			return errors.New("key 'port' missing")
		}
	case "vmess":
		if strings.TrimSpace(util.GetStringFromMap(p, "uuid")) == "" {
			return errors.New("key 'uuid' missing")
		}
		if strings.TrimSpace(util.GetStringFromMap(p, "server")) == "" {
			return errors.New("key 'server' missing")
		}
		if _, err := util.GetIntFromMap(p, "port"); err != nil {
			return errors.New("key 'port' missing")
		}
	case "vless":
		if strings.TrimSpace(util.GetStringFromMap(p, "uuid")) == "" {
			return errors.New("key 'uuid' missing")
		}
		if strings.TrimSpace(util.GetStringFromMap(p, "server")) == "" {
			return errors.New("key 'server' missing")
		}
		if _, err := util.GetIntFromMap(p, "port"); err != nil {
			return errors.New("key 'port' missing")
		}
	case "trojan":
		if strings.TrimSpace(util.GetStringFromMap(p, "password")) == "" {
			return errors.New("key 'password' missing")
		}
		if strings.TrimSpace(util.GetStringFromMap(p, "server")) == "" {
			return errors.New("key 'server' missing")
		}
		if _, err := util.GetIntFromMap(p, "port"); err != nil {
			return errors.New("key 'port' missing")
		}
		p["tls"] = true
	case "hysteria2", "hysteria":
		if strings.TrimSpace(util.GetStringFromMap(p, "server")) == "" {
			return errors.New("key 'server' missing")
		}
		if _, err := util.GetIntFromMap(p, "port"); err != nil {
			return errors.New("key 'port' missing")
		}
	case "tuic":
		if strings.TrimSpace(util.GetStringFromMap(p, "uuid")) == "" {
			return errors.New("key 'uuid' missing")
		}
		if strings.TrimSpace(util.GetStringFromMap(p, "server")) == "" {
			return errors.New("key 'server' missing")
		}
		if _, err := util.GetIntFromMap(p, "port"); err != nil {
			return errors.New("key 'port' missing")
		}
	case "socks5", "socks":
		if strings.TrimSpace(util.GetStringFromMap(p, "server")) == "" {
			return errors.New("key 'server' missing")
		}
		if _, err := util.GetIntFromMap(p, "port"); err != nil {
			return errors.New("key 'port' missing")
		}
	case "http", "https":
		if strings.TrimSpace(util.GetStringFromMap(p, "server")) == "" {
			return errors.New("key 'server' missing")
		}
		if _, err := util.GetIntFromMap(p, "port"); err != nil {
			return errors.New("key 'port' missing")
		}
	}

	// ✅ UUID 预校验，避免 mihomo 启动直接炸
	if typ == "vmess" || typ == "vless" {
		uuidRaw := util.GetStringFromMap(p, "uuid")
		uuid, err := parser.NormalizeUUID(uuidRaw)
		if err != nil {
			return fmt.Errorf("invalid uuid: %v", err)
		}
		p["uuid"] = uuid
	}

	// ✅ Reality 预校验：只要 reality-opts 非空，必须有合法 public-key
	if roRaw, ok := p["reality-opts"]; ok && roRaw != nil {
		ro, ok2 := util.ToStringMap(roRaw)
		if !ok2 {
			return errors.New("reality-opts invalid")
		}
		if len(ro) > 0 {
			pub := util.FirstNonEmpty(util.GetStringFromMap(ro, "public-key"), util.GetStringFromMap(ro, "publicKey"), util.GetStringFromMap(ro, "publickey"), util.GetStringFromMap(ro, "pbk"))
			if strings.TrimSpace(pub) == "" {
				return errors.New("reality-opts missing public-key")
			}
			pubNorm, err := parser.NormalizeRealityPublicKey(pub)
			if err != nil {
				return fmt.Errorf("invalid REALITY public key: %v", err)
			}
			ro["public-key"] = pubNorm

			sid := util.FirstNonEmpty(util.GetStringFromMap(ro, "short-id"), util.GetStringFromMap(ro, "shortId"), util.GetStringFromMap(ro, "shortid"), util.GetStringFromMap(ro, "sid"))
			if sid != "" {
				sidNorm, err := parser.NormalizeRealityShortID(sid)
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
