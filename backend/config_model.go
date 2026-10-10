package main

import (
	"fmt"
	"strconv"
	"strings"
)

type DnsUpstreamItem struct {
	Tag     string `json:"tag"`
	Address string `json:"address"`
}

type DnsRequestRule struct {
	Condition string `json:"condition"`
	Target    string `json:"target"`
}

type DnsConfig struct {
	IpVersionPrefer int               `json:"ipVersionPrefer"`
	FixedDomainTTL  int               `json:"fixedDomainTTL"`
	OptimisticCache bool              `json:"optimisticCache"`
	Upstreams       []DnsUpstreamItem `json:"upstreams"`
	RequestRules    []DnsRequestRule  `json:"requestRules"`
	Fallback        string            `json:"fallback"`
}

// DaeConfigModel represents parsed sections of a dae configuration file.
type DaeConfigModel struct {
	Globals       map[string]string `json:"globals"`
	Subscriptions []ConfigSubItem   `json:"subscriptions"`
	Nodes         []ConfigNodeItem  `json:"nodes"`
	Dns           DnsConfig         `json:"dns"`
	Groups        []ConfigGroupItem `json:"groups"`
	RoutingRules  []ConfigRouteRule `json:"routingRules"`
	RawDnsBlock   string            `json:"rawDnsBlock"`
	RawOtherLines []string          `json:"rawOtherLines"`
}

type ConfigSubItem struct {
	Tag string `json:"tag"`
	URL string `json:"url"`
}

type ConfigNodeItem struct {
	Key      string `json:"key"`
	URL      string `json:"url"`
	Protocol string `json:"protocol"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Port     string `json:"port"`
}

type ConfigGroupItem struct {
	Name    string   `json:"name"`
	Policy  string   `json:"policy"`
	Filters []string `json:"filters"`
}

type ConfigRouteRule struct {
	Condition string `json:"condition"`
	Target    string `json:"target"`
}

func parseDaeConfigToModel(text string) DaeConfigModel {
	model := DaeConfigModel{
		Globals:       make(map[string]string),
		Subscriptions: []ConfigSubItem{},
		Nodes:         []ConfigNodeItem{},
		Groups:        []ConfigGroupItem{},
		RoutingRules:  []ConfigRouteRule{},
		RawOtherLines: []string{},
	}

	lines := strings.Split(text, "\n")
	currentSection := ""
	currentGroup := (*ConfigGroupItem)(nil)
	var dnsLines []string
	dnsDepth := 0

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if currentSection == "dns" {
			dnsLines = append(dnsLines, rawLine)
			if strings.Contains(rawLine, "{") {
				dnsDepth += strings.Count(rawLine, "{")
			}
			if strings.Contains(rawLine, "}") {
				dnsDepth -= strings.Count(rawLine, "}")
				if dnsDepth <= 0 {
					currentSection = ""
					dnsDepth = 0
				}
			}
			continue
		}

		// Section headers
		if strings.HasSuffix(trimmed, "{") {
			secName := strings.TrimSpace(strings.TrimSuffix(trimmed, "{"))
			switch secName {
			case "global", "subscription", "node", "group", "routing":
				currentSection = secName
				continue
			case "dns":
				currentSection = "dns"
				dnsLines = append(dnsLines, rawLine)
				dnsDepth = 1
				continue
			default:
				if currentSection == "group" {
					// Sub-group definition: e.g. "proxy {"
					currentGroup = &ConfigGroupItem{
						Name: secName,
					}
					continue
				}
			}
		}

		if trimmed == "}" {
			if currentSection == "group" && currentGroup != nil {
				model.Groups = append(model.Groups, *currentGroup)
				currentGroup = nil
				continue
			} else {
				currentSection = ""
				continue
			}
		}

		switch currentSection {
		case "global":
			if k, v, ok := strings.Cut(trimmed, ":"); ok {
				k = strings.TrimSpace(k)
				v = strings.Trim(strings.TrimSpace(v), "',\"")
				model.Globals[k] = v
			}
		case "subscription":
			if k, v, ok := strings.Cut(trimmed, ":"); ok {
				k = strings.TrimSpace(k)
				v = strings.Trim(strings.TrimSpace(v), "',\"")
				model.Subscriptions = append(model.Subscriptions, ConfigSubItem{
					Tag: k,
					URL: v,
				})
			}
		case "node":
			if k, v, ok := strings.Cut(trimmed, ":"); ok {
				k = strings.TrimSpace(k)
				v = strings.Trim(strings.TrimSpace(v), "',\"")
				parsed := parseNodeURL(k, v)
				model.Nodes = append(model.Nodes, ConfigNodeItem{
					Key:      k,
					URL:      v,
					Protocol: parsed.Protocol,
					Name:     parsed.Name,
					Address:  parsed.Address,
					Port:     parsed.Port,
				})
			}
		case "dns":
			dnsLines = append(dnsLines, rawLine)
		case "group":
			if currentGroup != nil {
				if k, v, ok := strings.Cut(trimmed, ":"); ok {
					k = strings.TrimSpace(k)
					v = strings.TrimSpace(v)
					if k == "policy" {
						currentGroup.Policy = v
					} else if k == "filter" {
						currentGroup.Filters = append(currentGroup.Filters, v)
					}
				}
			}
		case "routing":
			if cond, target, ok := strings.Cut(trimmed, "->"); ok {
				model.RoutingRules = append(model.RoutingRules, ConfigRouteRule{
					Condition: strings.TrimSpace(cond),
					Target:    strings.TrimSpace(target),
				})
			} else if strings.HasPrefix(trimmed, "fallback:") {
				target := strings.TrimSpace(strings.TrimPrefix(trimmed, "fallback:"))
				model.RoutingRules = append(model.RoutingRules, ConfigRouteRule{
					Condition: "fallback",
					Target:    target,
				})
			}
		}
	}

	model.RawDnsBlock = strings.Join(dnsLines, "\n")
	model.Dns = parseDnsBlock(dnsLines)
	return model
}

func parseDnsBlock(lines []string) DnsConfig {
	cfg := DnsConfig{
		IpVersionPrefer: 4,
		Upstreams:       []DnsUpstreamItem{},
		RequestRules:    []DnsRequestRule{},
	}
	subSection := ""

	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasSuffix(trimmed, "{") {
			header := strings.TrimSpace(strings.TrimSuffix(trimmed, "{"))
			switch header {
			case "upstream":
				subSection = "upstream"
				continue
			case "request":
				subSection = "request"
				continue
			default:
				continue
			}
		}
		if trimmed == "}" {
			if subSection != "" {
				subSection = ""
			}
			continue
		}

		switch subSection {
		case "upstream":
			if k, v, ok := strings.Cut(trimmed, ":"); ok {
				tag := strings.TrimSpace(k)
				addr := strings.Trim(strings.TrimSpace(v), "',\"")
				cfg.Upstreams = append(cfg.Upstreams, DnsUpstreamItem{
					Tag:     tag,
					Address: addr,
				})
			}
		case "request":
			if cond, target, ok := strings.Cut(trimmed, "->"); ok {
				cfg.RequestRules = append(cfg.RequestRules, DnsRequestRule{
					Condition: strings.TrimSpace(cond),
					Target:    strings.TrimSpace(target),
				})
			} else if strings.HasPrefix(trimmed, "fallback:") {
				cfg.Fallback = strings.TrimSpace(strings.TrimPrefix(trimmed, "fallback:"))
			}
		default:
			if strings.HasPrefix(trimmed, "ipversion_prefer:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "ipversion_prefer:"))
				if n, err := strconv.Atoi(val); err == nil {
					cfg.IpVersionPrefer = n
				}
			} else if strings.HasPrefix(trimmed, "fixed_domain_ttl:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "fixed_domain_ttl:"))
				if n, err := strconv.Atoi(val); err == nil {
					cfg.FixedDomainTTL = n
				}
			} else if strings.HasPrefix(trimmed, "optimistic_cache:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "optimistic_cache:"))
				cfg.OptimisticCache = strings.EqualFold(val, "true")
			}
		}
	}

	// Default fallback if empty
	if cfg.Fallback == "" && len(cfg.Upstreams) > 0 {
		cfg.Fallback = cfg.Upstreams[len(cfg.Upstreams)-1].Tag
	}
	if len(cfg.Upstreams) == 0 {
		cfg.Upstreams = []DnsUpstreamItem{
			{Tag: "alidns", Address: "udp://223.5.5.5:53"},
			{Tag: "googledns", Address: "tcp+udp://dns.google:53"},
		}
		cfg.RequestRules = []DnsRequestRule{
			{Condition: "qname(geosite:cn)", Target: "alidns"},
		}
		cfg.Fallback = "googledns"
	}
	return cfg
}

func serializeDnsBlock(dns DnsConfig) string {
	var sb strings.Builder
	sb.WriteString("dns {\n")
	if dns.IpVersionPrefer > 0 {
		sb.WriteString(fmt.Sprintf("    ipversion_prefer: %d\n", dns.IpVersionPrefer))
	}
	if dns.FixedDomainTTL > 0 {
		sb.WriteString(fmt.Sprintf("    fixed_domain_ttl: %d\n", dns.FixedDomainTTL))
	}
	if dns.OptimisticCache {
		sb.WriteString("    optimistic_cache: true\n")
	}

	upstreams := dns.Upstreams
	if len(upstreams) == 0 {
		upstreams = []DnsUpstreamItem{
			{Tag: "alidns", Address: "udp://223.5.5.5:53"},
			{Tag: "googledns", Address: "tcp+udp://dns.google:53"},
		}
	}
	sb.WriteString("    upstream {\n")
	for _, u := range upstreams {
		addr := strings.Trim(strings.TrimSpace(u.Address), "'\"")
		sb.WriteString(fmt.Sprintf("        %s: '%s'\n", u.Tag, addr))
	}
	sb.WriteString("    }\n")

	sb.WriteString("    routing {\n        request {\n")
	for _, r := range dns.RequestRules {
		sb.WriteString(fmt.Sprintf("            %s -> %s\n", r.Condition, r.Target))
	}
	fb := dns.Fallback
	if fb == "" && len(upstreams) > 0 {
		fb = upstreams[len(upstreams)-1].Tag
	}
	if fb != "" {
		sb.WriteString(fmt.Sprintf("            fallback: %s\n", fb))
	}
	sb.WriteString("        }\n    }\n}\n")
	return sb.String()
}

func serializeModelToDae(model DaeConfigModel) string {
	var sb strings.Builder

	// Global block
	sb.WriteString("global {\n")
	standardGlobals := []string{
		"log_level", "lan_interface", "wan_interface", "tproxy_port", "tproxy_port_protect",
		"so_mark_from_dae", "tcp_check_url", "udp_check_dns", "check_interval", "check_tolerance",
		"dial_mode", "allow_insecure", "sniffing_timeout", "disable_waiting_network", "auto_config_kernel_parameter",
	}
	writtenGlobals := make(map[string]bool)
	for _, k := range standardGlobals {
		if v, ok := model.Globals[k]; ok && v != "" {
			if strings.Contains(v, " ") || strings.Contains(v, ",") || strings.Contains(v, ":") {
				sb.WriteString(fmt.Sprintf("    %s: '%s'\n", k, v))
			} else {
				sb.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
			}
			writtenGlobals[k] = true
		}
	}
	for k, v := range model.Globals {
		if !writtenGlobals[k] && v != "" {
			sb.WriteString(fmt.Sprintf("    %s: '%s'\n", k, v))
		}
	}
	sb.WriteString("}\n\n")

	// Subscription block
	if len(model.Subscriptions) > 0 {
		sb.WriteString("subscription {\n")
		for _, s := range model.Subscriptions {
			sb.WriteString(fmt.Sprintf("    %s: '%s'\n", s.Tag, s.URL))
		}
		sb.WriteString("}\n\n")
	}

	// Node block
	if len(model.Nodes) > 0 {
		sb.WriteString("node {\n")
		for _, n := range model.Nodes {
			sb.WriteString(fmt.Sprintf("    %s: '%s'\n", n.Key, n.URL))
		}
		sb.WriteString("}\n\n")
	}

	// DNS block (use model.Dns if populated or raw)
	if len(model.Dns.Upstreams) > 0 {
		sb.WriteString(serializeDnsBlock(model.Dns))
		sb.WriteString("\n")
	} else if strings.TrimSpace(model.RawDnsBlock) != "" {
		sb.WriteString(model.RawDnsBlock)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString(serializeDnsBlock(DnsConfig{}))
		sb.WriteString("\n")
	}

	// Group block
	if len(model.Groups) > 0 {
		sb.WriteString("group {\n")
		for _, g := range model.Groups {
			sb.WriteString(fmt.Sprintf("    %s {\n", g.Name))
			for _, f := range g.Filters {
				sb.WriteString(fmt.Sprintf("        filter: %s\n", f))
			}
			if g.Policy != "" {
				sb.WriteString(fmt.Sprintf("        policy: %s\n", g.Policy))
			}
			sb.WriteString("    }\n\n")
		}
		sb.WriteString("}\n\n")
	}

	// Routing block
	if len(model.RoutingRules) > 0 {
		sb.WriteString("routing {\n")
		for _, r := range model.RoutingRules {
			if r.Condition == "fallback" {
				sb.WriteString(fmt.Sprintf("    fallback: %s\n", r.Target))
			} else {
				sb.WriteString(fmt.Sprintf("    %s -> %s\n", r.Condition, r.Target))
			}
		}
		sb.WriteString("}\n")
	}

	return strings.TrimSpace(sb.String()) + "\n"
}
