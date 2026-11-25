//go:build !windows

package main

import "fmt"

func main() {
	fmt.Println("Qin书桌面全局输入法面向 Windows 11 编译运行。")
	fmt.Println("请在 Windows 11 上执行: go build -ldflags=\"-H windowsgui -s -w\" -o Qin书.exe ./cmd/qinshu")
	fmt.Println("或运行命令行体验与测试工具: go run ./cmd/qinshu-cli")
}
