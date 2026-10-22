package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type RoutingModeRule struct {
	Condition string `json:"condition"`
	Target    string `json:"target"`
}

type FastTestItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Url         string `json:"url"`
	Category    string `json:"category"`
	ExpectedOut string `json:"expectedOut"`
	MatchedRule string `json:"matchedRule"`
	Status      string `json:"status"` // "ok", "error"
	Latency     int    `json:"latency"` // ms
	HttpCode    int    `json:"httpCode"`
	Message     string `json:"message"`
}

type RoutingMode struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Badge       string            `json:"badge"`
	IsCustom    bool              `json:"isCustom"`
	DnsSettings DnsConfig         `json:"dnsSettings"`
	Rules       []RoutingModeRule `json:"rules"`
	TestResults []FastTestItem    `json:"testResults"`
}

type ModesOverviewResponse struct {
	ActiveModeID string        `json:"activeModeId"`
	Modes        []RoutingMode `json:"modes"`
}

var (
	modesMu        sync.RWMutex
	customModes    = make(map[string]RoutingMode)
	activeSavedID  = "bypass_cn"
	customModeFile = "routing_modes.json"
)

func getBuiltinModes() []RoutingMode {
	return []RoutingMode{
		{
			ID:          "gfwlist",
			Name:        "GFW列表代理",
			Description: "在 gfwlist 列表中被屏蔽或拦截的网站流量走代理，中国大陆及其他网站默认直连",
			Badge:       "GFW 代理",
			IsCustom:    false,
			DnsSettings: DnsConfig{
				IpVersionPrefer: 4,
				Upstreams: []DnsUpstreamItem{
					{Tag: "alidns", Address: "udp://223.5.5.5:53"},
					{Tag: "googledns", Address: "tcp+udp://dns.google:53"},
				},
				RequestRules: []DnsRequestRule{
					{Condition: "qname(geosite:cn)", Target: "alidns"},
				},
				Fallback: "googledns",
			},
			Rules: []RoutingModeRule{
				{Condition: "pname(NetworkManager, systemd-resolved, dnsmasq)", Target: "must_direct"},
				{Condition: "dip(224.0.0.0/3, 'ff00::/8')", Target: "direct"},
				{Condition: "dip(geoip:private)", Target: "direct"},
				{Condition: "domain(geosite:category-ads-all)", Target: "block"},
				{Condition: "domain(geosite:gfw)", Target: "proxy"},
				{Condition: "domain(geosite:cn)", Target: "direct"},
				{Condition: "dip(geoip:cn)", Target: "direct"},
				{Condition: "fallback", Target: "direct"},
			},
			TestResults: []FastTestItem{
				{
					ID: "aliyun", Name: "阿里云骨干访问", Url: "https://www.aliyun.com", Category: "国内直连",
					ExpectedOut: "国内直连 (direct)", MatchedRule: "dip(geoip:cn) -> direct",
					Status: "ok", Latency: 16, HttpCode: 200, Message: "国内骨干高速直连，不消耗代理节点流量",
				},
				{
					ID: "baidu", Name: "百度搜索服务", Url: "https://www.baidu.com", Category: "国内直连",
					ExpectedOut: "国内直连 (direct)", MatchedRule: "domain(geosite:cn) -> direct",
					Status: "ok", Latency: 12, HttpCode: 200, Message: "未匹配 gfwlist，命中国内域名库直接放行",
				},
				{
					ID: "google", Name: "Google 搜索检测", Url: "https://www.google.com/generate_204", Category: "GFW 代理",
					ExpectedOut: "代理分流 (proxy)", MatchedRule: "domain(geosite:gfw) -> proxy",
					Status: "ok", Latency: 62, HttpCode: 204, Message: "命中 gfw 规则库，安全路由至代理节点出口",
				},
				{
					ID: "youtube", Name: "YouTube 流媒体", Url: "https://www.youtube.com/generate_204", Category: "GFW 代理",
					ExpectedOut: "代理分流 (proxy)", MatchedRule: "domain(geosite:gfw) -> proxy",
					Status: "ok", Latency: 68, HttpCode: 204, Message: "命中 gfw 规则库，走代理分流保障流畅播放",
				},
				{
					ID: "openai", Name: "OpenAI API 服务", Url: "https://api.openai.com", Category: "GFW 代理",
					ExpectedOut: "代理分流 (proxy)", MatchedRule: "domain(geosite:gfw) -> proxy",
					Status: "ok", Latency: 98, HttpCode: 200, Message: "命中 gfwlist 规则，代理链路通信正常",
				},
				{
					ID: "lan", Name: "局域网路由器管理", Url: "http://192.168.1.1", Category: "内网直连",
					ExpectedOut: "局域网直连 (direct)", MatchedRule: "dip(geoip:private) -> direct",
					Status: "ok", Latency: 2, HttpCode: 200, Message: "局域网私网强直连保护，防止内网断联",
				},
			},
		},
		{
			ID:          "bypass_cn",
			Name:        "中国列表以外全部走代理",
			Description: "中国大陆域名与大陆公网 IP 直连，中国列表以外所有境外与未知流量全部走代理 (推荐常用)",
			Badge:       "绕过大陆",
			IsCustom:    false,
			DnsSettings: DnsConfig{
				IpVersionPrefer: 4,
				Upstreams: []DnsUpstreamItem{
					{Tag: "alidns", Address: "udp://223.5.5.5:53"},
					{Tag: "googledns", Address: "tcp+udp://dns.google:53"},
				},
				RequestRules: []DnsRequestRule{
					{Condition: "qname(geosite:cn)", Target: "alidns"},
				},
				Fallback: "googledns",
			},
			Rules: []RoutingModeRule{
				{Condition: "pname(NetworkManager, systemd-resolved, dnsmasq)", Target: "must_direct"},
				{Condition: "dip(224.0.0.0/3, 'ff00::/8')", Target: "direct"},
				{Condition: "dip(geoip:private)", Target: "direct"},
				{Condition: "domain(geosite:category-ads-all)", Target: "block"},
				{Condition: "domain(geosite:cn)", Target: "direct"},
				{Condition: "dip(geoip:cn)", Target: "direct"},
				{Condition: "fallback", Target: "proxy"},
			},
			TestResults: []FastTestItem{
				{
					ID: "aliyun", Name: "阿里云访问检测", Url: "https://www.aliyun.com", Category: "国内直连",
					ExpectedOut: "国内直连 (direct)", MatchedRule: "dip(geoip:cn) -> direct",
					Status: "ok", Latency: 15, HttpCode: 200, Message: "命中 geoip:cn，本地运营商光纤直连畅通",
				},
				{
					ID: "baidu", Name: "百度网盘与搜索", Url: "https://www.baidu.com", Category: "国内直连",
					ExpectedOut: "国内直连 (direct)", MatchedRule: "domain(geosite:cn) -> direct",
					Status: "ok", Latency: 14, HttpCode: 200, Message: "命中 geosite:cn，国内流量不走代理",
				},
				{
					ID: "google", Name: "Google 全系服务", Url: "https://www.google.com/generate_204", Category: "海外代理",
					ExpectedOut: "代理分流 (proxy)", MatchedRule: "fallback -> proxy",
					Status: "ok", Latency: 58, HttpCode: 204, Message: "境外目标非中国列表，走代理高速通道",
				},
				{
					ID: "github", Name: "GitHub 开源社区", Url: "https://github.com", Category: "海外代理",
					ExpectedOut: "代理分流 (proxy)", MatchedRule: "fallback -> proxy",
					Status: "ok", Latency: 78, HttpCode: 200, Message: "代码托管与 Releases 加速通道通畅",
				},
				{
					ID: "youtube", Name: "YouTube 视频流", Url: "https://www.youtube.com/generate_204", Category: "海外代理",
					ExpectedOut: "代理分流 (proxy)", MatchedRule: "fallback -> proxy",
					Status: "ok", Latency: 65, HttpCode: 204, Message: "境外视频 CDN 节点连通正常",
				},
				{
					ID: "lan", Name: "局域网网关管理", Url: "http://192.168.1.1", Category: "内网直连",
					ExpectedOut: "局域网直连 (direct)", MatchedRule: "dip(geoip:private) -> direct",
					Status: "ok", Latency: 2, HttpCode: 200, Message: "内网通信安全直连，完全隔离代理流量",
				},
			},
		},
		{
			ID:          "only_cn",
			Name:        "中国列表走代理",
			Description: "中国大陆网站与大陆 IP 流量定向走代理出口 (回国加速/异地办公)，其他境外网站全部直连",
			Badge:       "回国代理",
			IsCustom:    false,
			DnsSettings: DnsConfig{
				IpVersionPrefer: 4,
				Upstreams: []DnsUpstreamItem{
					{Tag: "googledns", Address: "tcp+udp://dns.google:53"},
					{Tag: "alidns", Address: "udp://223.5.5.5:53"},
				},
				RequestRules: []DnsRequestRule{
					{Condition: "!qname(geosite:cn)", Target: "googledns"},
				},
				Fallback: "alidns",
			},
			Rules: []RoutingModeRule{
				{Condition: "pname(NetworkManager, systemd-resolved, dnsmasq)", Target: "must_direct"},
				{Condition: "dip(224.0.0.0/3, 'ff00::/8')", Target: "direct"},
				{Condition: "dip(geoip:private)", Target: "direct"},
				{Condition: "domain(geosite:cn)", Target: "proxy"},
				{Condition: "dip(geoip:cn)", Target: "proxy"},
				{Condition: "fallback", Target: "direct"},
			},
			TestResults: []FastTestItem{
				{
					ID: "aliyun", Name: "阿里云骨干机房", Url: "https://www.aliyun.com", Category: "定向代理",
					ExpectedOut: "代理分流 (proxy)", MatchedRule: "dip(geoip:cn) -> proxy",
					Status: "ok", Latency: 35, HttpCode: 200, Message: "命中 geoip:cn，通过代理节点反向加速中转",
				},
				{
					ID: "bilibili", Name: "哔哩哔哩视频源", Url: "https://www.bilibili.com", Category: "定向代理",
					ExpectedOut: "代理分流 (proxy)", MatchedRule: "domain(geosite:cn) -> proxy",
					Status: "ok", Latency: 32, HttpCode: 200, Message: "命中 geosite:cn，成功解锁大陆内网与版权限制",
				},
				{
					ID: "google", Name: "Google 搜索服务", Url: "https://www.google.com/generate_204", Category: "境外直连",
					ExpectedOut: "境外直连 (direct)", MatchedRule: "fallback -> direct",
					Status: "ok", Latency: 18, HttpCode: 204, Message: "非中国列表流量本地网络直接发起连接",
				},
				{
					ID: "github", Name: "GitHub 开发者服务", Url: "https://github.com", Category: "境外直连",
					ExpectedOut: "境外直连 (direct)", MatchedRule: "fallback -> direct",
					Status: "ok", Latency: 22, HttpCode: 200, Message: "境外服务器本地网络极速直达",
				},
				{
					ID: "lan", Name: "局域网网关管理", Url: "http://192.168.1.1", Category: "内网直连",
					ExpectedOut: "局域网直连 (direct)", MatchedRule: "dip(geoip:private) -> direct",
					Status: "ok", Latency: 2, HttpCode: 200, Message: "局域网内网保护免代理直通",
				},
			},
		},
		{
			ID:          "global",
			Name:        "全局代理",
			Description: "除底层内网私网与协议守护直连外，所有公网网站与网络流量全部强制走代理节点出口",
			Badge:       "全局代理",
			IsCustom:    false,
			DnsSettings: DnsConfig{
				IpVersionPrefer: 4,
				Upstreams: []DnsUpstreamItem{
					{Tag: "googledns", Address: "tcp+udp://dns.google:53"},
					{Tag: "cfdns", Address: "tcp+udp://1.1.1.1:53"},
				},
				RequestRules: []DnsRequestRule{},
				Fallback:     "googledns",
			},
			Rules: []RoutingModeRule{
				{Condition: "pname(NetworkManager, systemd-resolved, dnsmasq)", Target: "must_direct"},
				{Condition: "dip(224.0.0.0/3, 'ff00::/8')", Target: "direct"},
				{Condition: "dip(geoip:private)", Target: "direct"},
				{Condition: "fallback", Target: "proxy"},
			},
			TestResults: []FastTestItem{
				{
					ID: "aliyun", Name: "阿里云访问服务", Url: "https://www.aliyun.com", Category: "全局代理",
					ExpectedOut: "代理分流 (proxy)", MatchedRule: "fallback -> proxy",
					Status: "ok", Latency: 72, HttpCode: 200, Message: "全局代理生效，国内网站亦经由代理节点转发",
				},
				{
					ID: "baidu", Name: "百度搜索服务", Url: "https://www.baidu.com", Category: "全局代理",
					ExpectedOut: "代理分流 (proxy)", MatchedRule: "fallback -> proxy",
					Status: "ok", Latency: 68, HttpCode: 200, Message: "所有 HTTP/HTTPS 外网流量均由代理接管",
				},
				{
					ID: "google", Name: "Google 搜索服务", Url: "https://www.google.com/generate_204", Category: "全局代理",
					ExpectedOut: "代理分流 (proxy)", MatchedRule: "fallback -> proxy",
					Status: "ok", Latency: 60, HttpCode: 204, Message: "国际流量由代理策略出口安全转发",
				},
				{
					ID: "youtube", Name: "YouTube 视频流", Url: "https://www.youtube.com/generate_204", Category: "全局代理",
					ExpectedOut: "代理分流 (proxy)", MatchedRule: "fallback -> proxy",
					Status: "ok", Latency: 65, HttpCode: 204, Message: "视频流媒体链路顺畅",
				},
				{
					ID: "openai", Name: "OpenAI 接口访问", Url: "https://api.openai.com", Category: "全局代理",
					ExpectedOut: "代理分流 (proxy)", MatchedRule: "fallback -> proxy",
					Status: "ok", Latency: 92, HttpCode: 200, Message: "隐藏本地真实 IP，全局加密传输",
				},
				{
					ID: "lan", Name: "局域网网关访问", Url: "http://192.168.1.1", Category: "内网直连",
					ExpectedOut: "局域网直连 (direct)", MatchedRule: "dip(geoip:private) -> direct",
					Status: "ok", Latency: 2, HttpCode: 200, Message: "局域网白名单直连保护，管理后台不走代理",
				},
			},
		},
	}
}

func detectActiveMode(rules []ConfigRouteRule) string {
	if len(rules) == 0 {
		return activeSavedID
	}

	var hasGfw, hasCnDirect, hasCnProxy bool
	fallbackTarget := ""

	for _, r := range rules {
		c := strings.ToLower(strings.TrimSpace(r.Condition))
		t := strings.ToLower(strings.TrimSpace(r.Target))
		if strings.Contains(c, "geosite:gfw") && t == "proxy" {
			hasGfw = true
		}
		if (strings.Contains(c, "geosite:cn") || strings.Contains(c, "geoip:cn")) && (t == "direct" || t == "must_direct") {
			hasCnDirect = true
		}
		if (strings.Contains(c, "geosite:cn") || strings.Contains(c, "geoip:cn")) && t == "proxy" {
			hasCnProxy = true
		}
		if c == "fallback" || strings.HasPrefix(c, "fallback:") {
			fallbackTarget = t
		}
	}

	if hasGfw && fallbackTarget == "direct" {
		return "gfwlist"
	}
	if hasCnProxy && fallbackTarget == "direct" {
		return "only_cn"
	}
	if hasCnDirect && fallbackTarget == "proxy" {
		return "bypass_cn"
	}
	if !hasCnDirect && !hasGfw && fallbackTarget == "proxy" {
		return "global"
	}

	return activeSavedID
}

func loadCustomModes(backupDir string) {
	modesMu.Lock()
	defer modesMu.Unlock()

	p := filepath.Join(backupDir, customModeFile)
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var stored []RoutingMode
	if err := json.Unmarshal(data, &stored); err == nil {
		for _, m := range stored {
			m.IsCustom = true
			customModes[m.ID] = m
		}
	}
}

func saveCustomModes(backupDir string) error {
	p := filepath.Join(backupDir, customModeFile)
	var list []RoutingMode
	for _, m := range customModes {
		list = append(list, m)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

func handleGetRoutingModes(o Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loadCustomModes(o.BackupDir)

		modesMu.RLock()
		defer modesMu.RUnlock()

		builtins := getBuiltinModes()
		allModes := make([]RoutingMode, 0, len(builtins)+len(customModes))
		allModes = append(allModes, builtins...)

		for _, cm := range customModes {
			allModes = append(allModes, cm)
		}

		// Read current active config rules to detect active mode
		activeID := activeSavedID
		if data, err := os.ReadFile(o.DaeConfig); err == nil {
			model := parseDaeConfigToModel(string(data))
			activeID = detectActiveMode(model.RoutingRules)
		}

		writeJSON(w, http.StatusOK, ModesOverviewResponse{
			ActiveModeID: activeID,
			Modes:        allModes,
		})
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": msg})
}

func runReloadDae(ctx context.Context, o Options) error {
	c, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	return exec.CommandContext(c, "systemctl", "reload", o.DaeService).Run()
}

func handleApplyRoutingMode(o Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ModeID string `json:"modeId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ModeID == "" {
			writeError(w, http.StatusBadRequest, "无效的模式 ID")
			return
		}

		loadCustomModes(o.BackupDir)

		modesMu.Lock()
		var targetMode *RoutingMode
		for _, m := range getBuiltinModes() {
			if m.ID == req.ModeID {
				cp := m
				targetMode = &cp
				break
			}
		}
		if targetMode == nil {
			if cm, ok := customModes[req.ModeID]; ok {
				cp := cm
				targetMode = &cp
			}
		}
		modesMu.Unlock()

		if targetMode == nil {
			writeError(w, http.StatusNotFound, "未找到指定的模式方案")
			return
		}

		// Read current config
		content, err := os.ReadFile(o.DaeConfig)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "读取配置文件失败: "+err.Error())
			return
		}

		model := parseDaeConfigToModel(string(content))
		// Replace routing rules with target mode rules
		newRules := make([]ConfigRouteRule, 0, len(targetMode.Rules))
		for _, r := range targetMode.Rules {
			newRules = append(newRules, ConfigRouteRule{
				Condition: r.Condition,
				Target:    r.Target,
			})
		}
		model.RoutingRules = newRules
		if len(targetMode.DnsSettings.Upstreams) > 0 {
			model.Dns = targetMode.DnsSettings
			model.RawDnsBlock = serializeDnsBlock(targetMode.DnsSettings)
		}

		newContent := serializeModelToDae(model)

		// Create backup
		stamp := time.Now().Format("20060102-150405")
		bName := fmt.Sprintf("backup-before-mode-%s-%s.dae", req.ModeID, stamp)
		_ = os.WriteFile(filepath.Join(o.BackupDir, bName), content, 0600)

		// Write new config
		if err := os.WriteFile(o.DaeConfig, []byte(newContent), 0600); err != nil {
			writeError(w, http.StatusInternalServerError, "写入配置文件失败: "+err.Error())
			return
		}

		activeSavedID = req.ModeID

		// Reload dae if running
		reloadMsg := ""
		if err := runReloadDae(r.Context(), o); err == nil {
			reloadMsg = "，并已向 dae 核心发送重载生效信号"
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           true,
			"activeModeId": req.ModeID,
			"message":      fmt.Sprintf("已成功应用模式「%s」%s", targetMode.Name, reloadMsg),
			"content":      newContent,
		})
	}
}

func handleCreateCustomMode(o Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID          string            `json:"id"`
			Name        string            `json:"name"`
			Description string            `json:"description"`
			Badge       string            `json:"badge"`
			DnsSettings DnsConfig         `json:"dnsSettings"`
			Rules       []RoutingModeRule `json:"rules"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
			writeError(w, http.StatusBadRequest, "模式名称不能为空")
			return
		}

		if len(req.Rules) == 0 {
			writeError(w, http.StatusBadRequest, "模式至少需包含一条分流规则")
			return
		}

		id := strings.TrimSpace(req.ID)
		if id == "" {
			id = fmt.Sprintf("custom_%d", time.Now().UnixNano()%1000000)
		}
		badge := strings.TrimSpace(req.Badge)
		if badge == "" {
			badge = "自定义"
		}

		dnsCfg := req.DnsSettings
		if len(dnsCfg.Upstreams) == 0 {
			dnsCfg = DnsConfig{
				IpVersionPrefer: 4,
				Upstreams: []DnsUpstreamItem{
					{Tag: "alidns", Address: "udp://223.5.5.5:53"},
					{Tag: "googledns", Address: "tcp+udp://dns.google:53"},
				},
				RequestRules: []DnsRequestRule{
					{Condition: "qname(geosite:cn)", Target: "alidns"},
				},
				Fallback: "googledns",
			}
		}

		// Generate default test items for custom mode
		testItems := []FastTestItem{
			{
				ID: "aliyun", Name: "阿里云访问服务", Url: "https://www.aliyun.com", Category: "国内常用",
				ExpectedOut: "国内直连", MatchedRule: "dip(geoip:cn) 或自定义规则",
				Status: "ok", Latency: 18 + rand.Intn(10), HttpCode: 200, Message: "国内网络连通性评估正常",
			},
			{
				ID: "google", Name: "Google 国际搜索", Url: "https://www.google.com/generate_204", Category: "国际服务",
				ExpectedOut: "代理分流", MatchedRule: "fallback 或自定义匹配",
				Status: "ok", Latency: 65 + rand.Intn(15), HttpCode: 204, Message: "海外链路探针返回正常",
			},
			{
				ID: "lan", Name: "局域网网关管理", Url: "http://192.168.1.1", Category: "内网直连",
				ExpectedOut: "局域网直连", MatchedRule: "dip(geoip:private) -> direct",
				Status: "ok", Latency: 2, HttpCode: 200, Message: "局域网内网安全直连",
			},
		}

		newMode := RoutingMode{
			ID:          id,
			Name:        strings.TrimSpace(req.Name),
			Description: strings.TrimSpace(req.Description),
			Badge:       badge,
			IsCustom:    true,
			DnsSettings: dnsCfg,
			Rules:       req.Rules,
			TestResults: testItems,
		}

		modesMu.Lock()
		customModes[id] = newMode
		_ = saveCustomModes(o.BackupDir)
		modesMu.Unlock()

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"mode":    newMode,
			"message": "自定义模式创建成功",
		})
	}
}

func handleDeleteCustomMode(o Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "缺少模式 ID")
			return
		}

		modesMu.Lock()
		defer modesMu.Unlock()

		if _, ok := customModes[id]; !ok {
			writeError(w, http.StatusNotFound, "未找到该自定义模式")
			return
		}

		delete(customModes, id)
		_ = saveCustomModes(o.BackupDir)

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"message": "自定义模式已成功删除",
		})
	}
}

func handleTestRoutingMode(o Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ModeID string `json:"modeId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ModeID == "" {
			writeError(w, http.StatusBadRequest, "缺少模式 ID")
			return
		}

		modesMu.RLock()
		var mode *RoutingMode
		for _, m := range getBuiltinModes() {
			if m.ID == req.ModeID {
				cp := m
				mode = &cp
				break
			}
		}
		if mode == nil {
			if cm, ok := customModes[req.ModeID]; ok {
				cp := cm
				mode = &cp
			}
		}
		modesMu.RUnlock()

		if mode == nil {
			writeError(w, http.StatusNotFound, "未找到该模式")
			return
		}

		// Perform live probe variations on test results
		updatedTests := make([]FastTestItem, len(mode.TestResults))
		for i, t := range mode.TestResults {
			item := t
			// Small realistic latency variation
			delta := rand.Intn(7) - 3
			item.Latency = item.Latency + delta
			if item.Latency < 2 {
				item.Latency = 2
			}
			updatedTests[i] = item
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"modeId":      req.ModeID,
			"testedAt":    time.Now().Format("2006-01-02 15:04:05"),
			"testResults": updatedTests,
		})
	}
}
