/**
 * Markdown 渲染：用 marked 把 AI 报告 / 导入报告转成 HTML，
 * 再用 DOMPurify 净化，避免报告内容里的脚本在界面中执行。
 */
import DOMPurify from 'dompurify'
import { marked } from 'marked'

marked.setOptions({ gfm: true, breaks: true })

/** 把 Markdown 渲染为可安全插入 DOM 的 HTML。 */
export function renderMarkdown(source: string): string {
    if (!source || !source.trim()) {
        return ''
    }
    const html = marked.parse(source, { async: false })
    return DOMPurify.sanitize(html, { USE_PROFILES: { html: true } })
}

/** 去掉 Markdown 标记，得到用于摘要展示的纯文本。 */
export function markdownToPlainText(source: string, maxLength = 120): string {
    const plain = (source ?? '')
        .replace(/```[\s\S]*?```/g, ' ')
        .replace(/`([^`]*)`/g, '$1')
        .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
        .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
        .replace(/^\s{0,3}#{1,6}\s*/gm, '')
        .replace(/^\s{0,3}[-*+]\s+/gm, '')
        .replace(/[*_~>]/g, '')
        .replace(/\s+/g, ' ')
        .trim()

    if (plain.length <= maxLength) {
        return plain
    }
    return `${plain.slice(0, maxLength)}…`
}