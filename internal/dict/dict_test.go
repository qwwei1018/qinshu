package dict

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestDict(t *testing.T) (*Dictionary, string) {
	t.Helper()
	tmp := t.TempDir()
	repoData := filepath.Join("..", "..", "data")
	for _, f := range []string{"wubi86.txt", "pinyin.txt", "english.txt", "phrases.txt"} {
		raw, err := os.ReadFile(filepath.Join(repoData, f))
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if err := os.WriteFile(filepath.Join(tmp, f), raw, 0o644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}
	return New(tmp), tmp
}

func TestSentenceCompositionAndPrefixSearch(t *testing.T) {
	d, _ := setupTestDict(t)
	rules := map[string]bool{"z_zh": true, "s_sh": true}

	cands := d.QueryPinyin("woshizhongguoren", true, rules, true, 10)
	if len(cands) == 0 || cands[0].Text != "我是中国人" || !cands[0].Composed {
		t.Fatalf("expected composed sentence '我是中国人', got %+v", cands)
	}

	cands2 := d.QueryPinyin("celue", false, nil, true, 10)
	found := false
	for _, c := range cands2 {
		if c.Text == "策略" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected celue -> '策略', got %+v", cands2)
	}

	cands3 := d.QueryPinyin("zhon", false, nil, true, 10)
	if len(cands3) == 0 {
		t.Fatalf("expected prefix suggestions for 'zhon'")
	}

	st := d.Stats()
	if st.WubiEntries < 200 || st.PinyinEntries < 400 || st.EnglishEntries < 100 || st.PhraseEntries < 5 {
		t.Fatalf("unexpected dict stats: %+v", st)
	}
}

func TestCustomPhrasesAndDynamicVars(t *testing.T) {
	d, _ := setupTestDict(t)

	// 测试内置短语 qs -> Qin书输入法
	qs := d.QueryPhrases("qs")
	if len(qs) == 0 || qs[0].Text != "Qin书输入法" || qs[0].Hint != "短语" {
		t.Fatalf("expected custom phrase 'Qin书输入法', got %+v", qs)
	}

	// 测试动态变量展开 fp -> 今日复盘：YYYY-MM-DD（星期X）
	fp := d.QueryPhrases("fp")
	if len(fp) == 0 || !strings.HasPrefix(fp[0].Text, "今日复盘：") || strings.Contains(fp[0].Text, "$year") {
		t.Fatalf("expected expanded dynamic phrase for fp, got %+v", fp)
	}

	// 测试运行时新增短语并持久化
	if err := d.AddPhrase("mytest", "自定义短语测试-$year"); err != nil {
		t.Fatalf("AddPhrase failed: %v", err)
	}
	mt := d.QueryPhrases("mytest")
	if len(mt) == 0 || !strings.HasPrefix(mt[0].Text, "自定义短语测试-20") {
		t.Fatalf("expected expanded added phrase, got %+v", mt)
	}
}

func TestImportFileAndDeduplication(t *testing.T) {
	d, tmp := setupTestDict(t)
	yamlPath := filepath.Join(tmp, "rime_wubi.yaml")
	content := "---\nname: test\n...\n测词\tceci\t900\n测词\tceci\t500\n"
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cnt, _, err := d.ImportFile("wubi", yamlPath)
	if err != nil || cnt != 2 {
		t.Fatalf("import failed: cnt=%d err=%v", cnt, err)
	}
	cands := d.QueryWubi("ceci", true, 10)
	matchCount := 0
	for _, c := range cands {
		if c.Text == "测词" {
			matchCount++
		}
	}
	if matchCount != 1 {
		t.Fatalf("expected deduplicated single '测词', got %d", matchCount)
	}
}

func TestDynamicFrequencySmartPromotionAndPersistence(t *testing.T) {
	d, tmp := setupTestDict(t)

	// 1. 拼音第 3 页候选（索引 >= 10）在选中 1~2 次后应迅速提前至首页甚至首位
	initial := d.QueryPinyin("shi", false, nil, false, 40)
	if len(initial) < 15 {
		t.Fatalf("expected >= 15 candidates for 'shi', got %d", len(initial))
	}
	target := initial[12].Text
	d.BoostSelection("pinyin", "shi", target, 12, 5)
	after1 := d.QueryPinyin("shi", false, nil, false, 40)
	pos1 := -1
	for i, c := range after1 {
		if c.Text == target {
			pos1 = i
			break
		}
	}
	if pos1 < 0 || pos1 >= 5 {
		t.Fatalf("expected page-3 char %q to jump to page 1 after 1 selection, got index %d", target, pos1)
	}
	d.BoostSelection("pinyin", "shi", target, pos1, 5)
	after2 := d.QueryPinyin("shi", false, nil, false, 40)
	if after2[0].Text != target {
		t.Fatalf("expected %q to reach #1 after 2nd selection, got #1=%q", target, after2[0].Text)
	}

	// 2. 简拼（如 zg）第 2/3 页词条被选中后也应提前并超越默认高权词
	jpInit := d.QueryPinyin("zg", false, nil, false, 40)
	if len(jpInit) < 10 {
		t.Fatalf("expected >= 10 initials candidates for 'zg', got %d", len(jpInit))
	}
	jpTarget := jpInit[8].Text
	d.BoostSelection("pinyin", "zg", jpTarget, 8, 5)
	jpAfter := d.QueryPinyin("zg", false, nil, false, 40)
	if jpAfter[0].Text != jpTarget {
		t.Fatalf("expected initials candidate %q to reach #1, got #1=%q", jpTarget, jpAfter[0].Text)
	}

	// 3. 英文补全也支持动态词频提前
	enInit := d.QueryEnglish("pro", 10)
	if len(enInit) < 5 {
		t.Fatalf("expected >= 5 english candidates for 'pro', got %d", len(enInit))
	}
	enTarget := enInit[4].Text
	d.BoostSelection("english", "pro", enTarget, 4, 5)
	d.BoostSelection("english", "pro", enTarget, 1, 5)
	enAfter := d.QueryEnglish("pro", 10)
	if enAfter[0].Text != enTarget {
		t.Fatalf("expected english candidate %q to reach #1, got #1=%q", enTarget, enAfter[0].Text)
	}

	// 4. 持久化落盘后重新加载词库仍保持首位
	d.FlushUserDict()
	reloaded := New(tmp)
	recheck := reloaded.QueryPinyin("shi", false, nil, false, 40)
	if len(recheck) == 0 || recheck[0].Text != target {
		t.Fatalf("expected persisted #1 %q after reload, got %+v", target, recheck[:3])
	}
}
