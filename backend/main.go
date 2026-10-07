package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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
}

func main() {
	var o Options
	var allowSystemConfig bool
	flag.StringVar(&o.Listen, "listen", envOr("DAE_WEBUI_LISTEN", "0.0.0.0:9091"), "HTTP listen address")
	flag.StringVar(&o.DaeBin, "dae-bin", envOr("DAE_BIN", "/usr/local/bin/dae"), "path to dae binary")
	// No default on purpose: development/testing must point at config.mock.dae explicitly.
	flag.StringVar(&o.DaeConfig, "dae-config", envOr("DAE_CONFIG", ""), "path to dae config (dev/test: config.mock.dae)")
	flag.StringVar(&o.DaeService, "dae-service", envOr("DAE_SERVICE", "dae"), "systemd unit name of dae")
	flag.BoolVar(&allowSystemConfig, "allow-system-config", false, "allow using a config under "+systemConfigDir+" (production only)")
	flag.Parse()

	if err := checkConfigPath(o.DaeConfig, allowSystemConfig); err != nil {
		log.Fatalf("refusing to start: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		host, _ := os.Hostname()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": version, "hostname": host, "time": time.Now()})
	})
	mux.HandleFunc("GET /api/dae/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, daeStatus(r.Context(), o))
	})
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		b, err := os.ReadFile(o.DaeConfig)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"path": o.DaeConfig, "content": string(b)})
	})
	mux.Handle("/", spaHandler())

	srv := &http.Server{Addr: o.Listen, Handler: logRequests(mux), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("dae-webui %s listening on %s (dae=%s config=%s)", version, o.Listen, o.DaeBin, o.DaeConfig)
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

// checkConfigPath enforces the project rule: development/testing only uses config.mock.dae,
// never the real production config. Symlinks are resolved so the guard can't be bypassed.
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
