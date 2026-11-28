package com.qinshu.ime.config

import android.content.Context
import android.content.SharedPreferences
import com.qinshu.ime.engine.FuzzyPinyin
import com.qinshu.ime.engine.LayoutMode

enum class OneHandedMode { OFF, LEFT, RIGHT }

enum class QinTheme(
    val id: String,
    val displayName: String,
    val keyboardBg: Int,
    val keyBg: Int,
    val keyFnBg: Int,
    val keyAccentBg: Int,
    val keyText: Int,
    val keySubText: Int,
    val candidateBg: Int,
    val capsuleBg: Int,
    val capsuleText: Int,
    val divider: Int,
) {
    CLASSIC_LIGHT(
        id = "classic_light",
        displayName = "简约白（默认）",
        keyboardBg = 0xFFECEFF4.toInt(),
        keyBg = 0xFFFFFFFF.toInt(),
        keyFnBg = 0xFFDCE3EC.toInt(),
        keyAccentBg = 0xFF2563EB.toInt(),
        keyText = 0xFF0F172A.toInt(),
        keySubText = 0xFF64748B.toInt(),
        candidateBg = 0xFFF8FAFC.toInt(),
        capsuleBg = 0xFFDBEAFE.toInt(),
        capsuleText = 0xFF1E3A8A.toInt(),
        divider = 0xFFCBD5E1.toInt(),
    ),
    WANNENG_BLUE(
        id = "wanneng_blue",
        displayName = "万能蓝",
        keyboardBg = 0xFFE0EDFF.toInt(),
        keyBg = 0xFFFFFFFF.toInt(),
        keyFnBg = 0xFFBFDBFE.toInt(),
        keyAccentBg = 0xFF1D4ED8.toInt(),
        keyText = 0xFF0F172A.toInt(),
        keySubText = 0xFF1D4ED8.toInt(),
        candidateBg = 0xFFF0F7FF.toInt(),
        capsuleBg = 0xFF1D4ED8.toInt(),
        capsuleText = 0xFFFFFFFF.toInt(),
        divider = 0xFF93C5FD.toInt(),
    ),
    DARK_SLATE(
        id = "dark_slate",
        displayName = "深邃暗",
        keyboardBg = 0xFF0F172A.toInt(),
        keyBg = 0xFF1E293B.toInt(),
        keyFnBg = 0xFF334155.toInt(),
        keyAccentBg = 0xFF2563EB.toInt(),
        keyText = 0xFFF1F5F9.toInt(),
        keySubText = 0xFF94A3B8.toInt(),
        candidateBg = 0xFF1E293B.toInt(),
        capsuleBg = 0xFF1D4ED8.toInt(),
        capsuleText = 0xFFFFFFFF.toInt(),
        divider = 0xFF334155.toInt(),
    ),
}

class PrefsManager(context: Context) {
    private val sp: SharedPreferences =
        context.getSharedPreferences("qinshu_ime_prefs", Context.MODE_PRIVATE)

    var layoutMode: LayoutMode
        get() = if (sp.getString("layout_mode", "QWERTY") == "T9") LayoutMode.T9 else LayoutMode.QWERTY
        set(value) = sp.edit().putString("layout_mode", value.name).apply()

    var themeId: String
        get() = sp.getString("theme_id", QinTheme.CLASSIC_LIGHT.id) ?: QinTheme.CLASSIC_LIGHT.id
        set(value) = sp.edit().putString("theme_id", value).apply()

    var fuzzyEnabled: Boolean
        get() = sp.getBoolean("fuzzy_enabled", true)
        set(value) = sp.edit().putBoolean("fuzzy_enabled", value).apply()

    var emojiSuggestionEnabled: Boolean
        get() = sp.getBoolean("emoji_suggest", true)
        set(value) = sp.edit().putBoolean("emoji_suggest", value).apply()

    var clipboardSmartEnabled: Boolean
        get() = sp.getBoolean("clipboard_smart", true)
        set(value) = sp.edit().putBoolean("clipboard_smart", value).apply()

    var hapticEnabled: Boolean
        get() = sp.getBoolean("haptic_enabled", true)
        set(value) = sp.edit().putBoolean("haptic_enabled", value).apply()

    var keyPreviewEnabled: Boolean
        get() = sp.getBoolean("key_preview", true)
        set(value) = sp.edit().putBoolean("key_preview", value).apply()

    /** 键盘高度缩放比例：85 (紧凑) / 100 (标准) / 115 (大键) */
    var keyboardHeightPercent: Int
        get() = sp.getInt("kb_height_pct", 100).coerceIn(80, 125)
        set(value) = sp.edit().putInt("kb_height_pct", value.coerceIn(80, 125)).apply()

    var oneHandedMode: OneHandedMode
        get() = runCatching {
            OneHandedMode.valueOf(sp.getString("one_handed", "OFF") ?: "OFF")
        }.getOrDefault(OneHandedMode.OFF)
        set(value) = sp.edit().putString("one_handed", value.name).apply()

    // WebDAV 双端词频同步配置（仅用户主动填写时才发起局域网/NAS 请求）
    var webdavUrl: String
        get() = sp.getString("webdav_url", "") ?: ""
        set(value) = sp.edit().putString("webdav_url", value.trim()).apply()

    var webdavUser: String
        get() = sp.getString("webdav_user", "") ?: ""
        set(value) = sp.edit().putString("webdav_user", value.trim()).apply()

    var webdavPass: String
        get() = sp.getString("webdav_pass", "") ?: ""
        set(value) = sp.edit().putString("webdav_pass", value).apply()

    var lastSyncTimeMs: Long
        get() = sp.getLong("webdav_last_sync", 0L)
        set(value) = sp.edit().putLong("webdav_last_sync", value).apply()

    fun recordEmojiUse(emoji: String) {
        val key = "em_cnt_$emoji"
        val next = sp.getInt(key, 0) + 1
        val all = sp.getStringSet("em_used_set", emptySet())?.toMutableSet() ?: HashSet()
        all.add(emoji)
        sp.edit().putInt(key, next).putStringSet("em_used_set", all).apply()
    }

    fun getTopUsedEmojis(limit: Int = 24): List<String> {
        val all = sp.getStringSet("em_used_set", emptySet()) ?: return emptyList()
        return all.sortedByDescending { sp.getInt("em_cnt_$it", 0) }.take(limit)
    }

    fun currentTheme(isSystemDark: Boolean): QinTheme {
        if (themeId == "auto") {
            return if (isSystemDark) QinTheme.DARK_SLATE else QinTheme.CLASSIC_LIGHT
        }
        return QinTheme.entries.firstOrNull { it.id == themeId } ?: QinTheme.CLASSIC_LIGHT
    }

    fun getFuzzyRules(): Map<String, Boolean> {
        val defaults = FuzzyPinyin.defaultFuzzyRules()
        return defaults.mapValues { (k, def) -> sp.getBoolean("fuzzy_$k", def) }
    }

    fun setFuzzyRule(rule: String, enabled: Boolean) {
        sp.edit().putBoolean("fuzzy_$rule", enabled).apply()
    }

    /** 注册/注销设置项变更监听（输入法服务据此实现模糊音等开关的热更新）。 */
    fun registerOnChangeListener(listener: SharedPreferences.OnSharedPreferenceChangeListener) {
        sp.registerOnSharedPreferenceChangeListener(listener)
    }

    fun unregisterOnChangeListener(listener: SharedPreferences.OnSharedPreferenceChangeListener) {
        sp.unregisterOnSharedPreferenceChangeListener(listener)
    }
}
