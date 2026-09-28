/**
 * 图片导出：把界面中的某个容器渲染成 PNG，并交给 Go 侧弹出保存对话框。
 *
 * 使用 html2canvas-pro（支持 Tailwind v4 输出的 oklch 颜色），按需动态加载以减小首包体积。
 */
import { SaveImage } from '../../wailsjs/go/main/App'

/** 导出结果：saved 为 true 时 path 是保存路径；用户取消时 saved 为 false。 */
export interface ExportImageResult {
    saved: boolean
    path: string
}

/** 把 DOM 元素导出为 PNG 图片。 */
export async function exportElementAsImage(
    element: HTMLElement,
    defaultName: string,
): Promise<ExportImageResult> {
    const { default: html2canvas } = await import('html2canvas-pro')
    const canvas = await html2canvas(element, {
        // 与暗色主题一致的背景色，避免透明边角
        backgroundColor: '#020617',
        scale: Math.max(2, Math.min(window.devicePixelRatio || 1, 3)),
        useCORS: true,
        logging: false,
    })

    const base64 = canvas.toDataURL('image/png').replace(/^data:image\/png;base64,/, '')
    const path = await SaveImage(base64, defaultName)
    return { saved: path !== '', path }
}

/** 生成带时间戳的导出文件名。 */
export function imageExportName(prefix: string): string {
    const now = new Date()
    const pad = (value: number) => String(value).padStart(2, '0')
    const stamp = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}`
    return `${prefix}_${stamp}.png`
}