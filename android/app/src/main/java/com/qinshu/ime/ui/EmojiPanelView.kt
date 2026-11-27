package com.qinshu.ime.ui

import android.content.Context
import android.graphics.Paint
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
import com.qinshu.ime.config.PrefsManager
import com.qinshu.ime.config.QinTheme
import com.qinshu.ime.engine.QDictReader

/**
 * 纯微表情（Unicode Emoji）分类面板（v1.1 升级版）：
 * - 不含颜文字，零图片资源包，直接调用系统 NotoColorEmoji
 * - 新增：顶部情绪关键词快捷反查（调用 qinshu.qdict 微表情表）
 * - 新增：常用页签按实际点击频次动态排序（持久化到 PrefsManager）
 * - 新增：长按手势微表情弹出 5 档肤色变体条
 */
class EmojiPanelView(
    context: Context,
    private val dictReader: QDictReader,
    private val prefs: PrefsManager,
) : LinearLayout(context) {

    interface Listener {
        fun onEmojiSelected(emoji: String)
        fun onClose()
        fun onBackspace()
    }

    var listener: Listener? = null
    private var theme: QinTheme = QinTheme.CLASSIC_LIGHT
    private val density = resources.displayMetrics.density
    private val glyphPaint = Paint()
    private var activeCategory = 0

    private val skinToneBases = setOf("👍", "👎", "👌", "🙏", "💪", "👋", "👏", "🤝", "✌️", "☝️", "🫶", "🙌", "✊", "👊")
    private val skinModifiers = arrayOf("", "\uD83C\uDFFB", "\uD83C\uDFFC", "\uD83C\uDFFD", "\uD83C\uDFFE", "\uD83C\uDFFF")

    private val quickSearchTags = listOf(
        "捂脸" to "wulian", "抱拳" to "baoquan", "破涕" to "potiweixiao", "偷笑" to "touxiao",
        "开心" to "kaixin", "哈哈" to "haha", "点赞" to "zan", "谢谢" to "xiexie",
        "加油" to "jiayou", "玫瑰" to "meigui", "爱" to "ai", "哭" to "ku", "无语" to "wuyu",
        "发财" to "facai", "祝福" to "zhufu", "吃饭" to "chifan", "工作" to "gongzuo",
    )

    private fun isDisplayableEmoji(s: String): Boolean =
        WeChatEmojiDrawable.supports(s) || glyphPaint.hasGlyph(s)

    private val categories: List<Pair<String, List<String>>> by lazy {
        listOf(
            "常用" to listOf("[捂脸]", "[抱拳]", "[破涕为笑]", "[偷笑]", "[憨笑]", "[玫瑰]", "[合十]", "[强]", "😄", "😂", "🤣", "😊", "🥰", "😍", "😘", "😅", "😭", "🥹", "🥺", "🙏", "👍", "👌", "💪", "🫡", "🫠", "🫶", "🫂", "👏", "🙌", "🤝", "🔥", "❤️", "🎉", "✨", "🤔", "😑", "🙄", "😱", "😴", "👋"),
            "情绪" to listOf("[捂脸]", "[破涕为笑]", "[偷笑]", "[憨笑]", "🤦", "🫣", "🥹", "🫡", "🫠", "😀", "😃", "😄", "😁", "😆", "😅", "🤣", "😂", "🙂", "😉", "😊", "😇", "🥰", "😍", "🤩", "😘", "😗", "😚", "😋", "😛", "😜", "🤪", "😝", "🤑", "🤗", "🤭", "🤫", "🤔", "🤐", "🤨", "😐", "😑", "😶", "😏", "😒", "🙄", "😬", "😌", "😔", "😪", "🤤", "😴", "😷", "🤒", "🤕", "🤢", "🥵", "🥶", "😵", "🤯", "🥳", "😎", "🤓", "😕", "😟", "🙁", "😮", "😯", "😲", "😳", "🥺", "😦", "😨", "😰", "😢", "😭", "😱", "😖", "😣", "😞", "😓", "😩", "😫", "😤", "😡", "😠", "🤬"),
            "手势" to listOf("[抱拳]", "[合十]", "[强]", "👋", "🤚", "🖐️", "✋", "🖖", "👌", "🤌", "🤏", "✌️", "🤞", "🤟", "🤘", "🤙", "👈", "👉", "👆", "👇", "☝️", "👍", "👎", "✊", "👊", "🤛", "🤜", "👏", "🙌", "🫶", "👐", "🤲", "🤝", "🙏", "💪", "🫂", "👀", "❤️", "🧡", "💛", "💚", "💙", "💜", "🖤", "🤍", "💔", "💕", "💖", "💯", "💢", "💥", "💦", "💤"),
            "生活" to listOf("[玫瑰]", "🌹", "💐", "🧧", "🎆", "🥳", "🍚", "🍜", "🍲", "🍢", "🧋", "☕", "🍵", "🍺", "🍻", "🥂", "🎂", "🍰", "🍎", "🍉", "🍓", "🍊", "🍦", "📱", "💻", "📧", "📞", "⏰", "📅", "💼", "🏠", "🏢", "✈️", "🚄", "🚗", "📍", "🎁", "💰", "📈", "📉", "🚀", "🏆", "🥇", "🎵", "🎬", "🎮", "📸", "⚽", "🏀", "🏃"),
            "自然" to listOf("☀️", "🌤️", "⛅", "🌧️", "⛈️", "🌩️", "❄️", "⛄", "🌈", "🔥", "💧", "🌊", "🌙", "⭐", "✨", "🌟", "🌸", "🌹", "🌺", "🌻", "🌳", "🍃", "🐱", "🐶", "🐼", "🐰", "🐷", "🐯", "🐉", "🐴", "🐑", "🐵", "🐔", "🐟"),
            "符号" to listOf("✅", "✔️", "❌", "❎", "⚠️", "❗", "❓", "💡", "🔍", "⚙️", "🔒", "🔑", "📌", "📝", "🗑️", "⏳", "🛑", "▶️", "🔄", "⬆️", "⬇️", "⬅️", "➡️", "0️⃣", "1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"),
        ).map { (name, list) -> name to list.filter { isDisplayableEmoji(it) } }
    }

    private val searchRow = LinearLayout(context).apply {
        orientation = HORIZONTAL
        gravity = Gravity.CENTER_VERTICAL
        setPadding((6 * density).toInt(), (4 * density).toInt(), (6 * density).toInt(), (4 * density).toInt())
    }
    private val searchScroll = HorizontalScrollView(context).apply {
        isHorizontalScrollBarEnabled = false
        layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, (34 * density).toInt())
        addView(searchRow)
    }

    private val skinBar = LinearLayout(context).apply {
        orientation = HORIZONTAL
        gravity = Gravity.CENTER
        visibility = View.GONE
        layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, (40 * density).toInt())
    }

    private val gridView = GridView(context).apply {
        numColumns = 8
        verticalSpacing = (4 * density).toInt()
        horizontalSpacing = (2 * density).toInt()
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
        layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, (46 * density).toInt())
        setPadding((6 * density).toInt(), (4 * density).toInt(), (6 * density).toInt(), (4 * density).toInt())
    }

    init {
        orientation = VERTICAL
        layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, (232 * density).toInt())
        addView(searchScroll)
        addView(skinBar)
        addView(gridView)
        addView(tabBar)
        renderQuickSearch()
        renderTabs()
        renderCategory(0)
    }

    fun applyTheme(t: QinTheme) {
        theme = t
        setBackgroundColor(t.keyboardBg)
        tabBar.setBackgroundColor(t.candidateBg)
        skinBar.setBackgroundColor(t.keyBg)
        renderQuickSearch()
        renderTabs()
    }

    private fun renderQuickSearch() {
        searchRow.removeAllViews()
        for ((label, py) in quickSearchTags) {
            searchRow.addView(
                TextView(context).apply {
                    text = label
                    setTextSize(TypedValue.COMPLEX_UNIT_SP, 12f)
                    setTextColor(theme.keyText)
                    background = GradientDrawable().apply {
                        setColor(theme.keyBg)
                        cornerRadius = 12f * density
                    }
                    setPadding((9 * density).toInt(), (3 * density).toInt(), (9 * density).toInt(), (3 * density).toInt())
                    layoutParams = LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT).apply {
                        marginEnd = (6 * density).toInt()
                    }
                    setOnClickListener {
                        skinBar.visibility = View.GONE
                        val hits = dictReader.lookupEmoji(py, 16).map { it.word }.filter { isDisplayableEmoji(it) }
                        if (hits.isNotEmpty()) bindGrid(hits)
                    }
                },
            )
        }
    }

    private fun renderTabs() {
        tabBar.removeAllViews()
        tabRow.removeAllViews()

        tabBar.addView(createPinnedBarButton("返回", 14f) { listener?.onClose() })
        tabBar.addView(tabScroll)
        categories.forEachIndexed { idx, (name, _) ->
            addCategoryTab(name, idx == activeCategory) {
                activeCategory = idx
                skinBar.visibility = View.GONE
                renderTabs()
                renderCategory(idx)
            }
        }
        tabBar.addView(createPinnedBarButton("⌫", 18f) { listener?.onBackspace() })
    }

    private fun createPinnedBarButton(label: String, textSizeSp: Float, onClick: () -> Unit): TextView =
        TextView(context).apply {
            text = label
            gravity = Gravity.CENTER
            setTextSize(TypedValue.COMPLEX_UNIT_SP, textSizeSp)
            setTextColor(theme.keyText)
            background = GradientDrawable().apply {
                setColor(theme.keyFnBg)
                cornerRadius = 8f * density
            }
            layoutParams = LayoutParams((58 * density).toInt(), LayoutParams.MATCH_PARENT).apply {
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
                setTextSize(TypedValue.COMPLEX_UNIT_SP, 14f)
                setTextColor(if (selected) theme.keyAccentBg else theme.keySubText)
                if (selected) {
                    background = GradientDrawable().apply {
                        setColor(theme.capsuleBg)
                        cornerRadius = 8f * density
                    }
                }
                setPadding((14 * density).toInt(), (6 * density).toInt(), (14 * density).toInt(), (6 * density).toInt())
                layoutParams = LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.MATCH_PARENT).apply {
                    marginStart = (2 * density).toInt()
                    marginEnd = (2 * density).toInt()
                }
                setOnClickListener { onClick() }
            },
        )
    }

    private fun renderCategory(idx: Int) {
        val items = if (idx == 0) {
            (prefs.getTopUsedEmojis(24) + categories[0].second).distinct().filter { isDisplayableEmoji(it) }
        } else {
            categories[idx].second
        }
        bindGrid(items)
    }

    private fun bindGrid(items: List<String>) {
        gridView.adapter = object : ArrayAdapter<String>(context, 0, items) {
            override fun getView(position: Int, convertView: View?, parent: ViewGroup): View {
                val tv = (convertView as? TextView) ?: TextView(context).apply {
                    gravity = Gravity.CENTER
                    maxLines = 1
                }
                val em = items[position]
                if (WeChatEmojiDrawable.supports(em)) {
                    tv.text = WeChatEmojiDrawable.toSpannable(em, (26 * density).toInt())
                    tv.background = null
                    tv.setPadding(0, (6 * density).toInt(), 0, (6 * density).toInt())
                } else if (em.startsWith("[")) {
                    tv.text = em
                    tv.setTextSize(TypedValue.COMPLEX_UNIT_DIP, 10.5f)
                    tv.setTextColor(theme.capsuleText)
                    tv.background = GradientDrawable().apply {
                        setColor(theme.capsuleBg)
                        cornerRadius = 8f * density
                    }
                    tv.setPadding((2 * density).toInt(), (9 * density).toInt(), (2 * density).toInt(), (9 * density).toInt())
                } else {
                    tv.text = em
                    tv.setTextSize(TypedValue.COMPLEX_UNIT_SP, 22f)
                    tv.setTextColor(theme.keyText)
                    tv.background = null
                    tv.setPadding(0, (7 * density).toInt(), 0, (7 * density).toInt())
                }
                tv.setOnClickListener {
                    skinBar.visibility = View.GONE
                    prefs.recordEmojiUse(em)
                    listener?.onEmojiSelected(em)
                }
                tv.setOnLongClickListener {
                    if (skinToneBases.contains(em)) {
                        showSkinTonePicker(em)
                        true
                    } else {
                        false
                    }
                }
                return tv
            }
        }
    }

    private fun showSkinTonePicker(baseEmoji: String) {
        skinBar.removeAllViews()
        val cleanBase = baseEmoji.replace("\uFE0F", "")
        for (mod in skinModifiers) {
            val variant = if (mod.isEmpty()) baseEmoji else "$cleanBase$mod"
            if (!glyphPaint.hasGlyph(variant)) continue
            skinBar.addView(
                TextView(context).apply {
                    text = variant
                    gravity = Gravity.CENTER
                    setTextSize(TypedValue.COMPLEX_UNIT_SP, 22f)
                    setPadding((10 * density).toInt(), (4 * density).toInt(), (10 * density).toInt(), (4 * density).toInt())
                    setOnClickListener {
                        skinBar.visibility = View.GONE
                        prefs.recordEmojiUse(variant)
                        listener?.onEmojiSelected(variant)
                    }
                },
            )
        }
        skinBar.visibility = View.VISIBLE
    }
}
