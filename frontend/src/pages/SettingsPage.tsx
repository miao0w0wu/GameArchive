import type { ReactNode } from 'react'

import AISettingsCard from '../components/AISettingsCard'
import CoverSettingsCard from '../components/CoverSettingsCard'
import SteamSettingsCard from '../components/SteamSettingsCard'
import { btnGhost, cardClass } from '../lib/ui'
import { useAppStore } from '../stores/appStore'

const ROADMAP = [
    { stage: '阶段 1', text: 'Wails 项目初始化 + SQLite + 游戏 / 分类 / 标签 CRUD', done: true },
    { stage: '阶段 2', text: '目录扫描、进程监控与时长统计', done: true },
    { stage: '阶段 3', text: 'ECharts 统计图表，按年 / 月 / 周分析游玩数据', done: true },
    { stage: '阶段 4', text: 'OpenAI 兼容接口生成 AI 报告、图片导出与报告导入导出', done: true },
    { stage: '阶段 5', text: '本地游戏存档导入、备份与备份历史管理', done: true },
    { stage: '阶段 6', text: 'Steam 游戏库、游玩时长同步与本地游戏匹配', done: true },
]

function InfoRow({ label, value, mono = false }: { label: string; value: ReactNode; mono?: boolean }) {
    return (
        <div className="flex flex-col gap-1 border-b border-slate-800/70 py-3 sm:flex-row sm:items-center sm:gap-4">
            <dt className="w-32 shrink-0 text-xs text-slate-500">{label}</dt>
            <dd className={`text-sm break-all text-slate-200 ${mono ? 'font-mono text-xs' : ''}`}>
                {value}
            </dd>
        </div>
    )
}

/** 设置页：展示运行环境信息与数据统计，并标注后续阶段的功能。 */
export default function SettingsPage() {
    const appInfo = useAppStore((state) => state.appInfo)
    const games = useAppStore((state) => state.games)
    const categories = useAppStore((state) => state.categories)
    const tags = useAppStore((state) => state.tags)
    const refreshAll = useAppStore((state) => state.refreshAll)
    const refreshing = useAppStore((state) => state.refreshing)
    const monitorStatus = useAppStore((state) => state.monitorStatus)
    const setMonitorEnabled = useAppStore((state) => state.setMonitorEnabled)

    return (
        <div className="mx-auto max-w-4xl px-8 py-8">
            <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
                <div>
                    <h1 className="text-xl font-semibold text-slate-100">设置</h1>
                    <p className="mt-1 text-sm text-slate-500">
                        管理进程监控、Steam 数据接入、AI 报告接口与本地数据存储。
                    </p>
                </div>
                <button
                    type="button"
                    className={btnGhost}
                    onClick={() => void refreshAll()}
                    disabled={refreshing}
                >
                    {refreshing ? '刷新中…' : '刷新数据'}
                </button>
            </header>

            {appInfo?.startupError ? (
                <div className="mb-5 rounded-xl border border-rose-500/40 bg-rose-500/10 px-4 py-3 text-sm text-rose-200">
                    数据库初始化失败：{appInfo.startupError}
                </div>
            ) : null}

            <section className={`${cardClass} p-5`}>
                <div className="flex flex-wrap items-center justify-between gap-4">
                    <div>
                        <h2 className="text-sm font-semibold text-slate-200">进程监控</h2>
                        <p className="mt-1 text-xs text-slate-500">
                            每 5 秒检查一次进程；暂停时会先结束当前游玩记录。
                        </p>
                    </div>
                    <button
                        type="button"
                        className={monitorStatus?.running ? btnGhost : 'inline-flex items-center justify-center rounded-lg bg-emerald-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-emerald-500 disabled:opacity-50'}
                        onClick={() => void setMonitorEnabled(!monitorStatus?.running)}
                        disabled={!monitorStatus}
                    >
                        {monitorStatus ? (monitorStatus.running ? '暂停监控' : '继续监控') : '读取状态…'}
                    </button>
                </div>
                <div className="mt-4 rounded-xl bg-slate-950/50 px-4 py-3">
                    <p className="text-xs text-slate-400">
                        状态：
                        <span className={monitorStatus?.running ? 'text-emerald-300' : 'text-slate-500'}>
                            {monitorStatus ? (monitorStatus.running ? '运行中' : '已暂停') : '读取中'}
                        </span>
                        <span className="ml-3">当前游玩：{monitorStatus?.activeGames.length ?? 0} 个</span>
                    </p>
                    {monitorStatus?.activeGames.length ? (
                        <ul className="mt-3 space-y-1">
                            {monitorStatus.activeGames.map((game) => (
                                <li key={game.gameId} className="flex justify-between text-xs">
                                    <span className="text-slate-300">{game.name}</span>
                                    <span className="text-slate-500">
                                        {Math.floor(game.durationSeconds / 60)} 分钟
                                    </span>
                                </li>
                            ))}
                        </ul>
                    ) : null}
                </div>
            </section>

            <AISettingsCard />
            <SteamSettingsCard />
            <CoverSettingsCard />

            <section className={`${cardClass} mt-5 p-5`}>
                <h2 className="text-sm font-semibold text-slate-200">运行环境</h2>
                <dl className="mt-2">
                    <InfoRow label="应用名称" value={appInfo?.appName ?? '游戏档案'} />
                    <InfoRow label="版本" value={appInfo?.version ?? '0.1.0'} />
                    <InfoRow label="当前阶段" value={appInfo?.stage ?? '阶段 6'} />
                    <InfoRow
                        label="数据库状态"
                        value={
                            appInfo?.ready ? (
                                <span className="text-emerald-300">已就绪</span>
                            ) : (
                                <span className="text-rose-300">未就绪</span>
                            )
                        }
                    />
                    <InfoRow label="数据目录" value={appInfo?.dataDir ?? '—'} mono />
                    <InfoRow label="数据库文件" value={appInfo?.dbPath ?? '—'} mono />
                </dl>
                <p className="mt-3 text-xs text-slate-500">
                    数据库使用纯 Go 的 SQLite 驱动（glebarez/sqlite），文件保存在用户配置目录下，卸载应用不会自动删除。
                </p>
            </section>

            <section className={`${cardClass} mt-5 p-5`}>
                <h2 className="text-sm font-semibold text-slate-200">数据统计</h2>
                <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-3">
                    <div className="rounded-xl bg-slate-950/50 px-4 py-3">
                        <p className="text-xs text-slate-500">游戏</p>
                        <p className="mt-1 text-lg font-semibold text-slate-100">
                            {appInfo?.gameCount ?? games.length}
                        </p>
                    </div>
                    <div className="rounded-xl bg-slate-950/50 px-4 py-3">
                        <p className="text-xs text-slate-500">分类</p>
                        <p className="mt-1 text-lg font-semibold text-slate-100">
                            {appInfo?.categoryCount ?? categories.length}
                        </p>
                    </div>
                    <div className="rounded-xl bg-slate-950/50 px-4 py-3">
                        <p className="text-xs text-slate-500">标签</p>
                        <p className="mt-1 text-lg font-semibold text-slate-100">
                            {appInfo?.tagCount ?? tags.length}
                        </p>
                    </div>
                </div>
            </section>

            <section className={`${cardClass} mt-5 p-5`}>
                <h2 className="text-sm font-semibold text-slate-200">阶段进度</h2>
                <ul className="mt-4 space-y-2">
                    {ROADMAP.map((item) => (
                        <li
                            key={item.stage}
                            className="flex items-start gap-3 rounded-xl bg-slate-950/50 px-4 py-3"
                        >
                            <span
                                className={`mt-0.5 shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${
                                    item.done
                                        ? 'bg-emerald-500/15 text-emerald-300'
                                        : 'bg-slate-800 text-slate-400'
                                }`}
                            >
                                {item.stage}
                            </span>
                            <span className="flex-1 text-sm text-slate-300">{item.text}</span>
                            {!item.done ? (
                                <span className="shrink-0 text-xs text-slate-500">未实现</span>
                            ) : null}
                        </li>
                    ))}
                </ul>
            </section>
        </div>
    )
}