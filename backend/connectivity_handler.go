package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ConnectivityTarget struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Url         string `json:"url"`
	ExpectedOut string `json:"expectedOut"`
	Status      string `json:"status"`  // "ok", "error"
	Latency     int    `json:"latency"` // ms (-1 if unreachable)
	HttpCode    int    `json:"httpCode"`
	Message     string `json:"message"`
	LastTested  string `json:"lastTested"`
}

var targetDefinitions = []ConnectivityTarget{
	{
		ID:          "aliyun",
		Name:        "阿里云访问检测",
		Category:    "国内直连",
		Url:         "https://www.aliyun.com",
		ExpectedOut: "国内直连 (direct)",
		Status:      "error",
		Latency:     -1,
		HttpCode:    0,
		Message:     "尚未检测",
	},
	{
		ID:          "google",
		Name:        "Google 访问检测",
		Category:    "国际搜索",
		Url:         "https://www.google.com/generate_204",
		ExpectedOut: "代理分流 (proxy)",
		Status:      "error",
		Latency:     -1,
		HttpCode:    0,
		Message:     "尚未检测",
	},
	{
		ID:          "github",
		Name:        "GitHub 访问检测",
		Category:    "开发者服务",
		Url:         "https://github.com",
		ExpectedOut: "代理分流 (proxy)",
		Status:      "error",
		Latency:     -1,
		HttpCode:    0,
		Message:     "尚未检测",
	},
	{
		ID:          "youtube",
		Name:        "YouTube 访问检测",
		Category:    "流媒体服务",
		Url:         "https://www.youtube.com/generate_204",
		ExpectedOut: "代理分流 (proxy)",
		Status:      "error",
		Latency:     -1,
		HttpCode:    0,
		Message:     "尚未检测",
	},
	{
		ID:          "openai",
		Name:        "OpenAI 访问检测",
		Category:    "AI 大模型",
		Url:         "https://api.openai.com/v1/models",
		ExpectedOut: "代理分流 (proxy)",
		Status:      "error",
		Latency:     -1,
		HttpCode:    0,
		Message:     "尚未检测",
	},
}

var (
	connectivityMu        sync.RWMutex
	lastTestedResults     = make(map[string]ConnectivityTarget)
	lastConnectivityCheck time.Time
)

func testSingleTarget(def ConnectivityTarget) ConnectivityTarget {
	res := def
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 2000*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", def.Url, nil)
	var realSuccess bool
	var statusCode int
	var errMsg string

	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 daex/connectivity-check")
		client := &http.Client{
			Timeout: 2000 * time.Millisecond,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		}
		resp, rErr := client.Do(req)
		if rErr == nil {
			_ = resp.Body.Close()
			statusCode = resp.StatusCode
			if statusCode > 0 && statusCode < 500 {
				realSuccess = true
			} else {
				errMsg = fmt.Sprintf("HTTP %d", statusCode)
			}
		} else {
			errMsg = rErr.Error()
		}
	} else {
		errMsg = err.Error()
	}

	elapsed := int(time.Since(start).Milliseconds())
	if elapsed <= 0 {
		elapsed = 1
	}

	if realSuccess {
		res.Latency = elapsed
		res.HttpCode = statusCode
		res.Status = "ok"
		res.Message = fmt.Sprintf("HTTP %d 连通正常 (%d ms)", statusCode, elapsed)
	} else {
		res.Latency = -1
		res.HttpCode = statusCode
		res.Status = "error"
		res.Message = "无法连接: " + errMsg
	}

	res.LastTested = time.Now().Format("15:04:05")
	return res
}

func runAllConnectivityProbes() []ConnectivityTarget {
	results := make([]ConnectivityTarget, len(targetDefinitions))
	var wg sync.WaitGroup
	for i, def := range targetDefinitions {
		wg.Add(1)
		go func(idx int, d ConnectivityTarget) {
			defer wg.Done()
			results[idx] = testSingleTarget(d)
		}(i, def)
	}
	wg.Wait()

	connectivityMu.Lock()
	for _, r := range results {
		lastTestedResults[r.ID] = r
	}
	lastConnectivityCheck = time.Now()
	connectivityMu.Unlock()

	return results
}

func handleGetConnectivity() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connectivityMu.RLock()
		needProbe := len(lastTestedResults) == 0 || time.Since(lastConnectivityCheck) > 20*time.Second
		connectivityMu.RUnlock()

		if needProbe {
			list := runAllConnectivityProbes()
			writeJSON(w, http.StatusOK, map[string]any{"targets": list})
			return
		}

		connectivityMu.RLock()
		defer connectivityMu.RUnlock()

		var list []ConnectivityTarget
		for _, def := range targetDefinitions {
			if item, exists := lastTestedResults[def.ID]; exists {
				list = append(list, item)
			} else {
				list = append(list, def)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"targets": list})
	}
}

func handleTestConnectivity() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Target string `json:"target"` // "all" or specific target id
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		targetID := strings.TrimSpace(req.Target)
		if targetID == "" || targetID == "all" {
			tested := runAllConnectivityProbes()
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":      true,
				"results": tested,
			})
			return
		}

		var tested []ConnectivityTarget
		for _, def := range targetDefinitions {
			if targetID == def.ID {
				testedItem := testSingleTarget(def)
				connectivityMu.Lock()
				lastTestedResults[def.ID] = testedItem
				connectivityMu.Unlock()
				tested = append(tested, testedItem)
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"results": tested,
		})
	}
}
