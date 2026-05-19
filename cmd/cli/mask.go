package main

import (
	"flag"
	"fmt"
	"os"
)

// MaskRule 数据结构
type MaskRule struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	TenantID  string `json:"tenant_id"`
	Name      string `json:"name"`
	Pattern   string `json:"pattern"`
	Replace   string `json:"replace"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

// MaskListData 列表响应
type MaskListData struct {
	Data []MaskRule `json:"data"`
}

// MaskCreateBody 创建请求体
type MaskCreateBody struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Replace string `json:"replace"`
}

// MaskUpdateBody 更新请求体
type MaskUpdateBody struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Replace string `json:"replace"`
	Enabled bool   `json:"enabled"`
}

func handleMask(client *Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: mask <list|create|delete> [选项]")
		os.Exit(1)
	}

	action := args[0]
	switch action {
	case "list":
		maskList(client, args[1:])
	case "create":
		maskCreate(client, args[1:])
	case "delete":
		maskDelete(client, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "未知操作: %s\n", action)
		os.Exit(1)
	}
}

func maskList(client *Client, args []string) {
	fs := flag.NewFlagSet("mask list", flag.ExitOnError)
	fs.Parse(args)

	var result MaskListData
	if err := client.doRequest("GET", "/api/memory/masks", nil, &result); err != nil {
		fmt.Fprintf(os.Stderr, "列出掩码规则失败: %v\n", err)
		os.Exit(1)
	}

	rows := make([][]string, 0, len(result.Data))
	for _, m := range result.Data {
		active := "否"
		if m.IsActive {
			active = "是"
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", m.ID),
			m.Name,
			m.Pattern,
			m.Replace,
			active,
			m.CreatedAt,
		})
	}
	printOutput(
		[]string{"ID", "名称", "匹配模式", "替换为", "启用", "创建时间"},
		rows,
		result,
	)
}

func maskCreate(client *Client, args []string) {
	fs := flag.NewFlagSet("mask create", flag.ExitOnError)
	name := fs.String("name", "", "规则名称（必填）")
	pattern := fs.String("pattern", "", "匹配模式（必填）")
	replace := fs.String("replace", "", "替换内容（必填）")
	fs.Parse(args)

	if *name == "" || *pattern == "" || *replace == "" {
		fmt.Fprintln(os.Stderr, "错误: --name, --pattern, --replace 均为必填项")
		fs.Usage()
		os.Exit(1)
	}

	body := MaskCreateBody{
		Name:    *name,
		Pattern: *pattern,
		Replace: *replace,
	}

	var result MaskRule
	if err := client.doRequest("POST", "/api/memory/masks", body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "创建掩码规则失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("创建成功:")
	if *output == "json" {
		printJSON(result)
	} else {
		fmt.Printf("  ID:          %d\n", result.ID)
		fmt.Printf("  名称:        %s\n", result.Name)
		fmt.Printf("  匹配模式:    %s\n", result.Pattern)
		fmt.Printf("  替换内容:    %s\n", result.Replace)
		fmt.Printf("  启用:        %v\n", result.IsActive)
		fmt.Printf("  创建时间:    %s\n", result.CreatedAt)
	}
}

func maskDelete(client *Client, args []string) {
	fs := flag.NewFlagSet("mask delete", flag.ExitOnError)
	id := fs.Int64("id", 0, "规则 ID（必填）")
	fs.Parse(args)

	if *id == 0 {
		fmt.Fprintln(os.Stderr, "错误: --id 为必填项")
		fs.Usage()
		os.Exit(1)
	}

	path := fmt.Sprintf("/api/memory/masks/%d", *id)
	if err := client.doRequest("DELETE", path, nil, nil); err != nil {
		fmt.Fprintf(os.Stderr, "删除掩码规则失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("掩码规则 %d 已删除\n", *id)
}
