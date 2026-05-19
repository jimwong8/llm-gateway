package main

import (
	"flag"
	"fmt"
	"os"
)

// TenantKey 数据结构
type TenantKey struct {
	TenantID  string `json:"tenant_id"`
	Provider  string `json:"provider"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

// TenantKeyListData 列表响应
type TenantKeyListData struct {
	Object string      `json:"object"`
	Data   []TenantKey `json:"data"`
}

// TenantKeyCreateBody 创建请求体
type TenantKeyCreateBody struct {
	TenantID string `json:"tenant_id"`
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
}

// QuotaStatus 配额状态
type QuotaStatus struct {
	TenantID  string `json:"tenant_id"`
	RPMLimit  int    `json:"rpm_limit"`
	TPDLimit  int64  `json:"tpd_limit"`
	RPMUsed   int    `json:"rpm_used"`
	TPDUsed   int64  `json:"tpd_used"`
	Remaining int64  `json:"remaining"`
}

// QuotaSetBody 设置配额请求体
type QuotaSetBody struct {
	TenantID string `json:"tenant_id"`
	RPMLimit int    `json:"rpm_limit"`
	TPDLimit int64  `json:"tpd_limit"`
}

func handleTenant(client *Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: tenant <list|show|quota> [选项]")
		os.Exit(1)
	}

	action := args[0]
	switch action {
	case "list":
		tenantList(client, args[1:])
	case "show":
		tenantShow(client, args[1:])
	case "quota":
		tenantQuota(client, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "未知操作: %s\n", action)
		os.Exit(1)
	}
}

func tenantList(client *Client, args []string) {
	fs := flag.NewFlagSet("tenant list", flag.ExitOnError)
	fs.Parse(args)

	var result TenantKeyListData
	if err := client.doRequest("GET", "/admin/tenant-keys", nil, &result); err != nil {
		fmt.Fprintf(os.Stderr, "列出租户失败: %v\n", err)
		os.Exit(1)
	}

	rows := make([][]string, 0, len(result.Data))
	for _, k := range result.Data {
		active := "否"
		if k.IsActive {
			active = "是"
		}
		rows = append(rows, []string{
			k.TenantID,
			k.Provider,
			active,
			k.CreatedAt,
		})
	}
	printOutput(
		[]string{"租户ID", "提供商", "启用", "创建时间"},
		rows,
		result,
	)
}

func tenantShow(client *Client, args []string) {
	fs := flag.NewFlagSet("tenant show", flag.ExitOnError)
	tenantID := fs.String("tenant-id", "", "租户 ID（必填）")
	fs.Parse(args)

	if *tenantID == "" {
		fmt.Fprintln(os.Stderr, "错误: --tenant-id 为必填项")
		fs.Usage()
		os.Exit(1)
	}

	var result TenantKeyListData
	path := fmt.Sprintf("/admin/tenant-keys?tenant_id=%s", *tenantID)
	if err := client.doRequest("GET", path, nil, &result); err != nil {
		fmt.Fprintf(os.Stderr, "查询租户失败: %v\n", err)
		os.Exit(1)
	}

	if *output == "json" {
		printJSON(result)
	} else {
		fmt.Printf("租户: %s\n", *tenantID)
		fmt.Printf("提供商数量: %d\n", len(result.Data))
		for _, k := range result.Data {
			status := "禁用"
			if k.IsActive {
				status = "启用"
			}
			fmt.Printf("  - %s [%s] 创建于 %s\n", k.Provider, status, k.CreatedAt)
		}
	}
}

func tenantQuota(client *Client, args []string) {
	fs := flag.NewFlagSet("tenant quota", flag.ExitOnError)
	tenantID := fs.String("tenant-id", "", "租户 ID（必填）")
	rpmLimit := fs.Int("rpm-limit", 0, "每分钟请求限制（设置时必填）")
	tpdLimit := fs.Int64("tpd-limit", 0, "每日 token 限制（设置时必填）")
	fs.Parse(args)

	if *tenantID == "" {
		fmt.Fprintln(os.Stderr, "错误: --tenant-id 为必填项")
		fs.Usage()
		os.Exit(1)
	}

	// 如果设置了 rpm-limit 或 tpd-limit，则更新配额
	if *rpmLimit > 0 || *tpdLimit > 0 {
		body := QuotaSetBody{
			TenantID: *tenantID,
			RPMLimit: *rpmLimit,
			TPDLimit: *tpdLimit,
		}
		var result QuotaStatus
		if err := client.doRequest("PUT", "/api/tenant/quota", body, &result); err != nil {
			fmt.Fprintf(os.Stderr, "设置配额失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("配额已更新:")
		printQuota(result)
	} else {
		// 查询配额
		var result QuotaStatus
		path := fmt.Sprintf("/api/tenant/quota?tenant_id=%s", *tenantID)
		if err := client.doRequest("GET", path, nil, &result); err != nil {
			fmt.Fprintf(os.Stderr, "查询配额失败: %v\n", err)
			os.Exit(1)
		}
		printQuota(result)
	}
}

func printQuota(q QuotaStatus) {
	if *output == "json" {
		printJSON(q)
	} else {
		fmt.Printf("租户配额: %s\n", q.TenantID)
		fmt.Printf("  RPM 限制:   %d (已用 %d)\n", q.RPMLimit, q.RPMUsed)
		fmt.Printf("  TPD 限制:   %d (已用 %d)\n", q.TPDLimit, q.TPDUsed)
		fmt.Printf("  剩余:       %d\n", q.Remaining)
	}
}
