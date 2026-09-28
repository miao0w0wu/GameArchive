import { useEffect, useState } from 'react'

import { AddGameDirectory, GetGameDirectory, ImportScannedGames, ScanGamesInDirectory, SelectGameDirectory } from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'
import { formatBytes } from '../lib/format'
import { btnGhost, btnPrimary, inputClass } from '../lib/ui'
import { extractError, useAppStore } from '../stores/appStore'
import Modal from './Modal'

interface ScanGamesModalProps {
    onClose: () => void
}

/** 选择游戏目录、扫描并在导入前编辑识别结果。 */
export default function ScanGamesModal({ onClose }: ScanGamesModalProps) {
    const categories = useAppStore((state) => state.categories)
    const refreshAll = useAppStore((state) => state.refreshAll)
    const notify = useAppStore((state) => state.notify)
    const [directory, setDirectory] = useState('')
    const [depth, setDepth] = useState(3)
    const [games, setGames] = useState<models.ScannedGame[]>([])
    const [selected, setSelected] = useState<Set<number>>(new Set())
    const [busy, setBusy] = useState(false)
    const [error, setError] = useState('')

    useEffect(() => {
        void GetGameDirectory()
            .then(setDirectory)
            .catch((reason: unknown) => setError(extractError(reason)))
    }, [])

    const chooseDirectory = async () => {
        setError('')
        try {
            const chosen = await SelectGameDirectory()
            if (!chosen) return
            await AddGameDirectory(chosen)
            setDirectory(chosen)
            setGames([])
            setSelected(new Set())
        } catch (reason) {
            setError(extractError(reason))
        }
    }

    const scan = async () => {
        if (!directory) {
            setError('请先选择游戏目录')
            return
        }
        setBusy(true)
        setError('')
        try {
            const results = await ScanGamesInDirectory(directory, depth)
            const rows = results.map((game) => {
                const category = categories.find((item) => item.name === game.suggestedCategory)
                return models.ScannedGame.createFrom({
                    ...game,
                    categoryId: category?.id,
                })
            })
            setGames(rows)
            setSelected(new Set(rows.map((_, index) => index)))
        } catch (reason) {
            setError(extractError(reason))
        } finally {
            setBusy(false)
        }
    }

    const importSelected = async () => {
        const selectedGames = games.filter((_, index) => selected.has(index))
        if (selectedGames.length === 0) {
            setError('请至少勾选一个游戏')
            return
        }
        setBusy(true)
        setError('')
        try {
            const result = await ImportScannedGames(selectedGames)
            await refreshAll()
            notify(
                'success',
                `已导入 ${result.imported} 个游戏${result.skipped ? `，跳过重复项 ${result.skipped} 个` : ''}`,
            )
            onClose()
        } catch (reason) {
            setError(extractError(reason))
        } finally {
            setBusy(false)
        }
    }

    const updateGame = (index: number, changes: Partial<models.ScannedGame>) => {
        setGames((items) =>
            items.map((game, itemIndex) =>
                itemIndex === index ? models.ScannedGame.createFrom({ ...game, ...changes }) : game,
            ),
        )
    }

    return (
        <Modal
            title="扫描游戏目录"
            onClose={onClose}
            widthClass="max-w-5xl"
            footer={
                <>
                    <button type="button" className={btnGhost} onClick={onClose} disabled={busy}>
                        关闭
                    </button>
                    <button
                        type="button"
                        className={btnPrimary}
                        onClick={() => void importSelected()}
                        disabled={busy || selected.size === 0}
                    >
                        {busy ? '处理中…' : `导入所选（${selected.size}）`}
                    </button>
                </>
            }
        >
            <div className="flex flex-wrap items-center gap-3">
                <input
                    className={`${inputClass} min-w-64 flex-1 font-mono text-xs`}
                    value={directory}
                    readOnly
                    placeholder="请选择游戏安装目录"
                />
                <button type="button" className={btnGhost} onClick={() => void chooseDirectory()} disabled={busy}>
                    选择目录
                </button>
                <select
                    className={`${inputClass} w-32`}
                    value={depth}
                    onChange={(event) => setDepth(Number(event.target.value))}
                    aria-label="扫描深度"
                >
                    {[1, 2, 3, 5, 10].map((value) => (
                        <option key={value} value={value}>
                            深度 {value}
                        </option>
                    ))}
                </select>
                <button type="button" className={btnPrimary} onClick={() => void scan()} disabled={busy || !directory}>
                    {busy ? '扫描中…' : '开始扫描'}
                </button>
            </div>
            <p className="mt-2 text-xs text-slate-500">
                递归识别 .exe 文件，自动过滤卸载程序、安装程序、崩溃报告和启动器。默认扫描深度为 3 层。
            </p>

            {error ? (
                <p className="mt-3 rounded-lg border border-rose-500/40 bg-rose-500/10 px-3 py-2 text-xs text-rose-200">
                    {error}
                </p>
            ) : null}

            {games.length > 0 ? (
                <div className="mt-4 space-y-2">
                    <div className="flex items-center justify-between text-xs text-slate-400">
                        <span>识别到 {games.length} 个候选项；导入前可修改名称与分类。</span>
                        <button
                            type="button"
                            className="text-indigo-300 hover:text-indigo-200"
                            onClick={() =>
                                setSelected(
                                    selected.size === games.length
                                        ? new Set()
                                        : new Set(games.map((_, index) => index)),
                                )
                            }
                        >
                            {selected.size === games.length ? '取消全选' : '全选'}
                        </button>
                    </div>
                    <div className="max-h-[48vh] space-y-2 overflow-y-auto pr-1">
                        {games.map((game, index) => (
                            <div
                                key={game.exePath}
                                className="grid grid-cols-[auto_minmax(0,1fr)_minmax(8rem,12rem)] items-center gap-3 rounded-xl border border-slate-800 bg-slate-950/50 p-3"
                            >
                                <input
                                    type="checkbox"
                                    checked={selected.has(index)}
                                    onChange={(event) =>
                                        setSelected((current) => {
                                            const next = new Set(current)
                                            if (event.target.checked) next.add(index)
                                            else next.delete(index)
                                            return next
                                        })
                                    }
                                    aria-label={`导入 ${game.name}`}
                                />
                                <div className="min-w-0">
                                    <input
                                        className={`${inputClass} py-1.5`}
                                        value={game.name}
                                        onChange={(event) => updateGame(index, { name: event.target.value })}
                                        aria-label="游戏名称"
                                    />
                                    <p className="mt-1 truncate font-mono text-[11px] text-slate-500" title={game.exePath}>
                                        {game.exePath}
                                    </p>
                                    <p className="mt-1 text-[11px] text-slate-600">
                                        {game.processName} · {formatBytes(game.sizeBytes)}
                                    </p>
                                </div>
                                <select
                                    className={`${inputClass} py-1.5`}
                                    value={game.categoryId ?? ''}
                                    onChange={(event) =>
                                        updateGame(index, {
                                            categoryId: event.target.value ? Number(event.target.value) : undefined,
                                        })
                                    }
                                    aria-label={`${game.name} 分类`}
                                >
                                    <option value="">未分类</option>
                                    {categories.map((category) => (
                                        <option key={category.id} value={category.id}>
                                            {category.name}
                                        </option>
                                    ))}
                                </select>
                            </div>
                        ))}
                    </div>
                </div>
            ) : null}
        </Modal>
    )
}
