import { useEffect, useMemo, useState } from 'react'

import { GetStatsByPeriod } from '../../wailsjs/go/main/App'
import type { services } from '../../wailsjs/go/models'
import EChart from '../components/EChart'
import { chartLineColor, chartTextColor, chartTooltipStyle, toHours } from '../lib/charts'
import { formatDuration, formatHours } from '../lib/format'
import { btnGhost, cardClass } from '../lib/ui'
import { extractError, useAppStore } from '../stores/appStore'

type PeriodType = 'week' | 'month' | 'year'

const PERIOD_OPTIONS: { value: PeriodType; label: string; previous: string; next: string }[] = [
    { value: 'week', label: '周', previous: '上周', next: '下周' },
    { value: 'month', label: '月', previous: '上个月', next: '下个月' },
    { value: 'year', label: '年', previous: '上一年', next: '下一年' },
]

const RANKING_COLOR = '#6366f1'

function pad(value: number): string {
    return String(value).padStart(2, '0')
}

/** 把本地日期格式化为 YYYY-MM-DD，与 Go 侧约定一致。 */
function toDateString(date: Date): string {
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

/** 返回给定日期所在周的周一（本地时间零点）。 */
function startOfWeek(date: Date): Date {
    const day = new Date(date.getFullYear(), date.getMonth(), date.getDate())
    const offset = (day.getDay() + 6) % 7
    day.setDate(day.getDate() - offset)
    return day
}

interface PeriodRange {
    /** 传给 Go 的开始日期。 */
    start: string
    /** 传给 Go 的结束日期（含当天）。 */
    end: string
    /** 界面上展示的周期名称。 */
    label: string
}

/** 计算锚点日期所在周期对应的统计范围。 */
function periodRange(periodType: PeriodType, anchor: Date): PeriodRange {
    if (periodType === 'year') {
        const start = new Date(anchor.getFullYear(), 0, 1)
        const end = new Date(anchor.getFullYear(), 11, 31)
        return {
            start: toDateString(start),
            end: toDateString(end),
            label: `${anchor.getFullYear()} 年`,
        }
    }

    if (periodType === 'month') {
        const start = new Date(anchor.getFullYear(), anchor.getMonth(), 1)
        const end = new Date(anchor.getFullYear(), anchor.getMonth() + 1, 0)
        return {
            start: toDateString(start),
            end: toDateString(end),
            label: `${anchor.getFullYear()} 年 ${anchor.getMonth() + 1} 月`,
        }
    }

    const start = startOfWeek(anchor)
    const end = new Date(start.getFullYear(), start.getMonth(), start.getDate() + 6)
    return {
        start: toDateString(start),
        end: toDateString(end),
        label: `${toDateString(start)} ~ ${toDateString(end)}`,
    }
}

/** 按周期前后平移锚点日期。 */
function shiftAnchor(periodType: PeriodType, anchor: Date, step: number): Date {
    const next = new Date(anchor.getFullYear(), anchor.getMonth(), anchor.getDate())
    if (periodType === 'year') {
        next.setFullYear(next.getFullYear() + step)
    } else if (periodType === 'month') {
        next.setMonth(next.getMonth() + step)
    } else {
        next.setDate(next.getDate() + step * 7)
    }
    return next
}

/** 趋势图横轴标签：按天显示 MM-DD，按月显示 N 月。 */
function formatTrendLabel(label: string, unit: string): string {
    if (unit === 'month') {
        return `${Number(label.slice(5, 7))} 月`
    }
    return label.slice(5)
}

interface RankingInput {
    name: string
    seconds: number
    color?: string
}

/** 生成横向条形排行图配置（用于标签与 Top 游戏）。 */
function buildRankingOption(items: RankingInput[], unitLabel: string) {
    const ordered = [...items].reverse()
    return {
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'shadow' },
            ...chartTooltipStyle,
            formatter: (params: unknown) => {
                const first = (Array.isArray(params) ? params[0] : params) as { name: string; value: number }
                return `${first.name}<br/>${first.value} ${unitLabel}`
            },
        },
        grid: { left: 8, right: 48, top: 12, bottom: 8, containLabel: true },
        xAxis: {
            type: 'value',
            axisLabel: { color: chartTextColor, fontSize: 11 },
            splitLine: { lineStyle: { color: '#1e293b' } },
        },
        yAxis: {
            type: 'category',
            data: ordered.map((item) => item.name),
            axisLabel: { color: '#cbd5e1', fontSize: 11, width: 140, overflow: 'truncate' },
            axisLine: { lineStyle: { color: chartLineColor } },
            axisTick: { show: false },
        },
        series: [
            {
                type: 'bar',
                barWidth: 14,
                data: ordered.map((item) => ({
                    value: toHours(item.seconds),
                    itemStyle: { color: item.color || RANKING_COLOR, borderRadius: [0, 6, 6, 0] },
                })),
            },
        ],
    }
}

/** 统计报告：按年 / 月 / 周分析游玩时长、分类标签占比与 Top 游戏。 */
export default function StatsPage() {
    const notify = useAppStore((state) => state.notify)
    const monitorStatus = useAppStore((state) => state.monitorStatus)

    const [periodType, setPeriodType] = useState<PeriodType>('week')
    const [anchor, setAnchor] = useState(() => new Date())
    const [result, setResult] = useState<services.StatsResult | null>(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')

    const range = useMemo(() => periodRange(periodType, anchor), [periodType, anchor])
    const periodOption = useMemo(
        () => PERIOD_OPTIONS.find((option) => option.value === periodType) ?? PERIOD_OPTIONS[0],
        [periodType],
    )
    const isCurrentPeriod = useMemo(() => {
        const current = periodRange(periodType, new Date())
        return current.start === range.start && current.end === range.end
    }, [periodType, range.start, range.end])

    // monitorStatus 变化表示监控有更新（例如正在游玩），顺带刷新图表
    useEffect(() => {
        let cancelled = false
        setLoading(true)
        GetStatsByPeriod(periodType, range.start, range.end)
            .then((data) => {
                if (!cancelled) {
                    setResult(data)
                    setError('')
                }
            })
            .catch((reason: unknown) => {
                if (!cancelled) {
                    const message = extractError(reason)
                    setError(message)
                    notify('error', `加载统计数据失败：${message}`)
                }
            })
            .finally(() => {
                if (!cancelled) {
                    setLoading(false)
                }
            })
        return () => {
                    cancelled = true
                }
            }, [periodType, range.start, range.end, monitorStatus, notify])

    const trendOption = useMemo(() => {
        const points = result?.trend ?? []
        const unit = result?.bucketUnit ?? 'day'
        const labels = points.map((point) => formatTrendLabel(point.label, unit))
        const hours = points.map((point) => toHours(point.seconds))
        const sessions = points.map((point) => point.sessions)

        return {
            tooltip: {
                trigger: 'axis',
                ...chartTooltipStyle,
                formatter: (params: unknown) => {
                    const first = (Array.isArray(params) ? params[0] : params) as { dataIndex: number }
                    const index = first?.dataIndex ?? 0
                    return `${points[index]?.label ?? ''}<br/>游玩 ${hours[index] ?? 0} 小时 · ${
                        sessions[index] ?? 0
                    } 次`
                },
            },
            grid: { left: 8, right: 20, top: 24, bottom: 8, containLabel: true },
            xAxis: {
                type: 'category',
                boundaryGap: false,
                data: labels,
                axisLabel: { color: chartTextColor, fontSize: 11 },
                axisLine: { lineStyle: { color: chartLineColor } },
                axisTick: { show: false },
            },
            yAxis: {
                type: 'value',
                name: '小时',
                nameTextStyle: { color: '#64748b', fontSize: 11 },
                axisLabel: { color: chartTextColor, fontSize: 11 },
                splitLine: { lineStyle: { color: '#1e293b' } },
            },
            series: [
                {
                    type: 'line',
                    smooth: true,
                    symbolSize: 6,
                    data: hours,
                    itemStyle: { color: RANKING_COLOR },
                    lineStyle: { width: 2, color: RANKING_COLOR },
                    areaStyle: { color: 'rgba(99,102,241,0.16)' },
                },
            ],
        }
    }, [result])

    const categoryOption = useMemo(() => {
        const items = (result?.categories ?? []).filter((item) => item.seconds > 0)
        return {
            tooltip: {
                trigger: 'item',
                ...chartTooltipStyle,
                formatter: (params: unknown) => {
                    const item = params as { name: string; value: number; percent: number }
                    return `${item.name}<br/>${item.value} 小时 · ${item.percent}%`
                },
            },
            legend: {
                bottom: 0,
                icon: 'circle',
                textStyle: { color: chartTextColor, fontSize: 11 },
            },
            series: [
                {
                    type: 'pie',
                    radius: ['45%', '70%'],
                    center: ['50%', '42%'],
                    data: items.map((item) => ({
                        name: item.name,
                        value: toHours(item.seconds),
                        itemStyle: { color: item.color },
                    })),
                    label: { color: '#cbd5e1', fontSize: 11, formatter: '{b} {d}%' },
                    labelLine: { lineStyle: { color: chartLineColor } },
                    itemStyle: { borderColor: '#020617', borderWidth: 2 },
                },
            ],
        }
    }, [result])

    const tagOption = useMemo(
        () =>
            buildRankingOption(
                (result?.tags ?? []).map((item) => ({
                    name: item.name,
                    seconds: item.seconds,
                    color: item.color,
                })),
                '小时',
            ),
        [result],
    )

    const topGameOption = useMemo(
        () =>
            buildRankingOption(
                (result?.topGames ?? []).map((item) => ({
                    name: item.name,
                    seconds: item.seconds,
                    color: item.categoryColor,
                })),
                '小时',
            ),
        [result],
    )

    const hasData = (result?.totalSeconds ?? 0) > 0

    return (
        <div className="mx-auto max-w-6xl px-8 py-8">
            <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
                <div>
                    <h1 className="text-xl font-semibold text-slate-100">统计报告</h1>
                    <p className="mt-1 text-sm text-slate-500">
                        按年 / 月 / 周分析游玩时长、分类与标签占比以及 Top 游戏。
                    </p>
                </div>

                <div className="flex flex-wrap items-center gap-2">
                    <div className="flex items-center gap-1 rounded-lg border border-slate-700 p-1">
                        {PERIOD_OPTIONS.map((option) => (
                            <button
                                key={option.value}
                                type="button"
                                onClick={() => setPeriodType(option.value)}
                                className={`rounded-md px-3 py-1.5 text-xs font-medium transition ${
                                    periodType === option.value
                                        ? 'bg-indigo-500/20 text-indigo-200'
                                        : 'text-slate-400 hover:text-slate-200'
                                }`}
                            >
                                {option.label}
                            </button>
                        ))}
                    </div>
                    <button
                        type="button"
                        className={btnGhost}
                        onClick={() => setAnchor((current) => shiftAnchor(periodType, current, -1))}
                    >
                                            {periodOption.previous}
                    </button>
                    <button
                        type="button"
                        className={btnGhost}
                        onClick={() => setAnchor((current) => shiftAnchor(periodType, current, 1))}
                    >
                                            {periodOption.next}
                    </button>
                    {!isCurrentPeriod ? (
                        <button type="button" className={btnGhost} onClick={() => setAnchor(new Date())}>
                            回到当前
                        </button>
                    ) : null}
                </div>
            </header>

            <div className={`${cardClass} mb-5 flex flex-wrap items-center justify-between gap-3 p-4`}>
                <div>
                    <p className="text-sm text-slate-200">{range.label}</p>
                    <p className="mt-0.5 text-xs text-slate-500">
                        {range.start} ~ {range.end}
                        {result ? ` · ${result.bucketUnit === 'month' ? '按月' : '按天'}分桶` : ''}
                    </p>
                </div>
                {loading ? <span className="text-xs text-slate-500">加载中…</span> : null}
            </div>

            {error ? (
                <div className="mb-5 rounded-xl border border-rose-500/40 bg-rose-500/10 px-4 py-3 text-sm text-rose-200">
                    {error}
                </div>
            ) : null}

            <section className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
                <div className={`${cardClass} p-4`}>
                    <p className="text-xs text-slate-500">周期内时长</p>
                    <p className="mt-1 text-xl font-semibold text-slate-100">
                        {formatHours(result?.totalSeconds ?? 0)}
                    </p>
                    <p className="mt-1 text-xs text-slate-500">
                        {formatDuration(result?.totalSeconds ?? 0)}
                    </p>
                </div>
                <div className={`${cardClass} p-4`}>
                    <p className="text-xs text-slate-500">游玩天数</p>
                    <p className="mt-1 text-xl font-semibold text-slate-100">{result?.activeDays ?? 0}</p>
                    <p className="mt-1 text-xs text-slate-500">天</p>
                </div>
                <div className={`${cardClass} p-4`}>
                    <p className="text-xs text-slate-500">日均时长</p>
                    <p className="mt-1 text-xl font-semibold text-slate-100">
                        {formatHours(result?.dailyAverage ?? 0)}
                    </p>
                    <p className="mt-1 text-xs text-slate-500">按游玩天数平均</p>
                </div>
                <div className={`${cardClass} p-4`}>
                    <p className="text-xs text-slate-500">游玩游戏数</p>
                    <p className="mt-1 text-xl font-semibold text-slate-100">
                        {result?.playedGameCount ?? 0}
                    </p>
                    <p className="mt-1 text-xs text-slate-500">个</p>
                </div>
                <div className={`${cardClass} p-4`}>
                    <p className="text-xs text-slate-500">游玩次数</p>
                    <p className="mt-1 text-xl font-semibold text-slate-100">{result?.sessionCount ?? 0}</p>
                    <p className="mt-1 text-xs text-slate-500">次</p>
                </div>
            </section>

            {!loading && !hasData ? (
                <div className={`${cardClass} mt-5 p-10 text-center text-sm text-slate-500`}>
                    该周期暂无游玩记录，先启动游戏让进程监控累计时长吧。
                </div>
            ) : null}

            {hasData ? (
                <>
                    <section className={`${cardClass} mt-5 p-5`}>
                        <h2 className="text-sm font-semibold text-slate-200">时长趋势</h2>
                        <p className="mt-1 text-xs text-slate-500">
                            按 {result?.bucketUnit === 'month' ? '月' : '天'}统计的游玩小时数。
                        </p>
                        <EChart option={trendOption} height={300} className="mt-3" />
                    </section>

                    <section className="mt-5 grid grid-cols-1 gap-4 lg:grid-cols-2">
                        <div className={`${cardClass} p-5`}>
                            <h2 className="text-sm font-semibold text-slate-200">分类时长占比</h2>
                            <p className="mt-1 text-xs text-slate-500">按游戏所属分类汇总，多标签游戏会重复计入标签统计。</p>
                            {(result?.categories.length ?? 0) === 0 ? (
                                <div className="py-10 text-center text-sm text-slate-500">暂无分类数据</div>
                            ) : (
                                <EChart option={categoryOption} height={300} className="mt-3" />
                            )}
                        </div>

                        <div className={`${cardClass} p-5`}>
                            <h2 className="text-sm font-semibold text-slate-200">标签时长排行</h2>
                            <p className="mt-1 text-xs text-slate-500">同一游戏的多个标签都会获得该次游玩时长。</p>
                            {(result?.tags.length ?? 0) === 0 ? (
                                <div className="py-10 text-center text-sm text-slate-500">
                                    暂无标签数据，可在「分类与标签」页为游戏添加标签
                                </div>
                            ) : (
                                <EChart option={tagOption} height={300} className="mt-3" />
                            )}
                        </div>
                    </section>

                    <section className={`${cardClass} mt-5 p-5`}>
                        <h2 className="text-sm font-semibold text-slate-200">Top 游戏</h2>
                        <p className="mt-1 text-xs text-slate-500">周期内游玩时长最多的前 10 个游戏。</p>
                        <EChart option={topGameOption} height={Math.max(220, (result?.topGames.length ?? 0) * 42)} className="mt-3" />
                    </section>
                </>
            ) : null}

            <p className="mt-3 text-xs text-slate-500">
                数据来自进程监控记录的游玩会话；跨天会话归属到开始日期，与游戏累计时长口径一致。
            </p>
        </div>
    )
}