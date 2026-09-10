package main

import (
	"flag"
	"fmt"
	"os"
)

// 全局配置
var (
	apiBase = flag.String("api-base", "http://localhost:8080", "API 服务器基础地址")
	token   = flag.String("token", "", "认证 Token (Bearer)")
	output  = flag.String("output", "table", "输出格式: json|table")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "llm-gateway CLI 管理工具\n\n")
		fmt.Fprintf(os.Stderr, "用法: %s [全局选项] <子命令> [子命令选项]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "全局选项:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n子命令:\n")
		fmt.Fprintf(os.Stderr, "  preset list          列出所有 Prompt 预设\n")
		fmt.Fprintf(os.Stderr, "  preset create        创建 Prompt 预设\n")
		fmt.Fprintf(os.Stderr, "  preset update        更新 Prompt 预设\n")
		fmt.Fprintf(os.Stderr, "  preset delete        删除 Prompt 预设\n")
		fmt.Fprintf(os.Stderr, "  mask list            列出所有掩码规则\n")
		fmt.Fprintf(os.Stderr, "  mask create          创建掩码规则\n")
		fmt.Fprintf(os.Stderr, "  mask delete          删除掩码规则\n")
		fmt.Fprintf(os.Stderr, "  tenant list          列出租户\n")
		fmt.Fprintf(os.Stderr, "  tenant show          查看租户详情\n")
		fmt.Fprintf(os.Stderr, "  tenant quota         查看/设置租户配额\n")
		fmt.Fprintf(os.Stderr, "  health               健康检查\n")
	}

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	// 校验全局参数
	if *output != "json" && *output != "table" {
		fmt.Fprintf(os.Stderr, "错误: --output 必须是 json 或 table\n")
		os.Exit(1)
	}

	client := NewClient(*apiBase, *token)

	cmd := flag.Arg(0)

	switch cmd {
	case "preset":
		handlePreset(client, flag.Args()[1:])
	case "mask":
		handleMask(client, flag.Args()[1:])
	case "tenant":
		handleTenant(client, flag.Args()[1:])
	case "health":
		handleHealth(client, flag.Args()[1:])
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n\n", cmd)
		flag.Usage()
		os.Exit(1)
	}
}
