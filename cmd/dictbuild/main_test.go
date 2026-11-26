package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestStringToT9(t *testing.T) {
	if got := stringToT9("zhongguo"); got != "94664486" {
		t.Fatalf("expected 94664486, got %s", got)
	}
	if got := stringToT9("nihao"); got != "64426" {
		t.Fatalf("expected 64426, got %s", got)
	}
}

func TestCommonWeightOrdering(t *testing.T) {
	commonChar := adjustCommonWeight("的", 2600)
	rareChar := adjustCommonWeight("菚", 300)
	if commonChar <= rareChar*5 {
		t.Fatalf("common char (%d) should strongly outrank rare char (%d)", commonChar, rareChar)
	}
	commonWord := adjustCommonWeight("我们", 3100)
	rareWord := adjustCommonWeight("形意", 200)
	if commonWord <= rareWord*5 {
		t.Fatalf("common word (%d) should strongly outrank rare word (%d)", commonWord, rareWord)
	}
}

func TestWriteQDictHeader(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "test.qdict")
	py := []keyBucket{{key: "nihao", entries: []rawEntry{{word: "你好", weight: 15000}}}}
	en := []rawEntry{{word: "hello", weight: 2400}}
	em := []keyBucket{{key: "kaixin", entries: []rawEntry{{word: "😄", weight: 1000}}}}
	if err := writeQDict(out, py, nil, nil, em, en); err != nil {
		t.Fatalf("writeQDict failed: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(raw[:4]) != qdictMagic {
		t.Fatalf("bad magic: %q", string(raw[:4]))
	}
	if ver := binary.LittleEndian.Uint32(raw[4:8]); ver != qdictVersion {
		t.Fatalf("bad version: %d", ver)
	}
}
