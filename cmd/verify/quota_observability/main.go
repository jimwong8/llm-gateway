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
	tenantID := fmt.Sprintf("quota-tenant-%d", time.Now().UnixNano())

	for i := 0; i < 3; i++ {
		postJSON(base+"/v1/chat/completions", map[string]any{
			"model": "gpt-4o-mini",
			"tenant_id": tenantID,
			"messages": []map[string]string{{"role": "user", "content": fmt.Sprintf("quota observe %d", i)}},
		}, nil)
	}

	status, body := getWithAdmin(base+"/admin/observability/quota?tenant_id="+tenantID, key)
	fmt.Println("quota", status, body)
	status, body = getWithAdmin(base+"/admin/observability/quota/trends?tenant_id="+tenantID+"&window_minutes=5", key)
	fmt.Println("quota-trends", status, body)
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
