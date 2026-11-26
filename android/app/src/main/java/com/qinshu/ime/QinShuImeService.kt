package com.qinshu.ime

import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.res.Configuration
import android.inputmethodservice.InputMethodService
import android.text.InputType
import android.view.KeyEvent
import android.view.View
import android.view.inputmethod.EditorInfo
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.Toast
import com.qinshu.ime.config.OneHandedMode
import com.qinshu.ime.config.PrefsManager
import com.qinshu.ime.engine.Candidate
import com.qinshu.ime.engine.CapsuleType
import com.qinshu.ime.engine.LanguageMode
import com.qinshu.ime.engine.LayoutMode
import com.qinshu.ime.engine.PinyinEngine
import com.qinshu.ime.engine.QDictReader
import com.qinshu.ime.engine.SmartCapsule
import com.qinshu.ime.engine.SmartRecognizer
import com.qinshu.ime.engine.UserDictStore
import com.qinshu.ime.smart.ClipboardStore
import com.qinshu.ime.smart.SmartCapsuleBus
import com.qinshu.ime.ui.CandidateBarView
import com.qinshu.ime.ui.ClipboardPanelView
import com.qinshu.ime.ui.EmojiPanelView
import com.qinshu.ime.ui.KeyboardCanvasView

class QinShuImeService : InputMethodService() {

    private lateinit var prefs: PrefsManager
    private lateinit var dictReader: QDictReader
    private lateinit var userStore: UserDictStore
    private lateinit var clipboardStore: ClipboardStore
    private lateinit var engine: PinyinEngine

    private var rootLayout: LinearLayout? = null
    private var candidateBar: CandidateBarView? = null
    private var panelContainer: FrameLayout? = null
    private var keyboardView: KeyboardCanvasView? = null
    private var emojiPanel: EmojiPanelView? = null
    private var clipboardPanel: ClipboardPanelView? = null

    private var activePanel: String = "keyboard" // "keyboard" | "emoji" | "clipboard"
    private var lastClipSignature: String = ""
    private val activeCapsules = ArrayList<SmartCapsule>(4)
    private var isPasswordField = false

    override fun onCreate() {
        super.onCreate()
        prefs = PrefsManager(this)
        dictReader = QDictReader.openFromAssets(this)
        userStore = UserDictStore(this)
        clipboardStore = ClipboardStore(this)
        engine = PinyinEngine(dictReader, userStore).apply {
            layoutMode = prefs.layoutMode
            fuzzyEnabled = prefs.fuzzyEnabled
            fuzzyRules = prefs.getFuzzyRules()
            emojiSuggestionEnabled = prefs.emojiSuggestionEnabled
        }
        SmartCapsuleBus.setListener { capsule ->
            rootLayout?.post {
                activeCapsules.removeAll { it.type == capsule.type || it.isExpired() }
                activeCapsules.add(0, capsule)
                refreshIdleBarIfNeeded()
            }
        }
    }

    override fun onCreateInputView(): View {
        val root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        val bar = CandidateBarView(this).apply {
            listener = object : CandidateBarView.Listener {
                override fun onCandidateClick(cand: Candidate, index: Int) = commitCandidate(cand, index)
                override fun onCandidateLongClick(cand: Candidate) {
                    if (cand.source == "pinyin" || cand.source == "t9" || cand.source == "english") {
                        val src = if (cand.source == "t9") "pinyin" else cand.source
                        userStore.pinWord(src, cand.code, cand.text, true)
                        Toast.makeText(this@QinShuImeService, "已置顶常用词：${cand.text}", Toast.LENGTH_SHORT).show()
                        updateCandidatesFromEngine()
                    }
                }
                override fun onCapsuleClick(capsule: SmartCapsule) {
                    currentInputConnection?.commitText(capsule.text, 1)
                    activeCapsules.remove(capsule)
                    if (capsule.type == CapsuleType.OTP) SmartCapsuleBus.clearOtp()
                    refreshIdleBarIfNeeded()
                }
                override fun onOpenEmoji() = toggleOverlayPanel("emoji")
                override fun onOpenClipboard() = toggleOverlayPanel("clipboard")
                override fun onToggleLayout() = toggleLayoutMode()
                override fun onOpenSettings() {
                    startActivity(
                        Intent(this@QinShuImeService, SettingsActivity::class.java).apply {
                            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                        },
                    )
                }
            }
        }
        val container = FrameLayout(this)
        val kb = KeyboardCanvasView(this).apply {
            hapticEnabled = prefs.hapticEnabled
            listener = object : KeyboardCanvasView.Listener {
                override fun onChar(ch: Char) = handleCharInput(ch)
                override fun onBackspace() = handleBackspace()
                override fun onClearBuffer() {
                    engine.clear()
                    currentInputConnection?.setComposingText("", 1)
                    updateCandidatesFromEngine()
                }
                override fun onEnter() = handleEnter()
                override fun onSpace() = handleSpace()
                override fun onMoveCursor(delta: Int) {
                    val key = if (delta > 0) KeyEvent.KEYCODE_DPAD_RIGHT else KeyEvent.KEYCODE_DPAD_LEFT
                    sendDownUpKeyEvents(key)
                }
                override fun onToggleLanguage() = toggleLanguageMode()
                override fun onToggleLayout() = toggleLayoutMode()
                override fun onToggleEmojiPanel() = toggleOverlayPanel("emoji")
                override fun onSelectT9Syllable(syl: String) {
                    engine.lockT9Syllable(syl)
                    updateCandidatesFromEngine()
                }
                override fun onExitOneHanded() {
                    prefs.oneHandedMode = OneHandedMode.OFF
                    applyDisplayPrefs()
                }
                override fun onNumPageChanged(isNumPage: Boolean) {
                    if (engine.hasBuffer()) {
                        if (isNumPage) {
                            val top = engine.query().candidates.firstOrNull()
                            if (top != null) commitCandidate(top, 0) else commitRawBuffer()
                        } else {
                            commitRawBuffer()
                        }
                    }
                    engine.numMode = isNumPage
                }
            }
        }
        container.addView(kb)
        root.addView(bar)
        root.addView(container)

        rootLayout = root
        candidateBar = bar
        panelContainer = container
        keyboardView = kb
        applyCurrentTheme()
        applyDisplayPrefs()
        kb.setState(engine.layoutMode, engine.languageMode)
        return root
    }

    override fun onStartInputView(info: EditorInfo?, restarting: Boolean) {
        super.onStartInputView(info, restarting)
        engine.layoutMode = prefs.layoutMode
        engine.fuzzyEnabled = prefs.fuzzyEnabled
        engine.fuzzyRules = prefs.getFuzzyRules()
        engine.emojiSuggestionEnabled = prefs.emojiSuggestionEnabled
        engine.numMode = false
        keyboardView?.hapticEnabled = prefs.hapticEnabled

        val variation = (info?.inputType ?: 0) and InputType.TYPE_MASK_VARIATION
        isPasswordField = variation == InputType.TYPE_TEXT_VARIATION_PASSWORD ||
            variation == InputType.TYPE_TEXT_VARIATION_VISIBLE_PASSWORD ||
            variation == InputType.TYPE_TEXT_VARIATION_WEB_PASSWORD

        showKeyboardOnly()
        applyCurrentTheme()
        applyDisplayPrefs()
        keyboardView?.setState(engine.layoutMode, engine.languageMode)
        inspectSmartSources()
        refreshIdleBarIfNeeded()
    }

    override fun onFinishInputView(finishingInput: Boolean) {
        super.onFinishInputView(finishingInput)
        engine.clear()
        releaseOverlayPanels()
    }

    override fun onTrimMemory(level: Int) {
        super.onTrimMemory(level)
        if (level >= TRIM_MEMORY_UI_HIDDEN) {
            releaseOverlayPanels()
            userStore.trimCache()
        }
    }

    override fun onDestroy() {
        SmartCapsuleBus.setListener(null)
        dictReader.close()
        userStore.close()
        super.onDestroy()
    }

    private fun inspectSmartSources() {
        activeCapsules.removeAll { it.isExpired() }
        if (isPasswordField) return

        SmartCapsuleBus.consumeActiveOtp()?.let { otpCap ->
            if (activeCapsules.none { it.text == otpCap.text }) activeCapsules.add(0, otpCap)
        }

        if (prefs.clipboardSmartEnabled) {
            val cm = getSystemService(Context.CLIPBOARD_SERVICE) as? ClipboardManager ?: return
            val clipText = cm.primaryClip?.takeIf { it.itemCount > 0 }?.getItemAt(0)?.coerceToText(this)?.toString().orEmpty()
            if (clipText.isNotBlank() && clipText != lastClipSignature) {
                lastClipSignature = clipText
                clipboardStore.add(clipText)
                SmartRecognizer.extractClipboardCapsules(clipText).forEach { cap ->
                    if (activeCapsules.none { it.text == cap.text }) activeCapsules.add(0, cap)
                }
                while (activeCapsules.size > 4) activeCapsules.removeAt(activeCapsules.lastIndex)
            }
        }
    }

    private fun handleCharInput(ch: Char) {
        if (isPasswordField) {
            currentInputConnection?.commitText(ch.toString(), 1)
            return
        }
        val isDirectPunct = ch in "，。？！、；：," || (
            ch == '.' &&
                !engine.numMode &&
                !engine.isSmartInputActive() &&
                !engine.getBuffer().equals("www", ignoreCase = true)
            )
        if (isDirectPunct) {
            if (engine.hasBuffer()) {
                val top = engine.query().candidates.firstOrNull()
                if (top != null) commitCandidate(top, 0) else commitRawBuffer()
            }
            currentInputConnection?.commitText(ch.toString(), 1)
            return
        }
        if (ch == '@' && !engine.hasBuffer()) {
            currentInputConnection?.commitText("@", 1)
            return
        }
        val res = engine.appendChar(ch)
        currentInputConnection?.setComposingText(res.composingDisplay.ifEmpty { engine.getBuffer() }, 1)
        candidateBar?.showCandidates(res.candidates, res.composingDisplay)
        keyboardView?.setT9Syllables(res.t9Syllables)
    }

    private fun handleBackspace() {
        if (engine.hasBuffer()) {
            val res = engine.backspace()
            if (engine.hasBuffer()) {
                currentInputConnection?.setComposingText(res.composingDisplay.ifEmpty { engine.getBuffer() }, 1)
                candidateBar?.showCandidates(res.candidates, res.composingDisplay)
                keyboardView?.setT9Syllables(res.t9Syllables)
            } else {
                currentInputConnection?.setComposingText("", 1)
                keyboardView?.setT9Syllables(emptyList())
                refreshIdleBarIfNeeded()
            }
        } else {
            currentInputConnection?.deleteSurroundingText(1, 0)
        }
    }

    private fun handleSpace() {
        if (engine.hasBuffer()) {
            val top = engine.query().candidates.firstOrNull()
            if (top != null) commitCandidate(top, 0) else commitRawBuffer()
        } else {
            currentInputConnection?.commitText(" ", 1)
        }
    }

    private fun handleEnter() {
        if (engine.hasBuffer()) commitRawBuffer() else sendDownUpKeyEvents(KeyEvent.KEYCODE_ENTER)
    }

    private fun commitRawBuffer() {
        val raw = engine.getBuffer()
        engine.clear()
        currentInputConnection?.commitText(raw, 1)
        keyboardView?.setT9Syllables(emptyList())
        refreshIdleBarIfNeeded()
    }

    private fun commitCandidate(cand: Candidate, index: Int) {
        val commit = if (cand.source == "english" && engine.languageMode == LanguageMode.EN) "${cand.text} " else cand.text
        currentInputConnection?.commitText(commit, 1)
        engine.onCandidateSelected(cand, index)
        keyboardView?.setT9Syllables(emptyList())
        refreshIdleBarIfNeeded()
    }

    private fun updateCandidatesFromEngine() {
        if (!engine.hasBuffer()) {
            keyboardView?.setT9Syllables(emptyList())
            refreshIdleBarIfNeeded()
            return
        }
        val res = engine.query()
        currentInputConnection?.setComposingText(res.composingDisplay.ifEmpty { engine.getBuffer() }, 1)
        candidateBar?.showCandidates(res.candidates, res.composingDisplay)
        keyboardView?.setT9Syllables(res.t9Syllables)
    }

    private fun toggleLanguageMode() {
        if (engine.hasBuffer()) commitRawBuffer()
        engine.languageMode = if (engine.languageMode == LanguageMode.ZH) LanguageMode.EN else LanguageMode.ZH
        keyboardView?.setState(engine.layoutMode, engine.languageMode)
    }

    private fun toggleLayoutMode() {
        if (engine.hasBuffer()) commitRawBuffer()
        engine.layoutMode = if (engine.layoutMode == LayoutMode.QWERTY) LayoutMode.T9 else LayoutMode.QWERTY
        prefs.layoutMode = engine.layoutMode
        keyboardView?.setState(engine.layoutMode, engine.languageMode)
    }

    private fun toggleOverlayPanel(target: String) {
        if (activePanel == target) {
            showKeyboardOnly()
            return
        }
        val container = panelContainer ?: return
        keyboardView?.visibility = View.GONE
        emojiPanel?.visibility = View.GONE
        clipboardPanel?.visibility = View.GONE

        if (target == "emoji") {
            val panel = emojiPanel ?: EmojiPanelView(this, dictReader, prefs).apply {
                applyTheme(currentTheme())
                listener = object : EmojiPanelView.Listener {
                    override fun onEmojiSelected(emoji: String) {
                        currentInputConnection?.commitText(emoji, 1)
                    }
                    override fun onClose() = showKeyboardOnly()
                    override fun onBackspace() = handleBackspace()
                }
                emojiPanel = this
            }
            if (panel.parent == null) container.addView(panel)
            panel.visibility = View.VISIBLE
        } else if (target == "clipboard") {
            val panel = clipboardPanel ?: ClipboardPanelView(this).apply {
                applyTheme(currentTheme())
                listener = object : ClipboardPanelView.Listener {
                    override fun onPaste(text: String) {
                        currentInputConnection?.commitText(text, 1)
                        showKeyboardOnly()
                    }
                    override fun onTogglePin(text: String) {
                        clipboardStore.togglePin(text)
                        setItems(clipboardStore.getAll())
                    }
                    override fun onClearUnpinned() {
                        clipboardStore.clearUnpinned()
                        setItems(clipboardStore.getAll())
                    }
                    override fun onClose() = showKeyboardOnly()
                }
                clipboardPanel = this
            }
            panel.setItems(if (isPasswordField) emptyList() else clipboardStore.getAll())
            if (panel.parent == null) container.addView(panel)
            panel.visibility = View.VISIBLE
        }
        activePanel = target
    }

    private fun showKeyboardOnly() {
        emojiPanel?.visibility = View.GONE
        clipboardPanel?.visibility = View.GONE
        keyboardView?.visibility = View.VISIBLE
        activePanel = "keyboard"
    }

    private fun releaseOverlayPanels() {
        activePanel = "keyboard"
        emojiPanel?.let { (it.parent as? FrameLayout)?.removeView(it) }
        clipboardPanel?.let { (it.parent as? FrameLayout)?.removeView(it) }
        emojiPanel = null
        clipboardPanel = null
        keyboardView?.visibility = View.VISIBLE
    }

    private fun refreshIdleBarIfNeeded() {
        if (!engine.hasBuffer()) {
            activeCapsules.removeAll { it.isExpired() }
            candidateBar?.showIdleBar(if (isPasswordField) emptyList() else activeCapsules)
        }
    }

    private fun currentTheme() = prefs.currentTheme(
        (resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES,
    )

    private fun applyCurrentTheme() {
        val t = currentTheme()
        candidateBar?.applyTheme(t)
        keyboardView?.applyTheme(t)
        emojiPanel?.applyTheme(t)
        clipboardPanel?.applyTheme(t)
    }

    private fun applyDisplayPrefs() {
        keyboardView?.applyDisplayPrefs(
            prefs.keyboardHeightPercent,
            prefs.oneHandedMode,
            prefs.keyPreviewEnabled,
        )
    }
}
