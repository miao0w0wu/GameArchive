import { useMemo } from 'react'

import { IconDashboard, IconLibrary, IconRoute, IconTags } from '../components/Icons'
import StatCard from '../components/StatCard'
import { formatDateTime, formatDuration, formatHours, initialOf } from '../lib/format'
import { cardClass } from '../lib/ui'
import { useAppStore } from '../stores/appStore'

const ROADMAP = [
    '阶段 2：目录扫描 + 进程监控 + 时长统计',
    '阶段 3：统计图表 + 按年 / 月 / 周分析',
    '阶段 4：AI 报告 + 图片导出 + 报告导入导出',
    '阶段 5：本地存档导入 / 备份 / 备份历史',
]

/** 仪表盘：总览数据与分类分布（时长相关统计在阶段 2 之后才会有数据）。 */
export default function Dashboard() {
    const games = useAppStore((state) => state.games)
    const categories = useAppStore((state) => state.categories)
    const tags = useAppStore((state) => state.tags)
    const navigate = useAppStore((state) => state.navigate)

    const totalSeconds = useMemo(
        () => games.reduce((sum, game) => sum + (game.totalSeconds || 0), 0),
        [games],
    )

    const categoryStats = useMemo(() => {
        const counter = new Map<number, number>()
        let uncategorized = 0

        for (const game of games) {
            if (game.categoryId) {
                counter.set(game.categoryId, (counter.get(game.categoryId) ?? 0) + 1)
            } else {
                uncategorized += 1
            }
        }

        const rows = categories
            .map((category) => ({
                id: category.id,
                name: category.name,
                color: category.color,
                count: counter.get(category.id) ?? 0,
            }))
            .sort((a, b) => b.count - a.count || a.id - b.id)

        return { rows, uncategorized }
    }, [games, categories])

    const recentGames = useMemo(
        () =>
            [...games]
                .sort(
                    (a, b) =>
                        new Date(b.updatedAt ?? 0).getTime() - new Date(a.updatedAt ?? 0).getTime(),
                )
                .slice(0, 5),
        [games],
    )

    const maxCount = Math.max(1, ...categoryStats.rows.map((row) => row.count))

    return (
        <div className="mx-auto max-w-5xl px-8 py-8">
            <header className="mb-6">
                <h1 className="text-xl font-semibold text-slate-100">仪表盘</h1>
                <p className="mt-1 text-sm text-slate-500">
                    当前为阶段 1，仅包含本地数据的管理功能；游玩时长会在阶段 2 接入进程监控后开始累积。
                </p>
            </header>

            <section className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <StatCard
                    label="游戏总数"
                    value={String(games.length)}
                    icon={<IconLibrary width={16} height={16} />}
                    accent="#6366f1"
                />
                <StatCard
                    label="分类数量"
                    value={String(categories.length)}
                    icon={<IconTags width={16} height={16} />}
                    accent="#22c55e"
                />
                <StatCard
                    label="标签数量"
                    value={String(tags.length)}
                    icon={<IconDashboard width={16} height={16} />}
                    accent="#f97316"
                />
                <StatCard
                    label="累计游玩时长"
                    value={formatHours(totalSeconds)}
                    hint={formatDuration(totalSeconds)}
                    icon={<IconRoute width={16} height={16} />}
                    accent="#0ea5e9"
                />
            </section>

            <section className="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
                <div className={`${cardClass} p-5`}>
                    <h2 className="text-sm font-semibold text-slate-200">最近更新</h2>
                    {recentGames.length === 0 ? (
                        <div className="py-8 text-center text-sm text-slate-500">
                            还没有游戏
                            <button
                                type="button"
                                className="ml-1 text-indigo-300 underline-offset-4 hover:underline"
                                onClick={() => navigate('library')}
                            >
                                去添加
                            </button>
                        </div>
                    ) : (
                        <ul className="mt-3 divide-y divide-slate-800/70">
                            {recentGames.map((game) => (
                                <li key={game.id} className="flex items-center gap-3 py-3">
                                    <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-slate-800 text-sm font-semibold text-slate-300">
                                                                            {initialOf(game.name)}
                                    </span>
                                    <span className="min-w-0 flex-1">
                                        <span className="block truncate text-sm text-slate-200">
                                            {game.name}
                                        </span>
                                        <span className="block truncate text-xs text-slate-500">
                                            {game.category?.name ?? '未分类'} · 更新时间{' '}
                                            {formatDateTime(game.updatedAt)}
                                        </span>
                                    </span>
                                    <span className="shrink-0 text-xs text-slate-400">
                                        {formatDuration(game.totalSeconds)}
                                    </span>
                                </li>
                            ))}
                        </ul>
                    )}
                </div>

                <div className={`${cardClass} p-5`}>
                    <h2 className="text-sm font-semibold text-slate-200">游戏数量分布</h2>
                    <p className="mt-1 text-xs text-slate-500">
                        按分类统计的游戏数量（时长占比统计将在阶段 3 提供）。
                    </p>
                    {categories.length === 0 ? (
                        <div className="py-8 text-center text-sm text-slate-500">暂无分类</div>
                    ) : (
                        <ul className="mt-4 space-y-3">
                            {categoryStats.rows.map((row) => (
                                <li key={row.id}>
                                    <div className="mb-1 flex items-center justify-between text-xs">
                                        <span className="text-slate-300">{row.name}</span>
                                        <span className="text-slate-500">{row.count} 个</span>
                                    </div>
                                    <div className="h-1.5 w-full overflow-hidden rounded-full bg-slate-800">
                                        <div
                                            className="h-full rounded-full transition-all"
                                            style={{
                                                width: `${(row.count / maxCount) * 100}%`,
                                                backgroundColor: row.color || '#6366f1',
                                            }}
                                        />
                                    </div>
                                </li>
                            ))}
                        </ul>
                    )}
                    {categoryStats.uncategorized > 0 ? (
                        <p className="mt-4 text-xs text-slate-500">
                            另有 {categoryStats.uncategorized} 个游戏未分类
                        </p>
                    ) : null}
                </div>
            </section>

            <section className={`${cardClass} mt-6 p-5`}>
                <h2 className="text-sm font-semibold text-slate-200">后续阶段（尚未实现）</h2>
                <ul className="mt-3 grid grid-cols-1 gap-2 text-sm text-slate-400 sm:grid-cols-2">
                    {ROADMAP.map((item) => (
                        <li key={item} className="rounded-lg bg-slate-950/50 px-3 py-2">
                            {item}
                        </li>
                    ))}
                </ul>
            </section>
        </div>
    )
}