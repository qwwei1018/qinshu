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
