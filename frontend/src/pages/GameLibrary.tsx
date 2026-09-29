import { useEffect, useMemo, useState } from 'react'

import type { models } from '../../wailsjs/go/models'
import {
    BatchFetchCovers,
    BatchFetchGameGenres,
    BatchFetchIcons,
    GetLibraryDisplaySettings,
    SaveLibraryDisplaySettings,
    SelectAndSetCover,
} from '../../wailsjs/go/main/App'
import ConfirmDialog from '../components/ConfirmDialog'
import GameDetailModal from '../components/GameDetailModal'
import GameFormModal from '../components/GameFormModal'
import GameImage from '../components/GameImage'
import ScanGamesModal from '../components/ScanGamesModal'
import { IconEdit, IconGrid, IconList, IconPlus, IconSearch, IconTrash } from '../components/Icons'
import { formatDate, formatDuration } from '../lib/format'
import { btnGhost, btnIcon, btnPrimary, cardClass, inputClass } from '../lib/ui'
import { extractError, useAppStore } from '../stores/appStore'

type ViewMode = 'list' | 'card'
type CardSize = 'small' | 'medium' | 'large'

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
    const [columns, setColumns] = useState(3)
    const [cardSize, setCardSize] = useState<CardSize>('medium')
    const [displayLoaded, setDisplayLoaded] = useState(false)
    const [selected, setSelected] = useState<number[]>([])
    const [formOpen, setFormOpen] = useState(false)
    const [scannerOpen, setScannerOpen] = useState(false)
    const [editingGame, setEditingGame] = useState<models.Game | null>(null)
    const [detailId, setDetailId] = useState<number | null>(null)
    const [deleteTarget, setDeleteTarget] = useState<models.Game | null>(null)
    const notify = useAppStore((state) => state.notify)
    const refreshAll = useAppStore((state) => state.refreshAll)

    useEffect(() => {
        let cancelled = false
        GetLibraryDisplaySettings()
            .then((settings) => {
                if (cancelled) return
                setView(settings.viewMode === 'card' ? 'card' : 'list')
                setColumns(settings.columns)
                setCardSize((settings.cardSize as CardSize) || 'medium')
            })
            .catch((reason: unknown) => {
                if (!cancelled) notify('error', `读取游戏库展示设置失败：${extractError(reason)}`)
            })
            .finally(() => {
                if (!cancelled) setDisplayLoaded(true)
            })
        return () => {
            cancelled = true
        }
    }, [notify])

    const persistDisplay = async (next: { viewMode: ViewMode; columns: number; cardSize: CardSize }) => {
        try {
            await SaveLibraryDisplaySettings(next)
            setView(next.viewMode)
            setColumns(next.columns)
            setCardSize(next.cardSize)
        } catch (reason) {
            notify('error', `保存展示设置失败：${extractError(reason)}`)
        }
    }

    const toggleSelected = (id: number) => {
        setSelected((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id])
    }

    const toggleAllVisible = () => {
        const visibleIDs = filtered.map((game) => game.id)
        const allSelected = visibleIDs.length > 0 && visibleIDs.every((id) => selected.includes(id))
        setSelected((current) => allSelected
            ? current.filter((id) => !visibleIDs.includes(id))
            : [...new Set([...current, ...visibleIDs])])
    }

    const selectCover = async (gameID: number) => {
        try {
            const path = await SelectAndSetCover(gameID, 'cover')
            if (path) await refreshAll()
        } catch (reason) {
            notify('error', `更换封面失败：${extractError(reason)}`)
        }
    }

    const batchFetch = async (target: 'cover' | 'icon') => {
        try {
            const result = target === 'cover'
                ? await BatchFetchCovers(selected)
                : await BatchFetchIcons(selected)
            notify('success', `批量获取完成：成功 ${result.fetched}，未获取 ${result.failed}，跳过 ${result.skipped}`)
            setSelected([])
            await refreshAll()
        } catch (reason) {
            notify('error', `批量获取图片失败：${extractError(reason)}`)
        }
    }

    const batchFetchGenres = async () => {
        try {
            const result = await BatchFetchGameGenres(selected)
            notify(
                'success',
                `Steam 类型获取完成：成功 ${result.succeeded}，失败 ${result.failed}，跳过 ${result.skipped}`,
            )
            setSelected([])
        } catch (reason) {
            notify('error', `批量获取 Steam 类型失败：${extractError(reason)}`)
        }
    }

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
                            onClick={() => void persistDisplay({ viewMode: 'list', columns, cardSize })}
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
                            onClick={() => void persistDisplay({ viewMode: 'card', columns, cardSize })}
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

                    {view === 'card' ? (
                        <>
                            <label className="flex items-center gap-2 text-xs text-slate-400">
                                每行
                                <select
                                    className={`${inputClass} w-20`}
                                    value={columns}
                                    onChange={(event) => void persistDisplay({
                                        viewMode: view,
                                        columns: Number(event.target.value),
                                        cardSize,
                                    })}
                                >
                                    {[2, 3, 4, 5, 6].map((count) => <option key={count} value={count}>{count}</option>)}
                                </select>
                            </label>
                            <label className="flex items-center gap-2 text-xs text-slate-400">
                                封面尺寸
                                <select
                                    className={`${inputClass} w-28`}
                                    value={cardSize}
                                    onChange={(event) => void persistDisplay({
                                        viewMode: view,
                                        columns,
                                        cardSize: event.target.value as CardSize,
                                    })}
                                >
                                    <option value="small">小</option>
                                    <option value="medium">中</option>
                                    <option value="large">大</option>
                                </select>
                            </label>
                        </>
                    ) : null}

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
                    {selected.length > 0 ? (
                        <div className="flex w-full flex-wrap items-center gap-2 border-t border-slate-800 pt-3">
                            <span className="mr-1 text-xs text-slate-400">已选 {selected.length} 个</span>
                            <button type="button" className={btnGhost} onClick={() => void batchFetch('cover')}>批量获取封面</button>
                            <button type="button" className={btnGhost} onClick={() => void batchFetch('icon')}>批量获取图标</button>
                            <button type="button" className={btnGhost} onClick={() => void batchFetchGenres()}>批量获取 Steam 类型</button>
                            <button type="button" className={btnGhost} onClick={() => setSelected([])}>取消选择</button>
                        </div>
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

            {!displayLoaded ? (
                <div className={`${cardClass} py-12 text-center text-sm text-slate-500`}>正在读取展示偏好…</div>
            ) : filtered.length === 0 ? (
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
                                <th className="px-3 py-3">
                                    <input
                                        type="checkbox"
                                        aria-label="全选当前游戏"
                                        checked={filtered.length > 0 && filtered.every((game) => selected.includes(game.id))}
                                        onChange={toggleAllVisible}
                                    />
                                </th>
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
                                <tr
                                    key={game.id}
                                    className="cursor-pointer hover:bg-slate-800/30"
                                    tabIndex={0}
                                    aria-label={`查看${game.name}详情`}
                                    onClick={() => setDetailId(game.id)}
                                    onKeyDown={(event) => {
                                        if (event.key === 'Enter') setDetailId(game.id)
                                    }}
                                >
                                    <td className="px-3 py-3">
                                        <input
                                            type="checkbox"
                                            aria-label={`选择${game.name}`}
                                            checked={selected.includes(game.id)}
                                            onChange={() => toggleSelected(game.id)}
                                            onClick={(event) => event.stopPropagation()}
                                        />
                                    </td>
                                    <td className="px-4 py-3">
                                        <div className="flex items-center gap-3">
                                            <GameImage
                                                gameId={game.id}
                                                name={game.name}
                                                target="icon"
                                                imageVersion={`${game.iconUpdatedAt ?? ''}${game.coverUpdatedAt ?? ''}`}
                                                className="h-10 w-10 shrink-0 rounded-lg object-cover"
                                            />
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
                                    <td className="px-4 py-3 text-xs text-slate-300">
                                        <div>累计 {formatDuration(Math.max(game.totalSeconds, game.steamPlaytimeSeconds))}</div>
                                        <div className="mt-1 text-slate-500">
                                            本地 {formatDuration(game.totalSeconds)}
                                            {game.steamPlaytimeSeconds > 0 ? ` · Steam ${formatDuration(game.steamPlaytimeSeconds)}` : ''}
                                        </div>
                                    </td>
                                    <td className="px-4 py-3 text-slate-400">
                                        {game.lastPlayedAt ? formatDate(game.lastPlayedAt) : '暂无记录'}
                                    </td>
                                    <td className="px-4 py-3">
                                        <div className="flex justify-end gap-2">
                                            <button
                                                type="button"
                                                className={btnIcon}
                                                onClick={(event) => {
                                                    event.stopPropagation()
                                                    openEdit(game)
                                                }}
                                                aria-label="编辑"
                                            >
                                                <IconEdit width={15} height={15} />
                                            </button>
                                            <button
                                                type="button"
                                                className={`${btnIcon} hover:border-rose-500/50 hover:text-rose-300`}
                                                onClick={(event) => {
                                                    event.stopPropagation()
                                                    setDeleteTarget(game)
                                                }}
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
                <div className="grid gap-4" style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}>
                    {filtered.map((game) => (
                        <div key={game.id} className={`${cardClass} group relative flex min-w-0 flex-col overflow-hidden`}>
                            <label className="absolute top-3 left-3 z-10 rounded bg-slate-950/80 p-1.5">
                                <input
                                    type="checkbox"
                                    aria-label={`选择${game.name}`}
                                    checked={selected.includes(game.id)}
                                    onChange={() => toggleSelected(game.id)}
                                />
                            </label>
                            <div className="relative">
                                <button
                                    type="button"
                                    className="block w-full overflow-hidden bg-slate-950"
                                    onClick={() => setDetailId(game.id)}
                                    aria-label={`查看${game.name}详情`}
                                >
                                    <GameImage
                                        gameId={game.id}
                                        name={game.name}
                                        target="cover"
                                        imageVersion={`${game.coverUpdatedAt ?? ''}${game.iconUpdatedAt ?? ''}`}
                                        className="mx-auto aspect-[2/3] w-full object-cover"
                                        style={{
                                            maxHeight: cardSize === 'small' ? 190 : cardSize === 'large' ? 360 : 270,
                                        }}
                                    />
                                </button>
                                <button
                                    type="button"
                                    className="absolute right-2 bottom-2 rounded-lg border border-slate-600 bg-slate-950/90 px-2.5 py-1.5 text-xs text-slate-100 opacity-0 transition group-hover:opacity-100 focus:opacity-100"
                                    onClick={() => void selectCover(game.id)}
                                >
                                    更换封面
                                </button>
                            </div>
                            <div className="flex flex-1 flex-col p-4">
                            <div className="flex items-start gap-3">
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
                                    <dd className="text-slate-300">
                                        {formatDuration(Math.max(game.totalSeconds, game.steamPlaytimeSeconds))}
                                    </dd>
                                </div>
                                <div className="flex justify-between">
                                    <dt>本地时长</dt>
                                    <dd>{formatDuration(game.totalSeconds)}</dd>
                                </div>
                                {game.steamPlaytimeSeconds > 0 ? (
                                    <div className="flex justify-between">
                                        <dt>Steam 时长</dt>
                                        <dd className="text-slate-300">{formatDuration(game.steamPlaytimeSeconds)}</dd>
                                    </div>
                                ) : null}
                                <div className="flex justify-between">
                                    <dt>最后游玩</dt>
                                    <dd>{game.lastPlayedAt ? formatDate(game.lastPlayedAt) : '暂无记录'}</dd>
                                </div>
                            </dl>
                            </div>

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
                            会移除游戏、存档与备份历史记录，但不会删除磁盘上的游戏文件、原始存档或备份文件。
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