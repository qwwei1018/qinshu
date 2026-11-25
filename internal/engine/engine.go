package engine

import (
	"strings"
	"sync"
	"time"
	"unicode"

	"qinshu/internal/config"
	"qinshu/internal/dict"
	"qinshu/internal/fuzzy"
)

// Action 表示输入法处理一次按键后的输出指令。
type Action struct {
	Handled    bool
	CommitText string
	UpdateUI   bool
}

// ViewState 提供给悬浮候选窗与状态栏渲染的只读快照。
type ViewState struct {
	Visible            bool
	Buffer             string
	Page               int
	TotalPages         int
	PageCandidates     []dict.Candidate
	SelectedIndex      int
	Language           string
	InputMode          string
	ChinesePunctuation bool
	FuzzyEnabled       bool
	TempEnglish        bool
	SmartMode          string // "" | "email" | "url"
}

var zhPunctMap = map[rune]string{
	',':  "，",
	'.':  "。",
	'?':  "？",
	'!':  "！",
	':':  "：",
	';':  "；",
	'(':  "（",
	')':  "）",
	'<':  "《",
	'>':  "》",
	'[':  "【",
	']':  "】",
	'\\': "、",
	'^':  "……",
	'_':  "——",
	'~':  "～",
	'$':  "￥",
}

var zhWeekdays = []string{"日", "一", "二", "三", "四", "五", "六"}

var quickSymbols = []string{"「」", "『』", "【】", "《》", "——", "……", "·", "℃", "￥", "→", "✓", "✗"}

// 常用邮箱后缀列表（按国内高频顺序排列）
var commonEmailDomains = []string{
	"gmail.com",
	"qq.com",
	"163.com",
	"126.com",
	"outlook.com",
	"foxmail.com",
	"icloud.com",
	"sina.com",
	"hotmail.com",
	"aliyun.com",
}

// 常用网址域名后缀列表
var commonURLTLDs = []string{
	".com",
	".cn",
	".net",
	".org",
	".com.cn",
	".io",
	".ai",
	".gov.cn",
	".edu.cn",
}

// Engine 是 Qin书 输入法核心状态机（参考万能五笔交互习惯）。
type Engine struct {
	mu      sync.Mutex
	baseDir string

	Config *config.Config
	Dict   *dict.Dictionary

	buffer            string
	candidates        []dict.Candidate
	page              int
	selectedIndex     int
	tempEnglish       bool
	smartMode         string // "" | "email" | "url"
	doubleQuoteOpen   bool
	singleQuoteOpen   bool
	lastCommittedRune rune
}

// New 创建 Qin书 核心引擎。
func New(cfg *config.Config, dictionary *dict.Dictionary, baseDir string) *Engine {
	return &Engine{
		baseDir:         baseDir,
		Config:          cfg,
		Dict:            dictionary,
		doubleQuoteOpen: true,
		singleQuoteOpen: true,
	}
}

// Flush 退出程序前立即将异步配置与用户词频刷盘。
func (e *Engine) Flush() {
	config.FlushAsync()
	_ = config.Save(e.Config, e.baseDir)
	e.Dict.FlushUserDict()
}

// HasActiveBuffer 返回当前是否处于编码输入状态。
func (e *Engine) HasActiveBuffer() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.buffer != ""
}

// ToggleLanguage 切换中/英文模式（对应 Shift 单击或 Ctrl+Space）。
func (e *Engine) ToggleLanguage() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	committed := ""
	if e.buffer != "" {
		committed = e.buffer
		e.clearLocked()
	}
	if e.Config.Language == config.LangZh {
		e.Config.Language = config.LangEn
	} else {
		e.Config.Language = config.LangZh
	}
	config.SaveAsync(e.Config, e.baseDir)
	return committed
}

// CycleInputMode 循环切换：五拼混输 -> 纯五笔 -> 纯拼音。
func (e *Engine) CycleInputMode() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch e.Config.InputMode {
	case config.ModeMixed:
		e.Config.InputMode = config.ModeWubi
	case config.ModeWubi:
		e.Config.InputMode = config.ModePinyin
	default:
		e.Config.InputMode = config.ModeMixed
	}
	e.Config.Language = config.LangZh
	config.SaveAsync(e.Config, e.baseDir)
	if e.buffer != "" {
		e.refreshCandidatesLocked()
	}
	return e.Config.InputMode
}

// SetInputMode 指定中文输入模式。
func (e *Engine) SetInputMode(mode string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if mode != config.ModeMixed && mode != config.ModeWubi && mode != config.ModePinyin {
		return
	}
	e.Config.InputMode = mode
	e.Config.Language = config.LangZh
	config.SaveAsync(e.Config, e.baseDir)
	if e.buffer != "" {
		e.refreshCandidatesLocked()
	}
}

// TogglePunctuation 切换中英文标点。
func (e *Engine) TogglePunctuation() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Config.ChinesePunctuation = !e.Config.ChinesePunctuation
	config.SaveAsync(e.Config, e.baseDir)
	return e.Config.ChinesePunctuation
}

// ToggleFuzzy 开关拼音模糊音。
func (e *Engine) ToggleFuzzy() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Config.FuzzyEnabled = !e.Config.FuzzyEnabled
	config.SaveAsync(e.Config, e.baseDir)
	if e.buffer != "" {
		e.refreshCandidatesLocked()
	}
	return e.Config.FuzzyEnabled
}

// Clear 清空当前编码缓冲。
func (e *Engine) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clearLocked()
}

func (e *Engine) clearLocked() {
	e.buffer = ""
	e.candidates = nil
	e.page = 0
	e.selectedIndex = 0
	e.tempEnglish = false
	e.smartMode = ""
}

// GetViewState 获取当前用于 UI 渲染的只读视图快照。
func (e *Engine) GetViewState() ViewState {
	e.mu.Lock()
	defer e.mu.Unlock()

	ps := e.Config.PageSize
	if ps < 5 {
		ps = 5
	}
	totalPages := 1
	if len(e.candidates) > 0 {
		totalPages = (len(e.candidates) + ps - 1) / ps
	}
	if e.page >= totalPages {
		e.page = totalPages - 1
	}
	if e.page < 0 {
		e.page = 0
	}
	start := e.page * ps
	end := start + ps
	if end > len(e.candidates) {
		end = len(e.candidates)
	}
	var pageItems []dict.Candidate
	if start < len(e.candidates) {
		pageItems = make([]dict.Candidate, end-start)
		copy(pageItems, e.candidates[start:end])
	}
	if len(pageItems) > 0 {
		if e.selectedIndex >= len(pageItems) {
			e.selectedIndex = len(pageItems) - 1
		}
		if e.selectedIndex < 0 {
			e.selectedIndex = 0
		}
	} else {
		e.selectedIndex = 0
	}

	return ViewState{
		Visible:            e.buffer != "",
		Buffer:             e.buffer,
		Page:               e.page,
		TotalPages:         totalPages,
		PageCandidates:     pageItems,
		SelectedIndex:      e.selectedIndex,
		Language:           e.Config.Language,
		InputMode:          e.Config.InputMode,
		ChinesePunctuation: e.Config.ChinesePunctuation,
		FuzzyEnabled:       e.Config.FuzzyEnabled,
		TempEnglish:        e.tempEnglish,
		SmartMode:          e.smartMode,
	}
}

// SelectCandidate 点击或快捷键选择当前页内第 indexInPage (0-based) 项候选词。
func (e *Engine) SelectCandidate(indexInPage int) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.selectCandidateLocked(indexInPage)
}

func (e *Engine) selectCandidateLocked(indexInPage int) (string, bool) {
	absIdx := e.page*e.Config.PageSize + indexInPage
	if absIdx < 0 || absIdx >= len(e.candidates) {
		return "", false
	}
	cand := e.candidates[absIdx]
	if cand.Composed && e.Config.AutoLearnPhrases {
		e.Dict.AddUserWord("pinyin", cand.Code, cand.Text, 2200)
	} else if e.Config.DynamicFrequency && (cand.Source == "wubi" || cand.Source == "pinyin" || cand.Source == "english") {
		e.Dict.BoostWord(cand.Source, cand.Code, cand.Text)
	}
	text := cand.Text
	runes := []rune(text)
	if len(runes) > 0 {
		e.lastCommittedRune = runes[len(runes)-1]
	}
	e.clearLocked()
	return text, true
}

func queryQuickCommands(code string) []dict.Candidate {
	now := time.Now()
	low := strings.ToLower(code)
	switch low {
	case "rq", "date":
		return []dict.Candidate{
			{Text: now.Format("2006年01月02日"), Code: low, Source: "command", Score: 3000, Hint: "日期"},
			{Text: now.Format("2006-01-02"), Code: low, Source: "command", Score: 2900, Hint: "日期"},
			{Text: now.Format("2006/01/02"), Code: low, Source: "command", Score: 2800, Hint: "日期"},
		}
	case "sj", "time":
		return []dict.Candidate{
			{Text: now.Format("15:04:05"), Code: low, Source: "command", Score: 3000, Hint: "时间"},
			{Text: now.Format("15:04"), Code: low, Source: "command", Score: 2900, Hint: "时间"},
			{Text: now.Format("2006-01-02 15:04:05"), Code: low, Source: "command", Score: 2800, Hint: "时间"},
		}
	case "xq", "week":
		wd := zhWeekdays[int(now.Weekday())]
		return []dict.Candidate{
			{Text: "星期" + wd, Code: low, Source: "command", Score: 3000, Hint: "星期"},
			{Text: "周" + wd, Code: low, Source: "command", Score: 2900, Hint: "星期"},
			{Text: now.Weekday().String(), Code: low, Source: "command", Score: 2800, Hint: "星期"},
		}
	case "fh":
		out := make([]dict.Candidate, 0, len(quickSymbols))
		for i, sym := range quickSymbols {
			out = append(out, dict.Candidate{
				Text:   sym,
				Code:   low,
				Source: "command",
				Score:  float64(3000 - i*10),
				Hint:   "符号",
			})
		}
		return out
	case "http":
		return []dict.Candidate{
			{Text: "http://", Code: low, Source: "command", Score: 3600, Hint: "网址"},
			{Text: "https://", Code: low, Source: "command", Score: 3550, Hint: "网址"},
			{Text: "http://www.", Code: low, Source: "command", Score: 3500, Hint: "网址"},
			{Text: "https://www.", Code: low, Source: "command", Score: 3450, Hint: "网址"},
		}
	case "https":
		return []dict.Candidate{
			{Text: "https://", Code: low, Source: "command", Score: 3600, Hint: "网址"},
			{Text: "https://www.", Code: low, Source: "command", Score: 3550, Hint: "网址"},
		}
	case "ftp":
		return []dict.Candidate{
			{Text: "ftp://", Code: low, Source: "command", Score: 3600, Hint: "网址"},
		}
	case "www":
		return []dict.Candidate{
			{Text: "www.", Code: low, Source: "command", Score: 3600, Hint: "网址"},
			{Text: "https://www.", Code: low, Source: "command", Score: 3550, Hint: "网址"},
			{Text: "http://www.", Code: low, Source: "command", Score: 3500, Hint: "网址"},
		}
	}
	return nil
}

// querySmartEmailCandidates 根据 zxc@ 或 zxc@g 生成常用邮箱后缀候选列表。
func querySmartEmailCandidates(buf string) []dict.Candidate {
	atIdx := strings.IndexByte(buf, '@')
	if atIdx <= 0 {
		return nil
	}
	localPart := buf[:atIdx]
	typedDomain := strings.ToLower(buf[atIdx+1:])

	var out []dict.Candidate
	seen := make(map[string]bool)
	for i, dom := range commonEmailDomains {
		if typedDomain == "" || strings.HasPrefix(dom, typedDomain) {
			full := localPart + "@" + dom
			if !seen[full] {
				seen[full] = true
				out = append(out, dict.Candidate{
					Text:   full,
					Code:   buf,
					Source: "command",
					Score:  float64(4000 - i*10),
					Hint:   "邮箱",
				})
			}
		}
	}
	// 如果用户输入的域名不在预设列表里，保留原串供随时上屏
	if !seen[buf] {
		out = append(out, dict.Candidate{
			Text:   buf,
			Code:   buf,
			Source: "command",
			Score:  3000,
			Hint:   "邮箱",
		})
	}
	return out
}

// querySmartURLCandidates 根据 http://baidu 或 www.google. 生成网址与常见顶级域名补全。
func querySmartURLCandidates(buf string) []dict.Candidate {
	var out []dict.Candidate
	seen := make(map[string]bool)
	add := func(text string, score float64) {
		if text != "" && !seen[text] {
			seen[text] = true
			out = append(out, dict.Candidate{
				Text:   text,
				Code:   buf,
				Source: "command",
				Score:  score,
				Hint:   "网址",
			})
		}
	}

	// 提取协议后的主机部分（不含路径 / ?）
	hostStart := 0
	if idx := strings.Index(buf, "://"); idx >= 0 {
		hostStart = idx + 3
	}
	rest := buf[hostStart:]
	hasPath := strings.ContainsAny(rest, "/?")

	if !hasPath && rest != "" && rest != "www." {
		// 判断最后一个 '.' 后面是否正在输入顶级域名（例如 www.baidu.c）
		lastDot := strings.LastIndexByte(rest, '.')
		if lastDot >= 0 && lastDot != len("www") {
			base := buf[:hostStart+lastDot]
			typedTLD := strings.ToLower(rest[lastDot:])
			for i, tld := range commonURLTLDs {
				if strings.HasPrefix(tld, typedTLD) {
					add(base+tld, float64(4000-i*10))
				}
			}
		} else if !strings.HasSuffix(buf, ".") {
			// 例如 http://baidu 或 www.google -> 直接补全 .com / .cn / .net ...
			for i, tld := range commonURLTLDs {
				add(buf+tld, float64(4000-i*10))
			}
		}
	}
	add(buf, 3200)
	return out
}

func isSmartURLChar(ch rune) bool {
	if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
		return true
	}
	switch ch {
	case '.', '-', '_', '/', ':', '?', '=', '&', '%', '#', '~', '+':
		return true
	}
	return false
}

func isSmartEmailChar(ch rune) bool {
	if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
		return true
	}
	switch ch {
	case '.', '-', '_', '+':
		return true
	}
	return false
}

func (e *Engine) refreshCandidatesLocked() {
	e.page = 0
	e.selectedIndex = 0
	if e.buffer == "" {
		e.candidates = nil
		return
	}

	// 0. 智能邮箱 / 网址模式
	if e.smartMode == "email" {
		e.candidates = querySmartEmailCandidates(e.buffer)
		return
	}
	if e.smartMode == "url" {
		e.candidates = querySmartURLCandidates(e.buffer)
		return
	}

	if e.tempEnglish {
		enCands := e.Dict.QueryEnglish(e.buffer, 10)
		if len(enCands) == 0 || enCands[0].Text != e.buffer {
			enCands = append([]dict.Candidate{{
				Text:   e.buffer,
				Code:   strings.ToLower(e.buffer),
				Source: "english",
				Score:  9999,
				Hint:   "英文",
			}}, enCands...)
		}
		e.candidates = enCands
		return
	}

	code := e.buffer
	mode := e.Config.InputMode
	showHint := e.Config.ShowReverseHint
	var combined []dict.Candidate
	seen := make(map[string]bool)

	addUnique := func(list []dict.Candidate) {
		for _, c := range list {
			if !seen[c.Text] {
				seen[c.Text] = true
				combined = append(combined, c)
			}
		}
	}

	// 1. 用户自定义短语（最高优先级，支持动态日期时间变量）
	addUnique(e.Dict.QueryPhrases(code))
	// 2. 内置快捷命令 (rq / sj / xq / fh / http / https / www / ftp)
	addUnique(queryQuickCommands(code))

	switch mode {
	case config.ModeWubi:
		addUnique(e.Dict.QueryWubi(code, showHint, 30))

	case config.ModePinyin:
		addUnique(e.Dict.QueryPinyin(code, e.Config.FuzzyEnabled, e.Config.FuzzyRules, showHint, 40))
		if len(code) >= 3 {
			addUnique(e.Dict.QueryEnglish(code, 3))
		}

	default:
		var wubiCands []dict.Candidate
		if len(code) <= 4 && !strings.Contains(code, "'") {
			wubiCands = e.Dict.QueryWubi(code, showHint, 15)
		}
		pinyinCands := e.Dict.QueryPinyin(code, e.Config.FuzzyEnabled, e.Config.FuzzyRules, showHint, 30)

		var exactWubi, prefixWubi []dict.Candidate
		for _, c := range wubiCands {
			if strings.HasPrefix(c.Hint, "~") {
				prefixWubi = append(prefixWubi, c)
			} else {
				exactWubi = append(exactWubi, c)
			}
		}
		topWubi := exactWubi
		var restWubi []dict.Candidate
		if len(exactWubi) > 3 {
			topWubi = exactWubi[:3]
			restWubi = exactWubi[3:]
		}
		addUnique(topWubi)
		addUnique(pinyinCands)
		addUnique(restWubi)
		addUnique(prefixWubi)
		if len(code) >= 3 {
			addUnique(e.Dict.QueryEnglish(code, 3))
		}
	}

	e.candidates = combined
}

func (e *Engine) commitPunctuationLocked(ch rune) string {
	if !e.Config.ChinesePunctuation {
		e.lastCommittedRune = ch
		return string(ch)
	}
	if ch == '.' && unicode.IsDigit(e.lastCommittedRune) {
		e.lastCommittedRune = '.'
		return "."
	}
	if ch == '"' {
		out := "“"
		if !e.doubleQuoteOpen {
			out = "”"
		}
		e.doubleQuoteOpen = !e.doubleQuoteOpen
		e.lastCommittedRune = []rune(out)[0]
		return out
	}
	if ch == '\'' {
		out := "‘"
		if !e.singleQuoteOpen {
			out = "’"
		}
		e.singleQuoteOpen = !e.singleQuoteOpen
		e.lastCommittedRune = []rune(out)[0]
		return out
	}
	if mapped, ok := zhPunctMap[ch]; ok {
		runes := []rune(mapped)
		if len(runes) > 0 {
			e.lastCommittedRune = runes[len(runes)-1]
		}
		return mapped
	}
	e.lastCommittedRune = ch
	return string(ch)
}

// HandleKey 处理单次按键事件，返回 Action。
func (e *Engine) HandleKey(keyName string, ch rune, ctrl, alt, shift bool) Action {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 1. 组合快捷键
	if ctrl && !alt {
		if !shift && keyName == "Space" && e.Config.CtrlSpaceToggle {
			committed := ""
			if e.buffer != "" {
				committed = e.buffer
				e.clearLocked()
			}
			if e.Config.Language == config.LangZh {
				e.Config.Language = config.LangEn
			} else {
				e.Config.Language = config.LangZh
			}
			config.SaveAsync(e.Config, e.baseDir)
			return Action{Handled: true, CommitText: committed, UpdateUI: true}
		}
		if ch == '.' || keyName == "period" {
			e.Config.ChinesePunctuation = !e.Config.ChinesePunctuation
			config.SaveAsync(e.Config, e.baseDir)
			return Action{Handled: true, UpdateUI: true}
		}
		if shift && strings.EqualFold(keyName, "m") {
			switch e.Config.InputMode {
			case config.ModeMixed:
				e.Config.InputMode = config.ModeWubi
			case config.ModeWubi:
				e.Config.InputMode = config.ModePinyin
			default:
				e.Config.InputMode = config.ModeMixed
			}
			e.Config.Language = config.LangZh
			config.SaveAsync(e.Config, e.baseDir)
			if e.buffer != "" {
				e.refreshCandidatesLocked()
			}
			return Action{Handled: true, UpdateUI: true}
		}
		if shift && strings.EqualFold(keyName, "f") {
			e.Config.FuzzyEnabled = !e.Config.FuzzyEnabled
			config.SaveAsync(e.Config, e.baseDir)
			if e.buffer != "" {
				e.refreshCandidatesLocked()
			}
			return Action{Handled: true, UpdateUI: true}
		}
		return Action{Handled: false}
	}
	if alt {
		return Action{Handled: false}
	}

	// 2. 英文模式直接放行
	if e.Config.Language == config.LangEn {
		if ch != 0 {
			e.lastCommittedRune = ch
		}
		return Action{Handled: false}
	}

	// 3. 候选框激活时的控制键与智能网址/邮箱识别
	if e.buffer != "" {
		switch keyName {
		case "BackSpace":
			e.buffer = e.buffer[:len(e.buffer)-1]
			if e.buffer == "" {
				e.clearLocked()
			} else {
				// 删掉 '@' 时退出邮箱模式，删掉 '://' 或 '.' 且不再是网址形态时退出网址模式
				if e.smartMode == "email" && !strings.Contains(e.buffer, "@") {
					e.smartMode = ""
				}
				if e.smartMode == "url" && !strings.Contains(e.buffer, "://") && !strings.HasPrefix(strings.ToLower(e.buffer), "www.") {
					e.smartMode = ""
				}
				e.refreshCandidatesLocked()
			}
			return Action{Handled: true, UpdateUI: true}

		case "Escape", "Delete":
			e.clearLocked()
			return Action{Handled: true, UpdateUI: true}

		case "Return":
			raw := e.buffer
			if len(raw) > 0 {
				e.lastCommittedRune = rune(raw[len(raw)-1])
			}
			e.clearLocked()
			return Action{Handled: true, CommitText: raw, UpdateUI: true}

		case "Space":
			text, ok := e.selectCandidateLocked(e.selectedIndex)
			if !ok {
				text = e.buffer
				e.clearLocked()
			}
			return Action{Handled: true, CommitText: text, UpdateUI: true}

		case "Down", "Right":
			ps := e.Config.PageSize
			start := e.page * ps
			pageLen := len(e.candidates) - start
			if pageLen > ps {
				pageLen = ps
			}
			if e.selectedIndex+1 < pageLen {
				e.selectedIndex++
			} else if (e.page+1)*ps < len(e.candidates) {
				e.page++
				e.selectedIndex = 0
			}
			return Action{Handled: true, UpdateUI: true}

		case "Up", "Left":
			ps := e.Config.PageSize
			if e.selectedIndex > 0 {
				e.selectedIndex--
			} else if e.page > 0 {
				e.page--
				e.selectedIndex = ps - 1
			}
			return Action{Handled: true, UpdateUI: true}
		}

		// 3a. 触发智能邮箱模式：任意英文串后输入 '@'（如 zxc@ -> gmail.com / qq.com / 163.com）
		if ch == '@' && e.smartMode == "" {
			e.smartMode = "email"
			e.tempEnglish = false
			e.buffer += "@"
			e.refreshCandidatesLocked()
			return Action{Handled: true, UpdateUI: true}
		}

		// 3b. 触发智能网址模式：http/https/ftp 后输入 ':' 自动补全 '://'；www 后输入 '.' 进入网址补全
		lowBuf := strings.ToLower(e.buffer)
		if ch == ':' && e.smartMode == "" && (lowBuf == "http" || lowBuf == "https" || lowBuf == "ftp") {
			e.smartMode = "url"
			e.tempEnglish = false
			e.buffer = lowBuf + "://"
			e.refreshCandidatesLocked()
			return Action{Handled: true, UpdateUI: true}
		}
		if ch == '.' && e.smartMode == "" && lowBuf == "www" {
			e.smartMode = "url"
			e.tempEnglish = false
			e.buffer = "www."
			e.refreshCandidatesLocked()
			return Action{Handled: true, UpdateUI: true}
		}

		// 3c. 已处于智能邮箱 / 网址模式时的字符输入与选词
		if e.smartMode != "" {
			if keyName == "Page_Down" {
				ps := e.Config.PageSize
				if (e.page+1)*ps < len(e.candidates) {
					e.page++
					e.selectedIndex = 0
				}
				return Action{Handled: true, UpdateUI: true}
			}
			if keyName == "Page_Up" {
				if e.page > 0 {
					e.page--
					e.selectedIndex = 0
				}
				return Action{Handled: true, UpdateUI: true}
			}
			// 刚输入完 zxc@ 时，数字键 1-9 直接选对应邮箱后缀（如 1=gmail.com, 2=qq.com, 3=163.com）
			if e.smartMode == "email" && strings.HasSuffix(e.buffer, "@") && ch >= '1' && ch <= '9' {
				idx := int(ch - '1')
				if idx < e.Config.PageSize {
					if text, ok := e.selectCandidateLocked(idx); ok {
						return Action{Handled: true, CommitText: text, UpdateUI: true}
					}
				}
			}
			if (e.smartMode == "email" && isSmartEmailChar(ch)) || (e.smartMode == "url" && isSmartURLChar(ch)) {
				if len(e.buffer) < 128 {
					e.buffer += string(ch)
					e.refreshCandidatesLocked()
				}
				return Action{Handled: true, UpdateUI: true}
			}
			// 智能邮箱/网址模式下输入逗号、分号等非地址标点：先上屏当前高亮地址再追加标点，避免误触发翻页
			if _, isP := zhPunctMap[ch]; isP || ch == '"' || ch == '\'' {
				prefix, ok := e.selectCandidateLocked(e.selectedIndex)
				if !ok {
					prefix = e.buffer
					e.clearLocked()
				}
				punct := e.commitPunctuationLocked(ch)
				return Action{Handled: true, CommitText: prefix + punct, UpdateUI: true}
			}
			return Action{Handled: true}
		}

		if keyName == "equal" || keyName == "Page_Down" || ch == '=' || ch == '.' {
			ps := e.Config.PageSize
			if (e.page+1)*ps < len(e.candidates) {
				e.page++
				e.selectedIndex = 0
			}
			return Action{Handled: true, UpdateUI: true}
		}
		if keyName == "minus" || keyName == "Page_Up" || ch == '-' || ch == ',' {
			if e.page > 0 {
				e.page--
				e.selectedIndex = 0
			}
			return Action{Handled: true, UpdateUI: true}
		}

		if ch >= '1' && ch <= '9' {
			idx := int(ch - '1')
			if idx < e.Config.PageSize {
				if text, ok := e.selectCandidateLocked(idx); ok {
					return Action{Handled: true, CommitText: text, UpdateUI: true}
				}
			}
			return Action{Handled: true}
		}

		if e.Config.SemicolonSelect2nd && !e.tempEnglish {
			if ch == ';' {
				if text, ok := e.selectCandidateLocked(1); ok {
					return Action{Handled: true, CommitText: text, UpdateUI: true}
				}
			}
			if ch == '\'' && e.Config.InputMode == config.ModeWubi {
				if text, ok := e.selectCandidateLocked(2); ok {
					return Action{Handled: true, CommitText: text, UpdateUI: true}
				}
			}
		}

		if ch == '\'' && !e.tempEnglish && (e.Config.InputMode == config.ModePinyin || e.Config.InputMode == config.ModeMixed) {
			if !strings.HasSuffix(e.buffer, "'") {
				e.buffer += "'"
				e.refreshCandidatesLocked()
			}
			return Action{Handled: true, UpdateUI: true}
		}
	}

	// 4. 字母键输入
	if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
		if e.buffer == "" && ch >= 'A' && ch <= 'Z' {
			e.tempEnglish = true
			e.buffer = string(ch)
			e.refreshCandidatesLocked()
			return Action{Handled: true, UpdateUI: true}
		}
		if e.tempEnglish {
			e.buffer += string(ch)
			e.refreshCandidatesLocked()
			return Action{Handled: true, UpdateUI: true}
		}

		lowerCh := string(unicode.ToLower(ch))

		if e.Config.InputMode == config.ModeWubi && e.Config.PushCommit5thCode && len(e.buffer) == 4 {
			topText, _ := e.selectCandidateLocked(0)
			e.buffer = lowerCh
			e.refreshCandidatesLocked()
			return Action{Handled: true, CommitText: topText, UpdateUI: true}
		}

		if len(e.buffer) >= 32 {
			return Action{Handled: true}
		}

		e.buffer += lowerCh
		e.refreshCandidatesLocked()

		if e.Config.AutoCommit4thCode && len(e.buffer) == 4 && !strings.Contains(e.buffer, "z") {
			exactWubiCount := 0
			exactPyCount := 0
			specialCount := 0
			for _, c := range e.candidates {
				if c.Source == "phrase" || c.Source == "command" {
					specialCount++
				} else if c.Source == "wubi" && c.Code == e.buffer {
					exactWubiCount++
				} else if c.Source == "pinyin" && c.Code == e.buffer {
					exactPyCount++
				}
			}
			if specialCount == 0 {
				if e.Config.InputMode == config.ModeWubi && exactWubiCount == 1 {
					committed, _ := e.selectCandidateLocked(0)
					return Action{Handled: true, CommitText: committed, UpdateUI: true}
				}
				if e.Config.InputMode == config.ModeMixed && exactWubiCount == 1 && exactPyCount == 0 && !fuzzy.IsPotentialPinyinPrefix(e.buffer) {
					committed, _ := e.selectCandidateLocked(0)
					return Action{Handled: true, CommitText: committed, UpdateUI: true}
				}
			}
		}

		return Action{Handled: true, UpdateUI: true}
	}

	// 5. 中文标点处理
	_, isZhPunct := zhPunctMap[ch]
	if isZhPunct || ch == '"' || ch == '\'' {
		prefix := ""
		if e.buffer != "" {
			if sel, ok := e.selectCandidateLocked(e.selectedIndex); ok {
				prefix = sel
			} else {
				prefix = e.buffer
				e.clearLocked()
			}
		}
		punct := e.commitPunctuationLocked(ch)
		return Action{Handled: true, CommitText: prefix + punct, UpdateUI: true}
	}

	if ch != 0 && e.buffer == "" {
		e.lastCommittedRune = ch
	}
	return Action{Handled: false}
}
