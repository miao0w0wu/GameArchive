import { useMemo, useState } from 'react'

import type { models } from '../../wailsjs/go/models'
import ConfirmDialog from '../components/ConfirmDialog'
import GameDetailModal from '../components/GameDetailModal'
import GameFormModal from '../components/GameFormModal'
import ScanGamesModal from '../components/ScanGamesModal'
import { IconEdit, IconGrid, IconList, IconPlus, IconSearch, IconTrash } from '../components/Icons'
import { formatDate, formatDuration, initialOf } from '../lib/format'
import { btnGhost, btnIcon, btnPrimary, cardClass, inputClass } from '../lib/ui'
import { useAppStore } from '../stores/appStore'

type ViewMode = 'list' | 'card'

/** 游戏库：列表 / 卡片展示，搜索、筛选、增删改查。 */
export default function GameLibrary() {
    const games = useAppStore((state) => state.games)
    const categories = useAppStore((state) => state.categories)
    const tags = useAppStore((state) => state.tags)
    const removeGame = useAppStore((state) => state.removeGame)

    const [search, setSearch] = useState('')
    const [categoryFilter, setCategoryFilter] = useState('')
    const [tagFilter, setTagFilter] = useState<number[]>([])
    const [view, setView] = useState<ViewMode>('list')
    const [formOpen, setFormOpen] = useState(false)
    const [scannerOpen, setScannerOpen] = useState(false)
    const [editingGame, setEditingGame] = useState<models.Game | null>(null)
    const [detailId, setDetailId] = useState<number | null>(null)
    const [deleteTarget, setDeleteTarget] = useState<models.Game | null>(null)

    const filtered = useMemo(() => {
        const keyword = search.trim().toLowerCase()

        return games.filter((game) => {
            if (keyword) {
                const haystack = [game.name, game.exePath, game.processName, game.installDir]
                    .join(' ')
                    .toLowerCase()
                if (!haystack.includes(keyword)) {
                    return false
                }
            }

            if (categoryFilter === 'none') {
                if (game.categoryId) {
                    return false
                }
            } else if (categoryFilter !== '' && String(game.categoryId ?? '') !== categoryFilter) {
                return false
            }

            if (tagFilter.length > 0) {
                const owned = new Set((game.tags ?? []).map((tag) => tag.id))
                if (!tagFilter.every((tagId) => owned.has(tagId))) {
                    return false
                }
            }

            return true
        })
    }, [games, search, categoryFilter, tagFilter])

    const toggleTagFilter = (tagId: number) => {
        setTagFilter((prev) =>
            prev.includes(tagId) ? prev.filter((id) => id !== tagId) : [...prev, tagId],
        )
    }

    const openCreate = () => {
        setEditingGame(null)
        setFormOpen(true)
    }

    const openEdit = (game: models.Game) => {
        setEditingGame(game)
        setFormOpen(true)
    }

    const hasFilter = search !== '' || categoryFilter !== '' || tagFilter.length > 0

    return (
        <div className="mx-auto max-w-6xl px-8 py-8">
            <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
                <div>
                    <h1 className="text-xl font-semibold text-slate-100">游戏库</h1>
                    <p className="mt-1 text-sm text-slate-500">
                        共 {games.length} 个游戏
                        {hasFilter ? `，筛选出 ${filtered.length} 个` : ''}
                    </p>
                </div>
                <div className="flex flex-wrap gap-2">
                    <button type="button" className={btnGhost} onClick={() => setScannerOpen(true)}>
                        扫描目录
                    </button>
                    <button type="button" className={btnPrimary} onClick={openCreate}>
                        <IconPlus width={16} height={16} />
                        添加游戏
                    </button>
                </div>
            </header>

            <div className={`${cardClass} mb-4 p-4`}>
                <div className="flex flex-wrap items-center gap-3">
                    <div className="relative min-w-56 flex-1">
                        <IconSearch
                            className="absolute top-1/2 left-3 -translate-y-1/2 text-slate-500"
                            width={16}
                            height={16}
                        />
                        <input
                            className={`${inputClass} pl-9`}
                            value={search}
                            onChange={(event) => setSearch(event.target.value)}
                            placeholder="搜索名称、路径或进程名"
                        />
                    </div>

                    <select
                        className={`${inputClass} w-44`}
                        value={categoryFilter}
                        onChange={(event) => setCategoryFilter(event.target.value)}
                    >
                        <option value="">全部分类</option>
                        <option value="none">未分类</option>
                        {categories.map((category) => (
                            <option key={category.id} value={category.id}>
                                {category.name}
                            </option>
                        ))}
                    </select>

                    <div className="flex items-center gap-1 rounded-lg border border-slate-700 p-1">
                        <button
                            type="button"
                            onClick={() => setView('list')}
                            className={`rounded-md p-1.5 transition ${
                                view === 'list'
                                    ? 'bg-indigo-500/20 text-indigo-200'
                                    : 'text-slate-500 hover:text-slate-300'
                            }`}
                            aria-label="列表视图"
                        >
                            <IconList width={16} height={16} />
                        </button>
                        <button
                            type="button"
                            onClick={() => setView('card')}
                            className={`rounded-md p-1.5 transition ${
                                view === 'card'
                                    ? 'bg-indigo-500/20 text-indigo-200'
                                    : 'text-slate-500 hover:text-slate-300'
                            }`}
                            aria-label="卡片视图"
                        >
                            <IconGrid width={16} height={16} />
                        </button>
                    </div>

                    {hasFilter ? (
                        <button
                            type="button"
                            className={btnGhost}
                            onClick={() => {
                                setSearch('')
                                setCategoryFilter('')
                                setTagFilter([])
                            }}
                        >
                            重置筛选
                        </button>
                    ) : null}
                </div>

                {tags.length > 0 ? (
                    <div className="mt-3 flex flex-wrap items-center gap-2 border-t border-slate-800 pt-3">
                        <span className="text-xs text-slate-500">标签筛选：</span>
                        {tags.map((tag) => {
                            const active = tagFilter.includes(tag.id)
                            return (
                                <button
                                    key={tag.id}
                                    type="button"
                                    onClick={() => toggleTagFilter(tag.id)}
                                    className="rounded-full border px-3 py-1 text-xs font-medium transition"
                                    style={{
                                        borderColor: active ? tag.color : '#334155',
                                        backgroundColor: active ? `${tag.color}33` : 'transparent',
                                        color: active ? '#f1f5f9' : '#94a3b8',
                                    }}
                                >
                                    {tag.name}
                                </button>
                            )
                        })}
                    </div>
                ) : null}
            </div>

            {filtered.length === 0 ? (
                <div className={`${cardClass} py-16 text-center`}>
                    <p className="text-sm text-slate-400">
                        {games.length === 0 ? '还没有游戏，点击右上角「添加游戏」开始记录' : '没有符合条件的游戏'}
                    </p>
                </div>
            ) : view === 'list' ? (
                <div className={`${cardClass} overflow-hidden`}>
                    <table className="w-full text-left text-sm">
                        <thead className="bg-slate-950/40 text-xs text-slate-500">
                            <tr>
                                <th className="px-4 py-3 font-medium">游戏</th>
                                <th className="px-4 py-3 font-medium">分类</th>
                                <th className="px-4 py-3 font-medium">标签</th>
                                <th className="px-4 py-3 font-medium">累计时长</th>
                                <th className="px-4 py-3 font-medium">最后游玩</th>
                                <th className="px-4 py-3 text-right font-medium">操作</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-800/70">
                            {filtered.map((game) => (
                                <tr key={game.id} className="hover:bg-slate-800/30">
                                    <td className="px-4 py-3">
                                        <div className="flex items-center gap-3">
                                            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-slate-800 text-xs font-semibold text-slate-300">
                                                {initialOf(game.name)}
                                            </span>
                                            <button
                                                type="button"
                                                className="min-w-0 text-left"
                                                onClick={() => setDetailId(game.id)}
                                            >
                                                <span className="block truncate text-slate-100 hover:text-indigo-300">
                                                    {game.name}
                                                </span>
                                                <span className="block max-w-72 truncate text-xs text-slate-500">
                                                    {game.exePath || '未填写路径'}
                                                </span>
                                            </button>
                                        </div>
                                    </td>
                                    <td className="px-4 py-3">
                                        {game.category ? (
                                            <span
                                                className="rounded-full px-2.5 py-1 text-xs"
                                                style={{
                                                    backgroundColor: `${game.category.color}22`,
                                                    color: game.category.color,
                                                }}
                                            >
                                                {game.category.name}
                                            </span>
                                        ) : (
                                            <span className="text-xs text-slate-500">未分类</span>
                                        )}
                                    </td>
                                    <td className="px-4 py-3">
                                        <div className="flex flex-wrap gap-1.5">
                                            {(game.tags ?? []).length === 0 ? (
                                                <span className="text-xs text-slate-500">—</span>
                                            ) : (
                                                game.tags.map((tag) => (
                                                    <span
                                                        key={tag.id}
                                                        className="rounded-full px-2 py-0.5 text-xs"
                                                        style={{
                                                            backgroundColor: `${tag.color}22`,
                                                            color: tag.color,
                                                        }}
                                                    >
                                                        {tag.name}
                                                    </span>
                                                ))
                                            )}
                                        </div>
                                    </td>
                                    <td className="px-4 py-3 text-slate-300">
                                        {formatDuration(game.totalSeconds)}
                                    </td>
                                    <td className="px-4 py-3 text-slate-400">
                                        {game.lastPlayedAt ? formatDate(game.lastPlayedAt) : '暂无记录'}
                                    </td>
                                    <td className="px-4 py-3">
                                        <div className="flex justify-end gap-2">
                                            <button
                                                type="button"
                                                className={btnIcon}
                                                onClick={() => openEdit(game)}
                                                aria-label="编辑"
                                            >
                                                <IconEdit width={15} height={15} />
                                            </button>
                                            <button
                                                type="button"
                                                className={`${btnIcon} hover:border-rose-500/50 hover:text-rose-300`}
                                                onClick={() => setDeleteTarget(game)}
                                                aria-label="删除"
                                            >
                                                <IconTrash width={15} height={15} />
                                            </button>
                                        </div>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            ) : (
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                    {filtered.map((game) => (
                        <div key={game.id} className={`${cardClass} flex flex-col p-4`}>
                            <div className="flex items-start gap-3">
                                <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500/30 to-fuchsia-500/30 text-base font-semibold text-slate-100">
                                    {initialOf(game.name)}
                                </span>
                                <div className="min-w-0 flex-1">
                                    <button
                                        type="button"
                                        className="block w-full truncate text-left text-sm font-medium text-slate-100 hover:text-indigo-300"
                                        onClick={() => setDetailId(game.id)}
                                    >
                                        {game.name}
                                    </button>
                                    <p className="mt-0.5 truncate text-xs text-slate-500">
                                        {game.category?.name ?? '未分类'}
                                    </p>
                                </div>
                            </div>

                            <div className="mt-3 flex flex-wrap gap-1.5">
                                {(game.tags ?? []).map((tag) => (
                                    <span
                                        key={tag.id}
                                        className="rounded-full px-2 py-0.5 text-xs"
                                        style={{ backgroundColor: `${tag.color}22`, color: tag.color }}
                                    >
                                        {tag.name}
                                    </span>
                                ))}
                            </div>

                            <dl className="mt-3 space-y-1 text-xs text-slate-400">
                                <div className="flex justify-between">
                                    <dt>累计时长</dt>
                                    <dd className="text-slate-300">{formatDuration(game.totalSeconds)}</dd>
                                </div>
                                <div className="flex justify-between">
                                    <dt>最后游玩</dt>
                                    <dd>{game.lastPlayedAt ? formatDate(game.lastPlayedAt) : '暂无记录'}</dd>
                                </div>
                            </dl>

                            <div className="mt-4 flex justify-end gap-2 border-t border-slate-800 pt-3">
                                <button
                                    type="button"
                                    className={btnIcon}
                                    onClick={() => openEdit(game)}
                                    aria-label="编辑"
                                >
                                    <IconEdit width={15} height={15} />
                                </button>
                                <button
                                    type="button"
                                    className={`${btnIcon} hover:border-rose-500/50 hover:text-rose-300`}
                                    onClick={() => setDeleteTarget(game)}
                                    aria-label="删除"
                                >
                                    <IconTrash width={15} height={15} />
                                </button>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {formOpen ? (
                <GameFormModal
                    key={editingGame?.id ?? 'new'}
                    game={editingGame}
                    onClose={() => setFormOpen(false)}
                />
            ) : null}

            {scannerOpen ? <ScanGamesModal onClose={() => setScannerOpen(false)} /> : null}

            {detailId !== null ? (
                <GameDetailModal gameId={detailId} onClose={() => setDetailId(null)} />
            ) : null}

            {deleteTarget ? (
                <ConfirmDialog
                    title="删除游戏"
                    message={
                        <>
                            确定要删除「{deleteTarget.name}」吗？
                            <br />
                            只会移除档案记录（含标签关联），不会删除磁盘上的游戏文件。
                        </>
                    }
                    onCancel={() => setDeleteTarget(null)}
                    onConfirm={async () => {
                        const target = deleteTarget
                        setDeleteTarget(null)
                        if (target) {
                            await removeGame(target.id)
                        }
                    }}
                />
            ) : null}
        </div>
    )
}