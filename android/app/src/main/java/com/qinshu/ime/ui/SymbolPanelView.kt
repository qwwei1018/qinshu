package com.qinshu.ime.ui

import android.content.Context
import android.graphics.drawable.GradientDrawable
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.view.ViewGroup
import android.widget.ArrayAdapter
import android.widget.GridView
import android.widget.HorizontalScrollView
import android.widget.LinearLayout
import android.widget.TextView
import com.qinshu.ime.config.QinTheme

/**
 * 原生零依赖标点符号面板（常用 / 中文 / 英文 / 数学 / 序号 / 特殊 6 大分类）：
 * - 未锁定：点击符号后自动收起面板回到键盘；锁定后可连续输入
 * - 成对符号（“”（）【】《》 等）上屏后自动把光标移到中间
 */
class SymbolPanelView(context: Context) : LinearLayout(context) {

    interface Listener {
        fun onSymbolSelected(symbol: String, autoClose: Boolean)
        fun onClose()
        fun onBackspace()
    }

    var listener: Listener? = null
    private var theme: QinTheme = QinTheme.CLASSIC_LIGHT
    private val density = resources.displayMetrics.density
    private var activeCategory = 0
    private var lockPanel = false

    private val categories: List<Pair<String, List<String>>> = listOf(
        "常用" to listOf(
            "，", "。", "？", "！", "、", "：", "；", "“”",
            "（）", "【】", "《》", "——", "……", "·", "~", "@",
            "#", "¥", "%", "&", "*", "-", "+", "=",
            "/", "\\", "|", "_", "()", "[]", "{}", "<>",
        ),
        "中文" to listOf(
            "，", "。", "、", "？", "！", "：", "；", "“",
            "”", "‘", "’", "“”", "‘’", "（", "）", "（）",
            "【", "】", "【】", "《", "》", "《》", "「", "」",
            "「」", "『』", "〈〉", "——", "……", "·", "～", "￥",
            "％", "＃", "＠", "＆", "＊", "－", "＋", "＝",
        ),
        "英文" to listOf(
            ",", ".", "?", "!", ":", ";", "'", "\"",
            "(", ")", "()", "[", "]", "[]", "{", "}",
            "{}", "<", ">", "<>", "@", "#", "$", "%",
            "^", "&", "*", "-", "_", "+", "=", "/",
            "\\", "|", "~", "`", "''", "\"\"", "...", "--",
        ),
        "数学" to listOf(
            "+", "-", "×", "÷", "=", "≠", "≈", "±",
            "<", ">", "≤", "≥", "%", "‰", "°", "℃",
            "℉", "∞", "√", "²", "³", "π", "∑", "∏",
            "∫", "∠", "⊥", "∥", "△", "○", "∵", "∴",
            "∈", "∉", "⊂", "∪", "∩", "∅", "→", "←",
        ),
        "序号" to listOf(
            "①", "②", "③", "④", "⑤", "⑥", "⑦", "⑧",
            "⑨", "⑩", "⑴", "⑵", "⑶", "⑷", "⑸", "⑹",
            "⑺", "⑻", "⑼", "⑽", "⒈", "⒉", "⒊", "⒋",
            "⒌", "⒍", "⒎", "⒏", "⒐", "⒑", "一、", "二、",
            "三、", "四、", "Ⅰ", "Ⅱ", "Ⅲ", "Ⅳ", "Ⅴ", "Ⅵ",
        ),
        "特殊" to listOf(
            "✓", "✗", "✔", "✘", "★", "☆", "♥", "♡",
            "●", "○", "■", "□", "▲", "△", "◆", "◇",
            "※", "§", "№", "℗", "©", "®", "™", "€",
            "£", "$", "¥", "¢", "↑", "↓", "←", "→",
            "↖", "↗", "↘", "↙", "↔", "↕", "⇒", "⇔",
        ),
    )

    private val gridView = GridView(context).apply {
        numColumns = 8
        verticalSpacing = (4 * density).toInt()
        horizontalSpacing = (2 * density).toInt()
        setPadding((6 * density).toInt(), (6 * density).toInt(), (6 * density).toInt(), (4 * density).toInt())
        layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, 0, 1f)
    }

    private val tabRow = LinearLayout(context).apply {
        orientation = HORIZONTAL
        gravity = Gravity.CENTER_VERTICAL
        setPadding((2 * density).toInt(), 0, (2 * density).toInt(), 0)
    }
    private val tabScroll = HorizontalScrollView(context).apply {
        isHorizontalScrollBarEnabled = false
        layoutParams = LayoutParams(0, LayoutParams.MATCH_PARENT, 1f)
        addView(tabRow)
    }
    private val tabBar = LinearLayout(context).apply {
        orientation = HORIZONTAL
        gravity = Gravity.CENTER_VERTICAL
        layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, (42 * density).toInt())
        setPadding((6 * density).toInt(), (4 * density).toInt(), (6 * density).toInt(), (4 * density).toInt())
    }

    init {
        orientation = VERTICAL
        layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, (232 * density).toInt())
        addView(gridView)
        addView(tabBar)
        renderTabs()
        renderCategory(0)
    }

    fun applyTheme(t: QinTheme) {
        theme = t
        setBackgroundColor(t.keyboardBg)
        tabBar.setBackgroundColor(t.candidateBg)
        renderTabs()
        renderCategory(activeCategory)
    }

    private fun renderTabs() {
        tabBar.removeAllViews()
        tabRow.removeAllViews()
        tabBar.addView(pinnedButton("返回") { listener?.onClose() })
        tabBar.addView(tabScroll)
        categories.forEachIndexed { idx, (name, _) ->
            addCategoryTab(name, idx == activeCategory) {
                activeCategory = idx
                renderTabs()
                renderCategory(idx)
            }
        }
        addCategoryTab(if (lockPanel) "已锁定" else "连输", lockPanel) {
            lockPanel = !lockPanel
            renderTabs()
        }
        tabBar.addView(pinnedButton("⌫") { listener?.onBackspace() })
    }

    private fun pinnedButton(label: String, onClick: () -> Unit): TextView =
        TextView(context).apply {
            text = label
            gravity = Gravity.CENTER
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 13f)
            setTextColor(theme.keyText)
            background = GradientDrawable().apply {
                setColor(theme.keyFnBg)
                cornerRadius = 8f * density
            }
            layoutParams = LayoutParams((52 * density).toInt(), LayoutParams.MATCH_PARENT).apply {
                marginStart = (3 * density).toInt()
                marginEnd = (3 * density).toInt()
            }
            setOnClickListener { onClick() }
        }

    private fun addCategoryTab(label: String, selected: Boolean, onClick: () -> Unit) {
        tabRow.addView(
            TextView(context).apply {
                text = label
                gravity = Gravity.CENTER
                setTextSize(TypedValue.COMPLEX_UNIT_SP, 13f)
                setTextColor(if (selected) theme.keyAccentBg else theme.keySubText)
                if (selected) {
                    background = GradientDrawable().apply {
                        setColor(theme.capsuleBg)
                        cornerRadius = 8f * density
                    }
                }
                setPadding((11 * density).toInt(), (6 * density).toInt(), (11 * density).toInt(), (6 * density).toInt())
                layoutParams = LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.MATCH_PARENT).apply {
                    marginStart = (2 * density).toInt()
                    marginEnd = (2 * density).toInt()
                }
                setOnClickListener { onClick() }
            },
        )
    }

    private fun renderCategory(idx: Int) {
        val items = categories[idx].second
        gridView.adapter = object : ArrayAdapter<String>(context, 0, items) {
            override fun getView(position: Int, convertView: View?, parent: ViewGroup): View {
                val tv = (convertView as? TextView) ?: TextView(context).apply {
                    gravity = Gravity.CENTER
                    maxLines = 1
                }
                val sym = items[position]
                tv.text = sym
                tv.setTextSize(
                    TypedValue.COMPLEX_UNIT_SP,
                    if (sym.length >= 3) 11f else if (sym.length == 2) 15f else 17f,
                )
                tv.setTextColor(theme.keyText)
                tv.background = GradientDrawable().apply {
                    setColor(theme.keyBg)
                    cornerRadius = 6f * density
                }
                tv.setPadding((2 * density).toInt(), (9 * density).toInt(), (2 * density).toInt(), (9 * density).toInt())
                tv.setOnClickListener {
                    listener?.onSymbolSelected(sym, autoClose = !lockPanel)
                }
                return tv
            }
        }
    }
}
