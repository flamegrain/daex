package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var webuiStartTime = time.Now()

type ServiceDetail struct {
	Name                 string         `json:"name"`
	DisplayName          string         `json:"displayName"`
	Description          string         `json:"description"`
	ActiveState          string         `json:"activeState"`   // active, inactive, failed, etc.
	SubState             string         `json:"subState"`      // running, dead, etc.
	UnitFileState        string         `json:"unitFileState"` // enabled, disabled
	MainPID              int            `json:"mainPid"`
	ActiveEnterTimestamp string         `json:"activeEnterTimestamp"`
	Uptime               string         `json:"uptime"`
	UptimeSeconds        int64          `json:"uptimeSeconds"`
	MemoryBytes          uint64         `json:"memoryBytes"`
	MemoryFormatted      string         `json:"memoryFormatted"`
	CPUTimeSeconds       float64        `json:"cpuTimeSeconds"`
	Version              string         `json:"version"`
	BinPath              string         `json:"binPath"`
	ConfigPath           string         `json:"configPath"`
	ListenAddr           string         `json:"listenAddr,omitempty"`
	Extra                map[string]any `json:"extra,omitempty"`
}

type ServicesOverviewResponse struct {
	Dae    ServiceDetail  `json:"dae"`
	WebUI  ServiceDetail  `json:"webui"`
	System map[string]any `json:"system"`
}

func getSystemdServiceProps(ctx context.Context, serviceName string) map[string]string {
	res := make(map[string]string)
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "systemctl", "show", serviceName,
		"--property=ActiveState,SubState,MainPID,ActiveEnterTimestamp,UnitFileState,MemoryCurrent,CPUUsageNSec").Output()
	if err != nil {
		return res
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			res[k] = v
		}
	}
	return res
}

func parseUptime(ts string) (string, int64) {
	if ts == "" || ts == "0" {
		return "-", 0
	}
	parts := strings.Fields(ts)
	var t time.Time
	var err error
	if len(parts) >= 3 {
		dateStr := parts[1] + " " + parts[2]
		t, err = time.ParseInLocation("2006-01-02 15:04:05", dateStr, time.Local)
	}
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
	}
	if err != nil {
		return ts, 0
	}

	d := time.Since(t)
	if d < 0 {
		return "-", 0
	}
	secs := int64(d.Seconds())
	days := secs / 86400
	hours := (secs % 86400) / 3600
	mins := (secs % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%d 天 %d 小时 %d 分钟", days, hours, mins), secs
	}
	if hours > 0 {
		return fmt.Sprintf("%d 小时 %d 分钟", hours, mins), secs
	}
	return fmt.Sprintf("%d 分钟", mins), secs
}

func formatBytesSize(bytes uint64) string {
	if bytes == 0 || bytes > (1<<60) {
		return "0 B"
	}
	const k = 1024
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(bytes)
	i := 0
	for f >= k && i < len(sizes)-1 {
		f /= k
		i++
	}
	return fmt.Sprintf("%.1f %s", f, sizes[i])
}

func detectWebuiServiceName() string {
	b, err := os.ReadFile("/proc/self/cgroup")
	if err == nil {
		s := string(b)
		if strings.Contains(s, "dae-webui-dev-api.service") {
			return "dae-webui-dev-api"
		}
		if strings.Contains(s, "dae-webui.service") {
			return "dae-webui"
		}
	}

	out, err := exec.Command("systemctl", "is-active", "dae-webui").Output()
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		return "dae-webui"
	}
	out, err = exec.Command("systemctl", "is-active", "dae-webui-dev-api").Output()
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		return "dae-webui-dev-api"
	}
	return "dae-webui"
}

func getDaeServiceDetail(ctx context.Context, o Options) ServiceDetail {
	props := getSystemdServiceProps(ctx, o.DaeService)

	pid, _ := strconv.Atoi(props["MainPID"])
	mem, _ := strconv.ParseUint(props["MemoryCurrent"], 10, 64)
	cpuNS, _ := strconv.ParseUint(props["CPUUsageNSec"], 10, 64)
	cpuSec := float64(cpuNS) / 1e9

	var verStr string
	if out, err := exec.CommandContext(ctx, o.DaeBin, "--version").Output(); err == nil {
		verStr = strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	}

	uptimeStr, uptimeSecs := parseUptime(props["ActiveEnterTimestamp"])

	activeState := props["ActiveState"]
	if activeState == "" {
		activeState = "unknown"
	}

	subState := props["SubState"]
	if subState == "" {
		subState = "unknown"
	}

	unitState := props["UnitFileState"]
	if unitState == "" {
		unitState = "unknown"
	}

	return ServiceDetail{
		Name:                 o.DaeService,
		DisplayName:          "dae 核心服务",
		Description:          "基于 eBPF 的高性能透明网络代理与流量分流内核引擎",
		ActiveState:          activeState,
		SubState:             subState,
		UnitFileState:        unitState,
		MainPID:              pid,
		ActiveEnterTimestamp: props["ActiveEnterTimestamp"],
		Uptime:               uptimeStr,
		UptimeSeconds:        uptimeSecs,
		MemoryBytes:          mem,
		MemoryFormatted:      formatBytesSize(mem),
		CPUTimeSeconds:       cpuSec,
		Version:              verStr,
		BinPath:              o.DaeBin,
		ConfigPath:           o.DaeConfig,
	}
}

func getWebuiServiceDetail(ctx context.Context, o Options) ServiceDetail {
	webuiSvc := detectWebuiServiceName()
	props := getSystemdServiceProps(ctx, webuiSvc)

	pid, _ := strconv.Atoi(props["MainPID"])
	if pid == 0 {
		pid = os.Getpid()
	}
	mem, _ := strconv.ParseUint(props["MemoryCurrent"], 10, 64)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	if mem == 0 {
		mem = m.Sys
	}

	cpuNS, _ := strconv.ParseUint(props["CPUUsageNSec"], 10, 64)
	cpuSec := float64(cpuNS) / 1e9

	uptimeStr, uptimeSecs := parseUptime(props["ActiveEnterTimestamp"])
	if uptimeSecs == 0 {
		d := time.Since(webuiStartTime)
		secs := int64(d.Seconds())
		uptimeSecs = secs
		days := secs / 86400
		hours := (secs % 86400) / 3600
		mins := (secs % 3600) / 60
		if days > 0 {
			uptimeStr = fmt.Sprintf("%d 天 %d 小时 %d 分钟", days, hours, mins)
		} else if hours > 0 {
			uptimeStr = fmt.Sprintf("%d 小时 %d 分钟", hours, mins)
		} else {
			uptimeStr = fmt.Sprintf("%d 分钟", mins)
		}
	}

	activeState := props["ActiveState"]
	if activeState == "" {
		activeState = "active"
	}
	subState := props["SubState"]
	if subState == "" {
		subState = "running"
	}
	unitFileState := props["UnitFileState"]
	if unitFileState == "" {
		unitFileState = "enabled"
	}

	host, _ := os.Hostname()

	return ServiceDetail{
		Name:                 webuiSvc,
		DisplayName:          "dae-webui 控制台服务",
		Description:          "可视化网络流量管控大盘、策略节点调度、路由分流管理及诊断服务",
		ActiveState:          activeState,
		SubState:             subState,
		UnitFileState:        unitFileState,
		MainPID:              pid,
		ActiveEnterTimestamp: props["ActiveEnterTimestamp"],
		Uptime:               uptimeStr,
		UptimeSeconds:        uptimeSecs,
		MemoryBytes:          mem,
		MemoryFormatted:      formatBytesSize(mem),
		CPUTimeSeconds:       cpuSec,
		Version:              version,
		BinPath:              "/usr/local/bin/dae-webui",
		ConfigPath:           o.DaeConfig,
		ListenAddr:           o.Listen,
		Extra: map[string]any{
			"goVersion":    runtime.Version(),
			"arch":         runtime.GOARCH,
			"os":           runtime.GOOS,
			"hostname":     host,
			"numGoroutine": runtime.NumGoroutine(),
			"memAlloc":     formatBytesSize(m.Alloc),
			"sysMem":       formatBytesSize(m.Sys),
		},
	}
}

func handleGetServicesOverview(o Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		daeDetail := getDaeServiceDetail(ctx, o)
		webuiDetail := getWebuiServiceDetail(ctx, o)

		host, _ := os.Hostname()
		sysInfo := map[string]any{
			"hostname":    host,
			"serverTime":  time.Now().Format("2006-01-02 15:04:05"),
			"numCPU":      runtime.NumCPU(),
			"goVersion":   runtime.Version(),
			"os":          runtime.GOOS,
			"arch":        runtime.GOARCH,
		}

		writeJSON(w, http.StatusOK, ServicesOverviewResponse{
			Dae:    daeDetail,
			WebUI:  webuiDetail,
			System: sysInfo,
		})
	}
}

func handleServiceAction(o Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Target string `json:"target"` // "dae" or "webui"
			Action string `json:"action"` // "start", "stop", "restart", "reload", "enable", "disable"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json payload: " + err.Error()})
			return
		}

		req.Target = strings.ToLower(strings.TrimSpace(req.Target))
		req.Action = strings.ToLower(strings.TrimSpace(req.Action))

		if req.Target != "dae" && req.Target != "webui" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "target must be 'dae' or 'webui'"})
			return
		}

		switch req.Action {
		case "start", "stop", "restart", "reload", "enable", "disable":
		default:
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported action: " + req.Action})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var serviceName string
		if req.Target == "dae" {
			serviceName = o.DaeService
		} else {
			serviceName = detectWebuiServiceName()
		}

		// If restarting webui, respond first then trigger in background
		if req.Target == "webui" && req.Action == "restart" {
			log.Printf("[ServiceAction] restarting webui service [%s] in 200ms...", serviceName)
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":      true,
				"message": fmt.Sprintf("WebUI 服务 [%s] 正在重启中，系统将自动重新连接...", serviceName),
			})
			go func() {
				time.Sleep(200 * time.Millisecond)
				_ = exec.Command("systemctl", "restart", serviceName).Run()
			}()
			return
		}

		// Execute systemctl command
		out, err := exec.CommandContext(ctx, "systemctl", req.Action, serviceName).CombinedOutput()
		outStr := strings.TrimSpace(string(out))
		if err != nil {
			log.Printf("[ServiceAction] systemctl %s %s failed: %v, output: %s", req.Action, serviceName, err, outStr)
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":   fmt.Sprintf("执行 systemctl %s %s 失败: %v", req.Action, serviceName, err),
				"details": outStr,
			})
			return
		}

		log.Printf("[ServiceAction] successfully executed systemctl %s %s", req.Action, serviceName)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"target":  req.Target,
			"action":  req.Action,
			"message": fmt.Sprintf("服务 [%s] 操作 [%s] 执行成功", serviceName, req.Action),
			"output":  outStr,
		})
	}
}

func handleGetServiceLogs(o Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("target")))
		linesStr := r.URL.Query().Get("lines")
		lines := 40
		if l, err := strconv.Atoi(linesStr); err == nil && l > 0 && l <= 500 {
			lines = l
		}

		var serviceName string
		if target == "webui" {
			serviceName = detectWebuiServiceName()
		} else {
			serviceName = o.DaeService
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		out, err := exec.CommandContext(ctx, "journalctl", "-u", serviceName, "-n", strconv.Itoa(lines), "--no-pager").CombinedOutput()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":   fmt.Sprintf("读取服务日志失败: %v", err),
				"details": strings.TrimSpace(string(out)),
			})
			return
		}

		rawLines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var logLines []string
		for _, line := range rawLines {
			line = strings.TrimSpace(line)
			if line != "" {
				logLines = append(logLines, line)
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"target":      target,
			"serviceName": serviceName,
			"lines":       len(logLines),
			"logs":        logLines,
		})
	}
}
