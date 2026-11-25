package fuzzy

import "testing"

func TestIncompletePrefixSegmentation(t *testing.T) {
	paths := SegmentPinyin("zhon", 4)
	if len(paths) == 0 || len(paths[0]) != 1 || paths[0][0] != "zhon" {
		t.Fatalf("expected incomplete trailing prefix [zhon], got %+v", paths)
	}

	paths2 := SegmentPinyin("zhongg", 4)
	if len(paths2) == 0 || len(paths2[0]) != 2 || paths2[0][0] != "zhong" || paths2[0][1] != "g" {
		t.Fatalf("expected [zhong g], got %+v", paths2)
	}
}

func TestUVToleranceAndFuzzyExpansion(t *testing.T) {
	vars := ExpandSyllable("lue", false, nil)
	foundLve := false
	for _, v := range vars {
		if v.FullKey == "lve" {
			foundLve = true
		}
	}
	if !foundLve {
		t.Fatalf("expected u/v tolerance lue -> lve, got %+v", vars)
	}

	rules := map[string]bool{
		"z_zh": true, "l_n": true, "f_h": true, "in_ing": true,
	}
	q := ExpandQuery("lanjing", true, rules, 32)
	foundNanjing := false
	for _, v := range q {
		if v.FullKey == "nanjing" {
			foundNanjing = true
		}
	}
	if !foundNanjing {
		t.Fatalf("expected fuzzy lanjing -> nanjing, got %+v", q)
	}
}

func TestIsPotentialPinyinPrefix(t *testing.T) {
	for _, s := range []string{"zhon", "shur", "jint", "xian"} {
		if !IsPotentialPinyinPrefix(s) {
			t.Errorf("expected %q to be a potential pinyin prefix", s)
		}
	}
	for _, s := range []string{"khlg", "dcgt", "dwnn", "aawt", "wqvb", "emsf"} {
		if IsPotentialPinyinPrefix(s) {
			t.Errorf("expected wubi code %q NOT to be a pinyin prefix", s)
		}
	}
}
