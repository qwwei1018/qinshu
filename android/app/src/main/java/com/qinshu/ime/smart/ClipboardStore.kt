package com.qinshu.ime.smart

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject

data class ClipItem(
    val text: String,
    val pinned: Boolean = false,
    val timestampMs: Long = System.currentTimeMillis(),
)

/**
 * 轻量本地剪贴板历史（最多保留 20 条，支持置顶与一键清空未置顶，纯本地持久化）。
 */
class ClipboardStore(context: Context) {
    private val sp = context.getSharedPreferences("qinshu_clip_history", Context.MODE_PRIVATE)
    private val items = ArrayList<ClipItem>(MAX_ITEMS)

    init {
        load()
    }

    fun getAll(): List<ClipItem> = items.sortedWith(
        compareByDescending<ClipItem> { it.pinned }.thenByDescending { it.timestampMs },
    )

    fun add(text: String) {
        val t = text.trim()
        if (t.isEmpty() || t.length > 2000) return
        val existingIdx = items.indexOfFirst { it.text == t }
        val wasPinned = if (existingIdx >= 0) items.removeAt(existingIdx).pinned else false
        items.add(0, ClipItem(t, pinned = wasPinned, timestampMs = System.currentTimeMillis()))
        trimToLimit()
        save()
    }

    fun togglePin(text: String) {
        val idx = items.indexOfFirst { it.text == text }
        if (idx >= 0) {
            val cur = items[idx]
            items[idx] = cur.copy(pinned = !cur.pinned)
            save()
        }
    }

    fun remove(text: String) {
        if (items.removeAll { it.text == text }) save()
    }

    fun clearUnpinned() {
        if (items.removeAll { !it.pinned }) save()
    }

    private fun trimToLimit() {
        while (items.size > MAX_ITEMS) {
            val idx = items.indexOfLast { !it.pinned }
            if (idx >= 0) items.removeAt(idx) else items.removeAt(items.lastIndex)
        }
    }

    private fun load() {
        items.clear()
        val raw = sp.getString("items_json", null) ?: return
        runCatching {
            val arr = JSONArray(raw)
            for (i in 0 until arr.length()) {
                val obj = arr.getJSONObject(i)
                items.add(
                    ClipItem(
                        text = obj.getString("t"),
                        pinned = obj.optBoolean("p", false),
                        timestampMs = obj.optLong("ts", 0L),
                    ),
                )
            }
        }
    }

    private fun save() {
        val arr = JSONArray()
        for (item in items) {
            arr.put(
                JSONObject().apply {
                    put("t", item.text)
                    put("p", item.pinned)
                    put("ts", item.timestampMs)
                },
            )
        }
        sp.edit().putString("items_json", arr.toString()).apply()
    }

    companion object {
        private const val MAX_ITEMS = 20
    }
}
