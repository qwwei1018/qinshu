package com.qinshu.ime.ui

import android.graphics.Canvas
import android.graphics.ColorFilter
import android.graphics.Paint
import android.graphics.Path
import android.graphics.PixelFormat
import android.graphics.RectF
import android.graphics.drawable.Drawable
import android.text.SpannableString
import android.text.Spanned
import android.text.style.ImageSpan

/**
 * 零图片资源包的矢量微表情渲染器：
 * 为 `[捂脸]`、`[抱拳]`、`[破涕为笑]`、`[偷笑]`、`[憨笑]`、`[玫瑰]`、`[合十]`、`[强]`
 * 等微信常用表情标签提供高清 Canvas 矢量图案渲染，点选上屏仍提交微信识别的 `[标签]` 文本。
 */
class WeChatEmojiDrawable(private val tag: String) : Drawable() {

    private val fillPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply { style = Paint.Style.FILL }
    private val strokePaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.STROKE
        strokeCap = Paint.Cap.ROUND
        strokeJoin = Paint.Join.ROUND
    }
    private val path = Path()
    private val rect = RectF()

    override fun draw(canvas: Canvas) {
        val w = bounds.width().toFloat()
        val h = bounds.height().toFloat()
        if (w <= 0f || h <= 0f) return

        canvas.save()
        canvas.translate(bounds.left.toFloat(), bounds.top.toFloat())
        canvas.scale(w / 100f, h / 100f)

        when (tag) {
            "[捂脸]" -> drawWulian(canvas)
            "[抱拳]" -> drawBaoquan(canvas)
            "[破涕为笑]" -> drawPotiweixiao(canvas)
            "[偷笑]" -> drawTouxiao(canvas)
            "[憨笑]" -> drawHanxiao(canvas)
            "[玫瑰]" -> drawMeigui(canvas)
            "[合十]" -> drawHeshi(canvas)
            "[强]" -> drawQiang(canvas)
        }

        canvas.restore()
    }

    private fun drawBaseFace(canvas: Canvas, cx: Float = 50f, cy: Float = 50f, r: Float = 40f) {
        fillPaint.color = 0xFFFFD53D.toInt()
        canvas.drawCircle(cx, cy, r, fillPaint)
        // 顶部高光柔化，增强立体感
        fillPaint.color = 0x40FFFBEB
        canvas.drawCircle(cx - r * 0.18f, cy - r * 0.25f, r * 0.55f, fillPaint)
        strokePaint.color = 0xFFD97706.toInt()
        strokePaint.strokeWidth = 3.2f
        canvas.drawCircle(cx, cy, r, strokePaint)
    }

    /** [捂脸]：哭笑捂脸（黄色圆脸 + 蓝泪流 + 笑嘴 + 右手捂半边脸） */
    private fun drawWulian(canvas: Canvas) {
        drawBaseFace(canvas, 48f, 50f, 39f)

        // 左眼弯弧（笑哭眯眼）
        strokePaint.color = 0xFF78350F.toInt()
        strokePaint.strokeWidth = 3.6f
        path.reset()
        path.moveTo(23f, 36f)
        path.quadTo(35f, 41f, 25f, 46f)
        canvas.drawPath(path, strokePaint)

        // 张嘴大笑 + 白色上牙
        path.reset()
        path.moveTo(26f, 56f)
        path.quadTo(46f, 52f, 66f, 58f)
        path.quadTo(56f, 79f, 32f, 74f)
        path.close()
        fillPaint.color = 0xFF7C2D12.toInt()
        canvas.drawPath(path, fillPaint)

        path.reset()
        path.moveTo(29f, 57f)
        path.quadTo(46f, 54f, 62f, 59f)
        path.lineTo(59f, 64f)
        path.quadTo(45f, 61f, 31f, 63f)
        path.close()
        fillPaint.color = 0xFFFFFFFF.toInt()
        canvas.drawPath(path, fillPaint)

        // 左侧蓝色眼泪条
        path.reset()
        path.moveTo(27f, 44f)
        path.lineTo(15f, 66f)
        path.lineTo(22f, 70f)
        path.lineTo(33f, 48f)
        path.close()
        fillPaint.color = 0xFF38BDF8.toInt()
        canvas.drawPath(path, fillPaint)
        strokePaint.color = 0xFF0284C7.toInt()
        strokePaint.strokeWidth = 1.8f
        canvas.drawPath(path, strokePaint)

        // 右手斜捂脸部（金色手掌 + 四指轮廓）
        canvas.save()
        canvas.rotate(-18f, 68f, 52f)
        rect.set(48f, 24f, 84f, 74f)
        fillPaint.color = 0xFFFBBF24.toInt()
        canvas.drawRoundRect(rect, 14f, 14f, fillPaint)
        strokePaint.color = 0xFFB45309.toInt()
        strokePaint.strokeWidth = 3f
        canvas.drawRoundRect(rect, 14f, 14f, strokePaint)
        // 指缝分隔线
        strokePaint.strokeWidth = 2.2f
        canvas.drawLine(57f, 25f, 57f, 46f, strokePaint)
        canvas.drawLine(66f, 24f, 66f, 47f, strokePaint)
        canvas.drawLine(75f, 25f, 75f, 46f, strokePaint)
        canvas.restore()
    }

    /** [抱拳]：左拳右掌抱拳礼 */
    private fun drawBaoquan(canvas: Canvas) {
        // 右手斜立掌（从左上斜向右下）
        canvas.save()
        canvas.rotate(-34f, 62f, 48f)
        rect.set(49f, 12f, 73f, 82f)
        fillPaint.color = 0xFFFBBF24.toInt()
        canvas.drawRoundRect(rect, 12f, 12f, fillPaint)
        strokePaint.color = 0xFFB45309.toInt()
        strokePaint.strokeWidth = 3.2f
        canvas.drawRoundRect(rect, 12f, 12f, strokePaint)
        // 掌缘指节微线
        strokePaint.strokeWidth = 2.2f
        canvas.drawLine(55f, 22f, 67f, 22f, strokePaint)
        canvas.drawLine(55f, 34f, 67f, 34f, strokePaint)
        canvas.restore()

        // 左手抱拳（贴合于立掌左侧）
        rect.set(14f, 42f, 56f, 78f)
        fillPaint.color = 0xFFFCD34D.toInt()
        canvas.drawRoundRect(rect, 16f, 16f, fillPaint)
        strokePaint.color = 0xFFB45309.toInt()
        strokePaint.strokeWidth = 3.2f
        canvas.drawRoundRect(rect, 16f, 16f, strokePaint)

        // 弯曲拇指与握拳指缝
        rect.set(27f, 46f, 50f, 64f)
        fillPaint.color = 0xFFFBBF24.toInt()
        canvas.drawRoundRect(rect, 9f, 9f, fillPaint)
        strokePaint.strokeWidth = 2.6f
        canvas.drawRoundRect(rect, 9f, 9f, strokePaint)
        canvas.drawArc(RectF(22f, 60f, 48f, 75f), 10f, 150f, false, strokePaint)
    }

    /** [破涕为笑]：斜脸笑哭 + 左右飞溅泪滴 */
    private fun drawPotiweixiao(canvas: Canvas) {
        canvas.save()
        canvas.rotate(-12f, 50f, 50f)
        drawBaseFace(canvas, 50f, 50f, 36f)

        strokePaint.color = 0xFF78350F.toInt()
        strokePaint.strokeWidth = 3.4f
        path.reset()
        path.moveTo(28f, 42f); path.quadTo(35f, 34f, 42f, 42f)
        path.moveTo(58f, 42f); path.quadTo(65f, 34f, 72f, 42f)
        canvas.drawPath(path, strokePaint)

        path.reset()
        path.moveTo(31f, 55f)
        path.quadTo(50f, 55f, 69f, 55f)
        path.quadTo(67f, 76f, 50f, 76f)
        path.quadTo(33f, 76f, 31f, 55f)
        fillPaint.color = 0xFF7C2D12.toInt()
        canvas.drawPath(path, fillPaint)
        fillPaint.color = 0xFFFFFFFF.toInt()
        canvas.drawRect(34f, 56f, 66f, 61f, fillPaint)
        canvas.restore()

        // 左右蓝色飞溅泪滴
        fillPaint.color = 0xFF38BDF8.toInt()
        canvas.drawCircle(16f, 56f, 7.5f, fillPaint)
        canvas.drawCircle(86f, 46f, 7.5f, fillPaint)
        strokePaint.color = 0xFF0284C7.toInt()
        strokePaint.strokeWidth = 2f
        canvas.drawCircle(16f, 56f, 7.5f, strokePaint)
        canvas.drawCircle(86f, 46f, 7.5f, strokePaint)
    }

    /** [偷笑]：红晕眯眼 + 小手遮嘴 */
    private fun drawTouxiao(canvas: Canvas) {
        drawBaseFace(canvas)
        // 腮红
        fillPaint.color = 0x70FB7185
        canvas.drawCircle(27f, 54f, 7f, fillPaint)
        canvas.drawCircle(73f, 54f, 7f, fillPaint)
        // 弯眼
        strokePaint.color = 0xFF78350F.toInt()
        strokePaint.strokeWidth = 3.6f
        path.reset()
        path.moveTo(27f, 41f); path.quadTo(34f, 33f, 41f, 41f)
        path.moveTo(59f, 41f); path.quadTo(66f, 33f, 73f, 41f)
        canvas.drawPath(path, strokePaint)
        // 嘴前小手
        rect.set(35f, 58f, 66f, 84f)
        fillPaint.color = 0xFFFBBF24.toInt()
        canvas.drawRoundRect(rect, 11f, 11f, fillPaint)
        strokePaint.color = 0xFFB45309.toInt()
        strokePaint.strokeWidth = 2.8f
        canvas.drawRoundRect(rect, 11f, 11f, strokePaint)
        canvas.drawLine(45f, 59f, 45f, 71f, strokePaint)
        canvas.drawLine(55f, 59f, 55f, 71f, strokePaint)
    }

    /** [憨笑]：眯眼红晕大笑 */
    private fun drawHanxiao(canvas: Canvas) {
        drawBaseFace(canvas)
        fillPaint.color = 0x70FB7185
        canvas.drawCircle(24f, 54f, 7.5f, fillPaint)
        canvas.drawCircle(76f, 54f, 7.5f, fillPaint)
        strokePaint.color = 0xFF78350F.toInt()
        strokePaint.strokeWidth = 3.6f
        path.reset()
        path.moveTo(25f, 41f); path.quadTo(34f, 31f, 43f, 41f)
        path.moveTo(57f, 41f); path.quadTo(66f, 31f, 75f, 41f)
        canvas.drawPath(path, strokePaint)
        path.reset()
        path.moveTo(28f, 56f)
        path.lineTo(72f, 56f)
        path.quadTo(68f, 80f, 50f, 80f)
        path.quadTo(32f, 80f, 28f, 56f)
        fillPaint.color = 0xFF7C2D12.toInt()
        canvas.drawPath(path, fillPaint)
        fillPaint.color = 0xFFFFFFFF.toInt()
        canvas.drawRect(32f, 57f, 68f, 63f, fillPaint)
    }

    /** [玫瑰]：红玫瑰花朵与绿叶 */
    private fun drawMeigui(canvas: Canvas) {
        strokePaint.color = 0xFF15803D.toInt()
        strokePaint.strokeWidth = 5f
        canvas.drawLine(50f, 48f, 50f, 90f, strokePaint)
        // 左右绿叶
        fillPaint.color = 0xFF22C55E.toInt()
        path.reset()
        path.moveTo(50f, 70f); path.quadTo(26f, 58f, 26f, 74f); path.quadTo(38f, 80f, 50f, 70f)
        path.moveTo(50f, 64f); path.quadTo(74f, 52f, 74f, 68f); path.quadTo(62f, 74f, 50f, 64f)
        canvas.drawPath(path, fillPaint)
        // 花瓣
        fillPaint.color = 0xFFEF4444.toInt()
        canvas.drawCircle(50f, 34f, 22f, fillPaint)
        strokePaint.color = 0xFFB91C1C.toInt()
        strokePaint.strokeWidth = 2.6f
        canvas.drawCircle(50f, 34f, 22f, strokePaint)
        canvas.drawArc(RectF(36f, 22f, 64f, 46f), 20f, 240f, false, strokePaint)
        canvas.drawArc(RectF(42f, 27f, 58f, 41f), 190f, 210f, false, strokePaint)
    }

    /** [合十]：双手合十 */
    private fun drawHeshi(canvas: Canvas) {
        fillPaint.color = 0xFFFBBF24.toInt()
        strokePaint.color = 0xFFB45309.toInt()
        strokePaint.strokeWidth = 3f
        // 左掌
        path.reset()
        path.moveTo(48f, 18f)
        path.lineTo(24f, 68f)
        path.lineTo(36f, 84f)
        path.lineTo(48f, 72f)
        path.close()
        canvas.drawPath(path, fillPaint)
        canvas.drawPath(path, strokePaint)
        // 右掌
        path.reset()
        path.moveTo(52f, 18f)
        path.lineTo(76f, 68f)
        path.lineTo(64f, 84f)
        path.lineTo(52f, 72f)
        path.close()
        canvas.drawPath(path, fillPaint)
        canvas.drawPath(path, strokePaint)
        // 顶部微光
        strokePaint.color = 0xFFF59E0B.toInt()
        strokePaint.strokeWidth = 2.8f
        canvas.drawLine(50f, 6f, 50f, 13f, strokePaint)
        canvas.drawLine(34f, 11f, 39f, 17f, strokePaint)
        canvas.drawLine(66f, 11f, 61f, 17f, strokePaint)
    }

    /** [强]：竖起大拇指点赞 */
    private fun drawQiang(canvas: Canvas) {
        // 蓝色袖口
        rect.set(12f, 48f, 28f, 82f)
        fillPaint.color = 0xFF2563EB.toInt()
        canvas.drawRoundRect(rect, 5f, 5f, fillPaint)
        // 竖起的大拇指与拳体
        path.reset()
        path.moveTo(28f, 50f)
        path.lineTo(44f, 34f)
        path.quadTo(48f, 14f, 58f, 16f)
        path.quadTo(65f, 18f, 61f, 40f)
        path.lineTo(80f, 40f)
        path.quadTo(88f, 40f, 86f, 52f)
        path.lineTo(82f, 76f)
        path.quadTo(80f, 84f, 70f, 84f)
        path.lineTo(28f, 82f)
        path.close()
        fillPaint.color = 0xFFFBBF24.toInt()
        canvas.drawPath(path, fillPaint)
        strokePaint.color = 0xFFB45309.toInt()
        strokePaint.strokeWidth = 3f
        canvas.drawPath(path, strokePaint)
        canvas.drawLine(68f, 52f, 84f, 52f, strokePaint)
        canvas.drawLine(68f, 64f, 83f, 64f, strokePaint)
    }

    override fun setAlpha(alpha: Int) {
        fillPaint.alpha = alpha
        strokePaint.alpha = alpha
    }

    override fun setColorFilter(colorFilter: ColorFilter?) {
        fillPaint.colorFilter = colorFilter
        strokePaint.colorFilter = colorFilter
    }

    @Deprecated("Deprecated in Java")
    override fun getOpacity(): Int = PixelFormat.TRANSLUCENT

    private class CenteredImageSpan(drawable: Drawable) : ImageSpan(drawable) {
        override fun getSize(
            paint: Paint,
            text: CharSequence,
            start: Int,
            end: Int,
            fm: Paint.FontMetricsInt?,
        ): Int {
            val rect = drawable.bounds
            if (fm != null) {
                val pfm = paint.fontMetricsInt
                val fontHeight = pfm.descent - pfm.ascent
                val drHeight = rect.height()
                val centerY = pfm.ascent + fontHeight / 2
                fm.ascent = centerY - drHeight / 2
                fm.top = fm.ascent
                fm.bottom = centerY + drHeight / 2
                fm.descent = fm.bottom
            }
            return rect.right
        }

        override fun draw(
            canvas: Canvas,
            text: CharSequence,
            start: Int,
            end: Int,
            x: Float,
            top: Int,
            y: Int,
            bottom: Int,
            paint: Paint,
        ) {
            val b = drawable
            canvas.save()
            val transY = top + (bottom - top - b.bounds.height()) / 2f
            canvas.translate(x, transY)
            b.draw(canvas)
            canvas.restore()
        }
    }

    companion object {
        private val supportedTags = setOf(
            "[捂脸]", "[抱拳]", "[破涕为笑]", "[偷笑]",
            "[憨笑]", "[玫瑰]", "[合十]", "[强]",
        )

        fun supports(tag: String): Boolean = supportedTags.contains(tag)

        fun toSpannable(tag: String, sizePx: Int): CharSequence {
            if (!supports(tag)) return tag
            val drawable = WeChatEmojiDrawable(tag).apply {
                setBounds(0, 0, sizePx, sizePx)
            }
            return SpannableString(" ").apply {
                setSpan(CenteredImageSpan(drawable), 0, 1, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE)
            }
        }
    }
}
