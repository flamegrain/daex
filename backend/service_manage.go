package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

type DoctorCheckItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"` // "pass", "warn", "fail"
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

type DoctorReport struct {
	Timestamp string            `json:"timestamp"`
	Status    string            `json:"status"` // "healthy", "warning", "error"
	Summary   map[string]int    `json:"summary"`
	Checks    []DoctorCheckItem `json:"checks"`
}

func handleGetSystemDoctor(o Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()

		var checks []DoctorCheckItem
		passed := 0
		warnings := 0
		failed := 0

		addCheck := func(id, name, status, msg, detail string) {
			checks = append(checks, DoctorCheckItem{
				ID:      id,
				Name:    name,
				Status:  status,
				Message: msg,
				Detail:  detail,
			})
			switch status {
			case "pass":
				passed++
			case "warn":
				warnings++
			default:
				failed++
			}
		}

		// 1. Kernel & eBPF
		kernelVer := ""
		if out, err := exec.CommandContext(ctx, "uname", "-r").Output(); err == nil {
			kernelVer = strings.TrimSpace(string(out))
		}
		bpffsMounted := false
		if out, err := exec.CommandContext(ctx, "mount").Output(); err == nil {
			if strings.Contains(string(out), "type bpf") {
				bpffsMounted = true
			}
		}

		if runtime.GOOS == "linux" {
			if bpffsMounted {
				addCheck("kernel", "Linux 内核与 eBPF 支持", "pass",
					fmt.Sprintf("Linux %s (%s), BPF 文件系统已就绪 (/sys/fs/bpf)", kernelVer, runtime.GOARCH), "")
			} else {
				addCheck("kernel", "Linux 内核与 eBPF 支持", "warn",
					fmt.Sprintf("Linux %s (%s), 但未检测到 /sys/fs/bpf 挂载", kernelVer, runtime.GOARCH), "请检查内核配置或执行 mount -t bpf bpf /sys/fs/bpf")
			}
		} else {
			addCheck("kernel", "Linux 内核与 eBPF 支持", "fail",
				fmt.Sprintf("非 Linux 环境 (%s/%s)，dae 仅支持 Linux 内核", runtime.GOOS, runtime.GOARCH), "")
		}

		// 2. systemd
		if _, err := os.Stat("/run/systemd/system"); err == nil {
			addCheck("systemd", "systemd 服务管理器", "pass", "systemd 守护进程正常活跃运行", "")
		} else {
			addCheck("systemd", "systemd 服务管理器", "warn", "未检测到 /run/systemd/system 守护进程目录", "后台守护功能可能受限")
		}

		// 3. dae Core
		if _, err := os.Stat(o.DaeBin); err == nil {
			daeVer := ""
			if out, err := exec.CommandContext(ctx, o.DaeBin, "--version").Output(); err == nil {
				daeVer = strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
			}
			daeActive := false
			if out, err := exec.CommandContext(ctx, "systemctl", "is-active", o.DaeService).Output(); err == nil {
				daeActive = strings.TrimSpace(string(out)) == "active"
			}
			if daeActive {
				addCheck("dae_core", "dae 核心程序与服务", "pass",
					fmt.Sprintf("%s (运行中 active/running)", daeVer), o.DaeBin)
			} else {
				addCheck("dae_core", "dae 核心程序与服务", "warn",
					fmt.Sprintf("%s 已安装，但 %s 服务当前未运行", daeVer, o.DaeService), "可点击服务管理中的「启动服务」")
			}
		} else {
			addCheck("dae_core", "dae 核心程序与服务", "fail",
				fmt.Sprintf("未找到 dae 可执行文件: %s", o.DaeBin), "请检查 dae 是否正确安装到 /usr/local/bin/dae")
		}

		// 4. dae Config
		if info, err := os.Stat(o.DaeConfig); err == nil {
			perm := fmt.Sprintf("%04o", info.Mode().Perm())
			content, err := os.ReadFile(o.DaeConfig)
			if err != nil {
				addCheck("dae_config", "dae 配置文件与语法校验", "fail", "无法读取配置文件", err.Error())
			} else {
				valid, valMsg := validateDaeConfig(ctx, o.DaeBin, string(content))
				if valid {
					status := "pass"
					if perm != "0600" && perm != "600" {
						status = "warn"
						addCheck("dae_config", "dae 配置文件与语法校验", status,
							fmt.Sprintf("语法校验通过，但文件权限为 %s (建议 0600)", perm), o.DaeConfig)
					} else {
						addCheck("dae_config", "dae 配置文件与语法校验", status,
							fmt.Sprintf("语法校验全部通过，安全权限 0600 (%s)", filepath.Base(o.DaeConfig)), o.DaeConfig)
					}
				} else {
					addCheck("dae_config", "dae 配置文件与语法校验", "fail", "配置语法校验存在错误", valMsg)
				}
			}
		} else {
			addCheck("dae_config", "dae 配置文件与语法校验", "fail",
				fmt.Sprintf("配置文件不存在: %s", o.DaeConfig), "")
		}

		// 5. GeoIP & GeoSite
		geositePath := ""
		geoipPath := ""
		datDirs := []string{"/usr/local/share/dae", "/usr/share/dae", "/etc/dae", "/usr/local/etc/dae", "/var/lib/dae"}
		for _, d := range datDirs {
			if geositePath == "" {
				p := filepath.Join(d, "geosite.dat")
				if _, err := os.Stat(p); err == nil {
					geositePath = p
				}
			}
			if geoipPath == "" {
				p := filepath.Join(d, "geoip.dat")
				if _, err := os.Stat(p); err == nil {
					geoipPath = p
				}
			}
		}
		if geositePath != "" && geoipPath != "" {
			addCheck("geo_assets", "分流规则库 (GeoIP / GeoSite)", "pass",
				"geosite.dat 与 geoip.dat 规则数据库均已就绪",
				fmt.Sprintf("Site: %s | IP: %s", geositePath, geoipPath))
		} else if geositePath != "" || geoipPath != "" {
			addCheck("geo_assets", "分流规则库 (GeoIP / GeoSite)", "warn",
				"规则库部分就绪 (部分 dat 缺失)",
				fmt.Sprintf("Site: %s | IP: %s", geositePath, geoipPath))
		} else {
			addCheck("geo_assets", "分流规则库 (GeoIP / GeoSite)", "warn",
				"未在系统标准路径中检测到 geosite.dat 或 geoip.dat",
				"可在「系统更新」中一键下载更新规则库")
		}

		// 6. WebUI
		addCheck("webui_service", "WebUI 控制台服务状态", "pass",
			fmt.Sprintf("监听于 %s, PID: %d, 版本: %s", o.Listen, os.Getpid(), version),
			fmt.Sprintf("Go %s (%s)", runtime.Version(), runtime.GOARCH))

		// 7. Network default route
		if out, err := exec.CommandContext(ctx, "ip", "route", "show", "default").Output(); err == nil && len(out) > 0 {
			line := strings.TrimSpace(strings.Split(string(out), "\n")[0])
			addCheck("network", "默认外网出口网卡", "pass", line, "")
		} else {
			addCheck("network", "默认外网出口网卡", "warn", "未获取到系统默认网关路由", "")
		}

		// 8. Storage
		if err := os.MkdirAll(o.BackupDir, 0700); err == nil {
			addCheck("storage", "数据与备份存储目录", "pass",
				fmt.Sprintf("备份目录正常可读写: %s", o.BackupDir), "")
		} else {
			addCheck("storage", "数据与备份存储目录", "warn",
				fmt.Sprintf("备份目录不可写: %v", err), o.BackupDir)
		}

		overall := "healthy"
		if failed > 0 {
			overall = "error"
		} else if warnings > 0 {
			overall = "warning"
		}

		writeJSON(w, http.StatusOK, DoctorReport{
			Timestamp: time.Now().Format("2006-01-02 15:04:05"),
			Status:    overall,
			Summary: map[string]int{
				"total":   len(checks),
				"passed":  passed,
				"warning": warnings,
				"failed":  failed,
			},
			Checks: checks,
		})
	}
}
