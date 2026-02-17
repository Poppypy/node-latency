package model

import (
	"net/url"
	"time"
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

	// ✅ 关键修复：保存"不会把 + 变空格"的 Query 解析结果（避免 pbk 被破坏）
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
	UseBatchMode     bool // 是否使用批次模式（默认 false）
}

const DefaultTemplateFile = "demo.yaml"
const DefaultIPLookupURL = "http://ip-api.com/json/{ip}?fields=status,message,country,regionName,city,isp,org,as,hosting,proxy,mobile,query"
const DefaultIPNameFmt = "{region}-{random}"
const DefaultLatencyFmt = "{avg}ms"
const DefaultCoreTestURL = "https://www.gstatic.com/generate_204"

// ✅ 原代码写成了 6,000,000ms（100分钟），改成更合理的 90s
const DefaultCoreStartTimeoutMs = 90000

// DTO types for frontend communication
type NodeDTO struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Scheme   string `json:"scheme"`
	Region   string `json:"region"`
	Security string `json:"security"`
}

type TestResultEvent struct {
	Index      int     `json:"index"`
	Done       bool    `json:"done"`
	Pass       bool    `json:"pass"`
	Err        string  `json:"err"`
	AvgMs      int64   `json:"avgMs"`
	MaxMs      int64   `json:"maxMs"`
	LatencyMs  []int64 `json:"latencyMs"`
	Attempts   int     `json:"attempts"`
	Successful int     `json:"successful"`
}

type TestProgressEvent struct {
	Total   int  `json:"total"`
	Done    int  `json:"done"`
	Passed  int  `json:"passed"`
	Running bool `json:"running"`
}

type ResultUpdate struct {
	Index  int
	Result Result
}

func ToNodeDTO(n Node) NodeDTO {
	name := n.Name
	if name == "" {
		name = n.Host
	}
	return NodeDTO{
		Index:    n.Index,
		Name:     name,
		Host:     n.Host,
		Port:     n.Port,
		Scheme:   n.Scheme,
		Region:   n.Region,
		Security: n.Security,
	}
}

func ToNodeDTOs(nodes []Node) []NodeDTO {
	out := make([]NodeDTO, len(nodes))
	for i, n := range nodes {
		out[i] = ToNodeDTO(n)
	}
	return out
}
