package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// HealthStatus 健康检查响应
type HealthStatus struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Env      string `json:"env"`
	MockMode bool   `json:"mock_mode"`
	Cache    string `json:"cache"`
	Audit    string `json:"audit"`
	Semantic string `json:"semantic_cache"`
	Memory   string `json:"memory"`
	Billing  string `json:"billing"`
	Time     string `json:"time"`
}

// AdminHealthStatus 管理员健康检查响应
type AdminHealthStatus struct {
	Service           string         `json:"service"`
	Time              string         `json:"time"`
	Providers         []any          `json:"providers"`
	ProviderSummary   map[string]int `json:"provider_summary"`
	RuntimeSummary    map[string]any `json:"runtime_summary"`
	CompensationStats map[string]int `json:"compensation_stats"`
	AdminAuth         string         `json:"admin_auth"`
}

func handleHealth(client *Client, args []string) {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	detailed := fs.Bool("detailed", false, "显示详细健康信息")
	admin := fs.Bool("admin", false, "使用管理员健康检查端点")
	fs.Parse(args)

	switch {
	case *admin:
		healthAdmin(client)
	case *detailed:
		healthDetailed(client)
	default:
		healthSimple(client)
	}
}

func healthSimple(client *Client) {
	var result HealthStatus
	if err := client.doRequest("GET", "/healthz", nil, &result); err != nil {
		fmt.Fprintf(os.Stderr, "健康检查失败: %v\n", err)
		os.Exit(1)
	}

	if *output == "json" {
		printJSON(result)
	} else {
		fmt.Printf("状态:     %s\n", result.Status)
		fmt.Printf("服务:     %s\n", result.Service)
		fmt.Printf("环境:     %s\n", result.Env)
		fmt.Printf("缓存:     %s\n", result.Cache)
		fmt.Printf("审计:     %s\n", result.Audit)
		fmt.Printf("语义缓存: %s\n", result.Semantic)
		fmt.Printf("记忆:     %s\n", result.Memory)
		fmt.Printf("计费:     %s\n", result.Billing)
		fmt.Printf("时间:     %s\n", result.Time)
	}
}

func healthDetailed(client *Client) {
	data, err := client.doRequestRaw("GET", "/healthz/detailed", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "详细健康检查失败: %v\n", err)
		os.Exit(1)
	}

	if *output == "json" {
		var parsed any
		if err := json.Unmarshal(data, &parsed); err == nil {
			printJSON(parsed)
		} else {
			fmt.Println(string(data))
		}
	} else {
		var result HealthStatus
		if err := json.Unmarshal(data, &result); err == nil {
			fmt.Printf("状态: %s\n", result.Status)
			fmt.Printf("服务: %s\n", result.Service)
			fmt.Printf("时间: %s\n", result.Time)
		} else {
			fmt.Println(string(data))
		}
	}
}

func healthAdmin(client *Client) {
	var result AdminHealthStatus
	if err := client.doRequest("GET", "/admin/health", nil, &result); err != nil {
		fmt.Fprintf(os.Stderr, "管理员健康检查失败: %v\n", err)
		os.Exit(1)
	}

	if *output == "json" {
		printJSON(result)
	} else {
		fmt.Printf("服务:       %s\n", result.Service)
		fmt.Printf("时间:       %s\n", result.Time)
		fmt.Printf("管理员认证: %s\n", result.AdminAuth)
		fmt.Printf("提供商摘要:\n")
		for k, v := range result.ProviderSummary {
			fmt.Printf("  %s: %d\n", k, v)
		}
	}
}
