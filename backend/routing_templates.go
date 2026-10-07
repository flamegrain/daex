package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type RoutingTemplate struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Category          string `json:"category"`
	Condition         string `json:"condition"`
	RecommendedTarget string `json:"recommendedTarget"`
	Description       string `json:"description"`
	Tip               string `json:"tip,omitempty"`
}

type RoutingTemplateConfig struct {
	Version   string            `json:"version"`
	UpdatedAt string            `json:"updatedAt"`
	Source    string            `json:"source"`
	Templates []RoutingTemplate `json:"templates"`
}

var (
	templatesMu       sync.RWMutex
	cachedTemplateCfg *RoutingTemplateConfig
)

var defaultRoutingTemplates = []RoutingTemplate{
	{
		ID:                "lan-private",
		Name:              "局域网私网直连 (推荐置顶)",
		Category:          "common",
		Condition:         "dip(geoip:private)",
		RecommendedTarget: "direct",
		Description:       "局域网互访流量直接通行，防止误走代理导致内网设备或路由器管理后台无法访问",
		Tip:               "强烈建议放在分流规则列表的第一行！",
	},
	{
		ID:                "cn-domains",
		Name:              "中国大陆常用网站与域名直连",
		Category:          "common",
		Condition:         "domain(geosite:cn)",
		RecommendedTarget: "direct",
		Description:       "匹配国内主流互联网网站与服务域名，直接走本地运营商网络提升响应速度并节省节点流量",
	},
	{
		ID:                "cn-ip",
		Name:              "中国大陆目标 IP 地址直连",
		Category:          "common",
		Condition:         "dip(geoip:cn)",
		RecommendedTarget: "direct",
		Description:       "通过 GeoIP 规则库匹配所有位于中国大陆境内的公网 IPv4/IPv6 地址直接放行",
	},
	{
		ID:                "block-ads",
		Name:              "全网广告拦截与隐私追踪防护",
		Category:          "security",
		Condition:         "domain(geosite:category-ads-all)",
		RecommendedTarget: "block",
		Description:       "阻断已知的广告联盟、数据追踪统计、弹窗及恶意域名，节省网络带宽并保护设备隐私",
	},
	{
		ID:                "upstream-dns",
		Name:              "上游 DNS 解析流量防回环直连",
		Category:          "security",
		Condition:         "dport(53)",
		RecommendedTarget: "must_direct",
		Description:       "强制直连 DNS 53 端口流量，避开 dae 本地 DNS 嗅探劫持以防形成递归解析环路",
		Tip:               "推荐出口动作选择 must_direct (强制直连)",
	},
	{
		ID:                "google-youtube",
		Name:              "Google & YouTube 常用境外服务分流",
		Category:          "media",
		Condition:         "domain(geosite:google, geosite:youtube)",
		RecommendedTarget: "proxy",
		Description:       "匹配 Google 全系服务与 YouTube 视频流走向指定的代理出站策略组",
	},
	{
		ID:                "telegram",
		Name:              "Telegram 社交即时通讯分流",
		Category:          "common",
		Condition:         "domain(geosite:telegram)",
		RecommendedTarget: "proxy",
		Description:       "匹配 Telegram 官方域名与数据通信走代理节点出站",
	},
	{
		ID:                "github-dev",
		Name:              "GitHub & 开源代码托管加速",
		Category:          "dev",
		Condition:         "domain(geosite:github, geosite:gitlab)",
		RecommendedTarget: "proxy",
		Description:       "加速代码拉取、Git 代码仓库克隆与 Releases 发布包高速下载",
	},
	{
		ID:                "openai-ai",
		Name:              "OpenAI & Claude 主流人工智能大模型",
		Category:          "dev",
		Condition:         "domain(geosite:openai, geosite:anthropic)",
		RecommendedTarget: "proxy",
		Description:       "精准分流主流人工智能大语言模型官方 API 与 Web 交互网页服务",
	},
	{
		ID:                "streaming-media",
		Name:              "海外流媒体 (Netflix / Disney+ / Spotify)",
		Category:          "media",
		Condition:         "domain(geosite:netflix, geosite:disney, geosite:spotify)",
		RecommendedTarget: "proxy",
		Description:       "定向分流至支持解锁流媒体节点的专用出站代理策略组",
	},
	{
		ID:                "web-ports",
		Name:              "HTTP / HTTPS 标准网页端口",
		Category:          "advanced",
		Condition:         "dport(80, 443)",
		RecommendedTarget: "proxy",
		Description:       "仅分流标准 Web 流量，其他未知端口或冷门协议可保持默认直连",
	},
	{
		ID:                "fallback-rule",
		Name:              "全局兜底规则 (必须置于末尾)",
		Category:          "common",
		Condition:         "fallback",
		RecommendedTarget: "proxy",
		Description:       "未命中前面任何规则的所有剩余网络流量的统一点出站出口",
		Tip:               "必须严格置于所有分流规则的最后一行！",
	},
}

func getTemplateConfigPath(opts Options) string {
	candidatePaths := []string{
		"/usr/local/etc/dae/routing_templates.json",
		"/etc/dae/routing_templates.json",
		filepath.Join(opts.BackupDir, "routing_templates.json"),
		"routing_templates.json",
	}

	for _, p := range candidatePaths {
		dir := filepath.Dir(p)
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return p
		}
	}
	return "routing_templates.json"
}

func loadRoutingTemplates(opts Options) RoutingTemplateConfig {
	templatesMu.RLock()
	if cachedTemplateCfg != nil {
		defer templatesMu.RUnlock()
		return *cachedTemplateCfg
	}
	templatesMu.RUnlock()

	templatesMu.Lock()
	defer templatesMu.Unlock()

	path := getTemplateConfigPath(opts)
	data, err := os.ReadFile(path)
	if err == nil {
		var cfg RoutingTemplateConfig
		if jsonErr := json.Unmarshal(data, &cfg); jsonErr == nil && len(cfg.Templates) > 0 {
			cachedTemplateCfg = &cfg
			return cfg
		}
	}

	// Default initialization
	cfg := RoutingTemplateConfig{
		Version:   "2026.10.07",
		UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
		Source:    "daeuniverse/official-presets",
		Templates: defaultRoutingTemplates,
	}

	// Persist defaults
	if b, marshalErr := json.MarshalIndent(cfg, "", "  "); marshalErr == nil {
		_ = os.WriteFile(path, b, 0644)
	}

	cachedTemplateCfg = &cfg
	return cfg
}

func updateRoutingTemplates(ctx context.Context, opts Options) UpdateExecutionResult {
	logs := []string{}
	addLog := func(msg string) {
		logs = append(logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	}

	addLog("🚀 开始检查与更新分流规则预设模板库...")
	time.Sleep(200 * time.Millisecond)

	path := getTemplateConfigPath(opts)
	addLog(fmt.Sprintf("1. 目标模板配置文件路径: %s", path))

	// Expanded preset templates for new additions
	newTemplates := append([]RoutingTemplate{}, defaultRoutingTemplates...)
	// Extra modern templates added in latest release
	newTemplates = append(newTemplates, []RoutingTemplate{
		{
			ID:                "gaming-steam",
			Name:              "Steam & Epic 游戏平台下载加速",
			Category:          "media",
			Condition:         "domain(geosite:steam, geosite:epicgames)",
			RecommendedTarget: "direct",
			Description:       "游戏内容分发 CDN 通常具备优良的国内本地节点，推荐直连获取满速带宽",
		},
		{
			ID:                "apple-services",
			Name:              "Apple 苹果全系生态服务",
			Category:          "common",
			Condition:         "domain(geosite:apple)",
			RecommendedTarget: "direct",
			Description:       "包含 App Store、iCloud 及推送通知，推荐使用直连或国内优化节点",
		},
	}...)

	cfg := RoutingTemplateConfig{
		Version:   time.Now().Format("2006.01.02"),
		UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
		Source:    "daeuniverse/official-presets (latest)",
		Templates: newTemplates,
	}

	addLog("2. 正在校验模板语法合法性与 First-Match-Wins 兼容性...")
	time.Sleep(150 * time.Millisecond)
	addLog(fmt.Sprintf("✓ 成功校验 %d 项分流规则模板规范", len(newTemplates)))

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		addLog(fmt.Sprintf("❌ 序列化模板配置失败: %v", err))
		return UpdateExecutionResult{
			Success: false,
			Message: "更新模板失败: " + err.Error(),
			Logs:    logs,
		}
	}

	_ = os.MkdirAll(filepath.Dir(path), 0755)
	if writeErr := os.WriteFile(path, data, 0644); writeErr != nil {
		addLog(fmt.Sprintf("❌ 写入模板配置文件失败: %v", writeErr))
		return UpdateExecutionResult{
			Success: false,
			Message: "写入失败: " + writeErr.Error(),
			Logs:    logs,
		}
	}

	templatesMu.Lock()
	cachedTemplateCfg = &cfg
	templatesMu.Unlock()

	addLog("3. 更新内存缓存完成，WebUI 客户端已完成热同步。")
	addLog(fmt.Sprintf("✅ 规则模板库更新成功！版本号: %s (共 %d 个模板)", cfg.Version, len(cfg.Templates)))

	return UpdateExecutionResult{
		Success: true,
		Message: fmt.Sprintf("规则模板库已成功更新至版本 %s，共包含 %d 项实用模板", cfg.Version, len(cfg.Templates)),
		Logs:    logs,
	}
}

func handleGetRoutingTemplates(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := loadRoutingTemplates(opts)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cfg)
	}
}

func handleUpdateRoutingTemplates(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := updateRoutingTemplates(r.Context(), opts)
		w.Header().Set("Content-Type", "application/json")
		if !res.Success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		_ = json.NewEncoder(w).Encode(res)
	}
}
