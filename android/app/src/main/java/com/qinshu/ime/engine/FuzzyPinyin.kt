package com.qinshu.ime.engine

/**
 * 完整移植自桌面版 internal/fuzzy/fuzzy.go，并新增九宫格 T9 音节映射表与 26 键邻键容错。
 */
object FuzzyPinyin {

    data class QueryVariant(
        val fullKey: String,
        val syllables: List<String>,
        val weight: Double,
    )

    data class FuzzyPair(val rule: String, val a: String, val b: String)

    val initialPairs = listOf(
        FuzzyPair("z_zh", "z", "zh"),
        FuzzyPair("c_ch", "c", "ch"),
        FuzzyPair("s_sh", "s", "sh"),
        FuzzyPair("l_n", "l", "n"),
        FuzzyPair("f_h", "f", "h"),
        FuzzyPair("r_l", "r", "l"),
        FuzzyPair("k_g", "k", "g"),
    )

    val finalPairs = listOf(
        FuzzyPair("ian_iang", "ian", "iang"),
        FuzzyPair("uan_uang", "uan", "uang"),
        FuzzyPair("an_ang", "an", "ang"),
        FuzzyPair("en_eng", "en", "eng"),
        FuzzyPair("in_ing", "in", "ing"),
    )

    private val uvEquiv = mapOf(
        "lu" to "lv", "lv" to "lu",
        "lue" to "lve", "lve" to "lue",
        "nu" to "nv", "nv" to "nu",
        "nue" to "nve", "nve" to "nue",
    )

    val validSyllables: Set<String> = setOf(
        "a", "ai", "an", "ang", "ao",
        "ba", "bai", "ban", "bang", "bao", "bei", "ben", "beng", "bi", "bian", "biao", "bie", "bin", "bing", "bo", "bu",
        "ca", "cai", "can", "cang", "cao", "ce", "cen", "ceng", "cha", "chai", "chan", "chang", "chao", "che", "chen", "cheng", "chi", "chong", "chou", "chu", "chua", "chuai", "chuan", "chuang", "chui", "chun", "chuo", "ci", "cong", "cou", "cu", "cuan", "cui", "cun", "cuo",
        "da", "dai", "dan", "dang", "dao", "de", "dei", "den", "deng", "di", "dia", "dian", "diao", "die", "ding", "diu", "dong", "dou", "du", "duan", "dui", "dun", "duo",
        "e", "ei", "en", "eng", "er",
        "fa", "fan", "fang", "fei", "fen", "feng", "fo", "fou", "fu",
        "ga", "gai", "gan", "gang", "gao", "ge", "gei", "gen", "geng", "gong", "gou", "gu", "gua", "guai", "guan", "guang", "gui", "gun", "guo",
        "ha", "hai", "han", "hang", "hao", "he", "hei", "hen", "heng", "hong", "hou", "hu", "hua", "huai", "huan", "huang", "hui", "hun", "huo",
        "ji", "jia", "jian", "jiang", "jiao", "jie", "jin", "jing", "jiong", "jiu", "ju", "juan", "jue", "jun",
        "ka", "kai", "kan", "kang", "kao", "ke", "kei", "ken", "keng", "kong", "kou", "ku", "kua", "kuai", "kuan", "kuang", "kui", "kun", "kuo",
        "la", "lai", "lan", "lang", "lao", "le", "lei", "leng", "li", "lia", "lian", "liang", "liao", "lie", "lin", "ling", "liu", "lo", "long", "lou", "lu", "luan", "lue", "lun", "luo", "lv", "lve",
        "ma", "mai", "man", "mang", "mao", "me", "mei", "men", "meng", "mi", "mian", "miao", "mie", "min", "ming", "miu", "mo", "mou", "mu",
        "na", "nai", "nan", "nang", "nao", "ne", "nei", "nen", "neng", "ni", "nian", "niang", "niao", "nie", "nin", "ning", "niu", "nong", "nou", "nu", "nuan", "nue", "nun", "nuo", "nv", "nve",
        "o", "ou",
        "pa", "pai", "pan", "pang", "pao", "pei", "pen", "peng", "pi", "pian", "piao", "pie", "pin", "ping", "po", "pou", "pu",
        "qi", "qia", "qian", "qiang", "qiao", "qie", "qin", "qing", "qiong", "qiu", "qu", "quan", "que", "qun",
        "ran", "rang", "rao", "re", "ren", "reng", "ri", "rong", "rou", "ru", "rua", "ruan", "rui", "run", "ruo",
        "sa", "sai", "san", "sang", "sao", "se", "sen", "seng", "sha", "shai", "shan", "shang", "shao", "she", "shei", "shen", "sheng", "shi", "shou", "shu", "shua", "shuai", "shuan", "shuang", "shui", "shun", "shuo", "si", "song", "sou", "su", "suan", "sui", "sun", "suo",
        "ta", "tai", "tan", "tang", "tao", "te", "tei", "teng", "ti", "tian", "tiao", "tie", "ting", "tong", "tou", "tu", "tuan", "tui", "tun", "tuo",
        "wa", "wai", "wan", "wang", "wei", "wen", "weng", "wo", "wu",
        "xi", "xia", "xian", "xiang", "xiao", "xie", "xin", "xing", "xiong", "xiu", "xu", "xuan", "xue", "xun",
        "ya", "yan", "yang", "yao", "ye", "yi", "yin", "ying", "yo", "yong", "you", "yu", "yuan", "yue", "yun",
        "za", "zai", "zan", "zang", "zao", "ze", "zei", "zen", "zeng", "zha", "zhai", "zhan", "zhang", "zhao", "zhe", "zhei", "zhen", "zheng", "zhi", "zhong", "zhou", "zhu", "zhua", "zhuai", "zhuan", "zhuang", "zhui", "zhun", "zhuo", "zi", "zong", "zou", "zu", "zuan", "zui", "zun", "zuo",
    )

    val validSyllablePrefixes: Set<String> by lazy {
        HashSet<String>(900).apply {
            for (syl in validSyllables) {
                for (i in 1..syl.length) add(syl.substring(0, i))
            }
        }
    }

    /** 预计算九宫格 T9 数字串 -> 所有合法单音节列表（用于九宫格音节格解码与左侧音节锁词栏） */
    val t9ToSyllables: Map<String, List<String>> by lazy {
        val m = HashMap<String, MutableList<String>>(256)
        for (syl in validSyllables) {
            val t9 = stringToT9(syl)
            m.getOrPut(t9) { ArrayList() }.add(syl)
        }
        m.values.forEach { it.sort() }
        m
    }

    /** 26 键邻键容错映射（手机拇指高频误触键对） */
    val adjacentKeys: Map<Char, String> = mapOf(
        'q' to "wa", 'w' to "qeas", 'e' to "wrsd", 'r' to "etdf", 't' to "ryfg",
        'y' to "tugh", 'u' to "yihj", 'i' to "uojk", 'o' to "ipkl", 'p' to "ol",
        'a' to "qwsz", 's' to "awedxz", 'd' to "serfcx", 'f' to "drtgvc", 'g' to "ftyhbv",
        'h' to "gyujnb", 'j' to "huikmn", 'k' to "jiolm", 'l' to "kop",
        'z' to "asx", 'x' to "zsdc", 'c' to "xdfv", 'v' to "cfgb", 'b' to "vghn",
        'n' to "bhjm", 'm' to "njk",
    )

    fun defaultFuzzyRules(): Map<String, Boolean> = mapOf(
        "z_zh" to true, "c_ch" to true, "s_sh" to true, "l_n" to true,
        "f_h" to false, "r_l" to false, "k_g" to false,
        "an_ang" to true, "en_eng" to true, "in_ing" to true,
        "ian_iang" to false, "uan_uang" to false,
    )

    fun stringToT9(s: String): String {
        val sb = StringBuilder(s.length)
        for (ch in s) {
            val d = when (ch) {
                'a', 'b', 'c' -> '2'
                'd', 'e', 'f' -> '3'
                'g', 'h', 'i' -> '4'
                'j', 'k', 'l' -> '5'
                'm', 'n', 'o' -> '6'
                'p', 'q', 'r', 's' -> '7'
                't', 'u', 'v' -> '8'
                'w', 'x', 'y', 'z' -> '9'
                else -> continue
            }
            sb.append(d)
        }
        return sb.toString()
    }

    fun splitInitialFinal(syl: String): Pair<String, String> {
        for (ini in arrayOf("zh", "ch", "sh")) {
            if (syl.startsWith(ini)) return ini to syl.substring(2)
        }
        if (syl.isNotEmpty() && "bpmfdtnlgkhjqxrzcsyw".indexOf(syl[0]) >= 0) {
            return syl.substring(0, 1) to syl.substring(1)
        }
        return "" to syl
    }

    fun expandSyllable(syl: String, enabled: Boolean, rules: Map<String, Boolean>): List<QueryVariant> {
        val s = syl.lowercase()
        if (s.isEmpty()) return emptyList()
        val res = LinkedHashMap<String, Double>()
        res[s] = 1.0
        uvEquiv[s]?.let { res[it] = 0.85 }

        if (enabled) {
            val (ini, fin) = splitInitialFinal(s)
            val iniVars = mutableListOf(ini to 1.0)
            for (p in initialPairs) {
                if (rules[p.rule] != true) continue
                if (ini == p.a) iniVars.add(p.b to 0.72)
                else if (ini == p.b) iniVars.add(p.a to 0.72)
            }
            val finVars = mutableListOf(fin to 1.0)
            for (p in finalPairs) {
                if (rules[p.rule] != true) continue
                when {
                    fin == p.a -> finVars.add(p.b to 0.75)
                    fin == p.b -> finVars.add(p.a to 0.75)
                    (p.rule == "an_ang" || p.rule == "en_eng" || p.rule == "in_ing") && fin.endsWith(p.a) ->
                        finVars.add(fin.dropLast(p.a.length) + p.b to 0.75)
                    (p.rule == "an_ang" || p.rule == "en_eng" || p.rule == "in_ing") && fin.endsWith(p.b) ->
                        finVars.add(fin.dropLast(p.b.length) + p.a to 0.75)
                }
            }
            for ((iv, iw) in iniVars) {
                for ((fv, fw) in finVars) {
                    val comb = iv + fv
                    val w = iw * fw
                    if (comb == s || validSyllables.contains(comb)) {
                        if ((res[comb] ?: 0.0) < w) res[comb] = w
                        uvEquiv[comb]?.let { eq ->
                            val ew = w * 0.85
                            if ((res[eq] ?: 0.0) < ew) res[eq] = ew
                        }
                    }
                }
            }
        }
        return res.entries
            .map { QueryVariant(it.key, listOf(it.key), it.value) }
            .sortedByDescending { it.weight }
    }

    fun segmentPinyin(raw: String, maxPaths: Int = 6): List<List<String>> {
        val text = raw.trim().lowercase()
        if (text.isEmpty()) return emptyList()
        if (text.contains('\'')) {
            var paths = listOf(emptyList<String>())
            for (part in text.split('\'')) {
                if (part.isEmpty()) continue
                val sub = segmentPinyin(part, 3)
                if (sub.isEmpty()) return emptyList()
                val next = ArrayList<List<String>>()
                for (p in paths) {
                    for (s in sub) {
                        next.add(p + s)
                        if (next.size >= maxPaths) break
                    }
                    if (next.size >= maxPaths) break
                }
                paths = next
            }
            return paths
        }

        val n = text.length
        val results = ArrayList<List<String>>()
        val acc = ArrayList<String>()

        fun dfs(pos: Int) {
            if (results.size >= maxPaths) return
            if (pos == n) {
                results.add(ArrayList(acc))
                return
            }
            var matchedFull = false
            val maxLen = minOf(6, n - pos)
            for (len in maxLen downTo 1) {
                val piece = text.substring(pos, pos + len)
                if (validSyllables.contains(piece)) {
                    matchedFull = true
                    acc.add(piece)
                    dfs(pos + len)
                    acc.removeAt(acc.size - 1)
                    if (results.size >= maxPaths) return
                }
            }
            val rem = text.substring(pos)
            if (!matchedFull && rem.length <= 5 && validSyllablePrefixes.contains(rem)) {
                acc.add(rem)
                dfs(n)
                acc.removeAt(acc.size - 1)
                if (results.size >= maxPaths) return
            }
            if (pos + 2 <= n) {
                val two = text.substring(pos, pos + 2)
                if (two == "zh" || two == "ch" || two == "sh") {
                    acc.add(two)
                    dfs(pos + 2)
                    acc.removeAt(acc.size - 1)
                    if (results.size >= maxPaths) return
                }
            }
            if (!matchedFull || pos == n - 1) {
                acc.add(text.substring(pos, pos + 1))
                dfs(pos + 1)
                acc.removeAt(acc.size - 1)
            }
        }

        dfs(0)
        return results
            .distinctBy { it.joinToString("'") }
            .sortedBy { it.size }
            .take(maxPaths)
    }

    fun expandQuery(raw: String, enabled: Boolean, rules: Map<String, Boolean>, maxVariants: Int = 24): List<QueryVariant> {
        val paths = segmentPinyin(raw, 4)
        if (paths.isEmpty()) return emptyList()
        val best = HashMap<String, Pair<List<String>, Double>>()
        for ((pIdx, path) in paths.withIndex()) {
            val penalty = if (pIdx == 0) 1.0 else 0.92
            var current = listOf(emptyList<String>() to penalty)
            for (syl in path) {
                var sylVars = expandSyllable(syl, enabled, rules)
                if (path.size >= 4 && sylVars.size > 3) sylVars = sylVars.take(3)
                val next = ArrayList<Pair<List<String>, Double>>()
                for ((curSyls, curW) in current) {
                    for (sv in sylVars) {
                        next.add((curSyls + sv.fullKey) to (curW * sv.weight))
                    }
                }
                next.sortByDescending { it.second }
                current = if (next.size > maxVariants) next.subList(0, maxVariants) else next
            }
            for ((syls, w) in current) {
                val k = syls.joinToString("'")
                val prev = best[k]
                if (prev == null || w > prev.second) best[k] = syls to w
            }
        }
        return best.values
            .map { QueryVariant(it.first.joinToString(""), it.first, it.second) }
            .sortedWith(compareByDescending<QueryVariant> { it.weight }.thenBy { it.syllables.size })
            .take(maxVariants)
    }
}
