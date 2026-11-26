package com.qinshu.ime

import android.app.Activity
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.provider.Settings
import android.text.InputType
import android.view.inputmethod.InputMethodManager
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.Switch
import android.widget.TextView
import android.widget.Toast
import com.qinshu.ime.config.OneHandedMode
import com.qinshu.ime.config.PrefsManager
import com.qinshu.ime.config.QinTheme
import com.qinshu.ime.engine.LayoutMode
import com.qinshu.ime.engine.UserDictStore
import com.qinshu.ime.smart.WebDavSyncManager
import kotlin.concurrent.thread

/**
 * 极简独立设置页（v1.1 升级版：新增键盘高度、单手模式、按键气泡预览与 NAS/WebDAV 双端同步）。
 */
class SettingsActivity : Activity() {

    private lateinit var prefs: PrefsManager
    private lateinit var userStore: UserDictStore

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        prefs = PrefsManager(this)
        userStore = UserDictStore(this)

        val verName = runCatching {
            packageManager.getPackageInfo(packageName, 0).versionName ?: "1.0.0"
        }.getOrDefault("1.0.0")

        val d = resources.displayMetrics.density
        val pad = (20 * d).toInt()
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(pad, pad, pad, pad)
        }

        root.addView(TextView(this).apply {
            text = "Qin书输入法 v$verName"
            textSize = 22f
            setPadding(0, 0, 0, (12 * d).toInt())
        })

        // 1. 启用与权限引导
        root.addView(Button(this).apply {
            text = "1. 在系统设置中启用「Qin书输入法」"
            setOnClickListener { startActivity(Intent(Settings.ACTION_INPUT_METHOD_SETTINGS)) }
        })
        root.addView(Button(this).apply {
            text = "2. 切换当前输入法为「Qin书」"
            setOnClickListener {
                (getSystemService(INPUT_METHOD_SERVICE) as InputMethodManager).showInputMethodPicker()
            }
        })
        root.addView(Button(this).apply {
            text = "3. 开启「短信验证码」通知智能捕获（零短信权限）"
            setOnClickListener { startActivity(Intent(Settings.ACTION_NOTIFICATION_LISTENER_SETTINGS)) }
        })

        // 2. 核心功能开关
        root.addView(Switch(this).apply {
            text = "默认使用九宫格键盘（关闭则为 26 键全键盘）"
            isChecked = prefs.layoutMode == LayoutMode.T9
            setOnCheckedChangeListener { _, c -> prefs.layoutMode = if (c) LayoutMode.T9 else LayoutMode.QWERTY }
        })
        root.addView(Switch(this).apply {
            text = "开启拼音模糊音（平翘舌 / 前后鼻音 / l-n）"
            isChecked = prefs.fuzzyEnabled
            setOnCheckedChangeListener { _, c -> prefs.fuzzyEnabled = c }
        })
        root.addView(Switch(this).apply {
            text = "拼音候选栏智能联想微表情（Emoji）"
            isChecked = prefs.emojiSuggestionEnabled
            setOnCheckedChangeListener { _, c -> prefs.emojiSuggestionEnabled = c }
        })
        root.addView(Switch(this).apply {
            text = "剪贴板智能提取（验证码 / 网址 / Email / 手机 / 单号）"
            isChecked = prefs.clipboardSmartEnabled
            setOnCheckedChangeListener { _, c -> prefs.clipboardSmartEnabled = c }
        })
        root.addView(Switch(this).apply {
            text = "按键放大气泡预览"
            isChecked = prefs.keyPreviewEnabled
            setOnCheckedChangeListener { _, c -> prefs.keyPreviewEnabled = c }
        })
        root.addView(Switch(this).apply {
            text = "按键轻触震动反馈"
            isChecked = prefs.hapticEnabled
            setOnCheckedChangeListener { _, c -> prefs.hapticEnabled = c }
        })

        // 3. 键盘高度与单手模式
        root.addView(sectionTitle("键盘高度与单手模式", d))
        val heightRow = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL }
        listOf("紧凑 88%" to 88, "标准 100%" to 100, "大键 115%" to 115).forEach { (lbl, pct) ->
            heightRow.addView(Button(this).apply {
                text = lbl
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                setOnClickListener {
                    prefs.keyboardHeightPercent = pct
                    Toast.makeText(this@SettingsActivity, "键盘高度已设为 $lbl", Toast.LENGTH_SHORT).show()
                }
            })
        }
        root.addView(heightRow)

        val handRow = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL }
        listOf("全宽" to OneHandedMode.OFF, "左手模式" to OneHandedMode.LEFT, "右手模式" to OneHandedMode.RIGHT).forEach { (lbl, mode) ->
            handRow.addView(Button(this).apply {
                text = lbl
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                setOnClickListener {
                    prefs.oneHandedMode = mode
                    Toast.makeText(this@SettingsActivity, "已切换：$lbl", Toast.LENGTH_SHORT).show()
                }
            })
        }
        root.addView(handRow)

        // 4. 主题切换
        root.addView(sectionTitle("键盘主题（与桌面版三套配色同步）", d))
        for (theme in QinTheme.entries) {
            root.addView(Button(this).apply {
                text = "切换主题：${theme.displayName}"
                setOnClickListener {
                    prefs.themeId = theme.id
                    Toast.makeText(this@SettingsActivity, "已切换为 ${theme.displayName}", Toast.LENGTH_SHORT).show()
                }
            })
        }

        // 5. NAS / WebDAV 双端词频双向同步
        root.addView(sectionTitle("NAS / WebDAV 桌面与手机词频同步（可选）", d))
        val urlInput = EditText(this).apply {
            hint = "WebDAV 目录地址，如 http://192.168.1.10:5005/qinshu"
            setText(prefs.webdavUrl)
        }
        val userInput = EditText(this).apply {
            hint = "WebDAV 用户名"
            setText(prefs.webdavUser)
        }
        val passInput = EditText(this).apply {
            hint = "WebDAV 密码"
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
            setText(prefs.webdavPass)
        }
        root.addView(urlInput)
        root.addView(userInput)
        root.addView(passInput)
        root.addView(Button(this).apply {
            text = "保存配置并立即双向同步 user_dict.json"
            setOnClickListener {
                prefs.webdavUrl = urlInput.text.toString()
                prefs.webdavUser = userInput.text.toString()
                prefs.webdavPass = passInput.text.toString()
                Toast.makeText(this@SettingsActivity, "正在连接 WebDAV 同步…", Toast.LENGTH_SHORT).show()
                thread {
                    val res = WebDavSyncManager.syncUserDict(prefs, userStore)
                    runOnUiThread {
                        Toast.makeText(this@SettingsActivity, res.message, Toast.LENGTH_LONG).show()
                    }
                }
            }
        })

        // 6. 本地文件手动导入桌面词频
        root.addView(Button(this).apply {
            text = "从本地文件手动导入桌面版 user_dict.json"
            setOnClickListener {
                val intent = Intent(Intent.ACTION_OPEN_DOCUMENT).apply {
                    addCategory(Intent.CATEGORY_OPENABLE)
                    type = "*/*"
                }
                startActivityForResult(intent, REQ_IMPORT_USER_DICT)
            }
        })

        setContentView(ScrollView(this).apply { addView(root) })
    }

    private fun sectionTitle(title: String, d: Float): TextView = TextView(this).apply {
        text = title
        textSize = 15f
        setPadding(0, (16 * d).toInt(), 0, (6 * d).toInt())
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == REQ_IMPORT_USER_DICT && resultCode == RESULT_OK) {
            val uri: Uri = data?.data ?: return
            runCatching {
                contentResolver.openInputStream(uri)?.bufferedReader()?.use { it.readText() } ?: ""
            }.onSuccess { text ->
                val count = userStore.importDesktopUserDictJson(text)
                Toast.makeText(this, "已成功导入 $count 条桌面高频词条！", Toast.LENGTH_LONG).show()
            }.onFailure { e ->
                Toast.makeText(this, "导入失败：${e.message}", Toast.LENGTH_LONG).show()
            }
        }
    }

    override fun onDestroy() {
        userStore.close()
        super.onDestroy()
    }

    companion object {
        private const val REQ_IMPORT_USER_DICT = 1001
    }
}
