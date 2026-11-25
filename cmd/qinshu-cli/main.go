package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"qinshu/internal/config"
	"qinshu/internal/dict"
	"qinshu/internal/engine"
)

func main() {
	importKind := flag.String("import-kind", "", "非交互导入词库类型: wubi 或 pinyin")
	importFile := flag.String("import-file", "", "非交互导入外部词库文件路径")
	flag.Parse()

	baseDir := config.BaseDir()
	cfg := config.Load(baseDir)
	dictionary := dict.New(filepath.Join(baseDir, "data"))

	// 支持批处理脚本非交互一键导入外部大词库
	if *importKind != "" && *importFile != "" {
		cnt, dst, err := dictionary.ImportFile(*importKind, *importFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "导入失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("成功导入 %d 条 [%s] 词条至 %s\n", cnt, *importKind, dst)
		return
	}

	eng := engine.New(cfg, dictionary, baseDir)
	defer eng.Flush()

	fmt.Printf("=== %s 输入法命令行体验与词库工具 (Go v%s) ===\n", config.AppName, config.AppVersion)
	fmt.Println("直接输入五笔/拼音/短语编码按回车查看候选（如 woshizhongguoren、zongguo、aawt、fp、qs、rq、xq、fh），或使用命令：")
	fmt.Println("  :mode mixed|wubi|pinyin   切换输入模式")
	fmt.Println("  :fuzzy on|off|south|all   设置模糊音开关/南方预设/全开")
	fmt.Println("  :phrase 缩写 短语内容      新增自定义短语（支持 $year/$month/$day/$weekday）")
	fmt.Println("  :add wubi|pinyin 编码 词条 手动造词")
	fmt.Println("  :import wubi|pinyin 路径  导入 Rime/万能五笔外部词库")
	fmt.Println("  :stats                    查看当前词库与短语统计")
	fmt.Println("  :quit                     退出")
	fmt.Println()

	sc := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("[%s|模糊:%v] > ", eng.Config.InputMode, eng.Config.FuzzyEnabled)
		if !sc.Scan() {
			break
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if line == ":quit" || line == ":q" {
			break
		}
		if line == ":stats" {
			st := dictionary.Stats()
			fmt.Printf("词库统计：五笔 %d 键 / %d 条 | 拼音 %d 键 / %d 条 | 英文 %d 条 | 自定义短语 %d 条 | 用户词频 %d 条\n",
				st.WubiKeys, st.WubiEntries, st.PinyinKeys, st.PinyinEntries, st.EnglishEntries, st.PhraseEntries, st.UserEntries)
			continue
		}
		if strings.HasPrefix(line, ":mode ") {
			m := strings.TrimSpace(strings.TrimPrefix(line, ":mode "))
			eng.SetInputMode(m)
			fmt.Println("已切换模式为:", eng.Config.InputMode)
			continue
		}
		if strings.HasPrefix(line, ":fuzzy ") {
			arg := strings.TrimSpace(strings.TrimPrefix(line, ":fuzzy "))
			switch arg {
			case "on":
				eng.Config.FuzzyEnabled = true
			case "off":
				eng.Config.FuzzyEnabled = false
			case "south":
				eng.Config.ApplySouthPreset()
			case "all":
				eng.Config.SetAllFuzzy(true)
			}
			config.SaveAsync(eng.Config, baseDir)
			fmt.Println("模糊音状态已更新:", eng.Config.FuzzyEnabled)
			continue
		}
		if strings.HasPrefix(line, ":phrase ") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, ":phrase "))
			parts := strings.SplitN(rest, " ", 2)
			if len(parts) == 2 {
				if err := dictionary.AddPhrase(parts[0], parts[1]); err != nil {
					fmt.Println("添加失败:", err)
				} else {
					fmt.Printf("已添加自定义短语: %s -> %s\n", parts[0], parts[1])
				}
			} else {
				fmt.Println("用法: :phrase 缩写 短语内容")
			}
			continue
		}
		if strings.HasPrefix(line, ":add ") {
			parts := strings.Fields(strings.TrimPrefix(line, ":add "))
			if len(parts) >= 3 {
				dictionary.AddUserWord(parts[0], parts[1], parts[2], 2000)
				fmt.Printf("已添加自定义词条: [%s] %s -> %s\n", parts[0], parts[1], parts[2])
			} else {
				fmt.Println("用法: :add wubi|pinyin 编码 词条")
			}
			continue
		}
		if strings.HasPrefix(line, ":import ") {
			parts := strings.Fields(strings.TrimPrefix(line, ":import "))
			if len(parts) >= 2 {
				cnt, dst, err := dictionary.ImportFile(parts[0], parts[1])
				if err != nil {
					fmt.Println("导入失败:", err)
				} else {
					fmt.Printf("成功导入 %d 条词条至 %s\n", cnt, dst)
				}
			} else {
				fmt.Println("用法: :import wubi|pinyin 词库文件路径")
			}
			continue
		}

		eng.Clear()
		var autoCommitted strings.Builder
		for _, r := range line {
			act := eng.HandleKey(strings.ToLower(string(r)), r, false, false, false)
			if act.CommitText != "" {
				autoCommitted.WriteString(act.CommitText)
			}
		}
		if autoCommitted.Len() > 0 {
			fmt.Println("自动上屏:", autoCommitted.String())
		}
		vs := eng.GetViewState()
		if vs.Visible {
			fmt.Printf("候选 (%s, %d/%d):\n", vs.Buffer, vs.Page+1, vs.TotalPages)
			for i, c := range vs.PageCandidates {
				if c.Hint != "" {
					fmt.Printf("  %d. %s  (%s)\n", i+1, c.Text, c.Hint)
				} else {
					fmt.Printf("  %d. %s\n", i+1, c.Text)
				}
			}
		}
	}
}
