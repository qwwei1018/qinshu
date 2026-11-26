package com.qinshu.ime.engine

import java.math.BigDecimal
import java.math.RoundingMode
import java.text.SimpleDateFormat
import java.util.Calendar
import java.util.Date
import java.util.Locale
import kotlin.math.abs

data class SmartCapsule(
    val type: CapsuleType,
    val label: String,
    val text: String,
    val createdAtMs: Long = System.currentTimeMillis(),
    val ttlMs: Long = 180_000L,
) {
    fun isExpired(nowMs: Long = System.currentTimeMillis()): Boolean =
        ttlMs > 0 && (nowMs - createdAtMs) > ttlMs
}

enum class CapsuleType { OTP, URL, EMAIL, PHONE, TRACKING }

/**
 * 纯本地、零重型模型智能识别引擎（v1.1 升级版）：
 * 1. 实时算式计算：11+12 -> [23, 11+12=23]，支持百分比 (200*15% -> 30) 与括号小数。
 * 2. 人民币中文大写金额：输入 =1280 或 =1280.5 -> [壹仟贰佰捌拾元伍角, 1280.5]。
 * 3. 短信/剪贴板智能提取：验证码（OTP）、网址（URL）、邮箱（Email）、手机号与快递单号。
 * 4. 智能邮箱（@）、智能网址（http:// / www.）与快捷指令（rq / sj / xq / fh）。
 */
object SmartRecognizer {

    private val OTP_KEYWORD_REGEX = Regex(
        """(?i)(验证码|校验码|动态码|确认码|安全码|登录码|取件码|口令|code|otp|passcode|verification)""",
    )
    private val OTP_CODE_NEAR_REGEX = Regex(
        """(?i)(?:验证码|校验码|动态码|确认码|安全码|登录码|code|otp|passcode)[\s:：为是()（）\[\]【】]*([0-9]{4,8})\b""",
    )
    private val OTP_CODE_BEFORE_REGEX = Regex(
        """\b([0-9]{4,8})[\s()（）\[\]【】]*(?:是您的|为您的|为)?(?:验证码|校验码|动态码|确认码|安全码|登录码)""",
    )
    private val URL_REGEX = Regex(
        """(?i)\b((?:https?://|www\.)[a-z0-9\-._~:/?#\[\]@!$&'()*+,;=%]+)""",
    )
    private val EMAIL_REGEX = Regex(
        """(?i)\b([a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,})\b""",
    )
    private val PHONE_REGEX = Regex(
        """(?<!\d)(1[3-9]\d{9})(?!\d)""",
    )
    private val TRACKING_PREFIX_REGEX = Regex(
        """(?i)\b((?:SF|YT|YD|ZT|JT|JD|EMS|STO|ZTO|YTO)[0-9A-Z]{8,16})\b""",
    )
    private val TRACKING_KEYWORD_REGEX = Regex(
        """(?:快递单号|运单号|单号|包裹号)[\s:：为是]*([0-9A-Za-z]{10,20})\b""",
    )

    private val commonEmailDomains = listOf(
        "qq.com", "163.com", "gmail.com", "126.com", "outlook.com",
        "foxmail.com", "icloud.com", "sina.com", "aliyun.com", "hotmail.com",
    )

    private val commonUrlTlds = listOf(
        ".com", ".cn", ".net", ".org", ".com.cn", ".io", ".ai", ".gov.cn", ".edu.cn",
    )

    private val zhWeekdays = arrayOf("日", "一", "二", "三", "四", "五", "六")
    private val quickSymbols = listOf("「」", "『』", "【】", "《》", "——", "……", "·", "℃", "￥", "→", "✓", "✗")

    // =========================================================================
    // 1. 实时算式计算 + 百分比 + 人民币大写（=1280 -> 壹仟贰佰捌拾元整）
    // =========================================================================

    fun evaluateExpressionCandidates(raw: String): List<Candidate> {
        val expr = raw.trim()
        if (expr.length < 2 || expr.length > 48) return emptyList()

        // 1a. 金融大写快捷触发：以 '=' 开头接纯数字金额（例如 "=1280.5"）
        if (expr.startsWith("=") && expr.length >= 2) {
            val numStr = expr.substring(1).trim()
            val bd = numStr.toBigDecimalOrNull()
            if (bd != null && bd >= BigDecimal.ZERO && bd < BigDecimal("1000000000000")) {
                val rmb = toRmbUppercase(bd)
                return listOf(
                    Candidate(rmb, expr, "calc", 96000.0, "大写金额"),
                    Candidate("¥$numStr", expr, "calc", 95500.0, "金额"),
                )
            }
        }

        if (expr.length < 3) return emptyList()
        val normalized = expr
            .replace('×', '*')
            .replace('÷', '/')
            .replace('（', '(')
            .replace('）', ')')
            .replace(" ", "")

        var hasOp = false
        for (i in normalized.indices) {
            val ch = normalized[i]
            when {
                ch in '0'..'9' || ch == '.' || ch == '(' || ch == ')' -> Unit
                ch == '+' || ch == '-' || ch == '*' || ch == '/' || ch == '%' -> {
                    if (i > 0) hasOp = true
                }
                else -> return emptyList()
            }
        }
        if (!hasOp) return emptyList()
        val last = normalized.last()
        if (last == '+' || last == '-' || last == '*' || last == '/' || last == '(' || last == '.') {
            return emptyList()
        }

        val value = ExprParser(normalized).parseOrNull() ?: return emptyList()
        if (value.isNaN() || value.isInfinite()) return emptyList()

        val formatted = formatCalcNumber(value)
        val out = mutableListOf(
            Candidate(formatted, expr, "calc", 95000.0, "结果"),
            Candidate("$expr=$formatted", expr, "calc", 94900.0, "算式"),
        )
        if (value > 0 && value < 1e12) {
            val rmb = toRmbUppercase(BigDecimal.valueOf(value))
            out.add(Candidate(rmb, expr, "calc", 94800.0, "大写"))
        }
        return out
    }

    fun toRmbUppercase(amount: BigDecimal): String {
        val digits = charArrayOf('零', '壹', '贰', '叁', '肆', '伍', '陆', '柒', '捌', '玖')
        val units = arrayOf("", "拾", "佰", "仟")
        val bigUnits = arrayOf("", "万", "亿")

        val scaled = amount.setScale(2, RoundingMode.HALF_UP)
        val totalCents = scaled.movePointRight(2).longValueExact()
        if (totalCents == 0L) return "零元整"

        val yuan = totalCents / 100
        val jiao = ((totalCents % 100) / 10).toInt()
        val fen = (totalCents % 10).toInt()

        val sb = StringBuilder()
        if (yuan > 0) {
            var remaining = yuan
            var groupIdx = 0
            var needZero = false
            val groups = ArrayList<String>()
            while (remaining > 0) {
                val part = (remaining % 10000).toInt()
                if (part == 0) {
                    needZero = true
                } else {
                    val g = StringBuilder()
                    var p = part
                    var u = 0
                    var zeroFlag = false
                    while (p > 0) {
                        val d = p % 10
                        if (d == 0) {
                            if (g.isNotEmpty() && !zeroFlag) {
                                g.insert(0, '零')
                                zeroFlag = true
                            }
                        } else {
                            g.insert(0, units[u])
                            g.insert(0, digits[d])
                            zeroFlag = false
                        }
                        p /= 10
                        u++
                    }
                    if (part < 1000 && remaining >= 10000) g.insert(0, '零')
                    g.append(bigUnits[groupIdx])
                    if (needZero && groups.isNotEmpty()) g.append('零')
                    groups.add(0, g.toString())
                    needZero = false
                }
                remaining /= 10000
                groupIdx++
            }
            sb.append(groups.joinToString("").trimStart('零')).append('元')
        }
        when {
            jiao == 0 && fen == 0 -> sb.append('整')
            jiao > 0 && fen == 0 -> sb.append(digits[jiao]).append("角整")
            jiao == 0 && fen > 0 -> {
                if (yuan > 0) sb.append('零')
                sb.append(digits[fen]).append('分')
            }
            else -> sb.append(digits[jiao]).append('角').append(digits[fen]).append('分')
        }
        return sb.toString()
    }

    private fun formatCalcNumber(v: Double): String {
        val rounded = kotlin.math.round(v)
        if (abs(v - rounded) < 1e-9 && abs(rounded) < 1e15) {
            return rounded.toLong().toString()
        }
        return BigDecimal.valueOf(v)
            .setScale(6, RoundingMode.HALF_UP)
            .stripTrailingZeros()
            .toPlainString()
    }

    private class ExprParser(private val s: String) {
        private var pos = 0

        fun parseOrNull(): Double? = try {
            val v = parseAddSub()
            if (pos != s.length) null else v
        } catch (_: Throwable) {
            null
        }

        private fun parseAddSub(): Double {
            var left = parseMulDiv()
            while (pos < s.length) {
                when (s[pos]) {
                    '+' -> { pos++; left += parseMulDiv() }
                    '-' -> { pos++; left -= parseMulDiv() }
                    else -> break
                }
            }
            return left
        }

        private fun parseMulDiv(): Double {
            var left = parseUnary()
            while (pos < s.length) {
                when (s[pos]) {
                    '*' -> { pos++; left *= parseUnary() }
                    '/' -> {
                        pos++
                        val right = parseUnary()
                        if (abs(right) < 1e-12) throw ArithmeticException("div0")
                        left /= right
                    }
                    else -> break
                }
            }
            return left
        }

        private fun parseUnary(): Double {
            if (pos < s.length && s[pos] == '+') { pos++; return parseUnary() }
            if (pos < s.length && s[pos] == '-') { pos++; return -parseUnary() }
            var v = parsePrimary()
            // 支持后缀百分号（例如 200*15% = 30，50% = 0.5）
            while (pos < s.length && s[pos] == '%') {
                pos++
                v /= 100.0
            }
            return v
        }

        private fun parsePrimary(): Double {
            if (pos < s.length && s[pos] == '(') {
                pos++
                val v = parseAddSub()
                if (pos >= s.length || s[pos] != ')') throw IllegalArgumentException("unclosed")
                pos++
                return v
            }
            val start = pos
            var dotSeen = false
            while (pos < s.length) {
                val c = s[pos]
                if (c in '0'..'9') {
                    pos++
                } else if (c == '.' && !dotSeen) {
                    dotSeen = true
                    pos++
                } else {
                    break
                }
            }
            if (pos == start) throw IllegalArgumentException("nan")
            return s.substring(start, pos).toDouble()
        }
    }

    // =========================================================================
    // 2. 短信 / 剪贴板智能提取（验证码、URL、Email、手机号、快递单号）
    // =========================================================================

    fun extractOtp(text: String): String? {
        if (text.length < 4 || text.length > 500) return null
        if (!OTP_KEYWORD_REGEX.containsMatchIn(text)) return null
        OTP_CODE_NEAR_REGEX.find(text)?.groupValues?.getOrNull(1)?.let { code ->
            if (!looksLikeYear(code)) return code
        }
        OTP_CODE_BEFORE_REGEX.find(text)?.groupValues?.getOrNull(1)?.let { code ->
            if (!looksLikeYear(code)) return code
        }
        Regex("""(?<!\d)([0-9]{4,6})(?!\d)""").findAll(text).forEach { m ->
            val cand = m.groupValues[1]
            if (!looksLikeYear(cand)) return cand
        }
        return null
    }

    private fun looksLikeYear(code: String): Boolean =
        code.length == 4 && (code.startsWith("19") || code.startsWith("20"))

    fun extractClipboardCapsules(clipText: String): List<SmartCapsule> {
        val text = clipText.trim()
        if (text.isEmpty() || text.length > 2000) return emptyList()
        val result = ArrayList<SmartCapsule>(4)
        extractOtp(text)?.let { otp ->
            result.add(SmartCapsule(CapsuleType.OTP, "验证码 $otp", otp, ttlMs = 180_000L))
        }
        EMAIL_REGEX.find(text)?.groupValues?.getOrNull(1)?.let { email ->
            result.add(SmartCapsule(CapsuleType.EMAIL, "邮箱 $email", email, ttlMs = 300_000L))
        }
        URL_REGEX.find(text)?.groupValues?.getOrNull(1)?.let { rawUrl ->
            val url = rawUrl.trimEnd('.', ',', ';', ')', ']', '，', '。')
            val shortLabel = if (url.length > 22) url.take(22) + "…" else url
            result.add(SmartCapsule(CapsuleType.URL, "网址 $shortLabel", url, ttlMs = 300_000L))
        }
        val tracking = TRACKING_PREFIX_REGEX.find(text)?.groupValues?.getOrNull(1)
            ?: TRACKING_KEYWORD_REGEX.find(text)?.groupValues?.getOrNull(1)
        if (tracking != null) {
            result.add(SmartCapsule(CapsuleType.TRACKING, "单号 $tracking", tracking, ttlMs = 300_000L))
        } else {
            PHONE_REGEX.find(text)?.groupValues?.getOrNull(1)?.let { phone ->
                result.add(SmartCapsule(CapsuleType.PHONE, "手机 $phone", phone, ttlMs = 300_000L))
            }
        }
        return result
    }

    // =========================================================================
    // 3. 智能邮箱、智能网址补全与桌面快捷指令
    // =========================================================================

    fun querySmartEmail(buf: String): List<Candidate> {
        val atIdx = buf.indexOf('@')
        if (atIdx <= 0) return emptyList()
        val local = buf.substring(0, atIdx)
        val typed = buf.substring(atIdx + 1).lowercase()
        val out = ArrayList<Candidate>()
        val seen = HashSet<String>()
        commonEmailDomains.forEachIndexed { idx, dom ->
            if (typed.isEmpty() || dom.startsWith(typed)) {
                val full = "$local@$dom"
                if (seen.add(full)) {
                    out.add(Candidate(full, buf, "command", 40000.0 - idx * 10, "邮箱"))
                }
            }
        }
        if (seen.add(buf)) out.add(Candidate(buf, buf, "command", 30000.0, "邮箱"))
        return out
    }

    fun querySmartUrl(buf: String): List<Candidate> {
        val out = ArrayList<Candidate>()
        val seen = HashSet<String>()
        fun add(text: String, score: Double) {
            if (text.isNotEmpty() && seen.add(text)) {
                out.add(Candidate(text, buf, "command", score, "网址"))
            }
        }
        val schemeIdx = buf.indexOf("://")
        val hostStart = if (schemeIdx >= 0) schemeIdx + 3 else 0
        val rest = buf.substring(hostStart)
        val hasPath = rest.contains('/') || rest.contains('?')
        if (!hasPath && rest.isNotEmpty() && rest != "www.") {
            val lastDot = rest.lastIndexOf('.')
            if (lastDot >= 0 && lastDot != 3) {
                val base = buf.substring(0, hostStart + lastDot)
                val typedTld = rest.substring(lastDot).lowercase()
                commonUrlTlds.forEachIndexed { i, tld ->
                    if (tld.startsWith(typedTld)) add(base + tld, 40000.0 - i * 10)
                }
            } else if (!buf.endsWith(".")) {
                commonUrlTlds.forEachIndexed { i, tld -> add(buf + tld, 40000.0 - i * 10) }
            }
        }
        add(buf, 32000.0)
        return out
    }

    fun queryQuickCommands(code: String): List<Candidate> {
        val low = code.lowercase()
        val now = Date()
        return when (low) {
            "rq", "date" -> listOf(
                Candidate(SimpleDateFormat("yyyy年MM月dd日", Locale.CHINA).format(now), low, "command", 42000.0, "日期"),
                Candidate(SimpleDateFormat("yyyy-MM-dd", Locale.US).format(now), low, "command", 41900.0, "日期"),
                Candidate(SimpleDateFormat("yyyy/MM/dd", Locale.US).format(now), low, "command", 41800.0, "日期"),
            )
            "sj", "time" -> listOf(
                Candidate(SimpleDateFormat("HH:mm:ss", Locale.US).format(now), low, "command", 42000.0, "时间"),
                Candidate(SimpleDateFormat("HH:mm", Locale.US).format(now), low, "command", 41900.0, "时间"),
                Candidate(SimpleDateFormat("yyyy-MM-dd HH:mm", Locale.US).format(now), low, "command", 41800.0, "时间"),
            )
            "xq", "week" -> {
                val wd = zhWeekdays[Calendar.getInstance().get(Calendar.DAY_OF_WEEK) - 1]
                listOf(
                    Candidate("星期$wd", low, "command", 42000.0, "星期"),
                    Candidate("周$wd", low, "command", 41900.0, "星期"),
                )
            }
            "fh" -> quickSymbols.mapIndexed { idx, sym ->
                Candidate(sym, low, "command", 42000.0 - idx * 10, "符号")
            }
            "http" -> listOf(
                Candidate("https://", low, "command", 44000.0, "网址"),
                Candidate("http://", low, "command", 43900.0, "网址"),
                Candidate("https://www.", low, "command", 43800.0, "网址"),
            )
            "www" -> listOf(
                Candidate("www.", low, "command", 44000.0, "网址"),
                Candidate("https://www.", low, "command", 43900.0, "网址"),
            )
            else -> emptyList()
        }
    }
}
