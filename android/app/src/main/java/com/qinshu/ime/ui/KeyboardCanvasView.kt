package com.qinshu.ime.ui

import android.content.Context
import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.RectF
import android.view.HapticFeedbackConstants
import android.view.MotionEvent
import android.view.View
import com.qinshu.ime.config.OneHandedMode
import com.qinshu.ime.config.QinTheme
import com.qinshu.ime.engine.LanguageMode
import com.qinshu.ime.engine.LayoutMode
import kotlin.math.abs

enum class KeyboardPage { QWERTY, T9, NUM_SYM }

enum class ShiftState { OFF, ONCE, LOCKED }

enum class KeyAction {
    CHAR, BACKSPACE, ENTER, SPACE, SHIFT,
    SWITCH_LANG, SWITCH_LAYOUT, SWITCH_NUM, SWITCH_EMOJI, SWITCH_SYMBOL,
    CLEAR_BUFFER, T9_SYLLABLE, EXIT_ONE_HANDED,
}

data class KeySpec(
    val label: String,
    val subLabel: String = "",
    val action: KeyAction,
    val charValue: Char = '\u0000',
    val isFn: Boolean = false,
    val isAccent: Boolean = false,
    val rect: RectF = RectF(),
)

/**
 * 超低内存单视图 Canvas 自绘键盘（v1.1 升级版）：
 * - 新增：键盘高度自定义（80%–125%）、左右单手模式缩进、零额外 View 的按键放大气泡预览
 * - 新增：数字/算式页增加 '=' 快捷键（支持 =1280 人民币大写）与「符号」面板入口
 * - 支持空格键左右滑动微调光标、退格键左滑快速清空缓冲
 */
class KeyboardCanvasView(context: Context) : View(context) {

    interface Listener {
        fun onChar(ch: Char)
        fun onBackspace()
        fun onClearBuffer()
        fun onEnter()
        fun onSpace()
        fun onMoveCursor(delta: Int)
        fun onToggleLanguage()
        fun onToggleLayout()
        fun onToggleEmojiPanel()
        fun onToggleSymbolPanel()
        fun onSelectT9Syllable(syl: String)
        fun onExitOneHanded()
        fun onNumPageChanged(isNumPage: Boolean) {}
    }

    var listener: Listener? = null
    var hapticEnabled: Boolean = true
    var keyPreviewEnabled: Boolean = true

    private var theme: QinTheme = QinTheme.CLASSIC_LIGHT
    private var page: KeyboardPage = KeyboardPage.QWERTY
    private var activeLayoutMode: LayoutMode = LayoutMode.QWERTY
    private var lang: LanguageMode = LanguageMode.ZH
    private var shiftState: ShiftState = ShiftState.OFF
    private var lastShiftTapMs: Long = 0L
    private var shiftLongPressed: Boolean = false
    private var t9Syllables: List<String> = emptyList()
    private var heightPercent: Int = 100
    private var oneHandedMode: OneHandedMode = OneHandedMode.OFF

    private val shiftLongPressRunnable = Runnable {
        if (pressedKey?.action == KeyAction.SHIFT && lang == LanguageMode.EN) {
            shiftLongPressed = true
            shiftState = ShiftState.LOCKED
            if (hapticEnabled) {
                performHapticFeedback(HapticFeedbackConstants.LONG_PRESS)
            }
            rebuildKeys()
            invalidate()
        }
    }

    val isNumPage: Boolean
        get() = page == KeyboardPage.NUM_SYM

    private val keys = ArrayList<KeySpec>(44)
    private var pressedKey: KeySpec? = null
    private var downX = 0f
    private var downY = 0f
    private var lastSlideX = 0f
    private var slidingCursor = false
    private var swipedBackspace = false

    private val bgPaint = Paint(Paint.ANTI_ALIAS_FLAG)
    private val keyPaint = Paint(Paint.ANTI_ALIAS_FLAG)
    private val previewPaint = Paint(Paint.ANTI_ALIAS_FLAG)
    private val textPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply { textAlign = Paint.Align.CENTER }
    private val subPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply { textAlign = Paint.Align.CENTER }
    private val previewRect = RectF()

    private val density = resources.displayMetrics.density
    private val keyRadius = 6f * density
    private val gap = 4.5f * density

    fun applyTheme(newTheme: QinTheme) {
        theme = newTheme
        invalidate()
    }

    fun applyDisplayPrefs(heightPct: Int, oneHanded: OneHandedMode, preview: Boolean) {
        val heightChanged = heightPercent != heightPct
        heightPercent = heightPct.coerceIn(80, 125)
        oneHandedMode = oneHanded
        keyPreviewEnabled = preview
        if (heightChanged) requestLayout()
        rebuildKeys()
        invalidate()
    }

    fun setState(layoutMode: LayoutMode, languageMode: LanguageMode, pageOverride: KeyboardPage? = null) {
        activeLayoutMode = layoutMode
        lang = languageMode
        if (languageMode != LanguageMode.EN) {
            shiftState = ShiftState.OFF
        }
        val nextPage = pageOverride ?: if (layoutMode == LayoutMode.T9 && languageMode == LanguageMode.ZH) {
            KeyboardPage.T9
        } else {
            KeyboardPage.QWERTY
        }
        val numChanged = (page == KeyboardPage.NUM_SYM) != (nextPage == KeyboardPage.NUM_SYM)
        page = nextPage
        if (numChanged) listener?.onNumPageChanged(page == KeyboardPage.NUM_SYM)
        rebuildKeys()
        invalidate()
    }

    fun toggleNumPage(layoutMode: LayoutMode = activeLayoutMode) {
        val nextPage = if (page == KeyboardPage.NUM_SYM) {
            if (layoutMode == LayoutMode.T9 && lang == LanguageMode.ZH) KeyboardPage.T9 else KeyboardPage.QWERTY
        } else {
            KeyboardPage.NUM_SYM
        }
        val numChanged = (page == KeyboardPage.NUM_SYM) != (nextPage == KeyboardPage.NUM_SYM)
        page = nextPage
        if (numChanged) listener?.onNumPageChanged(page == KeyboardPage.NUM_SYM)
        rebuildKeys()
        invalidate()
    }

    fun setT9Syllables(list: List<String>) {
        if (t9Syllables != list) {
            t9Syllables = list.take(5)
            if (page == KeyboardPage.T9) {
                rebuildKeys()
                invalidate()
            }
        }
    }

    override fun onMeasure(widthMeasureSpec: Int, heightMeasureSpec: Int) {
        val w = MeasureSpec.getSize(widthMeasureSpec)
        val h = (232f * density * (heightPercent / 100f)).toInt()
        setMeasuredDimension(w, h)
    }

    override fun onSizeChanged(w: Int, h: Int, oldw: Int, oldh: Int) {
        super.onSizeChanged(w, h, oldw, oldh)
        rebuildKeys()
    }

    private fun rebuildKeys() {
        keys.clear()
        val fullW = width.toFloat()
        val h = height.toFloat()
        if (fullW <= 0f || h <= 0f) return

        // 单手模式：预留 14% 侧边操作区（含一键展开按钮）
        val sidePad = if (oneHandedMode == OneHandedMode.OFF) 0f else fullW * 0.14f
        val xOffset = if (oneHandedMode == OneHandedMode.RIGHT) sidePad else 0f
        val availW = fullW - sidePad

        if (oneHandedMode != OneHandedMode.OFF) {
            val sideX = if (oneHandedMode == OneHandedMode.RIGHT) gap else (availW + gap)
            val btnW = sidePad - gap * 2
            keys.add(
                KeySpec(
                    label = "⇔",
                    subLabel = "全宽",
                    action = KeyAction.EXIT_ONE_HANDED,
                    isFn = true,
                    rect = RectF(sideX, h * 0.35f, sideX + btnW, h * 0.65f),
                ),
            )
        }

        when (page) {
            KeyboardPage.QWERTY -> buildQwerty(xOffset, availW, h)
            KeyboardPage.T9 -> buildT9(xOffset, availW, h)
            KeyboardPage.NUM_SYM -> buildNumSym(xOffset, availW, h)
        }
    }

    private fun buildQwerty(xOff: Float, w: Float, h: Float) {
        val rows = 4
        val rowH = (h - gap * (rows + 1)) / rows
        val r1 = "qwertyuiop"
        val r2 = "asdfghjkl"
        val r3 = "zxcvbnm"

        val isUpper = shiftState != ShiftState.OFF && lang == LanguageMode.EN
        val kw1 = (w - gap * (r1.length + 1)) / r1.length
        r1.forEachIndexed { i, c ->
            val ch = if (isUpper) c.uppercaseChar() else c
            val x = xOff + gap + i * (kw1 + gap)
            keys.add(KeySpec(ch.toString(), "", KeyAction.CHAR, ch, rect = RectF(x, gap, x + kw1, gap + rowH)))
        }

        val y2 = gap * 2 + rowH
        val x2Start = xOff + (w - (r2.length * kw1 + (r2.length - 1) * gap)) / 2f
        r2.forEachIndexed { i, c ->
            val ch = if (isUpper) c.uppercaseChar() else c
            val x = x2Start + i * (kw1 + gap)
            keys.add(KeySpec(ch.toString(), "", KeyAction.CHAR, ch, rect = RectF(x, y2, x + kw1, y2 + rowH)))
        }

        val y3 = gap * 3 + rowH * 2
        val sideW = (w - (7 * kw1 + 10 * gap)) / 2f
        val isEn = lang == LanguageMode.EN
        keys.add(
            KeySpec(
                label = if (isEn) "⇧" else "分词",
                subLabel = if (isEn && shiftState == ShiftState.LOCKED) "锁定" else "",
                action = if (isEn) KeyAction.SHIFT else KeyAction.CHAR,
                charValue = '\'',
                isFn = !isEn || shiftState == ShiftState.OFF,
                isAccent = isEn && shiftState != ShiftState.OFF,
                rect = RectF(xOff + gap, y3, xOff + gap + sideW, y3 + rowH),
            ),
        )
        r3.forEachIndexed { i, c ->
            val ch = if (isUpper) c.uppercaseChar() else c
            val x = xOff + gap * 2 + sideW + i * (kw1 + gap)
            keys.add(KeySpec(ch.toString(), "", KeyAction.CHAR, ch, rect = RectF(x, y3, x + kw1, y3 + rowH)))
        }
        val bx = xOff + w - gap - sideW
        keys.add(KeySpec("⌫", "", KeyAction.BACKSPACE, isFn = true, rect = RectF(bx, y3, bx + sideW, y3 + rowH)))

        val y4 = gap * 4 + rowH * 3
        val weights = floatArrayOf(1.15f, 1.15f, 0.9f, 2.85f, 0.9f, 0.95f, 1.0f, 1.4f)
        val totalW = weights.sum()
        val avail = w - gap * (weights.size + 1)
        val commaCh = if (lang == LanguageMode.ZH) '，' else ','
        val periodCh = if (lang == LanguageMode.ZH) '。' else '.'
        val specs = listOf(
            KeySpec("123", "", KeyAction.SWITCH_NUM, isFn = true),
            KeySpec(if (lang == LanguageMode.ZH) "中/En" else "En/中", "", KeyAction.SWITCH_LANG, isFn = true),
            KeySpec(commaCh.toString(), "", KeyAction.CHAR, commaCh, isFn = true),
            KeySpec(if (lang == LanguageMode.ZH) "Qin书 拼音" else "English", "", KeyAction.SPACE),
            KeySpec(periodCh.toString(), "", KeyAction.CHAR, periodCh, isFn = true),
            KeySpec("😊", "", KeyAction.SWITCH_EMOJI, isFn = true),
            KeySpec("9键", "", KeyAction.SWITCH_LAYOUT, isFn = true),
            KeySpec("换行", "", KeyAction.ENTER, isAccent = true),
        )
        var cx = xOff + gap
        specs.forEachIndexed { idx, sp ->
            val kw = avail * (weights[idx] / totalW)
            sp.rect.set(cx, y4, cx + kw, y4 + rowH)
            keys.add(sp)
            cx += kw + gap
        }
    }

    private fun buildT9(xOff: Float, w: Float, h: Float) {
        val rows = 4
        val rowH = (h - gap * (rows + 1)) / rows
        val leftColW = w * 0.17f
        val rightColW = w * 0.17f
        val midW = w - leftColW - rightColW - gap * 6
        val cellW = midW / 3f

        val sylAreaH = h - rowH - gap * 3
        val sylList = if (t9Syllables.isNotEmpty()) t9Syllables else listOf("，", "。", "？", "！", "@")
        val sylH = (sylAreaH - gap * (sylList.size - 1)) / sylList.size
        sylList.forEachIndexed { i, s ->
            val y = gap + i * (sylH + gap)
            val isSyl = t9Syllables.isNotEmpty()
            keys.add(
                KeySpec(
                    label = s,
                    action = if (isSyl) KeyAction.T9_SYLLABLE else KeyAction.CHAR,
                    charValue = if (!isSyl && s.isNotEmpty()) s[0] else '\u0000',
                    isFn = true,
                    rect = RectF(xOff + gap, y, xOff + gap + leftColW, y + sylH),
                ),
            )
        }
        val y4 = gap * 4 + rowH * 3
        keys.add(KeySpec("123", "", KeyAction.SWITCH_NUM, isFn = true, rect = RectF(xOff + gap, y4, xOff + gap + leftColW, y4 + rowH)))

        val t9Labels = arrayOf(
            "1" to "分词", "2" to "ABC", "3" to "DEF",
            "4" to "GHI", "5" to "JKL", "6" to "MNO",
            "7" to "PQRS", "8" to "TUV", "9" to "WXYZ",
        )
        val midStartX = xOff + gap * 2 + leftColW
        for (r in 0 until 3) {
            val y = gap + r * (rowH + gap)
            for (c in 0 until 3) {
                val idx = r * 3 + c
                val (num, sub) = t9Labels[idx]
                val x = midStartX + c * (cellW + gap)
                keys.add(
                    KeySpec(
                        label = sub,
                        subLabel = num,
                        action = KeyAction.CHAR,
                        charValue = if (num == "1") '\'' else num[0],
                        rect = RectF(x, y, x + cellW, y + rowH),
                    ),
                )
            }
        }
        keys.add(KeySpec("中/En", "", KeyAction.SWITCH_LANG, isFn = true, rect = RectF(midStartX, y4, midStartX + cellW * 0.85f, y4 + rowH)))
        val spX = midStartX + cellW * 0.85f + gap
        val spW = midW - cellW * 1.7f - gap * 2
        keys.add(KeySpec("空格", "", KeyAction.SPACE, rect = RectF(spX, y4, spX + spW, y4 + rowH)))
        val emX = spX + spW + gap
        keys.add(KeySpec("😊", "", KeyAction.SWITCH_EMOJI, isFn = true, rect = RectF(emX, y4, midStartX + midW, y4 + rowH)))

        val rx = xOff + w - gap - rightColW
        keys.add(KeySpec("⌫", "", KeyAction.BACKSPACE, isFn = true, rect = RectF(rx, gap, rx + rightColW, gap + rowH)))
        keys.add(KeySpec("重输", "", KeyAction.CLEAR_BUFFER, isFn = true, rect = RectF(rx, gap * 2 + rowH, rx + rightColW, gap * 2 + rowH * 2)))
        keys.add(KeySpec("26键", "", KeyAction.SWITCH_LAYOUT, isFn = true, rect = RectF(rx, gap * 3 + rowH * 2, rx + rightColW, gap * 3 + rowH * 3)))
        keys.add(KeySpec("换行", "", KeyAction.ENTER, isAccent = true, rect = RectF(rx, y4, rx + rightColW, y4 + rowH)))
    }

    private fun buildNumSym(xOff: Float, w: Float, h: Float) {
        val rows = 4
        val rowH = (h - gap * (rows + 1)) / rows
        val cols = 6
        val kw = (w - gap * (cols + 1)) / cols
        val grid = arrayOf(
            arrayOf("(", "1", "2", "3", "+", "⌫"),
            arrayOf(")", "4", "5", "6", "-", "@"),
            arrayOf("符号", "7", "8", "9", "×", "="),
            arrayOf("返回", "，", "0", ".", "/", "换行"),
        )
        for (r in 0 until rows) {
            val y = gap + r * (rowH + gap)
            for (c in 0 until cols) {
                val lbl = grid[r][c]
                val x = xOff + gap + c * (kw + gap)
                val spec = when (lbl) {
                    "⌫" -> KeySpec(lbl, "", KeyAction.BACKSPACE, isFn = true, rect = RectF(x, y, x + kw, y + rowH))
                    "返回" -> KeySpec(lbl, "", KeyAction.SWITCH_NUM, isFn = true, rect = RectF(x, y, x + kw, y + rowH))
                    "换行" -> KeySpec(lbl, "", KeyAction.ENTER, isAccent = true, rect = RectF(x, y, x + kw, y + rowH))
                    "符号" -> KeySpec(lbl, "", KeyAction.SWITCH_SYMBOL, isFn = true, rect = RectF(x, y, x + kw, y + rowH))
                    else -> KeySpec(
                        label = lbl,
                        subLabel = if (lbl == "=") "大写" else "",
                        action = KeyAction.CHAR,
                        charValue = if (lbl == "×") '*' else lbl[0],
                        isFn = c == 0 || c >= 4,
                        rect = RectF(x, y, x + kw, y + rowH),
                    )
                }
                keys.add(spec)
            }
        }
    }

    override fun onDraw(canvas: Canvas) {
        bgPaint.color = theme.keyboardBg
        canvas.drawRect(0f, 0f, width.toFloat(), height.toFloat(), bgPaint)

        for (k in keys) {
            val isPressed = k === pressedKey
            keyPaint.color = when {
                k.isAccent -> theme.keyAccentBg
                k.isFn -> theme.keyFnBg
                else -> theme.keyBg
            }
            keyPaint.alpha = if (isPressed) 185 else 255
            canvas.drawRoundRect(k.rect, keyRadius, keyRadius, keyPaint)

            textPaint.color = if (k.isAccent) 0xFFFFFFFF.toInt() else theme.keyText
            textPaint.textSize = when {
                k.label.length >= 3 -> 14f * density
                page == KeyboardPage.T9 && k.subLabel.isNotEmpty() -> 15f * density
                else -> 18f * density
            }
            val fm = textPaint.fontMetrics
            val cy = k.rect.centerY() - (fm.ascent + fm.descent) / 2f
            if (k.subLabel.isNotEmpty()) {
                subPaint.color = if (k.isAccent) 0xFFE2E8F0.toInt() else theme.keySubText
                subPaint.textSize = 10.5f * density
                canvas.drawText(k.subLabel, k.rect.centerX(), k.rect.top + 13f * density, subPaint)
                canvas.drawText(k.label, k.rect.centerX(), cy + 5f * density, textPaint)
            } else {
                canvas.drawText(k.label, k.rect.centerX(), cy, textPaint)
            }
        }

        // 零额外 PopupWindow 的 Canvas 内置按键放大气泡预览
        val pk = pressedKey
        if (keyPreviewEnabled && pk != null && pk.action == KeyAction.CHAR && pk.label.length == 1) {
            val bw = maxOf(pk.rect.width() * 1.25f, 40f * density)
            val bh = 44f * density
            val left = (pk.rect.centerX() - bw / 2f).coerceIn(4f * density, width - bw - 4f * density)
            val top = (pk.rect.top - bh - 4f * density).coerceAtLeast(2f * density)
            previewRect.set(left, top, left + bw, top + bh)
            previewPaint.color = theme.keyAccentBg
            canvas.drawRoundRect(previewRect, 10f * density, 10f * density, previewPaint)

            textPaint.color = 0xFFFFFFFF.toInt()
            textPaint.textSize = 22f * density
            val fm = textPaint.fontMetrics
            val cy = previewRect.centerY() - (fm.ascent + fm.descent) / 2f
            canvas.drawText(pk.label, previewRect.centerX(), cy, textPaint)
        }
    }

    override fun onDetachedFromWindow() {
        removeCallbacks(shiftLongPressRunnable)
        super.onDetachedFromWindow()
    }

    override fun onTouchEvent(event: MotionEvent): Boolean {
        when (event.actionMasked) {
            MotionEvent.ACTION_DOWN -> {
                downX = event.x
                downY = event.y
                lastSlideX = event.x
                slidingCursor = false
                swipedBackspace = false
                shiftLongPressed = false
                removeCallbacks(shiftLongPressRunnable)
                pressedKey = findKey(event.x, event.y)
                if (pressedKey?.action == KeyAction.SHIFT && lang == LanguageMode.EN) {
                    postDelayed(shiftLongPressRunnable, 420L)
                }
                if (pressedKey != null && hapticEnabled) {
                    performHapticFeedback(HapticFeedbackConstants.KEYBOARD_TAP)
                }
                invalidate()
                return true
            }
            MotionEvent.ACTION_MOVE -> {
                val pk = pressedKey ?: return true
                val dx = event.x - lastSlideX
                if (pk.action == KeyAction.SHIFT && !pk.rect.contains(event.x, event.y)) {
                    removeCallbacks(shiftLongPressRunnable)
                }
                if (pk.action == KeyAction.SPACE && abs(event.x - downX) > 18f * density) {
                    slidingCursor = true
                    val step = 14f * density
                    if (abs(dx) >= step) {
                        listener?.onMoveCursor(if (dx > 0) 1 else -1)
                        lastSlideX = event.x
                    }
                }
                if (pk.action == KeyAction.BACKSPACE && (downX - event.x) > 40f * density && !swipedBackspace) {
                    swipedBackspace = true
                    listener?.onClearBuffer()
                }
                return true
            }
            MotionEvent.ACTION_UP -> {
                removeCallbacks(shiftLongPressRunnable)
                val pk = pressedKey
                pressedKey = null
                invalidate()
                if (pk != null && !slidingCursor && !swipedBackspace && !(pk.action == KeyAction.SHIFT && shiftLongPressed)) {
                    dispatchKey(pk)
                }
                return true
            }
            MotionEvent.ACTION_CANCEL -> {
                removeCallbacks(shiftLongPressRunnable)
                pressedKey = null
                invalidate()
                return true
            }
        }
        return super.onTouchEvent(event)
    }

    private fun dispatchKey(k: KeySpec) {
        when (k.action) {
            KeyAction.CHAR -> {
                listener?.onChar(k.charValue)
                if (shiftState == ShiftState.ONCE && lang == LanguageMode.EN) {
                    shiftState = ShiftState.OFF
                    rebuildKeys()
                    invalidate()
                }
            }
            KeyAction.BACKSPACE -> listener?.onBackspace()
            KeyAction.CLEAR_BUFFER -> listener?.onClearBuffer()
            KeyAction.ENTER -> listener?.onEnter()
            KeyAction.SPACE -> listener?.onSpace()
            KeyAction.SHIFT -> {
                val now = System.currentTimeMillis()
                shiftState = when (shiftState) {
                    ShiftState.OFF -> {
                        lastShiftTapMs = now
                        ShiftState.ONCE
                    }
                    ShiftState.ONCE -> {
                        if (now - lastShiftTapMs <= 380L) ShiftState.LOCKED else ShiftState.OFF
                    }
                    ShiftState.LOCKED -> ShiftState.OFF
                }
                rebuildKeys()
                invalidate()
            }
            KeyAction.SWITCH_LANG -> listener?.onToggleLanguage()
            KeyAction.SWITCH_LAYOUT -> listener?.onToggleLayout()
            KeyAction.SWITCH_NUM -> toggleNumPage(activeLayoutMode)
            KeyAction.SWITCH_EMOJI -> listener?.onToggleEmojiPanel()
            KeyAction.SWITCH_SYMBOL -> listener?.onToggleSymbolPanel()
            KeyAction.T9_SYLLABLE -> listener?.onSelectT9Syllable(k.label)
            KeyAction.EXIT_ONE_HANDED -> listener?.onExitOneHanded()
        }
    }

    private fun findKey(x: Float, y: Float): KeySpec? =
        keys.firstOrNull { it.rect.contains(x, y) }
}
