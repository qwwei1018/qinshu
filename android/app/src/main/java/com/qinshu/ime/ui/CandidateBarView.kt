package com.qinshu.ime.ui

import android.content.Context
import android.graphics.drawable.GradientDrawable
import android.text.TextUtils
import android.util.TypedValue
import android.view.Gravity
import android.widget.HorizontalScrollView
import android.widget.LinearLayout
import android.widget.TextView
import com.qinshu.ime.config.QinTheme
import com.qinshu.ime.engine.Candidate
import com.qinshu.ime.engine.SmartCapsule

/**
 * 顶部候选栏 + 智能胶囊栏（验证码 / 网址 / Email / 手机号 / 单号 一键直填 + 剪贴板面板入口）。
 */
class CandidateBarView(context: Context) : LinearLayout(context) {

    interface Listener {
        fun onCandidateClick(cand: Candidate, index: Int)
        fun onCandidateLongClick(cand: Candidate)
        fun onCapsuleClick(capsule: SmartCapsule)
        fun onOpenEmoji()
        fun onOpenClipboard()
        fun onToggleLayout()
        fun onOpenSettings()
    }

    var listener: Listener? = null
    private var theme: QinTheme = QinTheme.CLASSIC_LIGHT
    private val density = resources.displayMetrics.density

    private val scrollView = HorizontalScrollView(context).apply {
        isHorizontalScrollBarEnabled = false
        layoutParams = LayoutParams(0, LayoutParams.MATCH_PARENT, 1f)
    }
    private val container = LinearLayout(context).apply {
        orientation = HORIZONTAL
        gravity = Gravity.CENTER_VERTICAL
        setPadding((8 * density).toInt(), 0, (8 * density).toInt(), 0)
    }

    init {
        orientation = HORIZONTAL
        gravity = Gravity.CENTER_VERTICAL
        layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, (44 * density).toInt())
        scrollView.addView(container)
        addView(scrollView)
    }

    fun applyTheme(t: QinTheme) {
        theme = t
        setBackgroundColor(t.candidateBg)
    }

    fun showCandidates(candidates: List<Candidate>, composing: String) {
        container.removeAllViews()
        scrollView.scrollTo(0, 0)
        if (composing.isNotEmpty()) {
            container.addView(
                TextView(context).apply {
                    text = composing
                    setTextSize(TypedValue.COMPLEX_UNIT_SP, 12f)
                    setTextColor(theme.keySubText)
                    setPadding((4 * density).toInt(), 0, (10 * density).toInt(), 0)
                },
            )
        }
        candidates.forEachIndexed { idx, cand ->
            val tv = TextView(context).apply {
                if (cand.source == "emoji" && WeChatEmojiDrawable.supports(cand.text)) {
                    text = WeChatEmojiDrawable.toSpannable(cand.text, (22 * density).toInt())
                } else {
                    text = cand.text
                    setTextSize(
                        TypedValue.COMPLEX_UNIT_SP,
                        if (cand.source == "emoji" && !cand.text.startsWith("[")) 18f else 16f,
                    )
                }
                setTextColor(if (idx == 0) theme.keyAccentBg else theme.keyText)
                setPadding((10 * density).toInt(), (6 * density).toInt(), (10 * density).toInt(), (6 * density).toInt())
                setOnClickListener { listener?.onCandidateClick(cand, idx) }
                setOnLongClickListener {
                    listener?.onCandidateLongClick(cand)
                    true
                }
            }
            container.addView(tv)
        }
    }

    fun showIdleBar(capsules: List<SmartCapsule>) {
        container.removeAllViews()
        scrollView.scrollTo(0, 0)

        for (cap in capsules) {
            val pill = TextView(context).apply {
                text = cap.label
                maxLines = 1
                ellipsize = TextUtils.TruncateAt.END
                setTextSize(TypedValue.COMPLEX_UNIT_SP, 13f)
                setTextColor(theme.capsuleText)
                background = GradientDrawable().apply {
                    setColor(theme.capsuleBg)
                    cornerRadius = 14f * density
                }
                setPadding((10 * density).toInt(), (4 * density).toInt(), (10 * density).toInt(), (4 * density).toInt())
                layoutParams = LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT).apply {
                    marginEnd = (8 * density).toInt()
                }
                setOnClickListener { listener?.onCapsuleClick(cap) }
            }
            container.addView(pill)
        }

        addToolChip("📋 剪贴板") { listener?.onOpenClipboard() }
        addToolChip("😊 微表情") { listener?.onOpenEmoji() }
        addToolChip("⌨️ 9/26键") { listener?.onToggleLayout() }
        addToolChip("⚙️ 设置") { listener?.onOpenSettings() }
    }

    private fun addToolChip(label: String, onClick: () -> Unit) {
        container.addView(
            TextView(context).apply {
                text = label
                setTextSize(TypedValue.COMPLEX_UNIT_SP, 13f)
                setTextColor(theme.keySubText)
                setPadding((9 * density).toInt(), (6 * density).toInt(), (9 * density).toInt(), (6 * density).toInt())
                setOnClickListener { onClick() }
            },
        )
    }
}
