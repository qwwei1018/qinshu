package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	AppName    = "Qin书"
	AppVersion = "2.0.2"

	ModeMixed  = "mixed"  // 五笔拼音混输（参考万能五笔）
	ModeWubi   = "wubi"   // 纯五笔86
	ModePinyin = "pinyin" // 纯拼音

	LangZh = "zh"
	LangEn = "en"
)

// Theme 定义 Win32 GDI 悬浮候选窗与状态栏的配色方案（BGR 0x00BBGGRR 格式）。
type Theme struct {
	Name     string
	Bg       uint32
	Border   uint32
	CodeFg   uint32
	TextFg   uint32
	IndexFg  uint32
	HintFg   uint32
	ActiveBg uint32
	ActiveFg uint32
	StatusBg uint32
	StatusFg uint32
	Accent   uint32
	Divider  uint32
}

func rgb(r, g, b uint32) uint32 {
	return (b << 16) | (g << 8) | r
}

// Themes 内置 3 套主题。
var Themes = map[string]Theme{
	"classic_light": {
		Name:     "简约白（默认）",
		Bg:       rgb(255, 255, 255),
		Border:   rgb(203, 213, 225),
		CodeFg:   rgb(29, 78, 216),
		TextFg:   rgb(30, 41, 59),
		IndexFg:  rgb(100, 116, 139),
		HintFg:   rgb(15, 118, 110),
		ActiveBg: rgb(239, 246, 255),
		ActiveFg: rgb(30, 64, 175),
		StatusBg: rgb(248, 250, 252),
		StatusFg: rgb(15, 23, 42),
		Accent:   rgb(37, 99, 235),
		Divider:  rgb(226, 232, 240),
	},
	"wanneng_blue": {
		Name:     "万能蓝",
		Bg:       rgb(240, 247, 255),
		Border:   rgb(96, 165, 250),
		CodeFg:   rgb(30, 58, 138),
		TextFg:   rgb(15, 23, 42),
		IndexFg:  rgb(59, 130, 246),
		HintFg:   rgb(3, 105, 161),
		ActiveBg: rgb(219, 234, 254),
		ActiveFg: rgb(29, 78, 216),
		StatusBg: rgb(29, 78, 216),
		StatusFg: rgb(255, 255, 255),
		Accent:   rgb(191, 219, 254),
		Divider:  rgb(59, 130, 246),
	},
	"dark_slate": {
		Name:     "深邃暗",
		Bg:       rgb(30, 41, 59),
		Border:   rgb(71, 85, 105),
		CodeFg:   rgb(56, 189, 248),
		TextFg:   rgb(241, 245, 249),
		IndexFg:  rgb(148, 163, 184),
		HintFg:   rgb(52, 211, 153),
		ActiveBg: rgb(51, 65, 85),
		ActiveFg: rgb(255, 255, 255),
		StatusBg: rgb(15, 23, 42),
		StatusFg: rgb(248, 250, 252),
		Accent:   rgb(56, 189, 248),
		Divider:  rgb(51, 65, 85),
	},
}

// Config 存储 Qin书 输入法全部可持久化设置。
type Config struct {
	InputMode            string          `json:"input_mode"`
	Language             string          `json:"language"`
	ChinesePunctuation   bool            `json:"chinese_punctuation"`
	FuzzyEnabled         bool            `json:"fuzzy_enabled"`
	FuzzyRules           map[string]bool `json:"fuzzy_rules"`
	AutoCommit4thCode    bool            `json:"auto_commit_4th_code"`
	PushCommit5thCode    bool            `json:"push_commit_5th_code"`
	ShowReverseHint      bool            `json:"show_reverse_hint"`
	PageSize             int             `json:"page_size"`
	HorizontalCandidates bool            `json:"horizontal_candidates"`
	SemicolonSelect2nd   bool            `json:"semicolon_select_2nd"`
	DynamicFrequency     bool            `json:"dynamic_frequency"`
	AutoLearnPhrases     bool            `json:"auto_learn_phrases"`
	CtrlSpaceToggle      bool            `json:"ctrl_space_toggle"`
	Theme                string          `json:"theme"`
	StatusBarX           int             `json:"status_bar_x"`
	StatusBarY           int             `json:"status_bar_y"`
	HasSavedStatusPos    bool            `json:"has_saved_status_pos"`
}

// DefaultFuzzyRules 返回默认开启的 12 组模糊音配置。
func DefaultFuzzyRules() map[string]bool {
	return map[string]bool{
		"z_zh":     true,
		"c_ch":     true,
		"s_sh":     true,
		"l_n":      true,
		"f_h":      false,
		"r_l":      false,
		"k_g":      false,
		"an_ang":   true,
		"en_eng":   true,
		"in_ing":   true,
		"ian_iang": false,
		"uan_uang": false,
	}
}

// Default 返回默认配置（参考万能五笔默认体验）。
func Default() *Config {
	return &Config{
		InputMode:            ModeMixed,
		Language:             LangZh,
		ChinesePunctuation:   true,
		FuzzyEnabled:         true,
		FuzzyRules:           DefaultFuzzyRules(),
		AutoCommit4thCode:    true,
		PushCommit5thCode:    true,
		ShowReverseHint:      true,
		PageSize:             5,
		HorizontalCandidates: true,
		SemicolonSelect2nd:   true,
		DynamicFrequency:     true,
		AutoLearnPhrases:     true,
		CtrlSpaceToggle:      true,
		Theme:                "classic_light",
	}
}

// Clone 返回配置的深拷贝，便于异步防抖落盘时无锁序列化。
func (c *Config) Clone() *Config {
	cp := *c
	cp.FuzzyRules = make(map[string]bool, len(c.FuzzyRules))
	for k, v := range c.FuzzyRules {
		cp.FuzzyRules[k] = v
	}
	return &cp
}

// ApplySouthPreset 一键套用南方常用模糊音预设。
func (c *Config) ApplySouthPreset() {
	southOn := map[string]bool{
		"z_zh": true, "c_ch": true, "s_sh": true,
		"l_n": true, "f_h": true,
		"an_ang": true, "en_eng": true, "in_ing": true,
	}
	if c.FuzzyRules == nil {
		c.FuzzyRules = DefaultFuzzyRules()
	}
	for k := range c.FuzzyRules {
		c.FuzzyRules[k] = southOn[k]
	}
	c.FuzzyEnabled = true
}

// SetAllFuzzy 全部开启或关闭 12 组模糊音。
func (c *Config) SetAllFuzzy(enabled bool) {
	if c.FuzzyRules == nil {
		c.FuzzyRules = DefaultFuzzyRules()
	}
	for k := range c.FuzzyRules {
		c.FuzzyRules[k] = enabled
	}
	c.FuzzyEnabled = enabled
}

// CurrentTheme 返回当前生效的主题配色。
func (c *Config) CurrentTheme() Theme {
	if t, ok := Themes[c.Theme]; ok {
		return t
	}
	return Themes["classic_light"]
}

// BaseDir 获取程序根目录（优先取可执行文件所在目录，若不存在 data/ 则回退到当前工作目录）。
func BaseDir() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if info, err2 := os.Stat(filepath.Join(dir, "data")); err2 == nil && info.IsDir() {
			return dir
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}


// UserDataDir 返回可写的用户数据目录：
// - 绿色便携模式下直接使用 <baseDir>/data
// - 当安装到只读目录（如 C:\Program Files\QinShu）时自动回退至 %APPDATA%\QinShu\data，避免 UAC 权限拒绝
func UserDataDir(baseDir string) string {
	if baseDir == "" {
		baseDir = BaseDir()
	}
	localData := filepath.Join(baseDir, "data")
	if os.MkdirAll(localData, 0o755) == nil {
		probe := filepath.Join(localData, ".write_probe")
		if f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
			_ = f.Close()
			_ = os.Remove(probe)
			return localData
		}
	}
	if cfgRoot, err := os.UserConfigDir(); err == nil && cfgRoot != "" {
		appData := filepath.Join(cfgRoot, "QinShu", "data")
		_ = os.MkdirAll(appData, 0o755)
		return appData
	}
	return localData
}

// Load 从用户数据目录加载 config.json，不存在则自动生成默认配置。
func Load(baseDir string) *Config {
	if baseDir == "" {
		baseDir = BaseDir()
	}
	cfg := Default()
	path := filepath.Join(UserDataDir(baseDir), "config.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		_ = Save(cfg, baseDir)
		return cfg
	}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return Default()
	}
	if cfg.FuzzyRules == nil {
		cfg.FuzzyRules = DefaultFuzzyRules()
	} else {
		for k, v := range DefaultFuzzyRules() {
			if _, exists := cfg.FuzzyRules[k]; !exists {
				cfg.FuzzyRules[k] = v
			}
		}
	}
	if cfg.InputMode != ModeMixed && cfg.InputMode != ModeWubi && cfg.InputMode != ModePinyin {
		cfg.InputMode = ModeMixed
	}
	if cfg.Language != LangZh && cfg.Language != LangEn {
		cfg.Language = LangZh
	}
	if cfg.PageSize < 5 {
		cfg.PageSize = 5
	}
	if cfg.PageSize > 9 {
		cfg.PageSize = 9
	}
	return cfg
}

// Save 同步将配置原子写入 data/config.json（适合退出前立即落盘）。
func Save(cfg *Config, baseDir string) error {
	if baseDir == "" {
		baseDir = BaseDir()
	}
	dataDir := UserDataDir(baseDir)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dst := filepath.Join(dataDir, "config.json")
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

var (
	asyncMu      sync.Mutex
	asyncTimer   *time.Timer
	pendingCfg   *Config
	pendingDir   string
)

// SaveAsync 异步防抖落盘配置（消除键盘钩子热路径上的同步磁盘 I/O）。
func SaveAsync(cfg *Config, baseDir string) {
	snap := cfg.Clone()
	asyncMu.Lock()
	defer asyncMu.Unlock()
	pendingCfg = snap
	pendingDir = baseDir
	if asyncTimer != nil {
		asyncTimer.Reset(800 * time.Millisecond)
		return
	}
	asyncTimer = time.AfterFunc(800*time.Millisecond, func() {
		FlushAsync()
	})
}

// FlushAsync 立即将尚未落盘的异步配置写入磁盘。
func FlushAsync() {
	asyncMu.Lock()
	cfg := pendingCfg
	dir := pendingDir
	pendingCfg = nil
	if asyncTimer != nil {
		asyncTimer.Stop()
		asyncTimer = nil
	}
	asyncMu.Unlock()
	if cfg != nil {
		_ = Save(cfg, dir)
	}
}
