package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"dae-webui/internal/webui"
)

var version = "dev"

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

	if err := checkConfigPath(o.DaeConfig, allowSystemConfig); err != nil {
		log.Fatalf("refusing to start: %v", err)
	}

	if o.BackupDir == "" {
		cfgDir := filepath.Dir(o.DaeConfig)
		o.BackupDir = filepath.Join(cfgDir, "backups")
	}
	_ = os.MkdirAll(o.BackupDir, 0700)

	mux := http.NewServeMux()

	// 1. Health & status
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		host, _ := os.Hostname()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": version, "hostname": host, "time": time.Now()})
	})
	mux.HandleFunc("GET /api/dae/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, daeStatus(r.Context(), o))
	})

	// 2. Config reading, saving, and validation
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		b, err := os.ReadFile(o.DaeConfig)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		fi, _ := os.Stat(o.DaeConfig)
		modTime := time.Time{}
		if fi != nil {
			modTime = fi.ModTime()
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"path":    o.DaeConfig,
			"content": string(b),
			"modTime": modTime,
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
			Content      string `json:"content"`
			AutoBackup   bool   `json:"autoBackup"`
			ReloadConfig bool   `json:"reloadConfig"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}

		// First validate with dae binary
		valid, msg := validateDaeConfig(r.Context(), o.DaeBin, req.Content)
		if !valid {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":   "configuration validation failed",
				"details": msg,
			})
			return
		}

		// Create auto-backup if requested or if file exists
		if req.AutoBackup {
			if _, err := createBackupFile(o.DaeConfig, o.BackupDir, "auto"); err != nil {
				log.Printf("failed to create auto backup: %v", err)
			}
		}

		// Write config file atomically with strict permission (0600)
		tmpFile := o.DaeConfig + ".tmp"
		if err := os.WriteFile(tmpFile, []byte(req.Content), 0600); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to write tmp file: " + err.Error()})
			return
		}
		if err := os.Rename(tmpFile, o.DaeConfig); err != nil {
			_ = os.Remove(tmpFile)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to replace config: " + err.Error()})
			return
		}
		_ = os.Chmod(o.DaeConfig, 0600)

		// Optionally reload dae (only if running mock or if authorized)
		reloaded := false
		var reloadMsg string
		if req.ReloadConfig {
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

	// 3. Export config
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

	// 4. Import config
	mux.HandleFunc("POST /api/config/import", func(w http.ResponseWriter, r *http.Request) {
		// Accept multipart/form-data or plain body
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

	// 5. Backups management
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

		// Validate backup content before restoring
		valid, msg := validateDaeConfig(r.Context(), o.DaeBin, string(b))
		if !valid {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "backup file failed dae validation", "details": msg})
			return
		}

		// Save current before restoring
		_, _ = createBackupFile(o.DaeConfig, o.BackupDir, "pre-restore")

		// Write to config
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

// spaHandler serves the embedded Vue build; unknown paths fall back to index.html.
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
