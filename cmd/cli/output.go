package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
)

// printJSON 以 JSON 格式输出
func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "2")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "JSON 编码失败: %v\n", err)
		os.Exit(1)
	}
}

// printTable 以表格格式输出
func printTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "\t")
		}
		fmt.Fprint(w, h)
	}
	fmt.Fprintln(w)
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(w, "\t")
			}
			fmt.Fprint(w, cell)
		}
		fmt.Fprintln(w)
	}
	w.Flush()
}

// printOutput 根据全局 --output 标志输出结果
func printOutput(headers []string, rows [][]string, v any) {
	switch *output {
	case "json":
		printJSON(v)
	case "table":
		if len(rows) == 0 {
			fmt.Println("(无数据)")
		} else {
			printTable(headers, rows)
		}
	}
}
