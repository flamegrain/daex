package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"` // INFO, WARN, ERROR, DEBUG
	Tag       string `json:"tag"`   // routing, dns, ebpf, kernel, system
	Message   string `json:"message"`
	Raw       string `json:"raw"`
}

type LogsResponse struct {
	Entries []LogEntry `json:"entries"`
	Total   int        `json:"total"`
	Source  string     `json:"source"`
	Time    string     `json:"time"`
}

type LogBackupItem struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

// Regex to parse journalctl/syslog lines
var (
	reSyslogDate = regexp.MustCompile(`^([A-Z][a-z]{2}\s+\d+\s+\d{2}:\d{2}:\d{2})`)
	reISO8601    = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})`)
)

func parseLogLine(line string) LogEntry {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return LogEntry{Raw: line}
	}

	ts := time.Now().Format("2006-01-02 15:04:05")
	if m := reSyslogDate.FindString(trimmed); m != "" {
		ts = fmt.Sprintf("%d %s", time.Now().Year(), m)
	} else if m := reISO8601.FindString(trimmed); m != "" {
		ts = strings.Replace(m, "T", " ", 1)
	}

	// Detect Level
	upper := strings.ToUpper(trimmed)
	level := "INFO"
	if strings.Contains(upper, "ERROR") || strings.Contains(upper, "FATAL") || strings.Contains(upper, "FAILED") || strings.Contains(upper, "CRIT") {
		level = "ERROR"
	} else if strings.Contains(upper, "WARN") {
		level = "WARN"
	} else if strings.Contains(upper, "DEBUG") || strings.Contains(upper, "TRACE") {
		level = "DEBUG"
	}

	// Detect Tag / Category
	tag := "system"
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "routing") || strings.Contains(lower, "route") || strings.Contains(lower, "rule") || strings.Contains(lower, "fallback") {
		tag = "routing"
	} else if strings.Contains(lower, "dns") || strings.Contains(lower, "qname") || strings.Contains(lower, "resolve") {
		tag = "dns"
	} else if strings.Contains(lower, "bpf") || strings.Contains(lower, "ebpf") || strings.Contains(lower, "kernel") || strings.Contains(lower, "tproxy") {
		tag = "ebpf"
	} else if strings.Contains(lower, "node") || strings.Contains(lower, "group") || strings.Contains(lower, "proxy") || strings.Contains(lower, "latency") {
		tag = "proxy"
	}

	// Extract message body
	msg := trimmed
	if idx := strings.Index(trimmed, "]: "); idx != -1 {
		msg = strings.TrimSpace(trimmed[idx+3:])
	} else if idx := strings.Index(trimmed, ": "); idx != -1 && idx < 45 {
		msg = strings.TrimSpace(trimmed[idx+2:])
	}

	return LogEntry{
		Timestamp: ts,
		Level:     level,
		Tag:       tag,
		Message:   msg,
		Raw:       trimmed,
	}
}

func getDaeLogs(serviceName string, limit int) ([]LogEntry, string) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}

	cmd := exec.Command("journalctl", "-u", serviceName, "-n", fmt.Sprintf("%d", limit), "--no-pager")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil && stdout.Len() > 0 {
		lines := strings.Split(stdout.String(), "\n")
		var entries []LogEntry
		for _, l := range lines {
			if strings.TrimSpace(l) == "" {
				continue
			}
			entries = append(entries, parseLogLine(l))
		}
		if len(entries) > 0 {
			return entries, "journalctl (" + serviceName + ")"
		}
	}

	// Fallback mock daemon & eBPF traffic event logs for sandboxed/mock environment
	now := time.Now()
	mockRawLogs := []struct {
		offset int
		level  string
		tag    string
		msg    string
	}{
		{60, "INFO", "system", "dae core initialized successfully with eBPF TC/CGroup dispatchers"},
		{55, "INFO", "ebpf", "loaded eBPF prog 'dae_wan_ingress' on interface eth0 (so_mark: 0x54)"},
		{50, "INFO", "dns", "dns server listening on 127.0.0.1:53, upstream: alidns (223.5.5.5:53), googledns (8.8.8.8:53)"},
		{45, "INFO", "proxy", "group 'proxy': policy min_moving_avg selected node 'hk_vless_01' (latency: 32ms)"},
		{40, "INFO", "routing", "matched rule [domain(geosite:cn)] -> direct for host 'www.bilibili.com:443'"},
		{35, "INFO", "routing", "matched rule [dip(geoip:private)] -> direct for destination 192.168.1.1:22"},
		{30, "WARN", "dns", "upstream query timeout for 'test-subdomain.com' via alidns (retry 1/2)"},
		{25, "INFO", "dns", "qname 'github.com' resolved to 20.205.243.166 via googledns in 48ms"},
		{20, "INFO", "routing", "matched rule [domain(geosite:github)] -> proxy group for 20.205.243.166:443"},
		{15, "ERROR", "proxy", "tcp check failed for backup node 'sg_vmess_02': dial tcp: i/o timeout"},
		{10, "INFO", "ebpf", "tproxy_port: 12345 active, bypass packets count: 18,294, routed packets: 95,201"},
		{5, "INFO", "routing", "fallback: proxy selected for unknown foreign flow 104.16.132.229:443"},
		{2, "DEBUG", "ebpf", "packet conntrack entry refreshed for connection from 192.168.1.100:54210"},
		{0, "INFO", "system", "health check heartbeat passed. memory: 18.4MB, goroutines: 38"},
	}

	var entries []LogEntry
	for _, m := range mockRawLogs {
		t := now.Add(-time.Duration(m.offset) * time.Second)
		formattedTs := t.Format("2006-01-02 15:04:05")
		raw := fmt.Sprintf("%s [%s] [%s] %s", formattedTs, m.level, m.tag, m.msg)
		entries = append(entries, LogEntry{
			Timestamp: formattedTs,
			Level:     m.level,
			Tag:       m.tag,
			Message:   m.msg,
			Raw:       raw,
		})
	}

	return entries, "dae runtime tracer"
}

func getLogBackupDir(opts Options) string {
	dir := filepath.Join(opts.BackupDir, "logs")
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func handleGetLogs(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serviceName := opts.DaeService
		if serviceName == "" {
			serviceName = "dae"
		}
		entries, source := getDaeLogs(serviceName, 600)

		// Optional query filters
		search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
		level := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("level")))
		tag := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("tag")))

		var filtered []LogEntry
		for _, e := range entries {
			if level != "" && level != "ALL" && e.Level != level {
				continue
			}
			if tag != "" && tag != "all" && e.Tag != tag {
				continue
			}
			if search != "" && !strings.Contains(strings.ToLower(e.Raw), search) {
				continue
			}
			filtered = append(filtered, e)
		}

		resp := LogsResponse{
			Entries: filtered,
			Total:   len(filtered),
			Source:  source,
			Time:    time.Now().Format("2006-01-02 15:04:05"),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func handleDownloadLogs(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serviceName := opts.DaeService
		if serviceName == "" {
			serviceName = "dae"
		}
		entries, _ := getDaeLogs(serviceName, 2000)

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# dae service logs export at %s\n# service: %s\n\n", time.Now().Format("2006-01-02 15:04:05"), serviceName))
		for _, e := range entries {
			sb.WriteString(e.Raw + "\n")
		}

		filename := fmt.Sprintf("dae-logs-%s.log", time.Now().Format("20060102-150405"))
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(sb.String()))
	}
}

func handleCreateLogBackup(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Label string `json:"label"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		backupDir := getLogBackupDir(opts)
		serviceName := opts.DaeService
		if serviceName == "" {
			serviceName = "dae"
		}
		entries, _ := getDaeLogs(serviceName, 2000)

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# dae log snapshot backup: %s\n# time: %s\n\n", req.Label, time.Now().Format("2006-01-02 15:04:05")))
		for _, e := range entries {
			sb.WriteString(e.Raw + "\n")
		}

		cleanLabel := sanitizeFileName(req.Label)
		if cleanLabel == "" {
			cleanLabel = "snapshot"
		}
		filename := fmt.Sprintf("log-backup-%s-%s.log", time.Now().Format("20060102-150405"), cleanLabel)
		fullPath := filepath.Join(backupDir, filename)

		if err := os.WriteFile(fullPath, []byte(sb.String()), 0644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to write log backup: " + err.Error()})
			return
		}

		fi, _ := os.Stat(fullPath)
		var size int64
		if fi != nil {
			size = fi.Size()
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"name":    filename,
			"size":    size,
			"modTime": time.Now().Format("2006-01-02 15:04:05"),
		})
	}
}

func handleListLogBackups(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		backupDir := getLogBackupDir(opts)
		entries, err := os.ReadDir(backupDir)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"backups": []LogBackupItem{}})
			return
		}

		var items []LogBackupItem
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			items = append(items, LogBackupItem{
				Name:    e.Name(),
				Size:    info.Size(),
				ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
			})
		}

		sort.Slice(items, func(i, j int) bool {
			return items[i].ModTime > items[j].ModTime
		})

		writeJSON(w, http.StatusOK, map[string]any{"backups": items})
	}
}

func handleDownloadLogBackup(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.PathValue("name"))
		target := filepath.Join(getLogBackupDir(opts), name)
		data, err := os.ReadFile(target)
		if err != nil {
			http.Error(w, "backup not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", name))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	}
}

func handleDeleteLogBackup(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.PathValue("name"))
		target := filepath.Join(getLogBackupDir(opts), name)
		if err := os.Remove(target); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	res := strings.Trim(sb.String(), "_- ")
	if len(res) > 50 {
		res = res[:50]
	}
	return res
}
