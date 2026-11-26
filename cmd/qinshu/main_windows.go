//go:build windows

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"qinshu/internal/config"
	"qinshu/internal/dict"
	"qinshu/internal/engine"
	"qinshu/internal/win32"
)

func main() {
	tsfMode := flag.Bool("tsf", false, "以 Win11 原生 TSF COM 后端轻量服务模式运行（通过命名管道服务 QinShuTSF.dll）")
	flag.Parse()

	// 低资源占用优化：适配十万级内置五笔86与拼音大词库，避免在 64MB 边界频繁触发 GC
	debug.SetGCPercent(60)
	debug.SetMemoryLimit(160 << 20)

	baseDir := config.BaseDir()
	cfg := config.Load(baseDir)
	dictionary := dict.NewWithUserDir(filepath.Join(baseDir, "data"), config.UserDataDir(baseDir))
	eng := engine.New(cfg, dictionary, baseDir)

	if err := win32.RunApp(eng, baseDir, *tsfMode); err != nil {
		fmt.Fprintf(os.Stderr, "Qin书输入法启动失败: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
