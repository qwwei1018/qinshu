package dict

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"qinshu/internal/fuzzy"
)

// Candidate 表示一条候选项。
type Candidate struct {
	Text     string
	Code     string
	Source   string // "wubi" | "pinyin" | "english" | "command" | "phrase"
	Score    float64
	Hint     string // 反查提示，例如 "五:khlg"、"拼:zhong"、"短语"
	Composed bool   // 是否由多词动态拼接的整句候选
}

// Stats 词库统计信息。
type Stats struct {
	WubiKeys       int
	WubiEntries    int
	PinyinKeys     int
	PinyinEntries  int
	EnglishEntries int
	PhraseEntries  int
	UserEntries    int
}

type wordEntry struct {
	word   string
	weight int
}

type pinyinEntry struct {
	word      string
	weight    int
	syllables string
}

var phraseWeekdays = []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}

// Dictionary 管理五笔86、拼音（含简拼、模糊音与整句组词）、英文词库、自定义短语及异步防抖用户动态词频。
type Dictionary struct {
	mu sync.RWMutex

	dataDir      string // 只读内置词库目录
	userDataDir  string // 可写用户数据目录
	userDictPath string
	phrasesPath  string

	wubiMap           map[string][]wordEntry
	wubiKeys          []string // 有序键表，支持 O(log N) 二分前缀查询
	pinyinMap         map[string][]pinyinEntry
	pinyinKeys        []string // 有序键表，支持 O(log N) 二分前缀查询
	pinyinInitialsMap map[string][]pinyinEntry
	englishPrefixMap  map[string][]wordEntry
	phraseMap         map[string][]string // 自定义短语: abbr -> []template

	wordToWubi   map[string]string
	charToPinyin map[string]string

	// source -> code -> word -> bonus
	userFreq map[string]map[string]map[string]int

	saveTimer *time.Timer
	saveDirty bool
}

// New 创建并加载词库实例（便携模式：内置词库与用户数据同目录）。
func New(dataDir string) *Dictionary {
	return NewWithUserDir(dataDir, dataDir)
}

// NewWithUserDir 创建词库实例，支持只读安装目录与可写 %APPDATA% 用户目录分离。
func NewWithUserDir(systemDataDir, userDataDir string) *Dictionary {
	if userDataDir == "" {
		userDataDir = systemDataDir
	}
	_ = os.MkdirAll(userDataDir, 0o755)
	userPhrases := filepath.Join(userDataDir, "phrases.txt")
	if userDataDir != systemDataDir {
		if _, err := os.Stat(userPhrases); os.IsNotExist(err) {
			if seed, err2 := os.ReadFile(filepath.Join(systemDataDir, "phrases.txt")); err2 == nil {
				_ = os.WriteFile(userPhrases, seed, 0o644)
			}
		}
	}
	d := &Dictionary{
		dataDir:           systemDataDir,
		userDataDir:       userDataDir,
		userDictPath:      filepath.Join(userDataDir, "user_dict.json"),
		phrasesPath:       userPhrases,
		wubiMap:           make(map[string][]wordEntry),
		pinyinMap:         make(map[string][]pinyinEntry),
		pinyinInitialsMap: make(map[string][]pinyinEntry),
		englishPrefixMap:  make(map[string][]wordEntry),
		phraseMap:         make(map[string][]string),
		wordToWubi:        make(map[string]string),
		charToPinyin:      make(map[string]string),
		userFreq: map[string]map[string]map[string]int{
			"wubi":    make(map[string]map[string]int),
			"pinyin":  make(map[string]map[string]int),
			"english": make(map[string]map[string]int),
		},
	}
	d.Reload()
	return d
}

// PhrasesPath 返回自定义短语文件路径。
func (d *Dictionary) PhrasesPath() string {
	return d.phrasesPath
}

// Stats 返回当前已加载词库的条目统计。
func (d *Dictionary) Stats() Stats {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var s Stats
	s.WubiKeys = len(d.wubiMap)
	for _, list := range d.wubiMap {
		s.WubiEntries += len(list)
	}
	s.PinyinKeys = len(d.pinyinMap)
	for _, list := range d.pinyinMap {
		s.PinyinEntries += len(list)
	}
	enSeen := make(map[string]bool)
	for _, list := range d.englishPrefixMap {
		for _, e := range list {
			enSeen[e.word] = true
		}
	}
	s.EnglishEntries = len(enSeen)
	for _, list := range d.phraseMap {
		s.PhraseEntries += len(list)
	}
	for _, sec := range d.userFreq {
		for _, bucket := range sec {
			s.UserEntries += len(bucket)
		}
	}
	return s
}

// Reload 重新加载全部内置词库、扩展词库、自定义短语与用户词频，并构建去重后的 O(log N) 有序前缀索引。
func (d *Dictionary) Reload() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.wubiMap = make(map[string][]wordEntry)
	d.pinyinMap = make(map[string][]pinyinEntry)
	d.pinyinInitialsMap = make(map[string][]pinyinEntry)
	d.englishPrefixMap = make(map[string][]wordEntry)
	d.phraseMap = make(map[string][]string)
	d.wordToWubi = make(map[string]string)
	d.charToPinyin = make(map[string]string)

	d.loadWubi(filepath.Join(d.dataDir, "wubi86.txt"))
	d.loadWubi(filepath.Join(d.userDataDir, "wubi86_extra.txt"))
	d.loadPinyin(filepath.Join(d.dataDir, "pinyin.txt"))
	d.loadPinyin(filepath.Join(d.userDataDir, "pinyin_extra.txt"))
	d.loadEnglish(filepath.Join(d.dataDir, "english.txt"))
	d.loadPhrases(d.phrasesPath)
	d.loadUserDict()

	d.wubiKeys = make([]string, 0, len(d.wubiMap))
	for k, list := range d.wubiMap {
		dedup := dedupWordEntries(list)
		sort.Slice(dedup, func(i, j int) bool {
			if dedup[i].weight != dedup[j].weight {
				return dedup[i].weight > dedup[j].weight
			}
			return dedup[i].word < dedup[j].word
		})
		d.wubiMap[k] = dedup
		d.wubiKeys = append(d.wubiKeys, k)
	}
	sort.Strings(d.wubiKeys)

	d.pinyinKeys = make([]string, 0, len(d.pinyinMap))
	for k, list := range d.pinyinMap {
		dedup := dedupPinyinEntries(list)
		sort.Slice(dedup, func(i, j int) bool {
			if dedup[i].weight != dedup[j].weight {
				return dedup[i].weight > dedup[j].weight
			}
			return dedup[i].word < dedup[j].word
		})
		d.pinyinMap[k] = dedup
		d.pinyinKeys = append(d.pinyinKeys, k)
	}
	sort.Strings(d.pinyinKeys)

	for k, list := range d.pinyinInitialsMap {
		dedup := dedupPinyinEntries(list)
		sort.Slice(dedup, func(i, j int) bool {
			if dedup[i].weight != dedup[j].weight {
				return dedup[i].weight > dedup[j].weight
			}
			return dedup[i].word < dedup[j].word
		})
		d.pinyinInitialsMap[k] = dedup
	}

	for k, list := range d.englishPrefixMap {
		dedup := dedupWordEntries(list)
		sort.Slice(dedup, func(i, j int) bool {
			if dedup[i].weight != dedup[j].weight {
				return dedup[i].weight > dedup[j].weight
			}
			if len(dedup[i].word) != len(dedup[j].word) {
				return len(dedup[i].word) < len(dedup[j].word)
			}
			return dedup[i].word < dedup[j].word
		})
		d.englishPrefixMap[k] = dedup
	}
}

func dedupWordEntries(list []wordEntry) []wordEntry {
	best := make(map[string]int, len(list))
	for _, e := range list {
		if prev, ok := best[e.word]; !ok || e.weight > prev {
			best[e.word] = e.weight
		}
	}
	out := make([]wordEntry, 0, len(best))
	for w, wt := range best {
		out = append(out, wordEntry{word: w, weight: wt})
	}
	return out
}

func dedupPinyinEntries(list []pinyinEntry) []pinyinEntry {
	best := make(map[string]pinyinEntry, len(list))
	for _, e := range list {
		if prev, ok := best[e.word]; !ok || e.weight > prev.weight {
			best[e.word] = e
		}
	}
	out := make([]pinyinEntry, 0, len(best))
	for _, e := range best {
		out = append(out, e)
	}
	return out
}

func (d *Dictionary) loadPhrases(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var abbr, content string
		if idx := strings.IndexByte(line, '='); idx > 0 {
			abbr = strings.ToLower(strings.TrimSpace(line[:idx]))
			content = strings.TrimSpace(line[idx+1:])
		} else if idx := strings.IndexByte(line, '\t'); idx > 0 {
			abbr = strings.ToLower(strings.TrimSpace(line[:idx]))
			content = strings.TrimSpace(line[idx+1:])
		}
		if abbr != "" && content != "" {
			d.phraseMap[abbr] = append(d.phraseMap[abbr], content)
		}
	}
}

// expandPhraseVars 展开自定义短语中的动态日期/时间占位符。
func expandPhraseVars(tpl string) string {
	if !strings.Contains(tpl, "$") {
		return tpl
	}
	now := time.Now()
	repl := strings.NewReplacer(
		"$year", now.Format("2006"),
		"$month", now.Format("01"),
		"$day", now.Format("02"),
		"$hour", now.Format("15"),
		"$minute", now.Format("04"),
		"$second", now.Format("05"),
		"$weekday", phraseWeekdays[int(now.Weekday())],
	)
	return repl.Replace(tpl)
}

// QueryPhrases 查询匹配的自定义短语。
func (d *Dictionary) QueryPhrases(code string) []Candidate {
	key := strings.ToLower(strings.TrimSpace(code))
	if key == "" {
		return nil
	}
	d.mu.RLock()
	templates := d.phraseMap[key]
	d.mu.RUnlock()
	if len(templates) == 0 {
		return nil
	}
	out := make([]Candidate, 0, len(templates))
	for i, tpl := range templates {
		out = append(out, Candidate{
			Text:   expandPhraseVars(tpl),
			Code:   key,
			Source: "phrase",
			Score:  float64(3500 - i*10),
			Hint:   "短语",
		})
	}
	return out
}

// AddPhrase 追加一条自定义短语到内存并持久化写入 data/phrases.txt。
func (d *Dictionary) AddPhrase(abbr, content string) error {
	key := strings.ToLower(strings.TrimSpace(abbr))
	val := strings.TrimSpace(content)
	if key == "" || val == "" {
		return fmt.Errorf("缩写与短语内容均不能为空")
	}
	d.mu.Lock()
	d.phraseMap[key] = append(d.phraseMap[key], val)
	path := d.phrasesPath
	dataDir := d.userDataDir
	d.mu.Unlock()

	_ = os.MkdirAll(dataDir, 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n%s=%s\n", key, val)
	return err
}

func (d *Dictionary) loadWubi(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		code := strings.ToLower(parts[0])
		word := parts[1]
		weight := 100
		if len(parts) >= 3 {
			if w, err := strconv.Atoi(parts[2]); err == nil {
				weight = w
			}
		}
		d.wubiMap[code] = append(d.wubiMap[code], wordEntry{word: word, weight: weight})
		if prev, ok := d.wordToWubi[word]; !ok || len(code) < len(prev) {
			d.wordToWubi[word] = code
		}
	}
}

func isASCIIPinyin(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '\'' || r == ' ') {
			return false
		}
	}
	return true
}

func (d *Dictionary) loadPinyin(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var cols []string
		if strings.Contains(line, "\t") {
			for _, c := range strings.Split(line, "\t") {
				c = strings.TrimSpace(c)
				if c != "" {
					cols = append(cols, c)
				}
			}
		} else {
			cols = strings.Fields(line)
		}
		if len(cols) < 2 {
			continue
		}

		var word string
		var sylsRaw string
		weight := 100

		if isASCIIPinyin(cols[0]) {
			sylsRaw = strings.ReplaceAll(strings.ToLower(cols[0]), "'", " ")
			word = cols[1]
			if len(cols) >= 3 {
				if w, err := strconv.Atoi(cols[2]); err == nil {
					weight = w
				}
			}
		} else {
			word = cols[0]
			if len(cols) >= 3 {
				if w, err := strconv.Atoi(cols[len(cols)-1]); err == nil {
					weight = w
					sylsRaw = strings.ToLower(strings.Join(cols[1:len(cols)-1], " "))
				} else {
					sylsRaw = strings.ToLower(strings.Join(cols[1:], " "))
				}
			} else {
				sylsRaw = strings.ToLower(cols[1])
			}
			sylsRaw = strings.ReplaceAll(sylsRaw, "'", " ")
		}

		syls := strings.Fields(sylsRaw)
		if len(syls) == 0 {
			continue
		}
		fullKey := strings.Join(syls, "")
		sylsStr := strings.Join(syls, "'")
		entry := pinyinEntry{word: word, weight: weight, syllables: sylsStr}
		d.pinyinMap[fullKey] = append(d.pinyinMap[fullKey], entry)

		if len(syls) >= 2 {
			var b1, b2 strings.Builder
			for _, s := range syls {
				b1.WriteByte(s[0])
				ini, _ := fuzzy.SplitInitialFinal(s)
				if ini != "" {
					b2.WriteString(ini)
				} else {
					b2.WriteByte(s[0])
				}
			}
			k1 := b1.String()
			k2 := b2.String()
			d.pinyinInitialsMap[k1] = append(d.pinyinInitialsMap[k1], entry)
			if k2 != k1 {
				d.pinyinInitialsMap[k2] = append(d.pinyinInitialsMap[k2], entry)
			}
		}

		runes := []rune(word)
		if len(runes) == 1 {
			if _, exists := d.charToPinyin[word]; !exists {
				d.charToPinyin[word] = syls[0]
			}
		}
	}
}

func (d *Dictionary) loadEnglish(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		word := parts[0]
		weight := 100
		if len(parts) >= 2 {
			if w, err := strconv.Atoi(parts[1]); err == nil {
				weight = w
			}
		}
		low := strings.ToLower(word)
		maxPref := len(low)
		if maxPref > 4 {
			maxPref = 4
		}
		for plen := 1; plen <= maxPref; plen++ {
			pref := low[:plen]
			d.englishPrefixMap[pref] = append(d.englishPrefixMap[pref], wordEntry{word: word, weight: weight})
		}
	}
}

func (d *Dictionary) loadUserDict() {
	raw, err := os.ReadFile(d.userDictPath)
	if err != nil {
		return
	}
	var parsed map[string]map[string]map[string]int
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return
	}
	for _, sec := range []string{"wubi", "pinyin", "english"} {
		if parsed[sec] != nil {
			d.userFreq[sec] = parsed[sec]
		}
	}
}

func (d *Dictionary) scheduleSaveUserDictLocked() {
	d.saveDirty = true
	if d.saveTimer != nil {
		d.saveTimer.Reset(1200 * time.Millisecond)
		return
	}
	d.saveTimer = time.AfterFunc(1200*time.Millisecond, func() {
		d.FlushUserDict()
	})
}

// FlushUserDict 立即将脏数据写入 data/user_dict.json（退出程序前调用）。
func (d *Dictionary) FlushUserDict() {
	d.mu.Lock()
	if !d.saveDirty {
		d.mu.Unlock()
		return
	}
	d.saveDirty = false
	if d.saveTimer != nil {
		d.saveTimer.Stop()
		d.saveTimer = nil
	}
	raw, err := json.MarshalIndent(d.userFreq, "", "  ")
	path := d.userDictPath
	dataDir := d.userDataDir
	d.mu.Unlock()

	if err != nil {
		return
	}
	_ = os.MkdirAll(dataDir, 0o755)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err == nil {
		_ = os.Rename(tmp, path)
	}
}

// BoostWord 动态提升选词权重（异步防抖落盘，不阻塞键盘钩子）。
func (d *Dictionary) BoostWord(source, code, word string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	sec, ok := d.userFreq[source]
	if !ok {
		return
	}
	ck := strings.ToLower(code)
	if sec[ck] == nil {
		sec[ck] = make(map[string]int)
	}
	sec[ck][word] += 150
	d.scheduleSaveUserDictLocked()
}

// AddUserWord 用户手动造词或整句自动学习，立即生效并异步落盘。
func (d *Dictionary) AddUserWord(source, code, word string, weight int) {
	codeLow := strings.ToLower(strings.TrimSpace(code))
	word = strings.TrimSpace(word)
	if codeLow == "" || word == "" {
		return
	}
	if weight <= 0 {
		weight = 2000
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	switch source {
	case "wubi":
		d.wubiMap[codeLow] = append([]wordEntry{{word: word, weight: weight}}, d.wubiMap[codeLow]...)
		d.wordToWubi[word] = codeLow
		idx := sort.SearchStrings(d.wubiKeys, codeLow)
		if idx >= len(d.wubiKeys) || d.wubiKeys[idx] != codeLow {
			d.wubiKeys = append(d.wubiKeys, "")
			copy(d.wubiKeys[idx+1:], d.wubiKeys[idx:])
			d.wubiKeys[idx] = codeLow
		}
	case "pinyin":
		fullKey := strings.ReplaceAll(strings.ReplaceAll(codeLow, " ", ""), "'", "")
		d.pinyinMap[fullKey] = append([]pinyinEntry{{word: word, weight: weight, syllables: codeLow}}, d.pinyinMap[fullKey]...)
		idx := sort.SearchStrings(d.pinyinKeys, fullKey)
		if idx >= len(d.pinyinKeys) || d.pinyinKeys[idx] != fullKey {
			d.pinyinKeys = append(d.pinyinKeys, "")
			copy(d.pinyinKeys[idx+1:], d.pinyinKeys[idx:])
			d.pinyinKeys[idx] = fullKey
		}
		codeLow = fullKey
	}
	if sec, ok := d.userFreq[source]; ok {
		if sec[codeLow] == nil {
			sec[codeLow] = make(map[string]int)
		}
		sec[codeLow][word] += 150
		d.scheduleSaveUserDictLocked()
	}
}

func (d *Dictionary) getWubiHintLocked(text string) string {
	if code, ok := d.wordToWubi[text]; ok {
		return "五:" + code
	}
	runes := []rune(text)
	if len(runes) >= 2 && len(runes) <= 4 {
		codes := make([]string, len(runes))
		for i, r := range runes {
			c, ok := d.wordToWubi[string(r)]
			if !ok || c == "" {
				return ""
			}
			codes[i] = c
		}
		take := func(s string, n int) string {
			if len(s) < n {
				return s
			}
			return s[:n]
		}
		switch len(runes) {
		case 2:
			return "五:" + take(codes[0], 2) + take(codes[1], 2)
		case 3:
			return "五:" + take(codes[0], 1) + take(codes[1], 1) + take(codes[2], 2)
		default:
			return "五:" + take(codes[0], 1) + take(codes[1], 1) + take(codes[2], 1) + take(codes[len(codes)-1], 1)
		}
	}
	return ""
}

func (d *Dictionary) getPinyinHintLocked(text string) string {
	runes := []rune(text)
	if len(runes) == 1 {
		if py, ok := d.charToPinyin[text]; ok {
			return "拼:" + py
		}
		return ""
	}
	if len(runes) >= 2 && len(runes) <= 4 {
		pys := make([]string, len(runes))
		for i, r := range runes {
			py, ok := d.charToPinyin[string(r)]
			if !ok || py == "" {
				return ""
			}
			pys[i] = py
		}
		return "拼:" + strings.Join(pys, "'")
	}
	return ""
}

// QueryWubi 查询五笔86候选词（支持 O(log N) 有序前缀补全与 z 键万能学习通配符）。
func (d *Dictionary) QueryWubi(code string, showHint bool, limit int) []Candidate {
	code = strings.ToLower(code)
	if code == "" || len(code) > 4 {
		return nil
	}
	if limit <= 0 {
		limit = 30
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	res := make(map[string]Candidate)
	userBucket := d.userFreq["wubi"][code]

	if !strings.Contains(code, "z") {
		for _, e := range d.wubiMap[code] {
			score := float64(e.weight+userBucket[e.word]) + 1000.0
			hint := ""
			if showHint {
				hint = d.getPinyinHintLocked(e.word)
			}
			if prev, ok := res[e.word]; !ok || score > prev.Score {
				res[e.word] = Candidate{Text: e.word, Code: code, Source: "wubi", Score: score, Hint: hint}
			}
		}
		for word, bonus := range userBucket {
			if _, ok := res[word]; !ok {
				hint := ""
				if showHint {
					hint = d.getPinyinHintLocked(word)
				}
				res[word] = Candidate{Text: word, Code: code, Source: "wubi", Score: 1500.0 + float64(bonus), Hint: hint}
			}
		}
		if len(res) < 5 && (len(code) == 2 || len(code) == 3) {
			idx := sort.SearchStrings(d.wubiKeys, code)
			for i := idx; i < len(d.wubiKeys) && len(res) < limit; i++ {
				k := d.wubiKeys[i]
				if !strings.HasPrefix(k, code) {
					break
				}
				if k == code {
					continue
				}
				entries := d.wubiMap[k]
				maxAdd := len(entries)
				if maxAdd > 3 {
					maxAdd = 3
				}
				for _, e := range entries[:maxAdd] {
					if _, ok := res[e.word]; !ok {
						res[e.word] = Candidate{
							Text:   e.word,
							Code:   k,
							Source: "wubi",
							Score:  float64(e.weight) * 0.35,
							Hint:   "~" + k[len(code):],
						}
					}
				}
			}
		}
	} else {
		for _, k := range d.wubiKeys {
			if len(k) != len(code) {
				continue
			}
			matched := true
			for i := 0; i < len(code); i++ {
				if code[i] != 'z' && code[i] != k[i] {
					matched = false
					break
				}
			}
			if !matched {
				continue
			}
			entries := d.wubiMap[k]
			maxAdd := len(entries)
			if maxAdd > 3 {
				maxAdd = 3
			}
			for _, e := range entries[:maxAdd] {
				if _, ok := res[e.word]; !ok {
					res[e.word] = Candidate{
						Text:   e.word,
						Code:   k,
						Source: "wubi",
						Score:  float64(e.weight) * 0.8,
						Hint:   "五:" + k,
					}
				}
			}
			if len(res) >= limit {
				break
			}
		}
	}

	out := make([]Candidate, 0, len(res))
	for _, c := range res {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Text < out[j].Text
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (d *Dictionary) composeSentenceLocked(syls []string, fuzzyEnabled bool, fuzzyRules map[string]bool) (string, float64, bool) {
	n := len(syls)
	if n < 2 || n > 12 {
		return "", 0, false
	}

	dpScore := make([]float64, n+1)
	dpText := make([]string, n+1)
	dpOK := make([]bool, n+1)
	dpOK[0] = true

	for i := 0; i < n; i++ {
		if !dpOK[i] {
			continue
		}
		maxSpan := n - i
		if maxSpan > 4 {
			maxSpan = 4
		}
		for span := maxSpan; span >= 1; span-- {
			var subVars []fuzzy.QueryVariant
			if span == 1 {
				subVars = fuzzy.ExpandSyllable(syls[i], fuzzyEnabled, fuzzyRules)
			} else {
				subVars = fuzzy.ExpandQuery(strings.Join(syls[i:i+span], "'"), fuzzyEnabled, fuzzyRules, 8)
			}
			bestWord := ""
			bestW := -1.0
			for _, sv := range subVars {
				entries := d.pinyinMap[sv.FullKey]
				if len(entries) == 0 {
					continue
				}
				top := entries[0]
				userBonus := d.userFreq["pinyin"][sv.FullKey][top.word]
				spanBonus := float64(span*span) * 600.0
				sc := (float64(top.weight+userBonus) + spanBonus) * sv.Weight
				if sc > bestW {
					bestW = sc
					bestWord = top.word
				}
			}
			if bestWord != "" {
				nextScore := dpScore[i] + bestW
				if !dpOK[i+span] || nextScore > dpScore[i+span] {
					dpOK[i+span] = true
					dpScore[i+span] = nextScore
					dpText[i+span] = dpText[i] + bestWord
				}
			}
		}
	}

	if !dpOK[n] || dpText[n] == "" {
		return "", 0, false
	}
	return dpText[n], dpScore[n], true
}

// QueryPinyin 查询拼音候选词（支持全拼、简拼、12 组模糊音、O(log N) 前缀联想与动态规划整句组词）。
func (d *Dictionary) QueryPinyin(rawCode string, fuzzyEnabled bool, fuzzyRules map[string]bool, showHint bool, limit int) []Candidate {
	code := strings.TrimSpace(strings.ToLower(rawCode))
	if code == "" {
		return nil
	}
	if limit <= 0 {
		limit = 40
	}

	variants := fuzzy.ExpandQuery(code, fuzzyEnabled, fuzzyRules, 48)
	paths := fuzzy.SegmentPinyin(code, 2)

	d.mu.RLock()
	defer d.mu.RUnlock()

	res := make(map[string]Candidate)

	for _, v := range variants {
		userBucket := d.userFreq["pinyin"][v.FullKey]
		isFuzzy := v.Weight < 0.99
		for _, e := range d.pinyinMap[v.FullKey] {
			bonus := userBucket[e.word]
			runeLen := len([]rune(e.word))
			lenBonus := 0.0
			if runeLen >= 2 {
				lenBonus = 250.0 * float64(runeLen-1)
			}
			score := (float64(e.weight) + float64(bonus) + lenBonus) * v.Weight
			if !isFuzzy {
				score += 800.0
			}
			if prev, ok := res[e.word]; !ok || score > prev.Score {
				hint := ""
				if showHint {
					hint = d.getWubiHintLocked(e.word)
				}
				if isFuzzy && hint != "" {
					hint = "模糊|" + hint
				} else if isFuzzy {
					hint = "模糊:" + v.FullKey
				}
				res[e.word] = Candidate{
					Text:   e.word,
					Code:   v.FullKey,
					Source: "pinyin",
					Score:  score,
					Hint:   hint,
				}
			}
		}

		for word, bonus := range userBucket {
			score := (1200.0 + float64(bonus)) * v.Weight
			if prev, ok := res[word]; !ok || score > prev.Score {
				hint := ""
				if showHint {
					hint = d.getWubiHintLocked(word)
				}
				res[word] = Candidate{Text: word, Code: v.FullKey, Source: "pinyin", Score: score, Hint: hint}
			}
		}
	}

	cleanKey := strings.ReplaceAll(code, "'", "")
	if len(paths) > 0 && len(paths[0]) >= 2 {
		if sentence, dpScore, ok := d.composeSentenceLocked(paths[0], fuzzyEnabled, fuzzyRules); ok {
			if _, exists := res[sentence]; !exists {
				normScore := 2600.0 + dpScore*0.05
				res[sentence] = Candidate{
					Text:     sentence,
					Code:     cleanKey,
					Source:   "pinyin",
					Score:    normScore,
					Hint:     "整句",
					Composed: true,
				}
			}
		}
	}

	if len(cleanKey) >= 2 && len(cleanKey) <= 6 {
		jpMap := map[string]float64{cleanKey: 1.0}
		if fuzzyEnabled {
			for idx := 0; idx < len(cleanKey); idx++ {
				ch := cleanKey[idx : idx+1]
				for _, pair := range fuzzy.FuzzyInitialPairs {
					if !fuzzyRules[pair.Rule] || len(pair.A) != 1 || len(pair.B) != 1 {
						continue
					}
					if ch == pair.A {
						nk := cleanKey[:idx] + pair.B + cleanKey[idx+1:]
						if prev, ok := jpMap[nk]; !ok || 0.72 < prev {
							jpMap[nk] = 0.72
						}
					} else if ch == pair.B {
						nk := cleanKey[:idx] + pair.A + cleanKey[idx+1:]
						if prev, ok := jpMap[nk]; !ok || 0.72 < prev {
							jpMap[nk] = 0.72
						}
					}
				}
			}
		}
		for jpKey, jpW := range jpMap {
			entries := d.pinyinInitialsMap[jpKey]
			maxAdd := len(entries)
			if maxAdd > 15 {
				maxAdd = 15
			}
			for _, e := range entries[:maxAdd] {
				score := float64(e.weight) * 0.55 * jpW
				if prev, ok := res[e.word]; !ok || score > prev.Score {
					hint := ""
					if showHint {
						hint = d.getWubiHintLocked(e.word)
					}
					res[e.word] = Candidate{Text: e.word, Code: jpKey, Source: "pinyin", Score: score, Hint: hint}
				}
			}
		}
	}

	if len(res) < 8 && len(cleanKey) >= 2 {
		idx := sort.SearchStrings(d.pinyinKeys, cleanKey)
		for i := idx; i < len(d.pinyinKeys) && len(res) < limit; i++ {
			k := d.pinyinKeys[i]
			if !strings.HasPrefix(k, cleanKey) {
				break
			}
			if k == cleanKey {
				continue
			}
			entries := d.pinyinMap[k]
			maxAdd := len(entries)
			if maxAdd > 3 {
				maxAdd = 3
			}
			for _, e := range entries[:maxAdd] {
				score := float64(e.weight) * 0.32
				if prev, ok := res[e.word]; !ok || score > prev.Score {
					hint := ""
					if showHint {
						hint = d.getWubiHintLocked(e.word)
					}
					res[e.word] = Candidate{Text: e.word, Code: k, Source: "pinyin", Score: score, Hint: hint}
				}
			}
		}
	}

	out := make([]Candidate, 0, len(res))
	for _, c := range res {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Text < out[j].Text
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// QueryEnglish 英文前缀补全。
func (d *Dictionary) QueryEnglish(prefix string, limit int) []Candidate {
	if prefix == "" {
		return nil
	}
	if limit <= 0 {
		limit = 10
	}
	low := strings.ToLower(prefix)
	bucketLen := len(low)
	if bucketLen > 4 {
		bucketLen = 4
	}
	bucketKey := low[:bucketLen]

	d.mu.RLock()
	defer d.mu.RUnlock()

	var out []Candidate
	seen := make(map[string]bool)
	allUpper := len(prefix) > 1 && strings.ToUpper(prefix) == prefix
	firstUpper := unicode.IsUpper(rune(prefix[0]))

	for _, e := range d.englishPrefixMap[bucketKey] {
		if strings.HasPrefix(strings.ToLower(e.word), low) && !seen[e.word] {
			seen[e.word] = true
			display := e.word
			if allUpper {
				display = strings.ToUpper(e.word)
			} else if firstUpper && len(e.word) > 0 {
				display = strings.ToUpper(e.word[:1]) + e.word[1:]
			}
			score := float64(e.weight)
			if strings.ToLower(e.word) == low {
				score += 500.0
			}
			out = append(out, Candidate{Text: display, Code: low, Source: "english", Score: score, Hint: "En"})
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

// ImportFile 导入外部 Rime YAML 或万能五笔/极点五笔 TXT 大词库。
func (d *Dictionary) ImportFile(kind, srcPath string) (int, string, error) {
	if kind != "wubi" && kind != "pinyin" {
		return 0, "", fmt.Errorf("不支持的词库类型: %s（仅支持 wubi / pinyin）", kind)
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()

	inBody := !strings.HasSuffix(strings.ToLower(srcPath), ".yaml") && !strings.HasSuffix(strings.ToLower(srcPath), ".yml")
	var outLines []string

	sc := bufio.NewScanner(f)
	// 支持超长行词库
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if !inBody {
			if line == "..." {
				inBody = true
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if kind == "wubi" {
			var parts []string
			if strings.Contains(line, "\t") {
				parts = strings.Split(line, "\t")
			} else {
				parts = strings.Fields(line)
			}
			if len(parts) < 2 {
				continue
			}
			if isASCIIPinyin(parts[0]) && !strings.Contains(parts[0], " ") {
				code := strings.ToLower(parts[0])
				for idx, word := range parts[1:] {
					if _, err := strconv.Atoi(word); err == nil {
						continue
					}
					w := 500 - idx*30
					if w < 10 {
						w = 10
					}
					outLines = append(outLines, fmt.Sprintf("%s\t%s\t%d\n", code, word, w))
				}
			} else {
				word := strings.TrimSpace(parts[0])
				code := strings.ToLower(strings.TrimSpace(parts[1]))
				if !isASCIIPinyin(code) {
					continue
				}
				w := 200
				if len(parts) >= 3 {
					if parsed, err := strconv.Atoi(strings.TrimSpace(parts[2])); err == nil {
						w = parsed
					}
				}
				outLines = append(outLines, fmt.Sprintf("%s\t%s\t%d\n", code, word, w))
			}
		} else {
			parts := strings.Split(line, "\t")
			if len(parts) < 2 {
				continue
			}
			word := strings.TrimSpace(parts[0])
			py := strings.ToLower(strings.TrimSpace(parts[1]))
			if word == "" || py == "" {
				continue
			}
			w := 200
			if len(parts) >= 3 {
				if parsed, err := strconv.Atoi(strings.TrimSpace(parts[2])); err == nil {
					w = parsed
				}
			}
			outLines = append(outLines, fmt.Sprintf("%s\t%s\t%d\n", word, py, w))
		}
	}

	dstName := "wubi86_extra.txt"
	if kind == "pinyin" {
		dstName = "pinyin_extra.txt"
	}
	dstPath := filepath.Join(d.userDataDir, dstName)
	_ = os.MkdirAll(d.userDataDir, 0o755)
	outFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, "", err
	}
	for _, l := range outLines {
		_, _ = outFile.WriteString(l)
	}
	_ = outFile.Close()
	d.Reload()
	return len(outLines), dstPath, nil
}
