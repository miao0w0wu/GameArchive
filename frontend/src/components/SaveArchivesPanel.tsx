import { useEffect, useState } from 'react'

import {
    AddSaveArchive,
    BackupSaveArchive,
    DeleteSaveArchive,
    DeleteSaveBackup,
    ListSaveArchives,
    ListSaveBackups,
    SelectSaveArchiveDirectory,
    SelectSaveArchiveFile,
    SelectSaveBackupDirectory,
} from '../../wailsjs/go/main/App'
import type { models } from '../../wailsjs/go/models'
import { formatDateTime } from '../lib/format'
import { btnDanger, btnGhost, btnPrimary, inputClass, labelClass } from '../lib/ui'
import { extractError, useAppStore } from '../stores/appStore'
import ConfirmDialog from './ConfirmDialog'

interface SaveArchivesPanelProps {
    gameId: number
    gameName: string
}

interface DeleteTarget {
    kind: 'archive' | 'backup'
    id: number
    label: string
}

function defaultArchiveName(path: string): string {
    return path.split(/[\\/]/).filter(Boolean).at(-1) ?? ''
}

/** 游戏详情中的存档登记、备份与历史管理。 */
export default function SaveArchivesPanel({ gameId, gameName }: SaveArchivesPanelProps) {
    const notify = useAppStore((state) => state.notify)
    const [archives, setArchives] = useState<models.SaveArchive[]>([])
    const [backups, setBackups] = useState<Record<number, models.SaveBackup[]>>({})
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [sourcePath, setSourcePath] = useState('')
    const [sourceIsDirectory, setSourceIsDirectory] = useState(false)
    const [archiveName, setArchiveName] = useState('')
    const [archiveNote, setArchiveNote] = useState('')
    const [backupNotes, setBackupNotes] = useState<Record<number, string>>({})
    const [backupTargets, setBackupTargets] = useState<Record<number, string>>({})
    const [busy, setBusy] = useState(false)
    const [busyArchiveId, setBusyArchiveId] = useState<number | null>(null)
    const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null)
    const [deleteFiles, setDeleteFiles] = useState(false)

    const refresh = async () => {
        setError('')
        const nextArchives = await ListSaveArchives(gameId)
        const entries = await Promise.all(
            nextArchives.map(async (archive) => [archive.id, await ListSaveBackups(archive.id)] as const),
        )
        setArchives(nextArchives)
        setBackups(Object.fromEntries(entries))
    }

    useEffect(() => {
        let cancelled = false
        setLoading(true)
        ListSaveArchives(gameId)
            .then(async (nextArchives) => {
                const entries = await Promise.all(
                    nextArchives.map(async (archive) => [archive.id, await ListSaveBackups(archive.id)] as const),
                )
                if (!cancelled) {
                    setArchives(nextArchives)
                    setBackups(Object.fromEntries(entries))
                    setError('')
                }
            })
            .catch((reason: unknown) => {
                if (!cancelled) {
                    const message = extractError(reason)
                    setError(message)
                    notify('error', `加载游戏存档失败：${message}`)
                }
            })
            .finally(() => {
                if (!cancelled) setLoading(false)
            })
        return () => {
            cancelled = true
        }
    }, [gameId, notify])

    const chooseSource = async (isDirectory: boolean) => {
        try {
            const path = isDirectory
                ? await SelectSaveArchiveDirectory()
                : await SelectSaveArchiveFile()
            if (!path) return
            setSourcePath(path)
            setSourceIsDirectory(isDirectory)
            setArchiveName((current) => current || defaultArchiveName(path))
        } catch (reason) {
            notify('error', `选择存档位置失败：${extractError(reason)}`)
        }
    }

    const addArchive = async () => {
        if (!sourcePath) {
            notify('error', '请先选择存档文件或文件夹')
            return
        }
        setBusy(true)
        try {
            const archive = await AddSaveArchive(
                gameId,
                archiveName,
                sourcePath,
                sourceIsDirectory,
                '',
                archiveNote,
            )
            setSourcePath('')
            setArchiveName('')
            setArchiveNote('')
            await refresh()
            notify('success', `已为「${gameName}」登记存档「${archive.name}」`)
        } catch (reason) {
            notify('error', `登记存档失败：${extractError(reason)}`)
        } finally {
            setBusy(false)
        }
    }

    const chooseBackupTarget = async (archive: models.SaveArchive) => {
        try {
            const path = await SelectSaveBackupDirectory(archive.backupDir ?? '')
            if (path) {
                setBackupTargets((current) => ({ ...current, [archive.id]: path }))
            }
        } catch (reason) {
            notify('error', `选择备份目标失败：${extractError(reason)}`)
        }
    }

    const backupArchive = async (archive: models.SaveArchive) => {
        const target = backupTargets[archive.id] || archive.backupDir
        if (!target) {
            notify('error', '请先选择备份目标文件夹')
            return
        }
        setBusyArchiveId(archive.id)
        try {
            await BackupSaveArchive(archive.id, target, backupNotes[archive.id] ?? '')
            await refresh()
            notify('success', `「${archive.name}」备份完成`)
        } catch (reason) {
            notify('error', `备份失败：${extractError(reason)}`)
        } finally {
            setBusyArchiveId(null)
        }
    }

    const confirmDelete = async () => {
        if (!deleteTarget) return
        try {
            if (deleteTarget.kind === 'archive') {
                await DeleteSaveArchive(deleteTarget.id, deleteFiles)
            } else {
                await DeleteSaveBackup(deleteTarget.id, deleteFiles)
            }
            await refresh()
            notify('success', deleteFiles ? '记录与选定文件已删除' : '记录已删除，磁盘文件已保留')
            setDeleteTarget(null)
            setDeleteFiles(false)
        } catch (reason) {
            notify('error', `删除失败：${extractError(reason)}`)
        }
    }

    return (
        <section className="mt-6 border-t border-slate-800 pt-5">
            <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                    <h3 className="text-sm font-semibold text-slate-100">本地游戏存档</h3>
                    <p className="mt-1 text-xs text-slate-500">
                        登记现有文件或文件夹；原始存档不会移动或修改。
                    </p>
                </div>
                <button type="button" className={btnGhost} onClick={() => void refresh()} disabled={loading}>
                    刷新存档
                </button>
            </div>

            <div className="mt-4 rounded-xl border border-slate-800 bg-slate-950/40 p-4">
                <p className="text-xs font-medium text-slate-300">添加存档</p>
                <div className="mt-3 flex flex-wrap gap-2">
                    <button type="button" className={btnGhost} onClick={() => void chooseSource(false)} disabled={busy}>
                        选择存档文件
                    </button>
                    <button type="button" className={btnGhost} onClick={() => void chooseSource(true)} disabled={busy}>
                        选择存档文件夹
                    </button>
                </div>
                {sourcePath ? (
                    <>
                        <p className="mt-2 break-all text-xs text-slate-500">
                            {sourceIsDirectory ? '文件夹' : '文件'}：{sourcePath}
                        </p>
                        <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
                            <div>
                                <label className={labelClass} htmlFor={`archive-name-${gameId}`}>存档名称</label>
                                <input
                                    id={`archive-name-${gameId}`}
                                    className={inputClass}
                                    value={archiveName}
                                    maxLength={255}
                                    onChange={(event) => setArchiveName(event.target.value)}
                                />
                            </div>
                            <div>
                                <label className={labelClass} htmlFor={`archive-note-${gameId}`}>备注</label>
                                <input
                                    id={`archive-note-${gameId}`}
                                    className={inputClass}
                                    value={archiveNote}
                                    maxLength={1024}
                                    onChange={(event) => setArchiveNote(event.target.value)}
                                    placeholder="可选"
                                />
                            </div>
                        </div>
                        <div className="mt-3 flex gap-2">
                            <button type="button" className={btnPrimary} onClick={() => void addArchive()} disabled={busy}>
                                {busy ? '保存中…' : '登记存档'}
                            </button>
                            <button
                                type="button"
                                className={btnGhost}
                                onClick={() => {
                                    setSourcePath('')
                                    setArchiveName('')
                                    setArchiveNote('')
                                }}
                                disabled={busy}
                            >
                                取消
                            </button>
                        </div>
                    </>
                ) : null}
            </div>

            {error ? (
                <p className="mt-3 rounded-lg border border-rose-500/40 bg-rose-500/10 px-3 py-2 text-xs text-rose-200">
                    {error}
                </p>
            ) : null}
            {loading ? <p className="mt-4 text-xs text-slate-500">正在加载存档…</p> : null}
            {!loading && archives.length === 0 ? (
                <p className="mt-4 text-center text-xs text-slate-500">尚未登记存档</p>
            ) : null}

            <div className="mt-4 space-y-3">
                {archives.map((archive) => {
                    const archiveBackups = backups[archive.id] ?? []
                    return (
                        <article key={archive.id} className="rounded-xl border border-slate-800 bg-slate-950/40 p-4">
                            <div className="flex flex-wrap items-start justify-between gap-3">
                                <div className="min-w-0 flex-1">
                                    <h4 className="text-sm font-medium text-slate-200">{archive.name}</h4>
                                    <p className="mt-1 break-all text-xs text-slate-500">{archive.sourcePath}</p>
                                    {archive.note ? <p className="mt-1 text-xs text-slate-400">备注：{archive.note}</p> : null}
                                    <p className="mt-1 text-xs text-slate-500">
                                        最近备份：{archive.lastBackupAt ? formatDateTime(archive.lastBackupAt) : '尚无'}
                                    </p>
                                </div>
                                <button
                                    type="button"
                                    className={btnDanger}
                                    onClick={() => {
                                        setDeleteFiles(false)
                                        setDeleteTarget({ kind: 'archive', id: archive.id, label: archive.name })
                                    }}
                                >
                                    删除存档记录
                                </button>
                            </div>

                            <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-[1fr_auto_auto]">
                                <input
                                    className={inputClass}
                                    value={backupTargets[archive.id] ?? archive.backupDir ?? ''}
                                    onChange={(event) =>
                                        setBackupTargets((current) => ({ ...current, [archive.id]: event.target.value }))
                                    }
                                    placeholder="备份目标文件夹"
                                    aria-label={`${archive.name}备份目标文件夹`}
                                />
                                <button
                                    type="button"
                                    className={btnGhost}
                                    onClick={() => void chooseBackupTarget(archive)}
                                >
                                    选择文件夹
                                </button>
                                <button
                                    type="button"
                                    className={btnPrimary}
                                    onClick={() => void backupArchive(archive)}
                                    disabled={busyArchiveId === archive.id}
                                >
                                    {busyArchiveId === archive.id ? '备份中…' : '立即备份'}
                                </button>
                            </div>
                            <div className="mt-2">
                                <label className={labelClass} htmlFor={`backup-note-${archive.id}`}>本次备份备注</label>
                                <input
                                    id={`backup-note-${archive.id}`}
                                    className={inputClass}
                                    value={backupNotes[archive.id] ?? ''}
                                    maxLength={1024}
                                    onChange={(event) =>
                                        setBackupNotes((current) => ({ ...current, [archive.id]: event.target.value }))
                                    }
                                    placeholder="可选，例如：更新前备份"
                                />
                            </div>

                            <div className="mt-4 border-t border-slate-800 pt-3">
                                <p className="text-xs font-medium text-slate-400">备份历史（{archiveBackups.length}）</p>
                                {archiveBackups.length === 0 ? (
                                    <p className="mt-2 text-xs text-slate-600">暂无备份记录</p>
                                ) : (
                                    <ul className="mt-2 space-y-2">
                                        {archiveBackups.map((backup) => (
                                            <li
                                                key={backup.id}
                                                className="flex flex-wrap items-start justify-between gap-2 rounded-lg bg-slate-900/70 px-3 py-2"
                                            >
                                                <div className="min-w-0 flex-1">
                                                    <p className="text-xs text-slate-300">{formatDateTime(backup.backedUpAt)}</p>
                                                    <p className="mt-1 break-all text-xs text-slate-500">{backup.backupPath}</p>
                                                    {backup.note ? (
                                                        <p className="mt-1 text-xs text-slate-400">备注：{backup.note}</p>
                                                    ) : null}
                                                </div>
                                                <button
                                                    type="button"
                                                    className={btnGhost}
                                                    onClick={() => {
                                                        setDeleteFiles(false)
                                                        setDeleteTarget({
                                                            kind: 'backup',
                                                            id: backup.id,
                                                            label: formatDateTime(backup.backedUpAt),
                                                        })
                                                    }}
                                                >
                                                    删除历史记录
                                                </button>
                                            </li>
                                        ))}
                                    </ul>
                                )}
                            </div>
                        </article>
                    )
                })}
            </div>

            {deleteTarget ? (
                <ConfirmDialog
                    title={deleteTarget.kind === 'archive' ? '删除存档记录' : '删除备份历史'}
                    message={
                        <div className="space-y-3">
                            <p>
                                确定删除「{deleteTarget.label}」的{deleteTarget.kind === 'archive' ? '存档记录' : '备份记录'}吗？
                                默认仅删除数据库记录，不会删除磁盘文件。
                            </p>
                            {deleteTarget.kind === 'archive' ? (
                                <p className="text-xs text-amber-300">
                                    删除存档记录后，其备份历史记录也会移除，但已生成的备份文件会保留在磁盘上。
                                </p>
                            ) : null}
                            <label className="flex cursor-pointer items-start gap-2 text-xs text-rose-200">
                                <input
                                    type="checkbox"
                                    checked={deleteFiles}
                                    onChange={(event) => setDeleteFiles(event.target.checked)}
                                    className="mt-0.5 accent-rose-500"
                                />
                                {deleteTarget.kind === 'archive'
                                    ? '我确认同时删除原始存档文件/文件夹（不可恢复）'
                                    : '我确认同时删除该备份文件/文件夹（不可恢复）'}
                            </label>
                        </div>
                    }
                    confirmText={deleteFiles ? '确认删除记录和文件' : '仅删除记录'}
                    onCancel={() => {
                        setDeleteTarget(null)
                        setDeleteFiles(false)
                    }}
                    onConfirm={confirmDelete}
                />
            ) : null}
        </section>
    )
}
