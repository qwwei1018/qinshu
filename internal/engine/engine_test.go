package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qinshu/internal/config"
	"qinshu/internal/dict"
)

func setupTestEngine(t *testing.T) (*Engine, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data failed: %v", err)
	}
	repoData := filepath.Join("..", "..", "data")
	for _, fname := range []string{"wubi86.txt", "pinyin.txt", "english.txt", "phrases.txt"} {
		raw, err := os.ReadFile(filepath.Join(repoData, fname))
		if err != nil {
			t.Fatalf("read %s failed: %v", fname, err)
		}
		if err := os.WriteFile(filepath.Join(dataDir, fname), raw, 0o644); err != nil {
			t.Fatalf("write %s failed: %v", fname, err)
		}
	}
	cfg := config.Default()
	d := dict.New(dataDir)
	eng := New(cfg, d, tmpDir)
	t.Cleanup(func() { eng.Flush() })
	return eng, tmpDir
}

func typeSeq(eng *Engine, s string) string {
	var b strings.Builder
	for _, r := range s {
		act := eng.HandleKey(strings.ToLower(string(r)), r, false, false, false)
		if act.CommitText != "" {
			b.WriteString(act.CommitText)
		}
	}
	return b.String()
}

func TestWubiStandard86AndAutoCommit4thCode(t *testing.T) {
	eng, _ := setupTestEngine(t)
	eng.SetInputMode(config.ModeWubi)

	eng.HandleKey("k", 'k', false, false, false)
	vs := eng.GetViewState()
	if !vs.Visible || len(vs.PageCandidates) == 0 || vs.PageCandidates[0].Text != "中" {
		t.Fatalf("expected '中' for wubi 'k', got %+v", vs.PageCandidates)
	}
	act := eng.HandleKey("Space", ' ', false, false, false)
	if act.CommitText != "中" {
		t.Fatalf("expected commit '中', got %q", act.CommitText)
	}

	standardCases := map[string]string{
		"dwnn": "秦书",
		"dcgt": "万能五笔",
		"aawt": "工作",
		"wqvb": "你好",
		"khyy": "中文",
		"amyy": "英文",
		"ruuj": "拼音",
		"ltif": "输入法",
		"souj": "模糊音",
		"emsf": "股票",
		"uqjq": "交易",
		"watc": "人工智能",
	}
	for code, want := range standardCases {
		eng.Clear()
		if out := typeSeq(eng, code); out != want {
			t.Errorf("standard wubi86 %q: want %q, got %q", code, want, out)
		}
	}
}

func TestWubi5thCodePushCommitAndWildcard(t *testing.T) {
	eng, _ := setupTestEngine(t)
	eng.SetInputMode(config.ModeWubi)
	eng.Config.AutoCommit4thCode = false

	typeSeq(eng, "khlg")
	act := eng.HandleKey("a", 'a', false, false, false)
	if act.CommitText != "中国" {
		t.Fatalf("expected 5th-code push commit '中国', got %q", act.CommitText)
	}
	vs := eng.GetViewState()
	if vs.Buffer != "a" || vs.PageCandidates[0].Text != "工" {
		t.Fatalf("expected buffer 'a' -> '工', got %+v", vs)
	}

	eng.Clear()
	typeSeq(eng, "dwnz")
	vs = eng.GetViewState()
	found := false
	for _, c := range vs.PageCandidates {
		if c.Text == "秦书" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected wildcard 'dwnz' to match '秦书', got %+v", vs.PageCandidates)
	}
}

func TestPinyinFuzzySentenceAndAutoLearn(t *testing.T) {
	eng, _ := setupTestEngine(t)
	eng.SetInputMode(config.ModePinyin)
	eng.Config.SetAllFuzzy(true)

	cases := []struct {
		input string
		want  string
	}{
		{"qinshu", "秦书"},
		{"wnwb", "万能五笔"},
		{"xi'an", "西安"},
		{"zongguo", "中国"},
		{"cengdu", "成都"},
		{"sanghai", "上海"},
		{"lanjing", "南京"},
		{"hujian", "福建"},
		{"funan", "湖南"},
		{"lenmin", "人民"},
		{"xingwen", "新闻"},
		{"chenxu", "程序"},
		{"liangxi", "联系"},
		{"guangxin", "关心"},
		{"celue", "策略"},
	}

	for _, tc := range cases {
		eng.Clear()
		cands := eng.Dict.QueryPinyin(tc.input, eng.Config.FuzzyEnabled, eng.Config.FuzzyRules, true, 40)
		found := false
		for _, c := range cands {
			if c.Text == tc.want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("pinyin/fuzzy input %q should match %q, got %+v", tc.input, tc.want, cands)
		}
	}

	eng.Clear()
	typeSeq(eng, "woshizhongguoren")
	act := eng.HandleKey("Space", ' ', false, false, false)
	if act.CommitText != "我是中国人" {
		t.Fatalf("expected composed commit '我是中国人', got %q", act.CommitText)
	}
	eng.Clear()
	typeSeq(eng, "woshizhongguoren")
	vs := eng.GetViewState()
	if len(vs.PageCandidates) == 0 || vs.PageCandidates[0].Text != "我是中国人" {
		t.Fatalf("expected auto-learned phrase '我是中国人', got %+v", vs.PageCandidates)
	}
}

func TestMixedModeCtrlSpaceCustomPhrasesAndQuickCommands(t *testing.T) {
	eng, _ := setupTestEngine(t)
	eng.SetInputMode(config.ModeMixed)

	// 自定义短语 qs -> Qin书输入法
	typeSeq(eng, "qs")
	vs := eng.GetViewState()
	if len(vs.PageCandidates) == 0 || vs.PageCandidates[0].Text != "Qin书输入法" || vs.PageCandidates[0].Source != "phrase" {
		t.Fatalf("expected custom phrase 'Qin书输入法', got %+v", vs.PageCandidates)
	}
	eng.Clear()

	// Ctrl+Space 切换中英文
	actToggle := eng.HandleKey("Space", ' ', true, false, false)
	if !actToggle.Handled || eng.Config.Language != config.LangEn {
		t.Fatalf("expected Ctrl+Space to switch to English")
	}
	eng.HandleKey("Space", ' ', true, false, false)
	if eng.Config.Language != config.LangZh {
		t.Fatalf("expected second Ctrl+Space to switch back to Chinese")
	}

	// Delete 清空缓冲
	typeSeq(eng, "zh")
	eng.HandleKey("Delete", 0, false, false, false)
	if eng.HasActiveBuffer() {
		t.Fatalf("expected Delete to clear active buffer")
	}

	// 快捷星期 xq 与符号面板 fh
	typeSeq(eng, "xq")
	vs = eng.GetViewState()
	if len(vs.PageCandidates) == 0 || !strings.HasPrefix(vs.PageCandidates[0].Text, "星期") {
		t.Fatalf("expected weekday command for xq, got %+v", vs.PageCandidates)
	}
	eng.Clear()

	typeSeq(eng, "fh")
	vs = eng.GetViewState()
	if len(vs.PageCandidates) == 0 || vs.PageCandidates[0].Text != "「」" {
		t.Fatalf("expected quick symbol panel for fh, got %+v", vs.PageCandidates)
	}
}

func TestSmartEmailAndURLRecognition(t *testing.T) {
	eng, _ := setupTestEngine(t)
	eng.SetInputMode(config.ModeMixed)

	// 1. 邮箱智能后缀: zxc@ -> zxc@gmail.com, zxc@qq.com, zxc@163.com...
	typeSeq(eng, "zxc@")
	vs := eng.GetViewState()
	if vs.SmartMode != "email" || len(vs.PageCandidates) < 3 {
		t.Fatalf("expected email smart mode with domains, got %+v", vs)
	}
	if vs.PageCandidates[0].Text != "zxc@gmail.com" || vs.PageCandidates[1].Text != "zxc@qq.com" {
		t.Fatalf("expected gmail.com and qq.com top candidates, got %+v", vs.PageCandidates)
	}
	// 继续输入 q 过滤为 qq.com
	eng.HandleKey("q", 'q', false, false, false)
	vs = eng.GetViewState()
	if vs.PageCandidates[0].Text != "zxc@qq.com" {
		t.Fatalf("expected filtered zxc@qq.com, got %+v", vs.PageCandidates)
	}
	act := eng.HandleKey("Space", ' ', false, false, false)
	if act.CommitText != "zxc@qq.com" {
		t.Fatalf("expected commit zxc@qq.com, got %q", act.CommitText)
	}

	// 2. 网址协议自动补全: http -> http://, https://
	typeSeq(eng, "http")
	vs = eng.GetViewState()
	if len(vs.PageCandidates) < 2 || vs.PageCandidates[0].Text != "http://" || vs.PageCandidates[1].Text != "https://" {
		t.Fatalf("expected http:// and https:// for 'http', got %+v", vs.PageCandidates)
	}
	// 输入 ':' 自动补全为 http:// 并进入网址补全模式
	eng.HandleKey("semicolon", ':', false, false, true)
	vs = eng.GetViewState()
	if vs.SmartMode != "url" || vs.Buffer != "http://" {
		t.Fatalf("expected auto-expanded http:// in url mode, got %+v", vs)
	}
	typeSeq(eng, "baidu")
	vs = eng.GetViewState()
	if len(vs.PageCandidates) == 0 || vs.PageCandidates[0].Text != "http://baidu.com" {
		t.Fatalf("expected http://baidu.com, got %+v", vs.PageCandidates)
	}
	eng.Clear()

	// 3. www. 网址补全
	typeSeq(eng, "www.google")
	vs = eng.GetViewState()
	if vs.SmartMode != "url" || len(vs.PageCandidates) == 0 || vs.PageCandidates[0].Text != "www.google.com" {
		t.Fatalf("expected www.google.com, got %+v", vs)
	}
}
