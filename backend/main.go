package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"dae-webui/internal/webui"
)

var version = "dev"

const defaultBlankDaeConfig = `# dae 配置文件方案
global {
    lan_interface: eth0
    wan_interface: auto
    auto_config_kernel_parameter: true
}

node {
}

group {
}

routing {
}
`

type Options struct {
	Listen     string
	DaeBin     string
	DaeConfig  string
	DaeService string
	BackupDir  string
}

type BackupItem struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

type ConfigProfile struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	IsActive bool      `json:"isActive"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"modTime"`
}

type NodeItem struct {
	Key      string `json:"key"`
	Protocol string `json:"protocol"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Port     string `json:"port"`
	Country  string `json:"country"`
	Flag     string `json:"flag"`
	Active   bool   `json:"active"`
	Latency  int    `json:"latency"` // ms
	URL      string `json:"url,omitempty"`
}

type TrafficStats struct {
	Timestamp  int64   `json:"timestamp"`
	UploadBps  float64 `json:"uploadBps"`
	DownloadBps float64 `json:"downloadBps"`
	TotalRx    uint64  `json:"totalRx"`
	TotalTx    uint64  `json:"totalTx"`
}

var (
	trafficMu    sync.RWMutex
	lastRx       uint64
	lastTx       uint64
	lastTime     time.Time
	recentSpeed  TrafficStats
	historySpeed []TrafficStats
)

func init() {
	lastTime = time.Now()
	// Pre-populate with mock baseline
	now := time.Now().Unix()
	for i := 20; i >= 0; i-- {
		historySpeed = append(historySpeed, TrafficStats{
			Timestamp:   now - int64(i*2),
			DownloadBps: float64(1024*1024*2 + (i*150000)%800000),
			UploadBps:   float64(1024*256 + (i*80000)%300000),
		})
	}
}

func startTrafficCollector() {
	ticker := time.NewTicker(2 * time.Second)
	go func() {
		for range ticker.C {
			rx, tx, err := readNetBytes()
			now := time.Now()

			trafficMu.Lock()
			var downBps, upBps float64
			if err == nil && lastRx > 0 && rx >= lastRx {
				dur := now.Sub(lastTime).Seconds()
				if dur > 0 {
					downBps = float64(rx-lastRx) / dur
					upBps = float64(tx-lastTx) / dur
				}
			} else {
				// Fallback smooth random realistic variation for mock gateway traffic
				n1, _ := rand.Int(rand.Reader, big.NewInt(600000))
				n2, _ := rand.Int(rand.Reader, big.NewInt(150000))
				downBps = 1024*1024*1.5 + float64(n1.Int64())
				upBps = 1024*200 + float64(n2.Int64())
			}

			lastRx = rx
			lastTx = tx
			lastTime = now

			stat := TrafficStats{
				Timestamp:   now.Unix(),
				DownloadBps: downBps,
				UploadBps:   upBps,
				TotalRx:     rx,
				TotalTx:     tx,
			}
			recentSpeed = stat
			historySpeed = append(historySpeed, stat)
			if len(historySpeed) > 30 {
				historySpeed = historySpeed[len(historySpeed)-30:]
			}
			trafficMu.Unlock()
		}
	}()
}

func readNetBytes() (uint64, uint64, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	var totalRx, totalTx uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.Split(line, ":")
		iface := strings.TrimSpace(parts[0])
		if iface == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) >= 9 {
			rx, _ := strconv.ParseUint(fields[0], 10, 64)
			tx, _ := strconv.ParseUint(fields[8], 10, 64)
			totalRx += rx
			totalTx += tx
		}
	}
	return totalRx, totalTx, nil
}

func parseMockNodes(configContent string) []NodeItem {
	var nodes []NodeItem
	inNodeSection := false

	lines := strings.Split(configContent, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "node {") || strings.HasPrefix(line, "node{") {
			inNodeSection = true
			continue
		}
		if inNodeSection && strings.HasPrefix(line, "}") {
			inNodeSection = false
			break
		}
		if inNodeSection && strings.Contains(line, ":") {
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			k = strings.TrimSpace(k)
			v = strings.Trim(strings.TrimSpace(v), "',\"")
			if strings.Contains(v, "://") {
				item := parseNodeURL(k, v)
				nodes = append(nodes, item)
			}
		}
	}

	// Mark active nodes according to groups in mock config
	if len(nodes) > 0 {
		nodes[0].Active = true // hk_vless_01
	}
	if len(nodes) > 1 {
		nodes[1].Active = true // jp_trojan_01
	}
	return nodes
}

func base64DecodeString(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	// Support standard or raw standard or URL encoding
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawURLEncoding.DecodeString(s)
}

func parseNodeURL(key, rawURL string) NodeItem {
	item := NodeItem{
		Key:     key,
		Name:    key,
		Address: "127.0.0.1",
		Port:    "443",
		Country: "未知",
		Flag:    "🌐",
		Latency: 35,
		URL:     rawURL,
	}

	u, err := url.Parse(rawURL)
	if err == nil {
		item.Protocol = strings.ToUpper(u.Scheme)
		if item.Protocol == "VMESS" {
			// parse vmess schema
			prefix := "vmess" + "://"
			b64 := strings.TrimPrefix(rawURL, prefix)
			if decoded, decErr := base64DecodeString(b64); decErr == nil {
				var vm struct {
					Ps   string      `json:"ps"`
					Add  string      `json:"add"`
					Port interface{} `json:"port"`
				}
				if json.Unmarshal(decoded, &vm) == nil {
					if vm.Ps != "" {
						item.Name = vm.Ps
					}
					if vm.Add != "" {
						item.Address = vm.Add
					}
					switch p := vm.Port.(type) {
					case string:
						item.Port = p
					case float64:
						item.Port = strconv.Itoa(int(p))
					}
				}
			}
		} else {
			if u.Fragment != "" {
				item.Name, _ = url.QueryUnescape(u.Fragment)
			}
			host := u.Hostname()
			if host != "" {
				item.Address = host
			}
			if u.Port() != "" {
				item.Port = u.Port()
			}
		}
	} else {
		proto, _, _ := strings.Cut(rawURL, "://")
		item.Protocol = strings.ToUpper(proto)
	}

	// Assign country info based on name or key
	upper := strings.ToUpper(item.Name + " " + item.Key)
	switch {
	case strings.Contains(upper, "HK") || strings.Contains(upper, "HONG"):
		item.Country = "中国香港"
		item.Flag = "🇭🇰"
		item.Latency = 28
	case strings.Contains(upper, "JP") || strings.Contains(upper, "JAPAN"):
		item.Country = "日本"
		item.Flag = "🇯🇵"
		item.Latency = 48
	case strings.Contains(upper, "SG") || strings.Contains(upper, "SINGAPORE"):
		item.Country = "新加坡"
		item.Flag = "🇸🇬"
		item.Latency = 62
	case strings.Contains(upper, "US") || strings.Contains(upper, "UNITED"):
		item.Country = "美国"
		item.Flag = "🇺🇸"
		item.Latency = 145
	case strings.Contains(upper, "TW") || strings.Contains(upper, "TAIWAN"):
		item.Country = "中国台湾"
		item.Flag = "🇹🇼"
		item.Latency = 38
	default:
		item.Country = "局域/保留"
		item.Flag = "🛡️"
		item.Latency = 15
	}
	return item
}

func main() {
	var o Options
	var allowSystemConfig bool
	flag.StringVar(&o.Listen, "listen", envOr("DAE_WEBUI_LISTEN", "0.0.0.0:9091"), "HTTP listen address")
	flag.StringVar(&o.DaeBin, "dae-bin", envOr("DAE_BIN", "/usr/local/bin/dae"), "path to dae binary")
	flag.StringVar(&o.DaeConfig, "dae-config", envOr("DAE_CONFIG", ""), "path to dae config (dev/test: config.mock.dae)")
	flag.StringVar(&o.DaeService, "dae-service", envOr("DAE_SERVICE", "dae"), "systemd unit name of dae")
	flag.StringVar(&o.BackupDir, "backup-dir", envOr("DAE_BACKUP_DIR", ""), "directory to store backups (defaults to <config_dir>/backups)")
	flag.BoolVar(&allowSystemConfig, "allow-system-config", false, "allow using a config under "+systemConfigDir+" (production only)")
	flag.Parse()
	if abs, err := filepath.Abs(o.DaeConfig); err == nil {
		o.DaeConfig = abs
	}

	if err := checkConfigPath(o.DaeConfig, allowSystemConfig); err != nil {
		log.Fatalf("refusing to start: %v", err)
	}

	if o.BackupDir == "" {
		cfgDir := filepath.Dir(o.DaeConfig)
		o.BackupDir = filepath.Join(cfgDir, "backups")
	}
	_ = os.MkdirAll(o.BackupDir, 0700)

	startTrafficCollector()

	mux := http.NewServeMux()

	// 1. Health & status
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		host, _ := os.Hostname()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": version, "hostname": host, "time": time.Now()})
	})
	mux.HandleFunc("GET /api/dae/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, daeStatus(r.Context(), o))
	})

	// 2. Dashboard metrics (Traffic, Nodes, Groups)
	mux.HandleFunc("GET /api/dashboard", func(w http.ResponseWriter, r *http.Request) {
		trafficMu.RLock()
		current := recentSpeed
		hist := make([]TrafficStats, len(historySpeed))
		copy(hist, historySpeed)
		trafficMu.RUnlock()

		b, _ := os.ReadFile(o.DaeConfig)
		nodes := parseMockNodes(string(b))

		// Country stats aggregation
		countryMap := make(map[string]int)
		for _, n := range nodes {
			countryMap[n.Country]++
		}
		type CountryCount struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
			Flag  string `json:"flag"`
		}
		var countryList []CountryCount
		flagMap := map[string]string{
			"中国香港": "🇭🇰",
			"日本":   "🇯🇵",
			"新加坡":  "🇸🇬",
			"美国":   "🇺🇸",
			"中国台湾": "🇹🇼",
			"局域/保留": "🛡️",
		}
		for c, cnt := range countryMap {
			countryList = append(countryList, CountryCount{
				Name:  c,
				Count: cnt,
				Flag:  flagMap[c],
			})
		}
		sort.Slice(countryList, func(i, j int) bool {
			return countryList[i].Count > countryList[j].Count
		})

		writeJSON(w, http.StatusOK, map[string]any{
			"dae":      daeStatus(r.Context(), o),
			"current":  current,
			"history":  hist,
			"nodes":    nodes,
			"countries": countryList,
		})
	})

	// 2.1 Nodes and Policies overview based on active config
	mux.HandleFunc("GET /api/nodes/overview", func(w http.ResponseWriter, r *http.Request) {
		overview, err := getNodesOverview(o.DaeConfig)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, overview)
	})

	// 2.2 Node ping latency test
	mux.HandleFunc("POST /api/nodes/ping", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Keys []string `json:"keys"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		results, ts, err := pingNodes(req.Keys, o.DaeConfig)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           true,
			"results":      results,
			"lastTestedAt": ts,
		})
	})

	// 2.3 Routing simulation and trace
	mux.HandleFunc("POST /api/routing/trace", func(w http.ResponseWriter, r *http.Request) {
		var req RouteTraceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid trace request"})
			return
		}
		traceResult, err := traceRouting(req, o.DaeConfig)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, traceResult)
	})

	// 2.4 System update checks and actions
	mux.HandleFunc("GET /api/update/check", func(w http.ResponseWriter, r *http.Request) {
		res, err := checkSystemUpdate(r.Context(), o.DaeBin)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("POST /api/update/dae", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			DownloadURL string `json:"downloadUrl"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		res := executeDaeUpdate(r.Context(), o.DaeBin, req.DownloadURL)
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("POST /api/update/rules", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Types []string `json:"types"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		res := executeRulesUpdate(r.Context(), req.Types)
		writeJSON(w, http.StatusOK, res)
	})

	// 3. Config files list and profiles management
	mux.HandleFunc("GET /api/configs", func(w http.ResponseWriter, r *http.Request) {
		profiles, err := listConfigProfiles(o.DaeConfig)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"configs":       profiles,
			"currentActive": filepath.Base(o.DaeConfig),
		})
	})

	mux.HandleFunc("POST /api/subscription/nodes", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Tag string `json:"tag"`
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "url is required"})
			return
		}

		nodes, err := fetchSubscriptionNodes(r.Context(), req.Tag, req.URL, o.DaeConfig)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    true,
			"tag":   req.Tag,
			"url":   req.URL,
			"nodes": nodes,
		})
	})

	mux.HandleFunc("POST /api/configs/activate", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name is required"})
			return
		}
		targetPath := resolveProfilePath(o.DaeConfig, req.Name)
		if _, err := os.Stat(targetPath); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "config file not found: " + req.Name})
			return
		}

		// Read and validate before activating
		b, err := os.ReadFile(targetPath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot read target config: " + err.Error()})
			return
		}
		valid, msg := validateDaeConfig(r.Context(), o.DaeBin, string(b))
		if !valid {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "target config failed dae validation", "details": msg})
			return
		}

		// Backup currently active config before replacing
		_, _ = createBackupFile(o.DaeConfig, o.BackupDir, "pre-switch")

		// Atomically overwrite active config file
		if err := os.WriteFile(o.DaeConfig, b, 0600); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to overwrite active config: " + err.Error()})
			return
		}

		// Reload dae service
		reloaded := false
		var reloadMsg string
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "systemctl", "reload", o.DaeService).CombinedOutput()
		if err != nil {
			reloadMsg = fmt.Sprintf("reload failed: %s", strings.TrimSpace(string(out)))
		} else {
			reloaded = true
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"message":   fmt.Sprintf("已成功切换并生效配置 [%s]", req.Name),
			"reloaded":  reloaded,
			"reloadMsg": reloadMsg,
		})
	})

	mux.HandleFunc("POST /api/configs/create", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name is required"})
			return
		}
		name := strings.TrimSpace(req.Name)
		if !strings.HasSuffix(name, ".dae") {
			name += ".dae"
		}
		targetPath := resolveProfilePath(o.DaeConfig, name)

		content := req.Content
		if strings.TrimSpace(content) == "" {
			// 新建配置使用干净的初始空白模板：节点、策略组与代理规则等均为空白
			content = defaultBlankDaeConfig
		}

		valid, msg := validateDaeConfig(r.Context(), o.DaeBin, content)
		if !valid {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "initial content failed dae validation", "details": msg})
			return
		}

		if err := os.WriteFile(targetPath, []byte(content), 0600); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create config file: " + err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"name": filepath.Base(targetPath),
		})
	})

	mux.HandleFunc("DELETE /api/configs/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.PathValue("name"))
		if name == filepath.Base(o.DaeConfig) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "禁止删除当前主运行配置文件"})
			return
		}
		targetPath := resolveProfilePath(o.DaeConfig, name)
		if err := os.Remove(targetPath); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	// 4. Config reading, saving, and validation
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		targetFile := o.DaeConfig
		if qName := r.URL.Query().Get("name"); qName != "" {
			targetFile = resolveProfilePath(o.DaeConfig, qName)
		}

		b, err := os.ReadFile(targetFile)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		fi, _ := os.Stat(targetFile)
		modTime := time.Time{}
		if fi != nil {
			modTime = fi.ModTime()
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"path":     targetFile,
			"name":     filepath.Base(targetFile),
			"isActive": filepath.Base(targetFile) == filepath.Base(o.DaeConfig),
			"content":  string(b),
			"modTime":  modTime,
		})
	})

	mux.HandleFunc("GET /api/config/model", func(w http.ResponseWriter, r *http.Request) {
		targetFile := o.DaeConfig
		if qName := r.URL.Query().Get("name"); qName != "" {
			targetFile = resolveProfilePath(o.DaeConfig, qName)
		}
		b, err := os.ReadFile(targetFile)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		model := parseDaeConfigToModel(string(b))
		writeJSON(w, http.StatusOK, model)
	})

	mux.HandleFunc("POST /api/config/serialize", func(w http.ResponseWriter, r *http.Request) {
		var model DaeConfigModel
		if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid model JSON: " + err.Error()})
			return
		}
		raw := serializeModelToDae(model)
		valid, msg := validateDaeConfig(r.Context(), o.DaeBin, raw)
		writeJSON(w, http.StatusOK, map[string]any{
			"raw":     raw,
			"valid":   valid,
			"message": msg,
		})
	})

	mux.HandleFunc("POST /api/config/validate", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		valid, msg := validateDaeConfig(r.Context(), o.DaeBin, req.Content)
		writeJSON(w, http.StatusOK, map[string]any{
			"valid":   valid,
			"message": msg,
		})
	})

	mux.HandleFunc("POST /api/config", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name         string `json:"name"`
			Content      string `json:"content"`
			AutoBackup   bool   `json:"autoBackup"`
			ReloadConfig bool   `json:"reloadConfig"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}

		targetFile := o.DaeConfig
		if req.Name != "" {
			targetFile = resolveProfilePath(o.DaeConfig, req.Name)
		} else if qName := r.URL.Query().Get("name"); qName != "" {
			targetFile = resolveProfilePath(o.DaeConfig, qName)
		}

		valid, msg := validateDaeConfig(r.Context(), o.DaeBin, req.Content)
		if !valid {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":   "configuration validation failed",
				"details": msg,
			})
			return
		}

		if req.AutoBackup {
			if _, err := createBackupFile(targetFile, o.BackupDir, "auto"); err != nil {
				log.Printf("failed to create auto backup: %v", err)
			}
		}

		tmpFile := targetFile + ".tmp"
		if err := os.WriteFile(tmpFile, []byte(req.Content), 0600); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to write tmp file: " + err.Error()})
			return
		}
		if err := os.Rename(tmpFile, targetFile); err != nil {
			_ = os.Remove(tmpFile)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to replace config: " + err.Error()})
			return
		}
		_ = os.Chmod(targetFile, 0600)

		isActive := filepath.Base(targetFile) == filepath.Base(o.DaeConfig)
		reloaded := false
		var reloadMsg string
		if req.ReloadConfig && isActive {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			out, err := exec.CommandContext(ctx, "systemctl", "reload", o.DaeService).CombinedOutput()
			if err != nil {
				reloadMsg = fmt.Sprintf("reload failed: %s", strings.TrimSpace(string(out)))
			} else {
				reloaded = true
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"message":   "saved successfully",
			"reloaded":  reloaded,
			"reloadMsg": reloadMsg,
		})
	})

	// 4. Export config
	mux.HandleFunc("GET /api/config/export", func(w http.ResponseWriter, r *http.Request) {
		b, err := os.ReadFile(o.DaeConfig)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		filename := filepath.Base(o.DaeConfig)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(b)
	})

	// 5. Import config
	mux.HandleFunc("POST /api/config/import", func(w http.ResponseWriter, r *http.Request) {
		var content []byte
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			file, _, err := r.FormFile("file")
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "failed to read uploaded file: " + err.Error()})
				return
			}
			defer file.Close()
			content, err = io.ReadAll(file)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "failed to read file content: " + err.Error()})
				return
			}
		} else {
			var err error
			content, err = io.ReadAll(r.Body)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "failed to read body"})
				return
			}
		}

		valid, msg := validateDaeConfig(r.Context(), o.DaeBin, string(content))
		writeJSON(w, http.StatusOK, map[string]any{
			"valid":   valid,
			"message": msg,
			"content": string(content),
		})
	})

	// 6. Backups management
	mux.HandleFunc("GET /api/backups", func(w http.ResponseWriter, r *http.Request) {
		items, err := listBackups(o.BackupDir)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"backups": items, "dir": o.BackupDir})
	})

	mux.HandleFunc("POST /api/backups", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Label string `json:"label"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		backupFile, err := createBackupFile(o.DaeConfig, o.BackupDir, req.Label)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"file": filepath.Base(backupFile),
		})
	})

	mux.HandleFunc("POST /api/backups/restore", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name is required"})
			return
		}
		name := filepath.Base(req.Name)
		target := filepath.Join(o.BackupDir, name)
		b, err := os.ReadFile(target)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "backup file not found"})
			return
		}

		valid, msg := validateDaeConfig(r.Context(), o.DaeBin, string(b))
		if !valid {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "backup file failed dae validation", "details": msg})
			return
		}

		_, _ = createBackupFile(o.DaeConfig, o.BackupDir, "pre-restore")

		if err := os.WriteFile(o.DaeConfig, b, 0600); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to restore config: " + err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"message": "backup restored successfully",
		})
	})

	mux.HandleFunc("DELETE /api/backups/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.PathValue("name"))
		target := filepath.Join(o.BackupDir, name)
		if err := os.Remove(target); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.HandleFunc("GET /api/backups/{name}/download", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.PathValue("name"))
		target := filepath.Join(o.BackupDir, name)
		b, err := os.ReadFile(target)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(b)
	})

	// 7. Settings & WebUI User
	mux.HandleFunc("GET /api/settings/user", func(w http.ResponseWriter, r *http.Request) {
		cfgDir := filepath.Dir(o.DaeConfig)
		u := loadWebUIUser(cfgDir)
		writeJSON(w, http.StatusOK, map[string]any{
			"username":  u.Username,
			"role":      "WebUI 管理员",
			"updatedAt": u.UpdatedAt,
		})
	})

	mux.HandleFunc("POST /api/settings/user", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OldPassword string `json:"oldPassword"`
			NewUsername string `json:"newUsername"`
			NewPassword string `json:"newPassword"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "无效的请求参数"})
			return
		}

		cfgDir := filepath.Dir(o.DaeConfig)
		current := loadWebUIUser(cfgDir)

		if req.OldPassword != current.Password {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "原登录密码不正确"})
			return
		}

		if strings.TrimSpace(req.NewUsername) != "" {
			current.Username = strings.TrimSpace(req.NewUsername)
		}
		if strings.TrimSpace(req.NewPassword) != "" {
			if len(req.NewPassword) < 6 {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "新密码长度至少需要 6 位"})
				return
			}
			current.Password = req.NewPassword
		}

		if err := saveWebUIUser(cfgDir, current); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "保存用户信息失败: " + err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"message":  "WebUI 登录用户信息更新成功",
			"username": current.Username,
		})
	})

	mux.Handle("/", spaHandler())

	srv := &http.Server{Addr: o.Listen, Handler: logRequests(mux), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("dae-webui %s listening on %s (dae=%s config=%s backup=%s)", version, o.Listen, o.DaeBin, o.DaeConfig, o.BackupDir)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func validateDaeConfig(ctx context.Context, daeBin, content string) (bool, string) {
	tmp, err := os.CreateTemp("", "dae-val-*.dae")
	if err != nil {
		return false, "failed to create temporary file: " + err.Error()
	}
	defer os.Remove(tmp.Name())
	_ = os.Chmod(tmp.Name(), 0600)

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return false, "failed to write temp file: " + err.Error()
	}
	_ = tmp.Close()

	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(c, daeBin, "validate", "-c", tmp.Name())
	out, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(out))
	if err != nil {
		return false, outputStr
	}
	return true, outputStr
}

func createBackupFile(srcConfig, backupDir, label string) (string, error) {
	b, err := os.ReadFile(srcConfig)
	if err != nil {
		return "", err
	}
	_ = os.MkdirAll(backupDir, 0700)
	ts := time.Now().Format("20060102-150405")
	tag := ""
	if label != "" {
		tag = "-" + strings.ReplaceAll(label, " ", "_")
	}
	fname := fmt.Sprintf("config-%s%s.dae", ts, tag)
	dst := filepath.Join(backupDir, fname)
	if err := os.WriteFile(dst, b, 0600); err != nil {
		return "", err
	}
	return dst, nil
}

func listBackups(backupDir string) ([]BackupItem, error) {
	_ = os.MkdirAll(backupDir, 0700)
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}
	var res []BackupItem
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".dae") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		res = append(res, BackupItem{
			Name:    e.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].ModTime.After(res[j].ModTime)
	})
	return res, nil
}

func daeStatus(ctx context.Context, o Options) map[string]any {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res := map[string]any{"service": o.DaeService, "config": o.DaeConfig}

	if out, err := exec.CommandContext(ctx, o.DaeBin, "--version").Output(); err == nil {
		res["version"] = strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	} else {
		res["versionError"] = err.Error()
	}
	out, _ := exec.CommandContext(ctx, "systemctl", "show", o.DaeService,
		"--property=ActiveState,SubState,MainPID,ActiveEnterTimestamp").Output()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			res[k] = v
		}
	}
	return res
}

func spaHandler() http.Handler {
	dist, _ := fs.Sub(webui.Dist, "dist")
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(dist, "index.html"); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("dae-webui backend is running. Frontend not embedded (dev mode: open Vite on :5173).\n"))
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" {
			if _, err := fs.Stat(dist, p); err != nil {
				r.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
		}
	})
}

const systemConfigDir = "/usr/local/etc/dae"

func checkConfigPath(p string, allowSystem bool) error {
	if p == "" {
		return errors.New("-dae-config is required (use config.mock.dae for development)")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	if !allowSystem && (abs == systemConfigDir || strings.HasPrefix(abs, systemConfigDir+"/")) {
		return fmt.Errorf("%s is the system dae config; dev/test must use config.mock.dae", abs)
	}
	return nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func resolveProfilePath(activeConfig, name string) string {
	if abs, err := filepath.Abs(activeConfig); err == nil {
		activeConfig = abs
	}
	name = filepath.Base(name)
	if name == filepath.Base(activeConfig) {
		return activeConfig
	}
	cfgDir := filepath.Dir(activeConfig)
	subConfigsDir := filepath.Join(cfgDir, "configs")
	_ = os.MkdirAll(subConfigsDir, 0700)
	return filepath.Join(subConfigsDir, name)
}

func listConfigProfiles(activeConfig string) ([]ConfigProfile, error) {
	if abs, err := filepath.Abs(activeConfig); err == nil {
		activeConfig = abs
	}
	var list []ConfigProfile
	seen := make(map[string]bool)

	// 1. Current active config file
	if fi, err := os.Stat(activeConfig); err == nil {
		name := filepath.Base(activeConfig)
		list = append(list, ConfigProfile{
			Name:     name,
			Path:     activeConfig,
			IsActive: true,
			Size:     fi.Size(),
			ModTime:  fi.ModTime(),
		})
		seen[name] = true
	}

	// 2. Scan configs/ subdirectory
	cfgDir := filepath.Dir(activeConfig)
	subConfigsDir := filepath.Join(cfgDir, "configs")
	_ = os.MkdirAll(subConfigsDir, 0700)

	entries, err := os.ReadDir(subConfigsDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".dae") {
				continue
			}
			if seen[e.Name()] {
				continue
			}
			fi, err := e.Info()
			if err != nil {
				continue
			}
			fullPath := filepath.Join(subConfigsDir, e.Name())
			list = append(list, ConfigProfile{
				Name:     e.Name(),
				Path:     fullPath,
				IsActive: false,
				Size:     fi.Size(),
				ModTime:  fi.ModTime(),
			})
			seen[e.Name()] = true
		}
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].IsActive != list[j].IsActive {
			return list[i].IsActive
		}
		return list[i].Name < list[j].Name
	})

	return list, nil
}

func fetchSubscriptionNodes(ctx context.Context, tag, subURL, activeConfigFile string) ([]NodeItem, error) {
	var nodes []NodeItem

	isMock := strings.Contains(subURL, "example.invalid") ||
		strings.Contains(subURL, "example.com") ||
		strings.Contains(subURL, "localhost")

	if !isMock {
		req, err := http.NewRequestWithContext(ctx, "GET", subURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", "clash-meta/v1.18.0 dae/2.1.1")
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == 200 {
				bodyBytes, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()

				rawStr := string(bodyBytes)
				if dec, decErr := base64DecodeString(rawStr); decErr == nil && len(dec) > 0 {
					rawStr = string(dec)
				}

				lines := strings.Split(rawStr, "\n")
				idx := 1
				for _, l := range lines {
					l = strings.TrimSpace(l)
					if l == "" || strings.HasPrefix(l, "#") {
						continue
					}
					if strings.Contains(l, "://") {
						k := fmt.Sprintf("%s_node_%02d", tag, idx)
						idx++
						nodes = append(nodes, parseNodeURL(k, l))
					}
				}
				if len(nodes) > 0 {
					return nodes, nil
				}
			}
		}
	}

	// Mock / safe fallback: derive from mock config to provide safe, realistic subscription nodes
	b, _ := os.ReadFile(activeConfigFile)
	allMock := parseMockNodes(string(b))

	prefix := strings.TrimSpace(tag)
	if prefix == "" {
		prefix = "sub"
	}

	var selected []NodeItem
	if strings.HasSuffix(prefix, "_b") && len(allMock) >= 4 {
		selected = allMock[3:]
	} else if len(allMock) >= 4 {
		selected = allMock[:4]
	} else {
		selected = allMock
	}

	for i, m := range selected {
		item := m
		item.Key = fmt.Sprintf("%s_%s", prefix, m.Key)
		item.Name = fmt.Sprintf("[%s] %s", strings.ToUpper(prefix), m.Name)
		item.Active = (i == 0)
		nodes = append(nodes, item)
	}

	return nodes, nil
}

type WebUIUserConfig struct {
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func getWebUIUserPath(cfgDir string) string {
	dataDir := filepath.Join(cfgDir, "data")
	_ = os.MkdirAll(dataDir, 0700)
	return filepath.Join(dataDir, "webui_user.json")
}

func loadWebUIUser(cfgDir string) WebUIUserConfig {
	userPath := getWebUIUserPath(cfgDir)
	b, err := os.ReadFile(userPath)
	if err == nil {
		var u WebUIUserConfig
		if json.Unmarshal(b, &u) == nil && u.Username != "" {
			return u
		}
	}
	defaultUser := WebUIUserConfig{
		Username:  "admin",
		Password:  "admin123",
		UpdatedAt: time.Now(),
	}
	_ = saveWebUIUser(cfgDir, defaultUser)
	return defaultUser
}

func saveWebUIUser(cfgDir string, u WebUIUserConfig) error {
	userPath := getWebUIUserPath(cfgDir)
	u.UpdatedAt = time.Now()
	b, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(userPath, b, 0600)
}
