package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	base := "http://127.0.0.1:8080"
	key := readAdminKey("/home/jimwong/llm-gateway/gateway/.env")

	postJSON(base+"/v1/chat/completions", map[string]any{
		"model": "gpt-4o-mini",
		"tenant_id": "t_demo",
		"messages": []map[string]string{{"role": "user", "content": fmt.Sprintf("observability verify miss %d", time.Now().UnixNano())}},
	}, nil)

	payload := map[string]any{
		"model": "gpt-4o-mini",
		"tenant_id": "t_demo",
		"messages": []map[string]string{{"role": "user", "content": "observability exact hit"}},
	}
	h1, _ := postJSON(base+"/v1/chat/completions", payload, nil)
	h2, _ := postJSON(base+"/v1/chat/completions", payload, nil)
	fmt.Println("exact-hit-1", h1.Get("X-Cache"))
	fmt.Println("exact-hit-2", h2.Get("X-Cache"))

	uid := fmt.Sprintf("%d", time.Now().UnixNano())
	h3, _ := postJSON(base+"/v1/chat/completions", map[string]any{
		"model": "gpt-4o-mini",
		"tenant_id": "t_demo",
		"messages": []map[string]string{{"role": "user", "content": "Can you explain semantic caching? " + uid}},
	}, nil)
	time.Sleep(500 * time.Millisecond)
	h4, _ := postJSON(base+"/v1/chat/completions", map[string]any{
		"model": "gpt-4o-mini",
		"tenant_id": "t_demo",
		"messages": []map[string]string{{"role": "user", "content": "Could you explain semantic caching to me? " + uid}},
	}, nil)
	fmt.Println("semantic-1", h3.Get("X-Cache"))
	fmt.Println("semantic-2", h4.Get("X-Cache"), h4.Get("X-Semantic-Score"))

	// failure path candidate
	h5, b5 := postJSON(base+"/v1/chat/completions", map[string]any{
		"model": "fail-code",
		"tenant_id": "t_demo",
		"messages": []map[string]string{{"role": "user", "content": "trigger provider failure path"}},
	}, nil)
	fmt.Println("failure-status", h5.Get("X-Cache"), b5)

	for _, path := range []string{
		"/admin/observability/summary",
		"/admin/observability/cache",
		"/admin/observability/providers",
		"/admin/observability/hotspots",
	} {
		status, body := getWithAdmin(base+path, key)
		fmt.Println(path, status, body)
	}
}

func readAdminKey(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "ADMIN_API_KEY=") {
			return strings.TrimSpace(strings.SplitN(line, "=", 2)[1])
		}
	}
	panic("ADMIN_API_KEY not found")
}

func postJSON(url string, payload any, extraHeaders map[string]string) (http.Header, string) {
	raw, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.Header, string(body)
}

func getWithAdmin(url, key string) (int, string) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-Admin-Key", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}
