package com.qinshu.ime.smart

import android.util.Base64
import com.qinshu.ime.config.PrefsManager
import com.qinshu.ime.engine.UserDictStore
import org.json.JSONObject
import java.io.OutputStreamWriter
import java.net.HttpURLConnection
import java.net.URL
import java.nio.charset.StandardCharsets

/**
 * 零第三方依赖的 NAS / WebDAV 双端词频双向增量同步器（纯 HttpURLConnection）。
 * 支持飞牛 FnOS / 群晖 Synology / Nextcloud / 坚果云等标准 WebDAV 服务，
 * 自动增量合并桌面版与安卓端的 user_dict.json（按每词取 max 权重，不互相覆盖丢失）。
 */
object WebDavSyncManager {

    data class SyncResult(val ok: Boolean, val message: String)

    fun syncUserDict(prefs: PrefsManager, userStore: UserDictStore): SyncResult {
        val baseUrl = prefs.webdavUrl.trimEnd('/')
        if (baseUrl.isEmpty()) {
            return SyncResult(false, "请先在设置中填写 WebDAV 目录地址")
        }
        val authHeader = buildBasicAuth(prefs.webdavUser, prefs.webdavPass)
        val remoteUrl = "$baseUrl/user_dict.json"

        return try {
            // 1. 拉取远端 user_dict.json（404 视为首次上传）
            val remoteJson = httpGet(remoteUrl, authHeader)
            val localJson = userStore.exportDesktopUserDictJson()

            // 2. 双向增量合并（对每个 source/code/word 取较大权重，保留双端所有高频词）
            val merged = if (remoteJson != null) {
                mergeUserDictJson(localJson, remoteJson)
            } else {
                localJson
            }

            // 3. 写回本地 SQLite 并上传合并后的最新文件到 NAS
            val count = userStore.importDesktopUserDictJson(merged)
            val putCode = httpPut(remoteUrl, authHeader, merged)
            if (putCode in 200..299) {
                prefs.lastSyncTimeMs = System.currentTimeMillis()
                SyncResult(true, "双端词频同步成功（已合并 $count 条高频词）")
            } else {
                SyncResult(false, "上传 WebDAV 失败（HTTP $putCode），请检查账号权限或目录是否存在")
            }
        } catch (e: Exception) {
            SyncResult(false, "同步异常：${e.message}")
        }
    }

    private fun mergeUserDictJson(localStr: String, remoteStr: String): String {
        val local = JSONObject(localStr)
        val remote = runCatching { JSONObject(remoteStr) }.getOrElse { JSONObject() }
        val merged = JSONObject()
        for (source in arrayOf("wubi", "pinyin", "english", "phrase", "command")) {
            val outSec = JSONObject()
            val lSec = local.optJSONObject(source)
            val rSec = remote.optJSONObject(source)

            fun mergeSec(sec: JSONObject?) {
                if (sec == null) return
                val codes = sec.keys()
                while (codes.hasNext()) {
                    val code = codes.next()
                    val words = sec.optJSONObject(code) ?: continue
                    val outBucket = outSec.optJSONObject(code) ?: JSONObject().also { outSec.put(code, it) }
                    val wk = words.keys()
                    while (wk.hasNext()) {
                        val w = wk.next()
                        val bonus = words.optInt(w, 0)
                        val prev = outBucket.optInt(w, 0)
                        if (bonus > prev) outBucket.put(w, bonus)
                    }
                }
            }
            mergeSec(rSec)
            mergeSec(lSec)
            merged.put(source, outSec)
        }
        return merged.toString(2)
    }

    private fun buildBasicAuth(user: String, pass: String): String? {
        if (user.isEmpty() && pass.isEmpty()) return null
        val token = Base64.encodeToString("$user:$pass".toByteArray(StandardCharsets.UTF_8), Base64.NO_WRAP)
        return "Basic $token"
    }

    private fun httpGet(urlStr: String, auth: String?): String? {
        val conn = (URL(urlStr).openConnection() as HttpURLConnection).apply {
            requestMethod = "GET"
            connectTimeout = 8000
            readTimeout = 10000
            if (auth != null) setRequestProperty("Authorization", auth)
        }
        return try {
            when (conn.responseCode) {
                200 -> conn.inputStream.bufferedReader(StandardCharsets.UTF_8).use { it.readText() }
                404 -> null
                else -> throw IllegalStateException("GET HTTP ${conn.responseCode}")
            }
        } finally {
            conn.disconnect()
        }
    }

    private fun httpPut(urlStr: String, auth: String?, body: String): Int {
        val conn = (URL(urlStr).openConnection() as HttpURLConnection).apply {
            requestMethod = "PUT"
            doOutput = true
            connectTimeout = 8000
            readTimeout = 10000
            setRequestProperty("Content-Type", "application/json; charset=utf-8")
            if (auth != null) setRequestProperty("Authorization", auth)
        }
        return try {
            OutputStreamWriter(conn.outputStream, StandardCharsets.UTF_8).use { it.write(body) }
            conn.responseCode
        } finally {
            conn.disconnect()
        }
    }
}
