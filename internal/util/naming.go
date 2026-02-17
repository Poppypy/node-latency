package util

import (
	"fmt"
	"math/rand"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"node-latency/internal/model"
)

// 初始化随机种子
func init() {
	rand.Seed(time.Now().UnixNano())
}

// RandomSuffix 生成 4 位随机数字后缀
func RandomSuffix() string {
	return fmt.Sprintf("%04d", rand.Intn(10000))
}

// FullyDecodeURL 循环解码 URL 编码，直到完全解码
func FullyDecodeURL(s string) string {
	if s == "" {
		return ""
	}
	// 循环解码，直到无法再解码
	for {
		decoded, err := url.PathUnescape(s)
		if err != nil || decoded == s {
			break
		}
		s = decoded
	}
	return s
}

// CountryToCode 国家/地区名称到缩写的映射
var CountryToCode = map[string]string{
	// 亚洲
	"Hong Kong":        "HK",
	"Taiwan":           "TW",
	"Singapore":        "SG",
	"Japan":            "JP",
	"Korea":            "KR",
	"South Korea":      "KR",
	"Thailand":         "TH",
	"Vietnam":          "VN",
	"Malaysia":         "MY",
	"Indonesia":        "ID",
	"Philippines":      "PH",
	"India":            "IN",
	"Pakistan":         "PK",
	"Bangladesh":       "BD",
	"Nepal":            "NP",
	"Sri Lanka":        "LK",
	"Myanmar":          "MM",
	"Cambodia":         "KH",
	"Laos":             "LA",
	"Mongolia":         "MN",
	"Kazakhstan":       "KZ",
	"Uzbekistan":       "UZ",
	"Georgia":          "GE",
	"Armenia":          "AM",
	"Azerbaijan":       "AZ",
	"Turkey":           "TR",
	"Israel":           "IL",
	"United Arab Emirates": "AE",
	"UAE":              "AE",
	"Saudi Arabia":     "SA",
	"Qatar":            "QA",
	"Kuwait":           "KW",
	"Bahrain":          "BH",
	"Oman":             "OM",
	"Iran":             "IR",
	"Iraq":             "IQ",
	"Jordan":           "JO",
	"Lebanon":          "LB",

	// 欧洲
	"United Kingdom":   "UK",
	"UK":               "UK",
	"Great Britain":    "UK",
	"England":          "UK",
	"Germany":          "DE",
	"France":           "FR",
	"Netherlands":      "NL",
	"Belgium":          "BE",
	"Luxembourg":       "LU",
	"Switzerland":      "CH",
	"Austria":          "AT",
	"Poland":           "PL",
	"Czech Republic":   "CZ",
	"Czechia":          "CZ",
	"Slovakia":         "SK",
	"Hungary":          "HU",
	"Romania":          "RO",
	"Bulgaria":         "BG",
	"Greece":           "GR",
	"Serbia":           "RS",
	"Croatia":          "HR",
	"Slovenia":         "SI",
	"Bosnia":           "BA",
	"North Macedonia":  "MK",
	"Albania":          "AL",
	"Montenegro":       "ME",
	"Kosovo":           "XK",
	"Moldova":          "MD",
	"Ukraine":          "UA",
	"Belarus":          "BY",
	"Lithuania":        "LT",
	"Latvia":           "LV",
	"Estonia":          "EE",
	"Finland":          "FI",
	"Sweden":           "SE",
	"Norway":           "NO",
	"Denmark":          "DK",
	"Iceland":          "IS",
	"Ireland":          "IE",
	"Portugal":         "PT",
	"Spain":            "ES",
	"Italy":            "IT",
	"Malta":            "MT",
	"Cyprus":           "CY",
	"Russia":           "RU",
	"Russian Federation": "RU",

	// 北美洲
	"United States":    "US",
	"USA":              "US",
	"Canada":           "CA",
	"Mexico":           "MX",
	"Panama":           "PA",
	"Costa Rica":       "CR",
	"Guatemala":        "GT",
	"Belize":           "BZ",
	"El Salvador":      "SV",
	"Honduras":         "HN",
	"Nicaragua":        "NI",
	"Cuba":             "CU",
	"Jamaica":          "JM",
	"Haiti":            "HT",
	"Dominican Republic": "DO",
	"Puerto Rico":      "PR",
	"Trinidad":         "TT",
	"Barbados":         "BB",
	"Bahamas":          "BS",
	"Bermuda":          "BM",
	"Cayman Islands":   "KY",
	"Virgin Islands":   "VG",

	// 南美洲
	"Brazil":           "BR",
	"Argentina":        "AR",
	"Chile":            "CL",
	"Peru":             "PE",
	"Colombia":         "CO",
	"Venezuela":        "VE",
	"Ecuador":          "EC",
	"Bolivia":          "BO",
	"Paraguay":         "PY",
	"Uruguay":          "UY",
	"Guyana":           "GY",
	"Suriname":         "SR",
	"Falkland Islands": "FK",

	// 大洋洲
	"Australia":        "AU",
	"New Zealand":      "NZ",
	"Fiji":             "FJ",
	"Papua New Guinea": "PG",
	"Solomon Islands":  "SB",
	"Vanuatu":          "VU",
	"Samoa":            "WS",
	"Tonga":            "TO",
	"Guam":             "GU",
	"Northern Mariana Islands": "MP",

	// 非洲
	"South Africa":     "ZA",
	"Egypt":            "EG",
	"Nigeria":          "NG",
	"Kenya":            "KE",
	"Ghana":            "GH",
	"Ethiopia":         "ET",
	"Tanzania":         "TZ",
	"Uganda":           "UG",
	"Morocco":          "MA",
	"Algeria":          "DZ",
	"Tunisia":          "TN",
	"Libya":            "LY",
	"Sudan":            "SD",
	"Angola":           "AO",
	"Mozambique":       "MZ",
	"Zimbabwe":         "ZW",
	"Zambia":           "ZM",
	"Botswana":         "BW",
	"Namibia":          "NA",
	"Mauritius":        "MU",
	"Seychelles":       "SC",
	"Réunion":          "RE",
	"Madagascar":       "MG",
	"Mauritania":       "MR",
	"Senegal":          "SN",
	"Ivory Coast":      "CI",
	"Côte d'Ivoire":    "CI",
	"Cameron":          "CM",
	"Cameroon":         "CM",
	"Congo":            "CG",
	"DR Congo":         "CD",
	"Cabo Verde":       "CV",
	"Cape Verde":       "CV",

	// 加拿大省份
	"Ontario":              "CA",
	"Quebec":               "CA",
	"British Columbia":     "CA",
	"Alberta":              "CA",
	"Manitoba":             "CA",
	"Saskatchewan":         "CA",
	"Nova Scotia":          "CA",
	"New Brunswick":        "CA",
	"Newfoundland":         "CA",
	"Prince Edward Island": "CA",
	"Northwest Territories": "CA",
	"Yukon":                "CA",
	"Nunavut":              "CA",

	// 美国州份
	"California":       "US",
	"Texas":            "US",
	"Florida":          "US",
	"New York":         "US",
	"Pennsylvania":     "US",
	"Illinois":         "US",
	"Ohio":             "US",
	"Georgia USA":      "US",
	"North Carolina":   "US",
	"Michigan":         "US",
	"New Jersey":       "US",
	"Virginia":         "US",
	"Washington":       "US",
	"Arizona":          "US",
	"Massachusetts":    "US",
	"Tennessee":        "US",
	"Indiana":          "US",
	"Maryland":         "US",
	"Missouri":         "US",
	"Wisconsin":        "US",
	"Colorado":         "US",
	"Minnesota":        "US",
	"South Carolina":   "US",
	"Alabama":          "US",
	"Louisiana":        "US",
	"Kentucky":         "US",
	"Oregon":           "US",
	"Oklahoma":         "US",
	"Connecticut":      "US",
	"Utah":             "US",
	"Iowa":             "US",
	"Nevada":           "US",
	"Arkansas":         "US",
	"Mississippi":      "US",
	"Kansas":           "US",
	"New Mexico":       "US",
	"Nebraska":         "US",
	"Idaho":            "US",
	"West Virginia":    "US",
	"Hawaii":           "US",
	"New Hampshire":    "US",
	"Maine":            "US",
	"Montana":          "US",
	"Rhode Island":     "US",
	"Delaware":         "US",
	"South Dakota":     "US",
	"North Dakota":     "US",
	"Alaska":           "US",
	"Vermont":          "US",
	"Wyoming":          "US",

	// 德国州份
	"Hesse":            "DE",
	"Hessen":           "DE",
	"Bavaria":          "DE",
	"Bayern":           "DE",
	"Baden-Württemberg": "DE",
	"North Rhine-Westphalia": "DE",
	"Nordrhein-Westfalen": "DE",
	"Lower Saxony":     "DE",
	"Niedersachsen":    "DE",
	"Saxony":           "DE",
	"Sachsen":          "DE",
	"Rhineland-Palatinate": "DE",
	"Rheinland-Pfalz":  "DE",
	"Schleswig-Holstein": "DE",
	"Brandenburg":      "DE",
	"Saxony-Anhalt":    "DE",
	"Sachsen-Anhalt":   "DE",
	"Thuringia":        "DE",
	"Thüringen":        "DE",
	"Mecklenburg-Vorpommern": "DE",
	"Saarland":         "DE",
	"Bremen":           "DE",
	"Hamburg":          "DE",
	"Berlin":           "DE",

	// 英国地区
	"Scotland":         "UK",
	"Wales":            "UK",
	"Northern Ireland": "UK",
	"London":           "UK",
	"Manchester":       "UK",
	"Birmingham":       "UK",
	"Liverpool":        "UK",
	"Leeds":            "UK",
	"Glasgow":          "UK",
	"Edinburgh":        "UK",

	// 伯利兹区份
	"Belize District":  "BZ",

	// 其他常见地区
	"Vienna":           "AT",    // 奥地利维也纳
	"North West":       "ZA",    // 南非西北省
	"Curaçao":          "CW",    // 库拉索
	"Curacao":          "CW",    // 库拉索（无变音符号）
	"North Holland":    "NL",    // 荷兰北荷兰省
	"South Holland":    "NL",    // 荷兰南荷兰省
	"Zuid-Holland":     "NL",    // 荷兰南荷兰省（荷兰语）
	"Noord-Holland":    "NL",    // 荷兰北荷兰省（荷兰语）
	"Amsterdam":        "NL",    // 阿姆斯特丹
	"Rotterdam":        "NL",    // 鹿特丹
	"The Hague":        "NL",    // 海牙
	"Zurich":           "CH",    // 苏黎世
	"Zürich":           "CH",    // 苏黎世（德语）
	"Geneva":           "CH",    // 日内瓦
	"Paris":            "FR",    // 巴黎
	"Marseille":        "FR",    // 马赛
	"Lyon":             "FR",    // 里昂
	"Madrid":           "ES",    // 马德里
	"Barcelona":        "ES",    // 巴塞罗那
	"Rome":             "IT",    // 罗马
	"Milan":            "IT",    // 米兰
	"Milano":           "IT",    // 米兰（意大利语）
	"Tokyo":            "JP",    // 东京
	"Osaka":            "JP",    // 大阪
	"Seoul":            "KR",    // 首尔
	"Shanghai":         "CN",    // 上海
	"Beijing":          "CN",    // 北京
	"Shenzhen":         "CN",    // 深圳
	"Guangzhou":        "CN",    // 广州
	"Hongkong":         "HK",    // 香港（另一种拼写）
	"Singapore City":   "SG",    // 新加坡市
	"Sydney":           "AU",    // 悉尼
	"Melbourne":        "AU",    // 墨尔本
	"Toronto":          "CA",    // 多伦多
	"Vancouver":        "CA",    // 温哥华
	"Montreal":         "CA",    // 蒙特利尔
	"New York City":    "US",    // 纽约市
	"Los Angeles":      "US",    // 洛杉矶
	"San Francisco":    "US",    // 旧金山
	"Chicago":          "US",    // 芝加哥
	"Dallas":           "US",    // 达拉斯
	"Houston":          "US",    // 休斯顿
	"Miami":            "US",    // 迈阿密
	"Seattle":          "US",    // 西雅图
	"Phoenix":          "US",    // 凤凰城
	"Denver":           "US",    // 丹佛
	"Silicon Valley":   "US",    // 硅谷
	"Ashburn":          "US",    // 阿什本（弗吉尼亚州，数据中心集中地）
	"Santa Clara":      "US",    // 圣克拉拉
	"San Jose":         "US",    // 圣何塞
	"Fremont":          "US",    // 弗里蒙特
	"Buffalo":          "US",    // 水牛城

	// 中国地区
	"China":            "CN",
	"Macao":            "MO",
	"Macau":            "MO",
}

// GetCountryCode 将国家/地区名称转换为缩写
func GetCountryCode(name string) string {
	if name == "" {
		return ""
	}
	// 先尝试完全匹配
	name = strings.TrimSpace(name)
	if code, ok := CountryToCode[name]; ok {
		return code
	}
	// 尝试小写匹配
	lower := strings.ToLower(name)
	for k, v := range CountryToCode {
		if strings.ToLower(k) == lower {
			return v
		}
	}
	// 尝试包含匹配
	for k, v := range CountryToCode {
		if strings.Contains(lower, strings.ToLower(k)) || strings.Contains(strings.ToLower(k), lower) {
			return v
		}
	}
	// 如果找不到，取前两个字符大写
	if len(name) >= 2 {
		return strings.ToUpper(name[:2])
	}
	return strings.ToUpper(name)
}

func DisplayName(node model.Node) string {
	if node.Name != "" {
		return node.Name
	}
	if node.Host != "" {
		return node.Host
	}
	return node.Raw
}

func ParseRegionRules(text string) []model.RegionRule {
	var rules []model.RegionRule
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
		rules = append(rules, model.RegionRule{Pattern: pattern, Region: region})
	}
	return rules
}

func ParseKeywords(text string) []string {
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

func FilterNodesByKeywords(nodes []model.Node, keywords []string) []model.Node {
	if len(keywords) == 0 {
		return nodes
	}
	var out []model.Node
	for _, n := range nodes {
		if ShouldExclude(n, keywords) {
			continue
		}
		out = append(out, n)
	}
	return out
}

func ShouldExclude(node model.Node, keywords []string) bool {
	target := strings.ToLower(strings.Join([]string{node.Name, node.OriginalName, node.Host, node.Raw}, " "))
	for _, kw := range keywords {
		if kw != "" && strings.Contains(target, kw) {
			return true
		}
	}
	return false
}

func MatchRegion(name, host string, rules []model.RegionRule) string {
	target := strings.ToLower(name + " " + host)
	for _, rule := range rules {
		if strings.Contains(target, strings.ToLower(rule.Pattern)) {
			return rule.Region
		}
	}
	return ""
}

func FormatName(fmtStr, region, name, host, scheme string, index int) string {
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

func ComputeNameAndRegion(node model.Node, settings model.TestSettings) (string, string) {
	baseName := BaseNameFromNode(node, settings)
	region := MatchRegion(baseName, node.Host, settings.RegionRules)
	if region == "" && node.IPInfo != nil {
		region = FormatIPRegion(node.IPInfo)
	}
	// 如果没有启用 IP 重命名，尝试将原始名称中的地区名转换为缩写
	if !settings.IPRename || node.IPInfo == nil {
		baseName = AbbreviateRegionInName(baseName)
	}
	if settings.Rename {
		return FormatName(settings.RenameFmt, region, baseName, node.Host, node.Scheme, node.Index), region
	}
	return baseName, region
}

func BuildOutputName(node model.Node, settings model.TestSettings, res model.Result) (string, string) {
	name, region := ComputeNameAndRegion(node, settings)
	if settings.LatencyName && res.Done && len(res.LatencyMs) > 0 {
		name = ApplyLatencyName(name, settings.LatencyFmt, res)
	}
	return name, region
}

func BaseNameFromNode(node model.Node, settings model.TestSettings) string {
	if settings.IPRename && node.IPInfo != nil {
		ipName := FormatIPName(settings.IPNameFmt, node.IPInfo)
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

// AbbreviateRegionInName 将名称中的地区全名替换为缩写
// 例如 "Ontario Ontario-7834" -> "CA-7834"
func AbbreviateRegionInName(name string) string {
	if name == "" {
		return ""
	}
	// 完全解码 URL 编码
	name = FullyDecodeURL(name)

	original := name

	// 尝试匹配已知的地区名称并替换
	// 按长度降序排序，避免短名称先匹配
	for _, region := range regionsByLength {
		code := CountryToCode[region]
		if code == "" {
			continue
		}
		// 不区分大小写匹配
		lowerName := strings.ToLower(name)
		lowerRegion := strings.ToLower(region)
		if strings.Contains(lowerName, lowerRegion) {
			// 替换所有出现
			name = strings.ReplaceAll(name, region, code)
			name = strings.ReplaceAll(name, strings.Title(region), code)
			// 处理已经是小写或大写的情况
			name = strings.ReplaceAll(name, strings.ToLower(region), code)
			name = strings.ReplaceAll(name, strings.ToUpper(region), code)
		}
	}

	// 如果替换后没有变化，直接返回
	if name == original {
		return strings.TrimSpace(name)
	}

	// 清理重复的 "CODE CODE" 模式（如 "CA CA-7834" -> "CA-7834"）
	// 用空格分割
	parts := strings.Fields(name)
	if len(parts) >= 2 {
		var cleaned []string
		for i, p := range parts {
			if i == 0 {
				cleaned = append(cleaned, p)
				continue
			}
			last := cleaned[len(cleaned)-1]
			// 只有当两个部分完全相同时才跳过（如 "CA CA" -> "CA"）
			// 或者当第一个是地区代码，第二个以相同代码开头时（如 "CA CA-7834" -> "CA-7834"）
			if p == last && isRegionCode(p) {
				// 完全相同的地区代码，跳过
				continue
			}
			if isRegionCode(last) && strings.HasPrefix(p, last+"-") {
				// 第一个是地区代码，第二个以 "CODE-" 开头，合并
				cleaned[len(cleaned)-1] = p
				continue
			}
			if isRegionCode(last) && strings.HasPrefix(p, last+"_") {
				// 第一个是地区代码，第二个以 "CODE_" 开头，合并
				cleaned[len(cleaned)-1] = p
				continue
			}
			cleaned = append(cleaned, p)
		}
		name = strings.Join(cleaned, " ")
	}

	// 清理多余的空格和连接符
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " - ", "-")
	name = strings.ReplaceAll(name, " -", "-")
	name = strings.ReplaceAll(name, "- ", "-")

	return name
}

// isRegionCode 检查是否是可能的地区代码（2-3 个大写字母）
func isRegionCode(s string) bool {
	if len(s) < 2 || len(s) > 3 {
		return false
	}
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

// 按长度降序排序的地区名称列表
var regionsByLength = func() []string {
	var regions []string
	for k := range CountryToCode {
		regions = append(regions, k)
	}
	// 按长度降序排序
	sort.Slice(regions, func(i, j int) bool {
		return len(regions[i]) > len(regions[j])
	})
	return regions
}()

func FormatIPRegion(info *model.IPInfo) string {
	if info == nil {
		return ""
	}
	// 优先使用 region，转为缩写
	if info.Region != "" {
		code := GetCountryCode(info.Region)
		if code != "" {
			return code
		}
		return info.Region
	}
	// 否则使用 country，转为缩写
	if info.Country != "" {
		code := GetCountryCode(info.Country)
		if code != "" {
			return code
		}
		return info.Country
	}
	return ""
}

func IsResidential(info *model.IPInfo) bool {
	if info == nil {
		return false
	}
	return !info.Hosting && !info.Proxy
}

func FormatIPName(fmtStr string, info *model.IPInfo) string {
	if info == nil {
		return ""
	}
	if fmtStr == "" {
		// 默认格式：地区代码+随机数字
		fmtStr = "{region_code}-{random}"
	}
	residential := "机房"
	if IsResidential(info) {
		residential = "家宽"
	}
	// 转换为缩写
	countryCode := GetCountryCode(info.Country)
	regionCode := GetCountryCode(info.Region)
	if regionCode == "" {
		regionCode = countryCode
	}
	out := fmtStr
	out = strings.ReplaceAll(out, "{country}", info.Country)
	out = strings.ReplaceAll(out, "{country_code}", countryCode)
	out = strings.ReplaceAll(out, "{region}", info.Region)
	out = strings.ReplaceAll(out, "{region_code}", regionCode)
	out = strings.ReplaceAll(out, "{city}", info.City)
	out = strings.ReplaceAll(out, "{isp}", info.ISP)
	out = strings.ReplaceAll(out, "{org}", info.Org)
	out = strings.ReplaceAll(out, "{asn}", info.ASN)
	out = strings.ReplaceAll(out, "{ip}", info.IP)
	out = strings.ReplaceAll(out, "{residential}", residential)
	out = strings.ReplaceAll(out, "{random}", RandomSuffix())
	out = strings.TrimSpace(out)
	return out
}

func ApplyLatencyName(name string, fmtStr string, res model.Result) string {
	if fmtStr == "" {
		fmtStr = model.DefaultLatencyFmt
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

func UniqueName(name string, counts map[string]int) string {
	// 先完全解码 URL 编码
	n := FullyDecodeURL(strings.TrimSpace(name))
	if n == "" {
		n = "节点"
	}
	// 再进行地区缩写转换
	n = AbbreviateRegionInName(n)
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

func DedupNodes(nodes []model.Node) []model.Node {
	seen := make(map[string]struct{})
	var out []model.Node
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

func ReindexNodes(nodes []model.Node) []model.Node {
	for i := range nodes {
		nodes[i].Index = i + 1
	}
	return nodes
}

func ApplyRegionsAndNames(nodes []model.Node, settings model.TestSettings) {
	for i := range nodes {
		node := &nodes[i]
		name, region := ComputeNameAndRegion(*node, settings)
		node.Region = region
		node.Name = name
	}
}
