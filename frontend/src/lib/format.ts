/**
 * 通用格式化工具与主题常量。
 */

/** 分类 / 标签可选的颜色预设。 */
export const COLOR_PRESETS = [
    '#ef4444',
    '#f97316',
    '#eab308',
    '#22c55e',
    '#14b8a6',
    '#0ea5e9',
    '#6366f1',
    '#a855f7',
    '#ec4899',
    '#a3a3a3',
]

/** 把 Go 的 time.Time（序列化为字符串）格式化为「年-月-日 时:分」。 */
export function formatDateTime(value: unknown): string {
    if (!value) {
        return '—'
    }
    const date = new Date(value as string)
    if (Number.isNaN(date.getTime())) {
        return '—'
    }
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

/** 把 Go 的 time.Time 格式化为「年-月-日」。 */
export function formatDate(value: unknown): string {
    if (!value) {
        return '—'
    }
    const date = new Date(value as string)
    if (Number.isNaN(date.getTime())) {
        return '—'
    }
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

/** 把秒格式化为易读的中文时长。 */
export function formatDuration(totalSeconds: number): string {
    const seconds = Math.max(0, Math.floor(totalSeconds || 0))
    if (seconds === 0) {
        return '暂无记录'
    }

    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    if (hours > 0 && minutes > 0) {
        return `${hours} 小时 ${minutes} 分`
    }
    if (hours > 0) {
        return `${hours} 小时`
    }
    if (minutes > 0) {
        return `${minutes} 分钟`
    }
    return `${seconds} 秒`
}

/** 累计秒数 -> 小时（保留一位小数，用于统计卡片）。 */
export function formatHours(totalSeconds: number): string {
    const hours = (totalSeconds || 0) / 3600
    return `${hours.toFixed(1)} h`
}

/** 把字节数格式化为文件大小。 */
export function formatBytes(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes < 0) {
        return '未知大小'
    }
    if (bytes < 1024) {
        return `${bytes} B`
    }
    const units = ['KB', 'MB', 'GB', 'TB']
    let size = bytes / 1024
    let unit = 0
    while (size >= 1024 && unit < units.length - 1) {
        size /= 1024
        unit += 1
    }
    return `${size.toFixed(1)} ${units[unit]}`
}

/** 取名称首字符作为封面占位。 */
export function initialOf(name: string): string {
    const trimmed = (name || '').trim()
    return trimmed ? trimmed[0].toUpperCase() : '?'
}