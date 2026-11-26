package com.qinshu.ime.smart

import android.app.Notification
import android.service.notification.NotificationListenerService
import android.service.notification.StatusBarNotification
import com.qinshu.ime.engine.CapsuleType
import com.qinshu.ime.engine.SmartCapsule
import com.qinshu.ime.engine.SmartRecognizer

/**
 * 零短信权限的短信验证码监听服务（NotificationListenerService）。
 * 仅在用户主动授予「通知使用权」时启用，纯本地提取系统短信通知中的 4-8 位验证码，
 * 通过内存总线推送给当前激活的 Qin书输入法候选胶囊，3 分钟自动过期。
 */
class OtpNotificationListener : NotificationListenerService() {

    override fun onNotificationPosted(sbn: StatusBarNotification?) {
        val notification = sbn?.notification ?: return
        val pkg = sbn.packageName.orEmpty().lowercase()
        // 仅过滤短信类通知，降低唤醒与内存消耗
        if (!isLikelySmsPackage(pkg)) return

        val extras = notification.extras ?: return
        val title = extras.getCharSequence(Notification.EXTRA_TITLE)?.toString().orEmpty()
        val text = extras.getCharSequence(Notification.EXTRA_TEXT)?.toString().orEmpty()
        val bigText = extras.getCharSequence(Notification.EXTRA_BIG_TEXT)?.toString().orEmpty()
        val combined = "$title $bigText $text".trim()

        SmartRecognizer.extractOtp(combined)?.let { code ->
            SmartCapsuleBus.post(
                SmartCapsule(
                    type = CapsuleType.OTP,
                    label = "短信验证码 $code",
                    text = code,
                    ttlMs = 180_000L,
                ),
            )
        }
    }

    private fun isLikelySmsPackage(pkg: String): Boolean =
        pkg.contains("mms") ||
            pkg.contains("messaging") ||
            pkg.contains("sms") ||
            pkg.contains("message") ||
            pkg == "com.google.android.apps.messaging"
}

/**
 * 进程内超轻量智能胶囊共享槽（零持久化、自动 TTL 过期）。
 */
object SmartCapsuleBus {
    @Volatile
    private var latestOtp: SmartCapsule? = null

    @Volatile
    private var listener: ((SmartCapsule) -> Unit)? = null

    fun post(capsule: SmartCapsule) {
        if (capsule.type == CapsuleType.OTP) {
            latestOtp = capsule
        }
        listener?.invoke(capsule)
    }

    fun consumeActiveOtp(): SmartCapsule? {
        val cur = latestOtp ?: return null
        if (cur.isExpired()) {
            latestOtp = null
            return null
        }
        return cur
    }

    fun clearOtp() {
        latestOtp = null
    }

    fun setListener(cb: ((SmartCapsule) -> Unit)?) {
        listener = cb
    }
}
