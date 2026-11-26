package com.qinshu.ime.engine

import android.content.Context
import java.io.Closeable
import java.io.FileInputStream
import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.nio.MappedByteBuffer
import java.nio.channels.FileChannel
import java.nio.charset.StandardCharsets

data class DictEntry(
    val word: String,
    val weight: Int,
)

/**
 * 只读 mmap 二进制词库加载器（qinshu.qdict v1）。
 * 零启动反序列化，O(log N) 二分检索，由 Android OS 按 4KB 页按需映射到内存，脏内存为 0。
 */
class QDictReader private constructor(
    private val buf: MappedByteBuffer,
    private val channel: FileChannel,
    private val fis: FileInputStream,
) : Closeable {

    private val pyCount: Int
    private val pyKeyOff: Int
    private val pyEntryOff: Int

    private val initCount: Int
    private val initKeyOff: Int
    private val initEntryOff: Int

    private val t9Count: Int
    private val t9KeyOff: Int
    private val t9EntryOff: Int

    private val enCount: Int
    private val enOff: Int

    private val emojiCount: Int
    private val emojiKeyOff: Int
    private val emojiEntryOff: Int

    private val strPoolOff: Int

    init {
        buf.order(ByteOrder.LITTLE_ENDIAN)
        val m0 = buf.get(0).toInt().toChar()
        val m1 = buf.get(1).toInt().toChar()
        val m2 = buf.get(2).toInt().toChar()
        val m3 = buf.get(3).toInt().toChar()
        require("$m0$m1$m2$m3" == "QDCT") { "Invalid qinshu.qdict magic" }
        val version = buf.getInt(4)
        require(version == 1) { "Unsupported qinshu.qdict version: $version" }

        pyCount = buf.getInt(12)
        pyKeyOff = buf.getInt(16)
        pyEntryOff = buf.getInt(20)

        initCount = buf.getInt(24)
        initKeyOff = buf.getInt(28)
        initEntryOff = buf.getInt(32)

        t9Count = buf.getInt(36)
        t9KeyOff = buf.getInt(40)
        t9EntryOff = buf.getInt(44)

        enCount = buf.getInt(48)
        enOff = buf.getInt(52)

        emojiCount = buf.getInt(56)
        emojiKeyOff = buf.getInt(60)
        emojiEntryOff = buf.getInt(64)

        strPoolOff = buf.getInt(68)
    }

    fun lookupPinyin(key: String, limit: Int = 30): List<DictEntry> =
        lookupKeyTable(pyCount, pyKeyOff, pyEntryOff, key, limit)

    fun lookupPinyinPrefix(prefix: String, maxKeys: Int = 6, maxPerKey: Int = 3): List<Pair<String, DictEntry>> =
        lookupKeyPrefix(pyCount, pyKeyOff, pyEntryOff, prefix, maxKeys, maxPerKey)

    fun lookupInitials(key: String, limit: Int = 12): List<DictEntry> =
        lookupKeyTable(initCount, initKeyOff, initEntryOff, key, limit)

    fun lookupT9(digits: String, limit: Int = 15): List<DictEntry> =
        lookupKeyTable(t9Count, t9KeyOff, t9EntryOff, digits, limit)

    fun lookupT9Prefix(digits: String, maxKeys: Int = 6, maxPerKey: Int = 3): List<Pair<String, DictEntry>> =
        lookupKeyPrefix(t9Count, t9KeyOff, t9EntryOff, digits, maxKeys, maxPerKey)

    fun lookupEmoji(pinyinKey: String, limit: Int = 4): List<DictEntry> =
        lookupKeyTable(emojiCount, emojiKeyOff, emojiEntryOff, pinyinKey, limit)

    fun lookupEnglishPrefix(prefix: String, limit: Int = 12): List<DictEntry> {
        if (prefix.isEmpty() || enCount == 0) return emptyList()
        val target = prefix.lowercase().toByteArray(StandardCharsets.UTF_8)
        var lo = 0
        var hi = enCount
        while (lo < hi) {
            val mid = (lo + hi) ushr 1
            val recOff = enOff + mid * 8
            val sOff = buf.getInt(recOff)
            val sLen = buf.getShort(recOff + 4).toInt() and 0xFFFF
            if (comparePoolBytes(sOff, sLen, target) < 0) {
                lo = mid + 1
            } else {
                hi = mid
            }
        }
        val out = ArrayList<DictEntry>(limit)
        var idx = lo
        val maxScan = 160
        var scanned = 0
        while (idx < enCount && scanned < maxScan) {
            val recOff = enOff + idx * 8
            val sOff = buf.getInt(recOff)
            val sLen = buf.getShort(recOff + 4).toInt() and 0xFFFF
            if (!poolStartsWith(sOff, sLen, target)) break
            val weight = buf.getShort(recOff + 6).toInt() and 0xFFFF
            out.add(DictEntry(readPoolString(sOff, sLen), weight))
            idx++
            scanned++
        }
        out.sortWith(compareByDescending<DictEntry> { it.weight }.thenBy { it.word.length })
        return if (out.size > limit) out.subList(0, limit) else out
    }

    private fun lookupKeyTable(
        count: Int,
        keyTableOff: Int,
        entryTableOff: Int,
        key: String,
        limit: Int,
    ): List<DictEntry> {
        if (count == 0 || key.isEmpty()) return emptyList()
        val target = key.toByteArray(StandardCharsets.UTF_8)
        var lo = 0
        var hi = count - 1
        while (lo <= hi) {
            val mid = (lo + hi) ushr 1
            val recOff = keyTableOff + mid * 12
            val sOff = buf.getInt(recOff)
            val sLen = buf.getShort(recOff + 4).toInt() and 0xFFFF
            val cmp = comparePoolBytes(sOff, sLen, target)
            when {
                cmp == 0 -> {
                    val entryCount = buf.getShort(recOff + 6).toInt() and 0xFFFF
                    val firstIdx = buf.getInt(recOff + 8)
                    val take = minOf(entryCount, limit)
                    val list = ArrayList<DictEntry>(take)
                    for (i in 0 until take) {
                        val eOff = entryTableOff + (firstIdx + i) * 8
                        val wOff = buf.getInt(eOff)
                        val wLen = buf.getShort(eOff + 4).toInt() and 0xFFFF
                        val weight = buf.getShort(eOff + 6).toInt() and 0xFFFF
                        list.add(DictEntry(readPoolString(wOff, wLen), weight))
                    }
                    return list
                }
                cmp < 0 -> lo = mid + 1
                else -> hi = mid - 1
            }
        }
        return emptyList()
    }

    private fun lookupKeyPrefix(
        count: Int,
        keyTableOff: Int,
        entryTableOff: Int,
        prefix: String,
        maxKeys: Int,
        maxPerKey: Int,
    ): List<Pair<String, DictEntry>> {
        if (count == 0 || prefix.isEmpty()) return emptyList()
        val target = prefix.toByteArray(StandardCharsets.UTF_8)
        var lo = 0
        var hi = count
        while (lo < hi) {
            val mid = (lo + hi) ushr 1
            val recOff = keyTableOff + mid * 12
            val sOff = buf.getInt(recOff)
            val sLen = buf.getShort(recOff + 4).toInt() and 0xFFFF
            if (comparePoolBytes(sOff, sLen, target) < 0) lo = mid + 1 else hi = mid
        }
        val out = ArrayList<Pair<String, DictEntry>>(maxKeys * maxPerKey)
        var idx = lo
        var keysAdded = 0
        while (idx < count && keysAdded < maxKeys) {
            val recOff = keyTableOff + idx * 12
            val sOff = buf.getInt(recOff)
            val sLen = buf.getShort(recOff + 4).toInt() and 0xFFFF
            if (!poolStartsWith(sOff, sLen, target)) break
            if (sLen > target.size) {
                val fullKey = readPoolString(sOff, sLen)
                val entryCount = buf.getShort(recOff + 6).toInt() and 0xFFFF
                val firstIdx = buf.getInt(recOff + 8)
                val take = minOf(entryCount, maxPerKey)
                for (i in 0 until take) {
                    val eOff = entryTableOff + (firstIdx + i) * 8
                    val wOff = buf.getInt(eOff)
                    val wLen = buf.getShort(eOff + 4).toInt() and 0xFFFF
                    val weight = buf.getShort(eOff + 6).toInt() and 0xFFFF
                    out.add(fullKey to DictEntry(readPoolString(wOff, wLen), weight))
                }
                keysAdded++
            }
            idx++
        }
        return out
    }

    private fun comparePoolBytes(sOff: Int, sLen: Int, target: ByteArray): Int {
        val base = strPoolOff + sOff
        val minLen = minOf(sLen, target.size)
        for (i in 0 until minLen) {
            val a = buf.get(base + i).toInt() and 0xFF
            val b = target[i].toInt() and 0xFF
            if (a != b) return a - b
        }
        return sLen - target.size
    }

    private fun poolStartsWith(sOff: Int, sLen: Int, prefix: ByteArray): Boolean {
        if (sLen < prefix.size) return false
        val base = strPoolOff + sOff
        for (i in prefix.indices) {
            if (buf.get(base + i) != prefix[i]) return false
        }
        return true
    }

    private fun readPoolString(sOff: Int, sLen: Int): String {
        val bytes = ByteArray(sLen)
        val dup: ByteBuffer = buf.duplicate()
        dup.position(strPoolOff + sOff)
        dup.get(bytes)
        return String(bytes, StandardCharsets.UTF_8)
    }

    override fun close() {
        runCatching { channel.close() }
        runCatching { fis.close() }
    }

    companion object {
        private const val ASSET_NAME = "qinshu.qdict"

        fun openFromAssets(context: Context): QDictReader {
            val afd = context.assets.openFd(ASSET_NAME)
            val fis = FileInputStream(afd.fileDescriptor)
            val channel = fis.channel
            val mapped = channel.map(FileChannel.MapMode.READ_ONLY, afd.startOffset, afd.length)
            afd.close()
            return QDictReader(mapped, channel, fis)
        }
    }
}
