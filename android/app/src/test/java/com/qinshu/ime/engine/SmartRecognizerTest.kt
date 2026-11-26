package com.qinshu.ime.engine

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class SmartRecognizerTest {

    @Test
    fun testArithmeticCandidatesSimple() {
        val cands = SmartRecognizer.evaluateExpressionCandidates("11+12")
        assertTrue(cands.size >= 2)
        assertEquals("23", cands[0].text)
        assertEquals("11+12=23", cands[1].text)
    }

    @Test
    fun testArithmeticPercentageAndComplex() {
        val pct = SmartRecognizer.evaluateExpressionCandidates("200*15%")
        assertEquals("30", pct[0].text)
        assertEquals("200*15%=30", pct[1].text)

        val cands = SmartRecognizer.evaluateExpressionCandidates("(10+5)*2-4/2")
        assertEquals("28", cands[0].text)
    }

    @Test
    fun testRmbUppercase() {
        val cands = SmartRecognizer.evaluateExpressionCandidates("=1280.5")
        assertEquals("壹仟贰佰捌拾元伍角整", cands[0].text)
    }

    @Test
    fun testArithmeticIncompleteIgnored() {
        assertTrue(SmartRecognizer.evaluateExpressionCandidates("11+").isEmpty())
        assertTrue(SmartRecognizer.evaluateExpressionCandidates("12345").isEmpty())
        assertTrue(SmartRecognizer.evaluateExpressionCandidates("10/0").isEmpty())
    }

    @Test
    fun testOtpExtraction() {
        assertEquals("482910", SmartRecognizer.extractOtp("【某某平台】您的登录验证码为：482910，5分钟内有效。"))
        assertEquals("6391", SmartRecognizer.extractOtp("Your verification code is 6391."))
        assertNull(SmartRecognizer.extractOtp("2026年11月26日会议纪要"))
    }

    @Test
    fun testClipboardCapsulesAllTypes() {
        val caps = SmartRecognizer.extractClipboardCapsules(
            "请联系 13812345678 或发邮件 test.user@qq.com，网址 https://github.com/qwwei1018/qinshu",
        )
        assertTrue(caps.any { it.type == CapsuleType.EMAIL && it.text == "test.user@qq.com" })
        assertTrue(caps.any { it.type == CapsuleType.URL && it.text == "https://github.com/qwwei1018/qinshu" })
        assertTrue(caps.any { it.type == CapsuleType.PHONE && it.text == "13812345678" })

        val trackCaps = SmartRecognizer.extractClipboardCapsules("顺丰快递单号 SF1234567890123 已签收")
        assertTrue(trackCaps.any { it.type == CapsuleType.TRACKING && it.text == "SF1234567890123" })
    }

    @Test
    fun testSmartEmailAndUrlCompletion() {
        val emailCands = SmartRecognizer.querySmartEmail("qinshu@q")
        assertTrue(emailCands.any { it.text == "qinshu@qq.com" })

        val urlCands = SmartRecognizer.querySmartUrl("https://github")
        assertTrue(urlCands.any { it.text == "https://github.com" })
    }
}
