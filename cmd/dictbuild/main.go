package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// qinshu.qdict v1 二进制只读词库编译器（零外部依赖，供 GitHub Actions 在构建安卓 APK 前运行）
//
// 核心特性：
// 1. 常用字词分层强置顶：对一级常用单字与高频日常词组加权，对生僻字与冷门词组显式降权。
// 2. 构建期预排序：每个 Key 下的候选词按最终权重严格降序排列，安卓端 O(log N) 零排序直读。
// 3. 完整支持全拼、简拼、九宫格 T9 数字索引、英文前缀索引、拼音→微表情（纯 Emoji）索引。
// 4. 全平坦二进制 + 去重 UTF-8 字符串池，安卓端 FileChannel.map(READ_ONLY) 零解析秒启。

const (
	qdictMagic   = "QDCT"
	qdictVersion = uint32(1)
	headerSize   = 72 // 18 * uint32

	maxEntriesPerPinyinKey   = 40
	maxEntriesPerInitialsKey = 12
	maxEntriesPerT9Key       = 15
	maxEntriesPerEmojiKey    = 8
)

type rawEntry struct {
	word   string
	weight uint16
}

type keyBucket struct {
	key     string
	entries []rawEntry
}

type stringPool struct {
	buf    []byte
	offset map[string]uint32
}

func newStringPool() *stringPool {
	return &stringPool{
		buf:    make([]byte, 0, 1024*1024),
		offset: make(map[string]uint32, 160000),
	}
}

func (sp *stringPool) intern(s string) (uint32, uint16) {
	if off, ok := sp.offset[s]; ok {
		return off, uint16(len(s))
	}
	off := uint32(len(sp.buf))
	sp.buf = append(sp.buf, s...)
	sp.offset[s] = off
	return off, uint16(len(s))
}

// letterToT9 将小写英文字母映射为手机九宫格数字键 (2-9)。
func letterToT9(ch byte) byte {
	switch ch {
	case 'a', 'b', 'c':
		return '2'
	case 'd', 'e', 'f':
		return '3'
	case 'g', 'h', 'i':
		return '4'
	case 'j', 'k', 'l':
		return '5'
	case 'm', 'n', 'o':
		return '6'
	case 'p', 'q', 'r', 's':
		return '7'
	case 't', 'u', 'v':
		return '8'
	case 'w', 'x', 'y', 'z':
		return '9'
	default:
		return 0
	}
}

func stringToT9(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		d := letterToT9(s[i])
		if d == 0 {
			return ""
		}
		b.WriteByte(d)
	}
	return b.String()
}

func splitInitial(syl string) string {
	for _, ini := range []string{"zh", "ch", "sh"} {
		if strings.HasPrefix(syl, ini) {
			return ini
		}
	}
	if len(syl) > 0 {
		return syl[:1]
	}
	return ""
}

// adjustCommonWeight 强化常用词语与一级常用字排序，压低生僻字与冷门词组。
func adjustCommonWeight(word string, rawWeight int) uint16 {
	runeLen := len([]rune(word))
	var score int
	if runeLen == 1 {
		// 单字分层：pinyin.txt 中高频常用字权重在 1500~2600，生僻字在 97~900
		switch {
		case rawWeight >= 2200:
			score = rawWeight*4 + 4000 // 核心高频单字强置顶
		case rawWeight >= 1400:
			score = rawWeight*3 + 1500 // 常用单字保持高优
		case rawWeight >= 900:
			score = rawWeight * 2
		default:
			score = rawWeight * 55 / 100 // 生僻字显式降权，避免挤占首屏
		}
	} else {
		// 词语分层：日常高频词组强置顶
		lenBonus := 0
		if runeLen == 2 {
			lenBonus = 2200
		} else if runeLen == 3 {
			lenBonus = 1800
		} else if runeLen == 4 {
			lenBonus = 1500
		}
		switch {
		case rawWeight >= 2400:
			score = rawWeight*4 + 5000 + lenBonus
		case rawWeight >= 1500:
			score = rawWeight*3 + 2500 + lenBonus
		case rawWeight >= 800:
			score = rawWeight*2 + 800
		default:
			score = rawWeight * 75 / 100 // 低频冷门词组降权
		}
	}
	if score < 1 {
		score = 1
	}
	if score > 60000 {
		score = 60000
	}
	return uint16(score)
}

func dedupAndSort(list []rawEntry, limit int) []rawEntry {
	best := make(map[string]uint16, len(list))
	for _, e := range list {
		if prev, ok := best[e.word]; !ok || e.weight > prev {
			best[e.word] = e.weight
		}
	}
	out := make([]rawEntry, 0, len(best))
	for w, wt := range best {
		out = append(out, rawEntry{word: w, weight: wt})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].weight != out[j].weight {
			return out[i].weight > out[j].weight
		}
		return out[i].word < out[j].word
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func buildBuckets(m map[string][]rawEntry, limit int) []keyBucket {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	buckets := make([]keyBucket, 0, len(keys))
	for _, k := range keys {
		entries := dedupAndSort(m[k], limit)
		if len(entries) > 0 {
			buckets = append(buckets, keyBucket{key: k, entries: entries})
		}
	}
	return buckets
}

func loadPinyin(path string) (pinyinMap, initialsMap, t9Map map[string][]rawEntry, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()

	pinyinMap = make(map[string][]rawEntry, 50000)
	initialsMap = make(map[string][]rawEntry, 30000)
	t9Map = make(map[string][]rawEntry, 40000)

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		word := strings.TrimSpace(parts[0])
		sylsRaw := strings.ToLower(strings.TrimSpace(parts[1]))
		rawWeight := 200
		if len(parts) >= 3 {
			if w, e := strconv.Atoi(strings.TrimSpace(parts[2])); e == nil {
				rawWeight = w
			}
		}
		syls := strings.Fields(strings.ReplaceAll(sylsRaw, "'", " "))
		if word == "" || len(syls) == 0 {
			continue
		}

		fullKey := strings.Join(syls, "")
		adjWeight := adjustCommonWeight(word, rawWeight)
		entry := rawEntry{word: word, weight: adjWeight}

		pinyinMap[fullKey] = append(pinyinMap[fullKey], entry)

		// 简拼索引（2~5 字常用词）
		if len(syls) >= 2 && len(syls) <= 5 && rawWeight >= 500 {
			var b1, b2 strings.Builder
			for _, s := range syls {
				b1.WriteByte(s[0])
				b2.WriteString(splitInitial(s))
			}
			k1 := b1.String()
			k2 := b2.String()
			initialsMap[k1] = append(initialsMap[k1], entry)
			if k2 != k1 {
				initialsMap[k2] = append(initialsMap[k2], entry)
			}
		}

		// 九宫格 T9 索引：所有单音节 + 高频多音节词（最多 16 键）
		if len(fullKey) <= 16 && (len(syls) == 1 || rawWeight >= 800) {
			if t9 := stringToT9(fullKey); t9 != "" {
				t9Map[t9] = append(t9Map[t9], entry)
			}
		}
	}
	return pinyinMap, initialsMap, t9Map, sc.Err()
}

func loadEnglish(path string) ([]rawEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	best := make(map[string]uint16, 6000)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		word := strings.ToLower(parts[0])
		w := 500
		if len(parts) >= 2 {
			if parsed, e := strconv.Atoi(parts[1]); e == nil {
				w = parsed
			}
		}
		if w > 60000 {
			w = 60000
		}
		if prev, ok := best[word]; !ok || uint16(w) > prev {
			best[word] = uint16(w)
		}
	}
	out := make([]rawEntry, 0, len(best))
	for word, wt := range best {
		out = append(out, rawEntry{word: word, weight: wt})
	}
	// 按字典序升序排列，支持安卓端 O(log N) 二分前缀区间检索
	sort.Slice(out, func(i, j int) bool {
		return out[i].word < out[j].word
	})
	return out, sc.Err()
}

func loadEmojiPinyin(path string) (map[string][]rawEntry, error) {
	m := make(map[string][]rawEntry, 512)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		emojis := strings.Split(parts[1], ",")
		for idx, em := range emojis {
			em = strings.TrimSpace(em)
			if em == "" {
				continue
			}
			w := uint16(1000 - idx*50)
			m[key] = append(m[key], rawEntry{word: em, weight: w})
		}
	}
	return m, sc.Err()
}

func writeQDict(outPath string, pyBuckets, initBuckets, t9Buckets, emojiBuckets []keyBucket, enEntries []rawEntry) error {
	sp := newStringPool()

	type keyRec struct {
		strOff     uint32
		strLen     uint16
		entryCount uint16
		firstIdx   uint32
	}
	type entryRec struct {
		strOff uint32
		strLen uint16
		weight uint16
	}

	encodeBuckets := func(buckets []keyBucket) ([]keyRec, []entryRec) {
		kRecs := make([]keyRec, 0, len(buckets))
		eRecs := make([]entryRec, 0, len(buckets)*3)
		for _, b := range buckets {
			kOff, kLen := sp.intern(b.key)
			first := uint32(len(eRecs))
			for _, e := range b.entries {
				wOff, wLen := sp.intern(e.word)
				eRecs = append(eRecs, entryRec{strOff: wOff, strLen: wLen, weight: e.weight})
			}
			kRecs = append(kRecs, keyRec{
				strOff:     kOff,
				strLen:     kLen,
				entryCount: uint16(len(b.entries)),
				firstIdx:   first,
			})
		}
		return kRecs, eRecs
	}

	pyKeys, pyEntries := encodeBuckets(pyBuckets)
	initKeys, initEntries := encodeBuckets(initBuckets)
	t9Keys, t9Entries := encodeBuckets(t9Buckets)
	emKeys, emEntries := encodeBuckets(emojiBuckets)

	enRecs := make([]entryRec, 0, len(enEntries))
	for _, e := range enEntries {
		wOff, wLen := sp.intern(e.word)
		enRecs = append(enRecs, entryRec{strOff: wOff, strLen: wLen, weight: e.weight})
	}

	// 计算各段绝对字节偏移
	cur := uint32(headerSize)
	pyKeyOff := cur
	cur += uint32(len(pyKeys) * 12)
	pyEntryOff := cur
	cur += uint32(len(pyEntries) * 8)

	initKeyOff := cur
	cur += uint32(len(initKeys) * 12)
	initEntryOff := cur
	cur += uint32(len(initEntries) * 8)

	t9KeyOff := cur
	cur += uint32(len(t9Keys) * 12)
	t9EntryOff := cur
	cur += uint32(len(t9Entries) * 8)

	enOff := cur
	cur += uint32(len(enRecs) * 8)

	emKeyOff := cur
	cur += uint32(len(emKeys) * 12)
	emEntryOff := cur
	cur += uint32(len(emEntries) * 8)

	strPoolOff := cur

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriterSize(f, 256*1024)

	writeU32 := func(v uint32) {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], v)
		_, _ = bw.Write(b[:])
	}
	writeU16 := func(v uint16) {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], v)
		_, _ = bw.Write(b[:])
	}

	// 写入 72 字节 Header
	_, _ = bw.WriteString(qdictMagic)
	writeU32(qdictVersion)
	writeU32(0) // flags
	writeU32(uint32(len(pyKeys)))
	writeU32(pyKeyOff)
	writeU32(pyEntryOff)
	writeU32(uint32(len(initKeys)))
	writeU32(initKeyOff)
	writeU32(initEntryOff)
	writeU32(uint32(len(t9Keys)))
	writeU32(t9KeyOff)
	writeU32(t9EntryOff)
	writeU32(uint32(len(enRecs)))
	writeU32(enOff)
	writeU32(uint32(len(emKeys)))
	writeU32(emKeyOff)
	writeU32(emEntryOff)
	writeU32(strPoolOff)

	writeKeyTable := func(recs []keyRec) {
		for _, r := range recs {
			writeU32(r.strOff)
			writeU16(r.strLen)
			writeU16(r.entryCount)
			writeU32(r.firstIdx)
		}
	}
	writeEntryTable := func(recs []entryRec) {
		for _, r := range recs {
			writeU32(r.strOff)
			writeU16(r.strLen)
			writeU16(r.weight)
		}
	}

	writeKeyTable(pyKeys)
	writeEntryTable(pyEntries)
	writeKeyTable(initKeys)
	writeEntryTable(initEntries)
	writeKeyTable(t9Keys)
	writeEntryTable(t9Entries)
	writeEntryTable(enRecs)
	writeKeyTable(emKeys)
	writeEntryTable(emEntries)
	_, _ = bw.Write(sp.buf)

	return bw.Flush()
}

func main() {
	dataDir := flag.String("data", "data", "桌面词库目录（包含 pinyin.txt, english.txt, emoji_pinyin.txt）")
	outPath := flag.String("out", "android/app/src/main/assets/qinshu.qdict", "输出的二进制词库文件路径")
	flag.Parse()

	pyMap, initMap, t9Map, err := loadPinyin(filepath.Join(*dataDir, "pinyin.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载 pinyin.txt 失败: %v\n", err)
		os.Exit(1)
	}
	enEntries, err := loadEnglish(filepath.Join(*dataDir, "english.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载 english.txt 失败: %v\n", err)
		os.Exit(1)
	}
	emMap, err := loadEmojiPinyin(filepath.Join(*dataDir, "emoji_pinyin.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载 emoji_pinyin.txt 失败: %v\n", err)
		os.Exit(1)
	}

	pyBuckets := buildBuckets(pyMap, maxEntriesPerPinyinKey)
	initBuckets := buildBuckets(initMap, maxEntriesPerInitialsKey)
	t9Buckets := buildBuckets(t9Map, maxEntriesPerT9Key)
	emBuckets := buildBuckets(emMap, maxEntriesPerEmojiKey)

	if err := writeQDict(*outPath, pyBuckets, initBuckets, t9Buckets, emBuckets, enEntries); err != nil {
		fmt.Fprintf(os.Stderr, "写入 %s 失败: %v\n", *outPath, err)
		os.Exit(1)
	}

	info, _ := os.Stat(*outPath)
	fmt.Printf("构建完成: %s (%.2f MB)\n", *outPath, float64(info.Size())/(1024*1024))
	fmt.Printf("  全拼键: %d | 简拼键: %d | 九宫格T9键: %d | 英文词: %d | 微表情键: %d\n",
		len(pyBuckets), len(initBuckets), len(t9Buckets), len(enEntries), len(emBuckets))
}
