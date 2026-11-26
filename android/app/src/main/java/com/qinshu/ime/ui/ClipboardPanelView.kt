package com.qinshu.ime.ui

import android.content.Context
import android.graphics.drawable.GradientDrawable
import android.text.TextUtils
import android.util.TypedValue
import android.view.Gravity
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import com.qinshu.ime.config.QinTheme
import com.qinshu.ime.smart.ClipItem

/**
 * 轻量本地剪贴板历史面板（最多 20 条，单击粘贴，长按置顶/取消置顶，一键清空未置顶）。
 */
class ClipboardPanelView(context: Context) : LinearLayout(context) {

    interface Listener {
        fun onPaste(text: String)
        fun onTogglePin(text: String)
        fun onClearUnpinned()
        fun onClose()
    }

    var listener: Listener? = null
    private var theme: QinTheme = QinTheme.CLASSIC_LIGHT
    private val density = resources.displayMetrics.density

    private val listContainer = LinearLayout(context).apply {
        orientation = VERTICAL
        setPadding((10 * density).toInt(), (6 * density).toInt(), (10 * density).toInt(), (6 * density).toInt())
    }
    private val scrollView = ScrollView(context).apply {
        layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, 0, 1f)
        addView(listContainer)
    }
    private val bottomBar = LinearLayout(context).apply {
        orientation = HORIZONTAL
        gravity = Gravity.CENTER_VERTICAL
        layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, (40 * density).toInt())
    }

    init {
        orientation = VERTICAL
        layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, (232 * density).toInt())
        addView(scrollView)
        addView(bottomBar)
        renderBottomBar()
    }

    fun applyTheme(t: QinTheme) {
        theme = t
        setBackgroundColor(t.keyboardBg)
        bottomBar.setBackgroundColor(t.candidateBg)
        renderBottomBar()
    }

    fun setItems(items: List<ClipItem>) {
        listContainer.removeAllViews()
        if (items.isEmpty()) {
            listContainer.addView(
                TextView(context).apply {
                    text = "暂无剪贴板记录（复制内容后将自动识别验证码、网址、邮箱、手机号与单号）"
                    gravity = Gravity.CENTER
                    setTextSize(TypedValue.COMPLEX_UNIT_SP, 13f)
                    setTextColor(theme.keySubText)
                    setPadding((12 * density).toInt(), (36 * density).toInt(), (12 * density).toInt(), (12 * density).toInt())
                },
            )
            return
        }
        for (item in items) {
            val tv = TextView(context).apply {
                text = if (item.pinned) "📌 ${item.text}" else item.text
                maxLines = 2
                ellipsize = TextUtils.TruncateAt.END
                setTextSize(TypedValue.COMPLEX_UNIT_SP, 14f)
                setTextColor(theme.keyText)
                background = GradientDrawable().apply {
                    setColor(theme.keyBg)
                    cornerRadius = 8f * density
                }
                setPadding((12 * density).toInt(), (9 * density).toInt(), (12 * density).toInt(), (9 * density).toInt())
                layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT).apply {
                    bottomMargin = (6 * density).toInt()
                }
                setOnClickListener { listener?.onPaste(item.text) }
                setOnLongClickListener {
                    listener?.onTogglePin(item.text)
                    true
                }
            }
            listContainer.addView(tv)
        }
    }

    private fun renderBottomBar() {
        bottomBar.removeAllViews()
        addBarBtn("返回键盘") { listener?.onClose() }
        addBarBtn("长按条目可置顶") { }
        addBarBtn("清空未置顶") { listener?.onClearUnpinned() }
    }

    private fun addBarBtn(label: String, onClick: () -> Unit) {
        bottomBar.addView(
            TextView(context).apply {
                text = label
                gravity = Gravity.CENTER
                setTextSize(TypedValue.COMPLEX_UNIT_SP, 12.5f)
                setTextColor(theme.keySubText)
                layoutParams = LayoutParams(0, LayoutParams.MATCH_PARENT, 1f)
                setOnClickListener { onClick() }
            },
        )
    }
}
