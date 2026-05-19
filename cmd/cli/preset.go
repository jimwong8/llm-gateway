package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// Preset 数据结构（对应 API 响应）
type Preset struct {
	ID          int64    `json:"id"`
	UserID      int64    `json:"user_id"`
	TenantID    string   `json:"tenant_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Template    string   `json:"template"`
	Variables   string   `json:"variables"`
	Tags        []string `json:"tags"`
	IsPublic    bool     `json:"is_public"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// PresetListData 列表响应
type PresetListData struct {
	Data []Preset `json:"data"`
}

// PresetCreateBody 创建请求体
type PresetCreateBody struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Template    string   `json:"template"`
	Variables   []string `json:"variables"`
	Tags        []string `json:"tags"`
	IsPublic    bool     `json:"is_public"`
}

// PresetUpdateBody 更新请求体
type PresetUpdateBody struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Template    string   `json:"template"`
	Variables   []string `json:"variables"`
	Tags        []string `json:"tags"`
}

func handlePreset(client *Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: preset <list|create|update|delete> [选项]")
		os.Exit(1)
	}

	action := args[0]
	switch action {
	case "list":
		presetList(client, args[1:])
	case "create":
		presetCreate(client, args[1:])
	case "update":
		presetUpdate(client, args[1:])
	case "delete":
		presetDelete(client, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "未知操作: %s\n", action)
		os.Exit(1)
	}
}

func presetList(client *Client, args []string) {
	fs := flag.NewFlagSet("preset list", flag.ExitOnError)
	_ = fs.Bool("all", false, "包含公开预设（默认 false）")
	fs.Parse(args)

	var result PresetListData
	if err := client.doRequest("GET", "/api/memory/presets", nil, &result); err != nil {
		fmt.Fprintf(os.Stderr, "列出预设失败: %v\n", err)
		os.Exit(1)
	}

	rows := make([][]string, 0, len(result.Data))
	for _, p := range result.Data {
		rows = append(rows, []string{
			fmt.Sprintf("%d", p.ID),
			p.Name,
			p.Description,
			strings.Join(p.Tags, ","),
			fmt.Sprintf("%v", p.IsPublic),
			p.CreatedAt,
		})
	}
	printOutput(
		[]string{"ID", "名称", "描述", "标签", "公开", "创建时间"},
		rows,
		result,
	)
}

func presetCreate(client *Client, args []string) {
	fs := flag.NewFlagSet("preset create", flag.ExitOnError)
	name := fs.String("name", "", "预设名称（必填）")
	desc := fs.String("description", "", "预设描述")
	template := fs.String("template", "", "提示词模板（必填）")
	variables := fs.String("variables", "", "变量列表（逗号分隔）")
	tags := fs.String("tags", "", "标签列表（逗号分隔）")
	isPublic := fs.Bool("public", false, "是否公开")
	fs.Parse(args)

	if *name == "" || *template == "" {
		fmt.Fprintln(os.Stderr, "错误: --name 和 --template 为必填项")
		fs.Usage()
		os.Exit(1)
	}

	body := PresetCreateBody{
		Name:        *name,
		Description: *desc,
		Template:    *template,
		IsPublic:    *isPublic,
	}
	if *variables != "" {
		body.Variables = strings.Split(*variables, ",")
	}
	if *tags != "" {
		body.Tags = strings.Split(*tags, ",")
	}

	var result Preset
	if err := client.doRequest("POST", "/api/memory/presets", body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "创建预设失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("创建成功:")
	if *output == "json" {
		printJSON(result)
	} else {
		fmt.Printf("  ID:          %d\n", result.ID)
		fmt.Printf("  名称:        %s\n", result.Name)
		fmt.Printf("  描述:        %s\n", result.Description)
		fmt.Printf("  模板:        %s\n", result.Template)
		fmt.Printf("  变量:        %s\n", result.Variables)
		fmt.Printf("  标签:        %v\n", result.Tags)
		fmt.Printf("  公开:        %v\n", result.IsPublic)
		fmt.Printf("  创建时间:    %s\n", result.CreatedAt)
	}
}

func presetUpdate(client *Client, args []string) {
	fs := flag.NewFlagSet("preset update", flag.ExitOnError)
	id := fs.Int64("id", 0, "预设 ID（必填）")
	name := fs.String("name", "", "预设名称")
	desc := fs.String("description", "", "预设描述")
	template := fs.String("template", "", "提示词模板")
	variables := fs.String("variables", "", "变量列表（逗号分隔）")
	tags := fs.String("tags", "", "标签列表（逗号分隔）")
	fs.Parse(args)

	if *id == 0 {
		fmt.Fprintln(os.Stderr, "错误: --id 为必填项")
		fs.Usage()
		os.Exit(1)
	}

	body := PresetUpdateBody{}
	if *name != "" {
		body.Name = *name
	}
	if *desc != "" {
		body.Description = *desc
	}
	if *template != "" {
		body.Template = *template
	}
	if *variables != "" {
		body.Variables = strings.Split(*variables, ",")
	}
	if *tags != "" {
		body.Tags = strings.Split(*tags, ",")
	}

	var result Preset
	path := fmt.Sprintf("/api/memory/presets/%d", *id)
	if err := client.doRequest("PUT", path, body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "更新预设失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("更新成功:")
	if *output == "json" {
		printJSON(result)
	} else {
		fmt.Printf("  ID:          %d\n", result.ID)
		fmt.Printf("  名称:        %s\n", result.Name)
		fmt.Printf("  描述:        %s\n", result.Description)
		fmt.Printf("  模板:        %s\n", result.Template)
		fmt.Printf("  更新时间:    %s\n", result.UpdatedAt)
	}
}

func presetDelete(client *Client, args []string) {
	fs := flag.NewFlagSet("preset delete", flag.ExitOnError)
	id := fs.Int64("id", 0, "预设 ID（必填）")
	fs.Parse(args)

	if *id == 0 {
		fmt.Fprintln(os.Stderr, "错误: --id 为必填项")
		fs.Usage()
		os.Exit(1)
	}

	path := fmt.Sprintf("/api/memory/presets/%d", *id)
	if err := client.doRequest("DELETE", path, nil, nil); err != nil {
		fmt.Fprintf(os.Stderr, "删除预设失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("预设 %d 已删除\n", *id)
}
