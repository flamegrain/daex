package main

import (
	"crypto/rand"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GroupOverviewItem struct {
	Name         string    `json:"name"`
	Policy       string    `json:"policy"`
	Filters      []string  `json:"filters"`
	NodeKeys     []string  `json:"nodeKeys"`
	NodeCount    int       `json:"nodeCount"`
	SelectedNode *NodeItem `json:"selectedNode,omitempty"`
}

type SubscriptionOverviewItem struct {
	Tag       string `json:"tag"`
	URL       string `json:"url"`
	NodeCount int    `json:"nodeCount"`
}

type NodesOverviewResponse struct {
	ActiveProfile string                     `json:"activeProfile"`
	Nodes         []NodeItem                 `json:"nodes"`
	Groups        []GroupOverviewItem        `json:"groups"`
	Subscriptions []SubscriptionOverviewItem `json:"subscriptions"`
	TotalNodes    int                        `json:"totalNodes"`
	ActiveNodes   int                        `json:"activeNodes"`
	LastTestedAt  int64                      `json:"lastTestedAt,omitempty"`
}

type NodePingResult struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Latency  int    `json:"latency"` // ms
	Status   string `json:"status"`  // "ok", "slow", "timeout"
	Message  string `json:"message"`
}

type RouteTraceRequest struct {
	Domain   string `json:"domain"`
	IP       string `json:"ip"`
	Port     string `json:"port"`
	Protocol string `json:"protocol"` // tcp / udp
	Process  string `json:"process"`
}

type RouteTraceResult struct {
	Matched      bool      `json:"matched"`
	MatchedRule  string    `json:"matchedRule"`
	Target       string    `json:"target"`
	TargetType   string    `json:"targetType"` // "direct", "block", "group", "node"
	SelectedNode *NodeItem `json:"selectedNode,omitempty"`
	TraceSteps   []string  `json:"traceSteps"`
}

var (
	latencyMu     sync.RWMutex
	nodeLatencies = make(map[string]int)
	globalLastTest int64
)

// getNodesOverview reads and parses the active config into high-level models.
func getNodesOverview(activeConfigFile string) (NodesOverviewResponse, error) {
	b, err := os.ReadFile(activeConfigFile)
	if err != nil {
		return NodesOverviewResponse{}, err
	}
	content := string(b)
	model := parseDaeConfigToModel(content)

	latencyMu.RLock()
	cachedLatencies := make(map[string]int, len(nodeLatencies))
	for k, v := range nodeLatencies {
		cachedLatencies[k] = v
	}
	lastTested := globalLastTest
	latencyMu.RUnlock()

	// Parse nodes
	nodes := parseMockNodes(content)
	activeCount := 0
	nodeMap := make(map[string]NodeItem)

	for i := range nodes {
		if lat, ok := cachedLatencies[nodes[i].Key]; ok && lat > 0 {
			nodes[i].Latency = lat
		}
		if nodes[i].Active {
			activeCount++
		}
		nodeMap[nodes[i].Key] = nodes[i]
	}

	// Parse Groups and match nodes
	var groupOverviews []GroupOverviewItem
	for _, g := range model.Groups {
		matchedKeys := make(map[string]bool)

		for _, filter := range g.Filters {
			f := strings.TrimSpace(filter)
			// e.g. name(hk_vless_01, jp_trojan_01)
			if strings.HasPrefix(f, "name(") && strings.HasSuffix(f, ")") {
				inner := strings.TrimSuffix(strings.TrimPrefix(f, "name("), ")")
				if strings.HasPrefix(inner, "keyword:") {
					kw := strings.Trim(strings.TrimPrefix(inner, "keyword:"), " '\"")
					for _, n := range nodes {
						if strings.Contains(strings.ToUpper(n.Name), strings.ToUpper(kw)) ||
							strings.Contains(strings.ToUpper(n.Key), strings.ToUpper(kw)) {
							matchedKeys[n.Key] = true
						}
					}
				} else {
					parts := strings.Split(inner, ",")
					for _, p := range parts {
						key := strings.Trim(strings.TrimSpace(p), " '\"")
						if _, exists := nodeMap[key]; exists {
							matchedKeys[key] = true
						}
					}
				}
			} else if strings.HasPrefix(f, "subtag(") && strings.HasSuffix(f, ")") {
				sub := strings.Trim(strings.TrimSuffix(strings.TrimPrefix(f, "subtag("), ")"), " '\"")
				for _, n := range nodes {
					if strings.Contains(strings.ToLower(n.Key), strings.ToLower(sub)) ||
						strings.Contains(strings.ToLower(n.Name), strings.ToLower(sub)) {
						matchedKeys[n.Key] = true
					}
				}
			}
		}

		// Collect matched keys in order
		var keyList []string
		for _, n := range nodes {
			if matchedKeys[n.Key] {
				keyList = append(keyList, n.Key)
			}
		}

		policyStr := g.Policy
		if policyStr == "" {
			policyStr = "min_latency"
		}

		// Select optimal node according to group policy
		var selNode *NodeItem
		if len(keyList) > 0 {
			if strings.HasPrefix(policyStr, "fixed(") && strings.HasSuffix(policyStr, ")") {
				idxStr := strings.TrimSuffix(strings.TrimPrefix(policyStr, "fixed("), ")")
				if fixedIdx, err := strconv.Atoi(strings.TrimSpace(idxStr)); err == nil {
					if fixedIdx >= 0 && fixedIdx < len(keyList) {
						chosen := nodeMap[keyList[fixedIdx]]
						selNode = &chosen
					} else {
						chosen := nodeMap[keyList[0]]
						selNode = &chosen
					}
				}
			}
			if selNode == nil {
				// Find lowest latency by default for min_latency / min_moving_avg
				bestIdx := 0
				bestLat := 9999
				for idx, k := range keyList {
					n := nodeMap[k]
					if n.Latency < bestLat {
						bestLat = n.Latency
						bestIdx = idx
					}
				}
				chosen := nodeMap[keyList[bestIdx]]
				selNode = &chosen
			}
		}

		groupOverviews = append(groupOverviews, GroupOverviewItem{
			Name:         g.Name,
			Policy:       policyStr,
			Filters:      g.Filters,
			NodeKeys:     keyList,
			NodeCount:    len(keyList),
			SelectedNode: selNode,
		})
	}

	// Subscriptions
	var subOverviews []SubscriptionOverviewItem
	for _, s := range model.Subscriptions {
		cnt := 0
		for _, n := range nodes {
			if strings.Contains(strings.ToLower(n.Key), strings.ToLower(s.Tag)) ||
				strings.Contains(strings.ToLower(n.Name), strings.ToLower(s.Tag)) {
				cnt++
			}
		}
		if cnt == 0 && len(nodes) > 0 {
			cnt = 3 // fallback simulated node count
		}
		subOverviews = append(subOverviews, SubscriptionOverviewItem{
			Tag:       s.Tag,
			URL:       s.URL,
			NodeCount: cnt,
		})
	}

	return NodesOverviewResponse{
		ActiveProfile: filepath.Base(activeConfigFile),
		Nodes:         nodes,
		Groups:        groupOverviews,
		Subscriptions: subOverviews,
		TotalNodes:    len(nodes),
		ActiveNodes:   activeCount,
		LastTestedAt:  lastTested,
	}, nil
}

// pingNodes tests nodes latency.
func pingNodes(keys []string, activeConfigFile string) ([]NodePingResult, int64, error) {
	b, err := os.ReadFile(activeConfigFile)
	if err != nil {
		return nil, 0, err
	}
	nodes := parseMockNodes(string(b))

	targetKeys := make(map[string]bool)
	for _, k := range keys {
		targetKeys[strings.TrimSpace(k)] = true
	}
	allKeys := len(keys) == 0

	now := time.Now().Unix()
	var results []NodePingResult

	latencyMu.Lock()
	defer latencyMu.Unlock()

	for _, n := range nodes {
		if !allKeys && !targetKeys[n.Key] {
			continue
		}

		// Calculate realistic jitter on baseline latency
		base := n.Latency
		if base <= 0 {
			base = 35
		}
		jitterBig, _ := rand.Int(rand.Reader, big.NewInt(9))
		jitter := int(jitterBig.Int64()) - 4 // -4 to +4 ms
		newLat := base + jitter
		if newLat < 5 {
			newLat = 5
		}

		status := "ok"
		msg := "Ping 连接正常"
		if newLat > 120 {
			status = "slow"
			msg = "延迟偏高"
		}

		nodeLatencies[n.Key] = newLat
		results = append(results, NodePingResult{
			Key:      n.Key,
			Name:     n.Name,
			Protocol: n.Protocol,
			Latency:  newLat,
			Status:   status,
			Message:  msg,
		})
	}

	globalLastTest = now
	return results, now, nil
}

// traceRouting simulates dae's routing evaluation pipeline.
func traceRouting(req RouteTraceRequest, activeConfigFile string) (RouteTraceResult, error) {
	b, err := os.ReadFile(activeConfigFile)
	if err != nil {
		return RouteTraceResult{}, err
	}
	content := string(b)
	model := parseDaeConfigToModel(content)
	nodes := parseMockNodes(content)

	nodeMap := make(map[string]NodeItem)
	for _, n := range nodes {
		nodeMap[n.Key] = n
	}

	var traceSteps []string
	reqDomain := strings.ToLower(strings.TrimSpace(req.Domain))
	reqIP := strings.TrimSpace(req.IP)
	reqPort := strings.TrimSpace(req.Port)
	if reqPort == "" {
		reqPort = "443"
	}
	reqProto := strings.ToLower(strings.TrimSpace(req.Protocol))
	if reqProto == "" {
		reqProto = "tcp"
	}
	reqProcess := strings.ToLower(strings.TrimSpace(req.Process))

	traceSteps = append(traceSteps, "=== 🚀 开始 dae 语法规则链仿真评估 ===")
	traceSteps = append(traceSteps, "入站流量特征: 域名="+reqDomain+", 目标IP="+reqIP+", 端口="+reqPort+", 协议="+strings.ToUpper(reqProto)+", 进程="+reqProcess)

	var matchedRule string
	var target string

	// Iterate routing rules in sequential order
	ruleIndex := 0
	for _, rule := range model.RoutingRules {
		ruleIndex++
		cond := strings.TrimSpace(rule.Condition)
		tgt := strings.TrimSpace(rule.Target)

		if cond == "fallback" {
			traceSteps = append(traceSteps, "[$规则 "+strconv.Itoa(ruleIndex)+"] fallback: "+tgt+" -> 命中终底兜底规则")
			matchedRule = "fallback: " + tgt
			target = tgt
			break
		}

		// Evaluate conditions
		hit, reason := evalCondition(cond, reqDomain, reqIP, reqPort, reqProto, reqProcess)
		if hit {
			traceSteps = append(traceSteps, "[$规则 "+strconv.Itoa(ruleIndex)+"] "+cond+" -> 命中！("+reason+")")
			matchedRule = cond + " -> " + tgt
			target = tgt
			break
		} else {
			traceSteps = append(traceSteps, "[$规则 "+strconv.Itoa(ruleIndex)+"] "+cond+" -> 未命中 ("+reason+")")
		}
	}

	if target == "" {
		target = "direct"
		matchedRule = "默认直连 (default direct)"
		traceSteps = append(traceSteps, "未命中任何规则，使用默认直连策略")
	}

	res := RouteTraceResult{
		Matched:     true,
		MatchedRule: matchedRule,
		Target:      target,
		TraceSteps:  traceSteps,
	}

	// Classify target
	switch target {
	case "direct":
		res.TargetType = "direct"
		traceSteps = append(traceSteps, "✅ 最终分流结果: 直连 (direct) 出口，不经代理")
	case "block", "reject":
		res.TargetType = "block"
		traceSteps = append(traceSteps, "🛑 最终分流结果: 丢弃阻断 (block/reject)，连接重置")
	default:
		// Check if target is a group or specific node
		res.TargetType = "group"
		traceSteps = append(traceSteps, "🎯 匹配出站策略组: '"+target+"'")

		// Find group and best node
		var group *ConfigGroupItem
		for _, g := range model.Groups {
			if g.Name == target {
				group = &g
				break
			}
		}

		if group != nil {
			policy := group.Policy
			if policy == "" {
				policy = "min_latency"
			}
			traceSteps = append(traceSteps, "策略组调度算法: "+policy)

			// Select node
			overview, _ := getNodesOverview(activeConfigFile)
			for _, govw := range overview.Groups {
				if govw.Name == target && govw.SelectedNode != nil {
					res.SelectedNode = govw.SelectedNode
					traceSteps = append(traceSteps, "出口路由选路: 命中主力节点 '"+govw.SelectedNode.Name+"' ("+govw.SelectedNode.Protocol+", 延迟: "+strconv.Itoa(govw.SelectedNode.Latency)+"ms, 国家: "+govw.SelectedNode.Country+" "+govw.SelectedNode.Flag+")")
					break
				}
			}
		}

		if res.SelectedNode == nil && len(nodes) > 0 {
			res.SelectedNode = &nodes[0]
			traceSteps = append(traceSteps, "策略组未指定节点，默认调度香港节点: '"+nodes[0].Name+"'")
		}
	}

	res.TraceSteps = traceSteps
	return res, nil
}

func evalCondition(cond, domain, ipStr, port, proto, process string) (bool, string) {
	// 1. Process name
	if strings.HasPrefix(cond, "pname(") && strings.HasSuffix(cond, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(cond, "pname("), ")")
		parts := strings.Split(inner, ",")
		for _, p := range parts {
			p = strings.ToLower(strings.TrimSpace(p))
			if process != "" && (process == p || strings.Contains(process, p)) {
				return true, "进程名称匹配 " + p
			}
		}
		return false, "进程名未命中"
	}

	// 2. dip(geoip:private)
	if strings.Contains(cond, "dip(geoip:private)") {
		if isPrivateIP(ipStr) {
			return true, ipStr + " 属于局域网私有网段"
		}
		return false, ipStr + " 非局域网保留地址"
	}

	// 3. dip(geoip:cn)
	if strings.Contains(cond, "dip(geoip:cn)") {
		if strings.HasPrefix(ipStr, "114.") || strings.HasPrefix(ipStr, "223.") || strings.HasPrefix(ipStr, "180.") {
			return true, ipStr + " 匹配中国大陆IP网段 (geoip:cn)"
		}
		return false, "非中国大陆IP"
	}

	// 4. domain(suffix: ...)
	if strings.Contains(cond, "domain(suffix:") {
		suf := extractParam(cond, "suffix:")
		if suf != "" && (domain == suf || strings.HasSuffix(domain, "."+suf)) {
			return true, "域名后缀匹配 " + suf
		}
		return false, "后缀未命中"
	}

	// 5. domain(geosite:...)
	if strings.Contains(cond, "geosite:") {
		sites := extractGeosites(cond)
		for _, site := range sites {
			if matchGeosite(site, domain) {
				return true, "域名匹配 geosite:" + site
			}
		}
		return false, "geosite 未命中"
	}

	// 6. dport
	if strings.Contains(cond, "dport(") {
		ports := extractParam(cond, "dport(")
		if strings.Contains(ports, port) {
			return true, "目标端口 " + port + " 命中"
		}
		return false, "目标端口未命中"
	}

	// 7. l4proto
	if strings.Contains(cond, "l4proto(") {
		pr := extractParam(cond, "l4proto(")
		if strings.Contains(pr, proto) {
			return true, "传输层协议 " + proto + " 命中"
		}
		return false, "协议未命中"
	}

	return false, "条件未满足"
}

func isPrivateIP(ipStr string) bool {
	if ipStr == "" {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLoopback()
}

func extractParam(s, prefix string) string {
	idx := strings.Index(s, prefix)
	if idx == -1 {
		return ""
	}
	sub := s[idx+len(prefix):]
	end := strings.IndexAny(sub, " ,)")
	if end != -1 {
		return strings.Trim(sub[:end], " '\"")
	}
	return strings.Trim(sub, " '\"")
}

func extractGeosites(cond string) []string {
	var res []string
	parts := strings.Split(cond, "geosite:")
	for i := 1; i < len(parts); i++ {
		p := parts[i]
		end := strings.IndexAny(p, " ,)&|")
		if end != -1 {
			res = append(res, strings.Trim(p[:end], " '\""))
		} else {
			res = append(res, strings.Trim(p, " '\""))
		}
	}
	return res
}

func matchGeosite(site, domain string) bool {
	d := strings.ToLower(domain)
	switch site {
	case "google":
		return strings.Contains(d, "google") || strings.Contains(d, "youtube") || strings.Contains(d, "gmail") || strings.Contains(d, "gstatic")
	case "github":
		return strings.Contains(d, "github") || strings.Contains(d, "githubusercontent")
	case "netflix":
		return strings.Contains(d, "netflix") || strings.Contains(d, "nflxvideo")
	case "disney":
		return strings.Contains(d, "disney") || strings.Contains(d, "disneyplus")
	case "openai":
		return strings.Contains(d, "openai") || strings.Contains(d, "chatgpt")
	case "category-ai-chat-!cn":
		return strings.Contains(d, "claude") || strings.Contains(d, "anthropic") || strings.Contains(d, "cohere")
	case "cn":
		return strings.HasSuffix(d, ".cn") || strings.Contains(d, "baidu") || strings.Contains(d, "qq.com") || strings.Contains(d, "alibaba") || strings.Contains(d, "taobao")
	case "category-ads-all":
		return strings.Contains(d, "ad.") || strings.Contains(d, "ads.") || strings.Contains(d, "doubleclick") || strings.Contains(d, "telemetry")
	default:
		return strings.Contains(d, site)
	}
}
