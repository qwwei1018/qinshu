package fuzzy

import (
	"math"
	"sort"
	"strings"
)

// QueryVariant 表示一次拼音模糊展开后的查询变体。
type QueryVariant struct {
	FullKey   string
	Syllables []string
	Weight    float64
}

type strWeight struct {
	val    string
	weight float64
}

// InitialPair 声母模糊音对照。
type InitialPair struct {
	Rule string
	A    string
	B    string
}

// FinalPair 韵母模糊音对照。
type FinalPair struct {
	Rule string
	A    string
	B    string
}

// FuzzyInitialPairs 7 组声母模糊音。
var FuzzyInitialPairs = []InitialPair{
	{"z_zh", "z", "zh"},
	{"c_ch", "c", "ch"},
	{"s_sh", "s", "sh"},
	{"l_n", "l", "n"},
	{"f_h", "f", "h"},
	{"r_l", "r", "l"},
	{"k_g", "k", "g"},
}

// FuzzyFinalPairs 5 组韵母模糊音（长韵母优先）。
var FuzzyFinalPairs = []FinalPair{
	{"ian_iang", "ian", "iang"},
	{"uan_uang", "uan", "uang"},
	{"an_ang", "an", "ang"},
	{"en_eng", "en", "eng"},
	{"in_ing", "in", "ing"},
}

var validInitials = map[string]bool{
	"zh": true, "ch": true, "sh": true,
	"b": true, "p": true, "m": true, "f": true,
	"d": true, "t": true, "n": true, "l": true,
	"g": true, "k": true, "h": true,
	"j": true, "q": true, "x": true,
	"r": true, "z": true, "c": true, "s": true,
	"y": true, "w": true,
}

// ValidSyllables 全部标准汉语拼音音节集合。
var ValidSyllables = map[string]bool{
	"a": true, "ai": true, "an": true, "ang": true, "ao": true,
	"ba": true, "bai": true, "ban": true, "bang": true, "bao": true, "bei": true, "ben": true, "beng": true, "bi": true, "bian": true, "biao": true, "bie": true, "bin": true, "bing": true, "bo": true, "bu": true,
	"ca": true, "cai": true, "can": true, "cang": true, "cao": true, "ce": true, "cen": true, "ceng": true, "cha": true, "chai": true, "chan": true, "chang": true, "chao": true, "che": true, "chen": true, "cheng": true, "chi": true, "chong": true, "chou": true, "chu": true, "chua": true, "chuai": true, "chuan": true, "chuang": true, "chui": true, "chun": true, "chuo": true, "ci": true, "cong": true, "cou": true, "cu": true, "cuan": true, "cui": true, "cun": true, "cuo": true,
	"da": true, "dai": true, "dan": true, "dang": true, "dao": true, "de": true, "dei": true, "den": true, "deng": true, "di": true, "dia": true, "dian": true, "diao": true, "die": true, "ding": true, "diu": true, "dong": true, "dou": true, "du": true, "duan": true, "dui": true, "dun": true, "duo": true,
	"e": true, "ei": true, "en": true, "eng": true, "er": true,
	"fa": true, "fan": true, "fang": true, "fei": true, "fen": true, "feng": true, "fo": true, "fou": true, "fu": true,
	"ga": true, "gai": true, "gan": true, "gang": true, "gao": true, "ge": true, "gei": true, "gen": true, "geng": true, "gong": true, "gou": true, "gu": true, "gua": true, "guai": true, "guan": true, "guang": true, "gui": true, "gun": true, "guo": true,
	"ha": true, "hai": true, "han": true, "hang": true, "hao": true, "he": true, "hei": true, "hen": true, "heng": true, "hong": true, "hou": true, "hu": true, "hua": true, "huai": true, "huan": true, "huang": true, "hui": true, "hun": true, "huo": true,
	"ji": true, "jia": true, "jian": true, "jiang": true, "jiao": true, "jie": true, "jin": true, "jing": true, "jiong": true, "jiu": true, "ju": true, "juan": true, "jue": true, "jun": true,
	"ka": true, "kai": true, "kan": true, "kang": true, "kao": true, "ke": true, "kei": true, "ken": true, "keng": true, "kong": true, "kou": true, "ku": true, "kua": true, "kuai": true, "kuan": true, "kuang": true, "kui": true, "kun": true, "kuo": true,
	"la": true, "lai": true, "lan": true, "lang": true, "lao": true, "le": true, "lei": true, "leng": true, "li": true, "lia": true, "lian": true, "liang": true, "liao": true, "lie": true, "lin": true, "ling": true, "liu": true, "lo": true, "long": true, "lou": true, "lu": true, "luan": true, "lue": true, "lun": true, "luo": true, "lv": true, "lve": true,
	"ma": true, "mai": true, "man": true, "mang": true, "mao": true, "me": true, "mei": true, "men": true, "meng": true, "mi": true, "mian": true, "miao": true, "mie": true, "min": true, "ming": true, "miu": true, "mo": true, "mou": true, "mu": true,
	"na": true, "nai": true, "nan": true, "nang": true, "nao": true, "ne": true, "nei": true, "nen": true, "neng": true, "ni": true, "nian": true, "niang": true, "niao": true, "nie": true, "nin": true, "ning": true, "niu": true, "nong": true, "nou": true, "nu": true, "nuan": true, "nue": true, "nun": true, "nuo": true, "nv": true, "nve": true,
	"o": true, "ou": true,
	"pa": true, "pai": true, "pan": true, "pang": true, "pao": true, "pei": true, "pen": true, "peng": true, "pi": true, "pian": true, "piao": true, "pie": true, "pin": true, "ping": true, "po": true, "pou": true, "pu": true,
	"qi": true, "qia": true, "qian": true, "qiang": true, "qiao": true, "qie": true, "qin": true, "qing": true, "qiong": true, "qiu": true, "qu": true, "quan": true, "que": true, "qun": true,
	"ran": true, "rang": true, "rao": true, "re": true, "ren": true, "reng": true, "ri": true, "rong": true, "rou": true, "ru": true, "rua": true, "ruan": true, "rui": true, "run": true, "ruo": true,
	"sa": true, "sai": true, "san": true, "sang": true, "sao": true, "se": true, "sen": true, "seng": true, "sha": true, "shai": true, "shan": true, "shang": true, "shao": true, "she": true, "shei": true, "shen": true, "sheng": true, "shi": true, "shou": true, "shu": true, "shua": true, "shuai": true, "shuan": true, "shuang": true, "shui": true, "shun": true, "shuo": true, "si": true, "song": true, "sou": true, "su": true, "suan": true, "sui": true, "sun": true, "suo": true,
	"ta": true, "tai": true, "tan": true, "tang": true, "tao": true, "te": true, "tei": true, "teng": true, "ti": true, "tian": true, "tiao": true, "tie": true, "ting": true, "tong": true, "tou": true, "tu": true, "tuan": true, "tui": true, "tun": true, "tuo": true,
	"wa": true, "wai": true, "wan": true, "wang": true, "wei": true, "wen": true, "weng": true, "wo": true, "wu": true,
	"xi": true, "xia": true, "xian": true, "xiang": true, "xiao": true, "xie": true, "xin": true, "xing": true, "xiong": true, "xiu": true, "xu": true, "xuan": true, "xue": true, "xun": true,
	"ya": true, "yan": true, "yang": true, "yao": true, "ye": true, "yi": true, "yin": true, "ying": true, "yo": true, "yong": true, "you": true, "yu": true, "yuan": true, "yue": true, "yun": true,
	"za": true, "zai": true, "zan": true, "zang": true, "zao": true, "ze": true, "zei": true, "zen": true, "zeng": true, "zha": true, "zhai": true, "zhan": true, "zhang": true, "zhao": true, "zhe": true, "zhei": true, "zhen": true, "zheng": true, "zhi": true, "zhong": true, "zhou": true, "zhu": true, "zhua": true, "zhuai": true, "zhuan": true, "zhuang": true, "zhui": true, "zhun": true, "zhuo": true, "zi": true, "zong": true, "zou": true, "zu": true, "zuan": true, "zui": true, "zun": true, "zuo": true,
}

// ValidSyllablePrefixes 预计算全部合法音节的前缀集合（用于识别如 zhon / shua 等未打完的尾音节）。
var ValidSyllablePrefixes map[string]bool

// u/v 撮口呼容错映射（例如输入 celue 可匹配 策略 ce lve，输入 xiaolu 可匹配 效率 xiao lv）。
var uvEquiv = map[string]string{
	"lu":  "lv",
	"lv":  "lu",
	"lue": "lve",
	"lve": "lue",
	"nu":  "nv",
	"nv":  "nu",
	"nue": "nve",
	"nve": "nue",
}

func init() {
	ValidSyllablePrefixes = make(map[string]bool, 900)
	for syl := range ValidSyllables {
		for i := 1; i <= len(syl); i++ {
			ValidSyllablePrefixes[syl[:i]] = true
		}
	}
}

// SplitInitialFinal 将拼音音节拆分为 (声母, 韵母)。
func SplitInitialFinal(syl string) (string, string) {
	for _, ini := range []string{"zh", "ch", "sh"} {
		if strings.HasPrefix(syl, ini) {
			return ini, syl[2:]
		}
	}
	if len(syl) > 0 && strings.ContainsRune("bpmfdtnlgkhjqxrzcsyw", rune(syl[0])) {
		return syl[:1], syl[1:]
	}
	return "", syl
}

// ExpandSyllable 展开单个音节的所有模糊音与 u/v 容错变体。
func ExpandSyllable(syl string, enabled bool, rules map[string]bool) []QueryVariant {
	s := strings.ToLower(syl)
	if s == "" {
		return nil
	}

	resMap := map[string]float64{s: 1.0}

	// 即使模糊音关闭，仍保留常见 u/v 撮口呼自然容错（略微降权）
	if eq, ok := uvEquiv[s]; ok {
		resMap[eq] = 0.85
	}

	if enabled {
		ini, fin := SplitInitialFinal(s)

		iniVars := []strWeight{{ini, 1.0}}
		seenIni := map[string]bool{ini: true}
		for _, pair := range FuzzyInitialPairs {
			if !rules[pair.Rule] {
				continue
			}
			if ini == pair.A && !seenIni[pair.B] {
				iniVars = append(iniVars, strWeight{pair.B, 0.72})
				seenIni[pair.B] = true
			} else if ini == pair.B && !seenIni[pair.A] {
				iniVars = append(iniVars, strWeight{pair.A, 0.72})
				seenIni[pair.A] = true
			}
		}

		finVars := []strWeight{{fin, 1.0}}
		seenFin := map[string]bool{fin: true}
		for _, pair := range FuzzyFinalPairs {
			if !rules[pair.Rule] {
				continue
			}
			if fin == pair.A && !seenFin[pair.B] {
				finVars = append(finVars, strWeight{pair.B, 0.75})
				seenFin[pair.B] = true
			} else if fin == pair.B && !seenFin[pair.A] {
				finVars = append(finVars, strWeight{pair.A, 0.75})
				seenFin[pair.A] = true
			} else if (pair.Rule == "an_ang" || pair.Rule == "en_eng" || pair.Rule == "in_ing") && strings.HasSuffix(fin, pair.A) {
				cand := fin[:len(fin)-len(pair.A)] + pair.B
				if !seenFin[cand] {
					finVars = append(finVars, strWeight{cand, 0.75})
					seenFin[cand] = true
				}
			} else if (pair.Rule == "an_ang" || pair.Rule == "en_eng" || pair.Rule == "in_ing") && strings.HasSuffix(fin, pair.B) {
				cand := fin[:len(fin)-len(pair.B)] + pair.A
				if !seenFin[cand] {
					finVars = append(finVars, strWeight{cand, 0.75})
					seenFin[cand] = true
				}
			}
		}

		for _, iv := range iniVars {
			for _, fv := range finVars {
				comb := iv.val + fv.val
				w := math.Round(iv.weight*fv.weight*10000) / 10000
				if comb == s || ValidSyllables[comb] || (fv.val == "" && validInitials[iv.val]) {
					if prev, ok := resMap[comb]; !ok || w > prev {
						resMap[comb] = w
					}
					if eq, ok := uvEquiv[comb]; ok {
						ew := math.Round(w*0.85*10000) / 10000
						if prev, ok2 := resMap[eq]; !ok2 || ew > prev {
							resMap[eq] = ew
						}
					}
				}
			}
		}
	}

	out := make([]QueryVariant, 0, len(resMap))
	for k, w := range resMap {
		out = append(out, QueryVariant{FullKey: k, Syllables: []string{k}, Weight: w})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Weight != out[j].Weight {
			return out[i].Weight > out[j].Weight
		}
		return out[i].FullKey < out[j].FullKey
	})
	return out
}

// SegmentPinyin 将连续拼音串切分为多个音节路径（支持 `'` 分词符、简拼及末尾未输完音节前缀）。
func SegmentPinyin(raw string, maxPaths int) [][]string {
	text := strings.TrimSpace(strings.ToLower(raw))
	if text == "" {
		return nil
	}
	if maxPaths <= 0 {
		maxPaths = 8
	}

	if strings.Contains(text, "'") {
		rawParts := strings.Split(text, "'")
		paths := [][]string{{}}
		for _, part := range rawParts {
			if part == "" {
				continue
			}
			sub := SegmentPinyin(part, 3)
			if len(sub) == 0 {
				return nil
			}
			var next [][]string
			for _, p := range paths {
				for _, s := range sub {
					merged := make([]string, 0, len(p)+len(s))
					merged = append(merged, p...)
					merged = append(merged, s...)
					next = append(next, merged)
					if len(next) >= maxPaths {
						break
					}
				}
				if len(next) >= maxPaths {
					break
				}
			}
			paths = next
		}
		return paths
	}

	n := len(text)
	var results [][]string
	var acc []string

	var dfs func(pos int)
	dfs = func(pos int) {
		if len(results) >= maxPaths {
			return
		}
		if pos == n {
			cp := make([]string, len(acc))
			copy(cp, acc)
			results = append(results, cp)
			return
		}

		// 1. 尝试完整合法音节（最长优先）
		matchedFull := false
		maxLen := n - pos
		if maxLen > 6 {
			maxLen = 6
		}
		for length := maxLen; length >= 1; length-- {
			piece := text[pos : pos+length]
			if ValidSyllables[piece] {
				matchedFull = true
				acc = append(acc, piece)
				dfs(pos + length)
				acc = acc[:len(acc)-1]
				if len(results) >= maxPaths {
					return
				}
			}
		}

		// 2. 末尾未输完的合法音节前缀（例如 zhon / shua / chuang 的前缀）
		rem := text[pos:]
		if !matchedFull && len(rem) <= 5 && ValidSyllablePrefixes[rem] {
			acc = append(acc, rem)
			dfs(n)
			acc = acc[:len(acc)-1]
			if len(results) >= maxPaths {
				return
			}
		}

		// 3. 双字母翘舌声母简拼
		if pos+2 <= n {
			two := text[pos : pos+2]
			if two == "zh" || two == "ch" || two == "sh" {
				acc = append(acc, two)
				dfs(pos + 2)
				acc = acc[:len(acc)-1]
				if len(results) >= maxPaths {
					return
				}
			}
		}

		// 4. 单字母简拼兜底
		if !matchedFull || pos == n-1 {
			acc = append(acc, text[pos:pos+1])
			dfs(pos + 1)
			acc = acc[:len(acc)-1]
		}
	}

	dfs(0)

	sort.Slice(results, func(i, j int) bool {
		if len(results[i]) != len(results[j]) {
			return len(results[i]) < len(results[j])
		}
		return strings.Join(results[i], "'") < strings.Join(results[j], "'")
	})

	unique := make([][]string, 0, len(results))
	seen := make(map[string]bool)
	for _, p := range results {
		key := strings.Join(p, "'")
		if !seen[key] {
			seen[key] = true
			unique = append(unique, p)
			if len(unique) >= maxPaths {
				break
			}
		}
	}
	return unique
}

// ExpandQuery 将用户输入拼音串展开为所有模糊音查询变体（带权重降序）。
func ExpandQuery(raw string, enabled bool, rules map[string]bool, maxVariants int) []QueryVariant {
	if maxVariants <= 0 {
		maxVariants = 48
	}
	paths := SegmentPinyin(raw, 4)
	if len(paths) == 0 {
		return nil
	}

	type accState struct {
		syls   []string
		weight float64
	}
	bestMap := make(map[string]accState)

	for pathIdx, path := range paths {
		penalty := 1.0
		if pathIdx > 0 {
			penalty = 0.92
		}
		current := []accState{{syls: nil, weight: penalty}}
		for _, syl := range path {
			sylVars := ExpandSyllable(syl, enabled, rules)
			if len(path) >= 4 && len(sylVars) > 3 {
				sylVars = sylVars[:3]
			}
			var next []accState
			for _, cur := range current {
				for _, sv := range sylVars {
					ns := make([]string, len(cur.syls)+1)
					copy(ns, cur.syls)
					ns[len(cur.syls)] = sv.FullKey
					nw := math.Round(cur.weight*sv.Weight*10000) / 10000
					next = append(next, accState{syls: ns, weight: nw})
				}
			}
			sort.Slice(next, func(i, j int) bool {
				return next[i].weight > next[j].weight
			})
			if len(next) > maxVariants {
				next = next[:maxVariants]
			}
			current = next
		}

		for _, st := range current {
			key := strings.Join(st.syls, "'")
			if prev, ok := bestMap[key]; !ok || st.weight > prev.weight {
				bestMap[key] = st
			}
		}
	}

	out := make([]QueryVariant, 0, len(bestMap))
	for _, st := range bestMap {
		out = append(out, QueryVariant{
			FullKey:   strings.Join(st.syls, ""),
			Syllables: st.syls,
			Weight:    st.weight,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Weight != out[j].Weight {
			return out[i].Weight > out[j].Weight
		}
		return len(out[i].Syllables) < len(out[j].Syllables)
	})
	if len(out) > maxVariants {
		out = out[:maxVariants]
	}
	return out
}

// IsPotentialPinyinPrefix 判断输入串（通常为 4 字母）是否可能是合法拼音词的前缀（如 zhon->zhongguo, shur->shurufa, jint->jintian）。
// 用于五拼混输模式下防止四码五笔自动上屏误截断正在输入的拼音长词。
func IsPotentialPinyinPrefix(raw string) bool {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return false
	}
	if ValidSyllables[s] || ValidSyllablePrefixes[s] {
		return true
	}
	// 检查是否可拆为 [合法音节] + [合法音节或合法音节前缀]
	for i := 1; i < len(s); i++ {
		left := s[:i]
		right := s[i:]
		if ValidSyllables[left] && (ValidSyllables[right] || ValidSyllablePrefixes[right]) {
			return true
		}
	}
	return false
}
