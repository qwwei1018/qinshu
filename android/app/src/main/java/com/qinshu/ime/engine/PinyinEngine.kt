package com.qinshu.ime.engine

data class Candidate(
    val text: String,
    val code: String,
    val source: String, // "pinyin" | "t9" | "english" | "emoji" | "calc" | "command" | "phrase"
    val score: Double,
    val hint: String = "",
    val composed: Boolean = false,
)

enum class LayoutMode { QWERTY, T9 }
enum class LanguageMode { ZH, EN }

data class EngineResult(
    val candidates: List<Candidate>,
    val t9Syllables: List<String> = emptyList(),
    val composingDisplay: String = "",
)

/**
 * Qin书安卓端统一解码内核（v1.1 升级版）：
 * - 全键盘（26 键）与九宫格（T9）无缝切换
 * - 新增 26 键邻键容错纠错（解决手机拇指误触）与九宫格逐音节回删
 * - 强化常用词语与常用字置顶排序 + 16 音节 DP 整句
 * - 实时算式计算（11+12 -> 23 / 11+12=23 / 百分比 / =1280 人民币大写）
 * - 拼音实时联想纯 Emoji 微表情
 */
class PinyinEngine(
    private val dict: QDictReader,
    private val userStore: UserDictStore,
) {
    var layoutMode: LayoutMode = LayoutMode.QWERTY
    var languageMode: LanguageMode = LanguageMode.ZH
    var fuzzyEnabled: Boolean = true
    var fuzzyRules: Map<String, Boolean> = FuzzyPinyin.defaultFuzzyRules()
    var emojiSuggestionEnabled: Boolean = true
    var numMode: Boolean = false

    private val buffer = StringBuilder()
    private var lockedT9Syllable: String? = null
    private var smartMode: String = ""

    fun getBuffer(): String = buffer.toString()
    fun hasBuffer(): Boolean = buffer.isNotEmpty()
    fun isSmartInputActive(): Boolean = smartMode.isNotEmpty()

    fun clear() {
        buffer.setLength(0)
        lockedT9Syllable = null
        smartMode = ""
    }

    fun lockT9Syllable(syl: String) {
        lockedT9Syllable = if (lockedT9Syllable == syl) null else syl
    }

    fun appendChar(ch: Char): EngineResult {
        if (ch == '@' && smartMode.isEmpty() && buffer.isNotEmpty()) {
            smartMode = "email"
        } else if (ch == ':' && smartMode.isEmpty()) {
            val low = buffer.toString().lowercase()
            if (low == "http" || low == "https" || low == "ftp") {
                smartMode = "url"
                buffer.append("://")
                return query()
            }
        } else if (ch == '.' && smartMode.isEmpty() && buffer.toString().equals("www", ignoreCase = true)) {
            smartMode = "url"
        }
        buffer.append(ch)
        return query()
    }

    /** 退格支持：九宫格下若已锁音节或含分词符，优先逐段回退而非整串清空 */
    fun backspace(): EngineResult {
        if (buffer.isEmpty()) return EngineResult(emptyList())
        if (layoutMode == LayoutMode.T9 && lockedT9Syllable != null) {
            val cur = buffer.toString()
            if (cur.length <= lockedT9Syllable!!.length) {
                lockedT9Syllable = null
                return query()
            }
        }
        buffer.deleteCharAt(buffer.length - 1)
        if (buffer.isEmpty()) {
            clear()
            return EngineResult(emptyList())
        }
        val cur = buffer.toString()
        if (smartMode == "email" && !cur.contains('@')) smartMode = ""
        if (smartMode == "url" && !cur.contains("://") && !cur.lowercase().startsWith("www.")) smartMode = ""
        if (lockedT9Syllable != null && cur.length < lockedT9Syllable!!.length) {
            lockedT9Syllable = null
        }
        return query()
    }

    fun onCandidateSelected(cand: Candidate, index: Int) {
        when (cand.source) {
            "pinyin", "t9", "english", "phrase", "command" -> {
                val src = if (cand.source == "t9") "pinyin" else cand.source
                userStore.boostSelection(src, cand.code, cand.text, index, 5)
            }
        }
        clear()
    }

    fun query(): EngineResult {
        val raw = buffer.toString()
        if (raw.isEmpty()) return EngineResult(emptyList())

        // 0. 实时算式 / 百分比 / 人民币大写（如 11+12 -> [23, 11+12=23], =1280 -> 壹仟贰佰捌拾元整）
        val calcCands = SmartRecognizer.evaluateExpressionCandidates(raw)
        if (calcCands.isNotEmpty()) {
            return EngineResult(candidates = calcCands, composingDisplay = raw)
        }

        if (smartMode == "email") {
            return EngineResult(SmartRecognizer.querySmartEmail(raw), composingDisplay = raw)
        }
        if (smartMode == "url") {
            return EngineResult(SmartRecognizer.querySmartUrl(raw), composingDisplay = raw)
        }
        if (numMode) {
            return EngineResult(listOf(Candidate(raw, raw, "num", 99999.0, "数字")), composingDisplay = raw)
        }
        if (languageMode == LanguageMode.EN) {
            return queryEnglishMode(raw)
        }
        if (layoutMode == LayoutMode.T9 && raw.all { it in '2'..'9' || it == '\'' }) {
            return queryT9Mode(raw)
        }
        return queryQwertyPinyin(raw)
    }

    private fun queryEnglishMode(raw: String): EngineResult {
        val low = raw.lowercase()
        val allUpper = raw.length > 1 && raw == raw.uppercase()
        val firstUpper = raw.first().isUpperCase()
        val list = ArrayList<Candidate>()
        val seen = HashSet<String>()

        fun add(c: Candidate) {
            if (seen.add(c.text)) list.add(c)
        }

        add(Candidate(raw, low, "english", 99999.0, "En"))
        userStore.queryPhrases(low).forEach(::add)
        SmartRecognizer.queryQuickCommands(low).forEach(::add)

        val userFreqs = userStore.getUserFreqs("english", low).associateBy { it.word.lowercase() }
        dict.lookupEnglishPrefix(low, 14).forEach { e ->
            val disp = when {
                allUpper -> e.word.uppercase()
                firstUpper -> e.word.replaceFirstChar { it.uppercase() }
                else -> e.word
            }
            val uBonus = userFreqs[e.word.lowercase()]?.let { userStore.effectiveBonus(it) } ?: 0.0
            val exactBonus = if (e.word.equals(low, ignoreCase = true)) 1200.0 else 0.0
            add(Candidate(disp, low, "english", e.weight + uBonus + exactBonus, "En"))
        }

        return EngineResult(candidates = list.sortedByDescending { it.score }.take(20), composingDisplay = raw)
    }

    private fun queryQwertyPinyin(raw: String): EngineResult {
        val code = raw.lowercase()
        val cleanKey = code.replace("'", "")
        val resMap = LinkedHashMap<String, Candidate>(64)

        fun putBest(c: Candidate) {
            val prev = resMap[c.text]
            if (prev == null || c.score > prev.score) resMap[c.text] = c
        }

        userStore.queryPhrases(cleanKey).forEach(::putBest)
        SmartRecognizer.queryQuickCommands(cleanKey).forEach(::putBest)

        val variants = FuzzyPinyin.expandQuery(code, fuzzyEnabled, fuzzyRules, 20)
        for (v in variants) {
            val isExact = v.weight >= 0.99
            val exactBoost = if (isExact) 6000.0 else 0.0
            val userMap = userStore.getUserFreqs("pinyin", v.fullKey).associateBy { it.word }
            val entries = dict.lookupPinyin(v.fullKey, if (isExact) 30 else 10)
            for (e in entries) {
                val uBonus = userMap[e.word]?.let { userStore.effectiveBonus(it) } ?: 0.0
                val score = (e.weight + uBonus + exactBoost) * v.weight
                putBest(Candidate(e.word, v.fullKey, "pinyin", score))
            }
            for ((w, item) in userMap) {
                val score = (8000.0 + userStore.effectiveBonus(item)) * v.weight
                putBest(Candidate(w, v.fullKey, "pinyin", score))
            }
        }

        // 26 键邻键误触容错（当精确匹配不足 3 条且长度 2..10 时触发）
        if (resMap.size < 3 && cleanKey.length in 2..10) {
            var checked = 0
            outer@ for (i in cleanKey.indices) {
                val adj = FuzzyPinyin.adjacentKeys[cleanKey[i]] ?: continue
                for (rep in adj) {
                    val alt = cleanKey.substring(0, i) + rep + cleanKey.substring(i + 1)
                    val hits = dict.lookupPinyin(alt, 4)
                    for (e in hits) {
                        putBest(Candidate(e.word, alt, "pinyin", e.weight * 0.72, "纠错"))
                    }
                    if (++checked >= 24) break@outer
                }
            }
        }

        val paths = FuzzyPinyin.segmentPinyin(code, 2)
        if (paths.isNotEmpty() && paths[0].size in 2..16) {
            composeSentence(paths[0])?.let { (sentence, dpScore) ->
                if (!resMap.containsKey(sentence)) {
                    putBest(Candidate(sentence, cleanKey, "pinyin", 14000.0 + dpScore * 0.08, "整句", composed = true))
                }
            }
        }

        if (cleanKey.length in 2..5) {
            val jpUser = userStore.getUserFreqs("pinyin", cleanKey).associateBy { it.word }
            dict.lookupInitials(cleanKey, 12).forEach { e ->
                val uBonus = jpUser[e.word]?.let { userStore.effectiveBonus(it) } ?: 0.0
                putBest(Candidate(e.word, cleanKey, "pinyin", e.weight * 0.62 + uBonus))
            }
        }

        if (resMap.size < 8 && cleanKey.length >= 2) {
            dict.lookupPinyinPrefix(cleanKey, 6, 3).forEach { (fullK, e) ->
                putBest(Candidate(e.word, fullK, "pinyin", e.weight * 0.35))
            }
        }

        if (cleanKey.length >= 3) {
            dict.lookupEnglishPrefix(cleanKey, 2).forEach { e ->
                putBest(Candidate(e.word, cleanKey, "english", e.weight * 0.45, "En"))
            }
        }

        val sorted = resMap.values.sortedByDescending { it.score }.take(32).toMutableList()
        if (emojiSuggestionEnabled) {
            val emojis = dict.lookupEmoji(cleanKey, 2)
            if (emojis.isNotEmpty()) {
                val insertAt = minOf(3, sorted.size)
                emojis.forEachIndexed { idx, em ->
                    sorted.add(insertAt + idx, Candidate(em.word, cleanKey, "emoji", 1.0, "微表情"))
                }
            }
        }

        val composing = if (paths.isNotEmpty()) paths[0].joinToString("'") else raw
        return EngineResult(candidates = sorted, composingDisplay = composing)
    }

    private fun queryT9Mode(raw: String): EngineResult {
        val digits = raw.replace("'", "")
        if (digits.isEmpty()) return EngineResult(emptyList())

        val leftSyllables = LinkedHashSet<String>()
        val maxPrefix = minOf(6, digits.length)
        for (len in maxPrefix downTo 1) {
            val sub = digits.substring(0, len)
            FuzzyPinyin.t9ToSyllables[sub]?.forEach { leftSyllables.add(it) }
            if (leftSyllables.size >= 8) break
        }
        val sylList = leftSyllables.toList()

        val locked = lockedT9Syllable
        if (locked != null && digits.startsWith(FuzzyPinyin.stringToT9(locked))) {
            val restDigits = digits.substring(locked.length)
            if (restDigits.isEmpty()) {
                val pyRes = queryQwertyPinyin(locked)
                return pyRes.copy(t9Syllables = sylList, composingDisplay = locked)
            }
            val res = ArrayList<Candidate>()
            dict.lookupPinyin(locked, 6).forEach { e ->
                res.add(Candidate(e.word, locked, "t9", e.weight.toDouble() + 2000.0))
            }
            dict.lookupT9(digits, 12).forEach { e ->
                res.add(Candidate(e.word, digits, "t9", e.weight.toDouble()))
            }
            return EngineResult(
                candidates = res.distinctBy { it.text }.sortedByDescending { it.score }.take(24),
                t9Syllables = sylList,
                composingDisplay = "$locked'$restDigits",
            )
        }

        val resMap = LinkedHashMap<String, Candidate>(40)
        fun putBest(c: Candidate) {
            val prev = resMap[c.text]
            if (prev == null || c.score > prev.score) resMap[c.text] = c
        }

        dict.lookupT9(digits, 15).forEach { e ->
            putBest(Candidate(e.word, digits, "t9", e.weight.toDouble() + 5000.0))
        }

        if (digits.length >= 4 && resMap.size < 6) {
            composeT9Sentence(digits)?.let { (sentence, score) ->
                putBest(Candidate(sentence, digits, "t9", 12000.0 + score * 0.08, "整句", composed = true))
            }
        }

        if (resMap.size < 10) {
            dict.lookupT9Prefix(digits, 6, 3).forEach { (k, e) ->
                putBest(Candidate(e.word, k, "t9", e.weight * 0.45))
            }
        }

        sylList.take(3).forEach { syl ->
            dict.lookupPinyin(syl, 4).forEach { e ->
                putBest(Candidate(e.word, syl, "t9", e.weight * 0.5))
            }
        }

        val sorted = resMap.values.sortedByDescending { it.score }.take(30).toMutableList()
        if (emojiSuggestionEnabled && sylList.isNotEmpty()) {
            dict.lookupEmoji(sylList.first(), 2).forEachIndexed { idx, em ->
                val at = minOf(3, sorted.size)
                sorted.add(at + idx, Candidate(em.word, sylList.first(), "emoji", 1.0, "微表情"))
            }
        }
        return EngineResult(candidates = sorted, t9Syllables = sylList, composingDisplay = raw)
    }

    private fun composeSentence(syls: List<String>): Pair<String, Double>? {
        val n = syls.size
        val dpScore = DoubleArray(n + 1) { -1.0 }
        val dpText = arrayOfNulls<String>(n + 1)
        dpScore[0] = 0.0
        dpText[0] = ""

        for (i in 0 until n) {
            if (dpScore[i] < 0) continue
            val maxSpan = minOf(4, n - i)
            for (span in maxSpan downTo 1) {
                val key = syls.subList(i, i + span).joinToString("")
                val top = dict.lookupPinyin(key, 1).firstOrNull() ?: continue
                val spanBonus = when (span) {
                    2 -> 4000.0
                    3 -> 8500.0
                    4 -> 13000.0
                    else -> 0.0
                }
                val nextScore = dpScore[i] + top.weight + spanBonus
                if (nextScore > dpScore[i + span]) {
                    dpScore[i + span] = nextScore
                    dpText[i + span] = dpText[i] + top.word
                }
            }
        }
        val finalStr = dpText[n] ?: return null
        return if (finalStr.isNotEmpty()) finalStr to dpScore[n] else null
    }

    private fun composeT9Sentence(digits: String): Pair<String, Double>? {
        val n = minOf(digits.length, 16)
        val dpScore = DoubleArray(n + 1) { -1.0 }
        val dpText = arrayOfNulls<String>(n + 1)
        dpScore[0] = 0.0
        dpText[0] = ""

        for (i in 0 until n) {
            if (dpScore[i] < 0) continue
            val maxLen = minOf(8, n - i)
            for (len in maxLen downTo 1) {
                val sub = digits.substring(i, i + len)
                val top = dict.lookupT9(sub, 1).firstOrNull() ?: continue
                val nextScore = dpScore[i] + top.weight + len * 800.0
                if (nextScore > dpScore[i + len]) {
                    dpScore[i + len] = nextScore
                    dpText[i + len] = dpText[i] + top.word
                }
            }
        }
        val finalStr = dpText[n] ?: return null
        return if (finalStr.isNotEmpty()) finalStr to dpScore[n] else null
    }
}
