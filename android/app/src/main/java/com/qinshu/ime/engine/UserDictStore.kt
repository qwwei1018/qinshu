package com.qinshu.ime.engine

import android.content.ContentValues
import android.content.Context
import android.database.sqlite.SQLiteDatabase
import android.database.sqlite.SQLiteOpenHelper
import android.util.LruCache
import org.json.JSONObject
import java.text.SimpleDateFormat
import java.util.Calendar
import java.util.Date
import java.util.Locale

/**
 * 低内存用户动态词频与自定义短语存储（SQLite WAL + 有界 LRU 缓存）。
 * - 继承桌面版分档提权（首页首位 +600，首页非首位 +1500，第 2 页及以上 +2500，上限 15,000）
 * - 新增 7 天近期活跃加权，确保近期常用词稳居首屏
 * - 支持与桌面版 user_dict.json / phrases.txt 双向导入导出
 */
class UserDictStore(context: Context) : SQLiteOpenHelper(context, DB_NAME, null, DB_VERSION) {

    data class FreqItem(
        val word: String,
        val bonus: Int,
        val lastUsedMs: Long,
        val pinned: Boolean,
    )

    // 有界 LRU 缓存（最多缓存 512 个高频 code 桶，约 < 300 KB RAM），支持 onTrimMemory 一键清空
    private val cache = LruCache<String, List<FreqItem>>(512)
    private val phraseMap = LinkedHashMap<String, MutableList<String>>()
    private val zhWeekdays = arrayOf("星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六")

    init {
        setWriteAheadLoggingEnabled(true)
        loadPhrasesFromDb()
    }

    override fun onCreate(db: SQLiteDatabase) {
        db.execSQL(
            """
            CREATE TABLE IF NOT EXISTS user_freq (
                source TEXT NOT NULL,
                code TEXT NOT NULL,
                word TEXT NOT NULL,
                bonus INTEGER NOT NULL DEFAULT 0,
                last_used_ms INTEGER NOT NULL DEFAULT 0,
                pinned INTEGER NOT NULL DEFAULT 0,
                PRIMARY KEY (source, code, word)
            )
            """.trimIndent(),
        )
        db.execSQL("CREATE INDEX IF NOT EXISTS idx_user_freq_lookup ON user_freq(source, code)")
        db.execSQL(
            """
            CREATE TABLE IF NOT EXISTS custom_phrases (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                abbr TEXT NOT NULL,
                content TEXT NOT NULL
            )
            """.trimIndent(),
        )
    }

    override fun onUpgrade(db: SQLiteDatabase, oldVersion: Int, newVersion: Int) = Unit

    fun trimCache() {
        cache.evictAll()
    }

    fun getUserFreqs(source: String, code: String): List<FreqItem> {
        val ck = code.lowercase()
        val cacheKey = "$source:$ck"
        cache.get(cacheKey)?.let { return it }

        val list = ArrayList<FreqItem>()
        readableDatabase.rawQuery(
            "SELECT word, bonus, last_used_ms, pinned FROM user_freq WHERE source=? AND code=? ORDER BY pinned DESC, bonus DESC LIMIT 20",
            arrayOf(source, ck),
        ).use { c ->
            while (c.moveToNext()) {
                list.add(
                    FreqItem(
                        word = c.getString(0),
                        bonus = c.getInt(1),
                        lastUsedMs = c.getLong(2),
                        pinned = c.getInt(3) != 0,
                    ),
                )
            }
        }
        cache.put(cacheKey, list)
        return list
    }

    /** 计算带近期活跃加成的动态词频总分 */
    fun effectiveBonus(item: FreqItem, nowMs: Long = System.currentTimeMillis()): Double {
        var score = item.bonus.toDouble()
        if (item.pinned) score += 25000.0
        val ageDays = ((nowMs - item.lastUsedMs).coerceAtLeast(0L)) / 86_400_000.0
        if (ageDays <= 7.0) {
            // 7 天内刚用过的常用词额外加权（最高 +2400），快速稳固首屏排序
            score += (1.0 - ageDays / 7.0) * 2400.0
        }
        return score
    }

    fun boostSelection(source: String, code: String, word: String, absIndex: Int, pageSize: Int = 5) {
        val ck = code.lowercase().trim()
        if (ck.isEmpty() || word.isEmpty()) return
        val delta = when {
            absIndex >= pageSize -> 2500
            absIndex > 0 -> 1500
            else -> 600
        }
        upsertBonus(source, ck, word, delta)
    }

    /** 生词自动学习：把未入库的英文手打词按前 2-4 字母前缀与全词索引写入本地动态词频库 */
    fun learnEnglishWord(word: String) {
        val clean = word.trim().lowercase()
        if (clean.length !in 2..32 || !clean.all { it in 'a'..'z' }) return
        val maxPrefix = minOf(4, clean.length)
        for (len in 2..maxPrefix) {
            upsertBonus("english", clean.substring(0, len), clean, 700)
        }
        if (clean.length > 4) {
            upsertBonus("english", clean, clean, 900)
        }
    }

    fun pinWord(source: String, code: String, word: String, pinned: Boolean) {
        val ck = code.lowercase().trim()
        val db = writableDatabase
        val cv = ContentValues().apply {
            put("source", source)
            put("code", ck)
            put("word", word)
            put("bonus", if (pinned) 8000 else 0)
            put("last_used_ms", System.currentTimeMillis())
            put("pinned", if (pinned) 1 else 0)
        }
        db.insertWithOnConflict("user_freq", null, cv, SQLiteDatabase.CONFLICT_REPLACE)
        cache.remove("$source:$ck")
    }

    fun deleteUserWord(source: String, code: String, word: String) {
        val ck = code.lowercase().trim()
        writableDatabase.delete("user_freq", "source=? AND code=? AND word=?", arrayOf(source, ck, word))
        cache.remove("$source:$ck")
    }

    private fun upsertBonus(source: String, ck: String, word: String, delta: Int) {
        val now = System.currentTimeMillis()
        val db = writableDatabase
        var curBonus = 0
        var curPinned = 0
        db.rawQuery(
            "SELECT bonus, pinned FROM user_freq WHERE source=? AND code=? AND word=?",
            arrayOf(source, ck, word),
        ).use { c ->
            if (c.moveToFirst()) {
                curBonus = c.getInt(0)
                curPinned = c.getInt(1)
            }
        }
        val next = (curBonus + delta).coerceAtMost(MAX_USER_BONUS)
        val cv = ContentValues().apply {
            put("source", source)
            put("code", ck)
            put("word", word)
            put("bonus", next)
            put("last_used_ms", now)
            put("pinned", curPinned)
        }
        db.insertWithOnConflict("user_freq", null, cv, SQLiteDatabase.CONFLICT_REPLACE)
        cache.remove("$source:$ck")
    }

    // =========================================================================
    // 自定义短语与桌面版 user_dict.json / phrases.txt 互通
    // =========================================================================

    fun queryPhrases(code: String): List<Candidate> {
        val list = phraseMap[code.lowercase()] ?: return emptyList()
        return list.mapIndexed { idx, tpl ->
            Candidate(
                text = expandPhraseVars(tpl),
                code = code.lowercase(),
                source = "phrase",
                score = 38000.0 - idx * 20,
                hint = "短语",
            )
        }
    }

    fun importDesktopUserDictJson(jsonText: String): Int {
        val root = JSONObject(jsonText)
        val now = System.currentTimeMillis()
        var imported = 0
        val db = writableDatabase
        db.beginTransaction()
        try {
            for (source in arrayOf("pinyin", "english", "phrase", "command")) {
                val sec = root.optJSONObject(source) ?: continue
                val codes = sec.keys()
                while (codes.hasNext()) {
                    val code = codes.next().lowercase()
                    val wordsObj = sec.optJSONObject(code) ?: continue
                    val words = wordsObj.keys()
                    while (words.hasNext()) {
                        val word = words.next()
                        val bonus = wordsObj.optInt(word, 600).coerceIn(0, MAX_USER_BONUS)
                        val cv = ContentValues().apply {
                            put("source", source)
                            put("code", code)
                            put("word", word)
                            put("bonus", bonus)
                            put("last_used_ms", now)
                            put("pinned", 0)
                        }
                        db.insertWithOnConflict("user_freq", null, cv, SQLiteDatabase.CONFLICT_REPLACE)
                        imported++
                    }
                }
            }
            db.setTransactionSuccessful()
        } finally {
            db.endTransaction()
        }
        cache.evictAll()
        return imported
    }

    fun exportDesktopUserDictJson(): String {
        val root = JSONObject()
        val sections = mapOf(
            "pinyin" to JSONObject(),
            "english" to JSONObject(),
            "phrase" to JSONObject(),
            "command" to JSONObject(),
        )
        readableDatabase.rawQuery("SELECT source, code, word, bonus FROM user_freq", null).use { c ->
            while (c.moveToNext()) {
                val src = c.getString(0)
                val sec = sections[src] ?: continue
                val code = c.getString(1)
                val bucket = sec.optJSONObject(code) ?: JSONObject().also { sec.put(code, it) }
                bucket.put(c.getString(2), c.getInt(3))
            }
        }
        sections.forEach { (k, v) -> root.put(k, v) }
        return root.toString(2)
    }

    fun importPhrasesTxt(text: String): Int {
        val db = writableDatabase
        var count = 0
        db.beginTransaction()
        try {
            db.delete("custom_phrases", null, null)
            for (rawLine in text.lineSequence()) {
                val line = rawLine.removePrefix("\uFEFF").trim()
                if (line.isEmpty() || line.startsWith("#")) continue
                val eq = line.indexOf('=')
                val tab = line.indexOf('\t')
                val sep = if (eq > 0) eq else tab
                if (sep <= 0) continue
                val abbr = line.substring(0, sep).trim().lowercase()
                val content = line.substring(sep + 1).trim()
                if (abbr.isNotEmpty() && content.isNotEmpty()) {
                    val cv = ContentValues().apply {
                        put("abbr", abbr)
                        put("content", content)
                    }
                    db.insert("custom_phrases", null, cv)
                    count++
                }
            }
            db.setTransactionSuccessful()
        } finally {
            db.endTransaction()
        }
        loadPhrasesFromDb()
        return count
    }

    private fun loadPhrasesFromDb() {
        phraseMap.clear()
        readableDatabase.rawQuery("SELECT abbr, content FROM custom_phrases ORDER BY id ASC", null).use { c ->
            while (c.moveToNext()) {
                phraseMap.getOrPut(c.getString(0)) { ArrayList() }.add(c.getString(1))
            }
        }
        if (phraseMap.isEmpty()) {
            phraseMap["qs"] = mutableListOf("Qin书输入法", "QinShu IME")
            phraseMap["ok"] = mutableListOf("好的，没问题！", "收到，马上处理。")
            phraseMap["dt"] = mutableListOf("\$year-\$month-\$day \$hour:\$minute:\$second")
        }
    }

    private fun expandPhraseVars(tpl: String): String {
        if (!tpl.contains('$')) return tpl
        val now = Date()
        val cal = Calendar.getInstance()
        return tpl
            .replace("\$year", SimpleDateFormat("yyyy", Locale.US).format(now))
            .replace("\$month", SimpleDateFormat("MM", Locale.US).format(now))
            .replace("\$day", SimpleDateFormat("dd", Locale.US).format(now))
            .replace("\$hour", SimpleDateFormat("HH", Locale.US).format(now))
            .replace("\$minute", SimpleDateFormat("mm", Locale.US).format(now))
            .replace("\$second", SimpleDateFormat("ss", Locale.US).format(now))
            .replace("\$weekday", zhWeekdays[cal.get(Calendar.DAY_OF_WEEK) - 1])
    }

    companion object {
        private const val DB_NAME = "qinshu_user.db"
        private const val DB_VERSION = 1
        private const val MAX_USER_BONUS = 15000
    }
}
