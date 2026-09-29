import { useEffect, useState } from 'react'

import {
    AutoFetchCover,
    AutoFetchIcon,
    ClearCover,
    GetGameDetail,
    SelectAndSetCover,
} from '../../wailsjs/go/main/App'
import { services } from '../../wailsjs/go/models'
import GameImage from './GameImage'
import { btnGhost } from '../lib/ui'
import { formatDateTime, formatDuration } from '../lib/format'
import { extractError, useAppStore } from '../stores/appStore'
import Modal from './Modal'
import SaveArchivesPanel from './SaveArchivesPanel'

interface GameDetailModalProps {
    gameId: number
    onClose: () => void
}

function Field({ label, value }: { label: string; value: string }) {
    return (
        <div className="border-b border-slate-800/70 py-2.5">
            <dt className="text-xs text-slate-500">{label}</dt>
            <dd className="mt-1 text-sm break-all text-slate-200">{value || '—'}</dd>
        </div>
    )
}

/** 游戏详情弹窗，数据来自 Go 侧的 GetGameDetail。 */
export default function GameDetailModal({ gameId, onClose }: GameDetailModalProps) {
    const [detail, setDetail] = useState<services.GameDetail | null>(null)
    const [error, setError] = useState('')
    const [busyTarget, setBusyTarget] = useState<'cover' | 'icon' | ''>('')
    const refreshAll = useAppStore((state) => state.refreshAll)
    const notify = useAppStore((state) => state.notify)

    const loadDetail = () =>
        GetGameDetail(gameId)
            .then((result) => {
                setDetail(result)
                setError('')
            })
            .catch((err: unknown) => {
                setError(extractError(err))
            })

    useEffect(() => {
        let cancelled = false
        GetGameDetail(gameId)
            .then((result) => {
                if (!cancelled) setDetail(result)
            })
            .catch((err: unknown) => {
                if (!cancelled) setError(extractError(err))
            })
        return () => {
            cancelled = true
        }
    }, [gameId])

    const runImageAction = async (target: 'cover' | 'icon', action: 'select' | 'auto' | 'clear') => {
        setBusyTarget(target)
        setError('')
        try {
            if (action === 'select') {
                const path = await SelectAndSetCover(gameId, target)
                if (!path) return
            } else if (action === 'auto') {
                const fetched = target === 'cover' ? await AutoFetchCover(gameId) : await AutoFetchIcon(gameId)
                if (!fetched.found) {
                    notify('info', fetched.reason || '未找到匹配图片')
                    return
                }
                notify('success', `已更新${target === 'cover' ? '封面' : '图标'}（${fetched.source}）`)
            } else {
                await ClearCover(gameId, target)
                notify('info', `已清除${target === 'cover' ? '封面' : '图标'}，将显示默认图片`)
            }
            await loadDetail()
            await refreshAll()
        } catch (reason) {
            const message = extractError(reason)
            setError(message)
            notify('error', `更新${target === 'cover' ? '封面' : '图标'}失败：${message}`)
        } finally {
            setBusyTarget('')
        }
    }

    return (
        <Modal title="游戏详情" onClose={onClose} widthClass="max-w-3xl">
            {error ? (
                <p className="rounded-lg border border-rose-500/40 bg-rose-500/10 px-3 py-2 text-xs text-rose-200">
                    {error}
                </p>
            ) : null}

            {!detail && !error ? (
                <p className="text-sm text-slate-400">加载中…</p>
            ) : null}

            {detail ? (
                <div>
                    <div className="mb-5 grid grid-cols-1 gap-4 sm:grid-cols-[minmax(0,1fr)_12rem]">
                        <div>
                            <p className="mb-2 text-xs text-slate-500">游戏封面</p>
                            <GameImage
                                gameId={detail.id}
                                name={detail.name}
                                target="cover"
                                imageVersion={`${detail.coverUpdatedAt ?? ''}${detail.iconUpdatedAt ?? ''}`}
                                className="aspect-[2/3] max-h-80 w-full rounded-xl object-cover"
                            />
                            <div className="mt-2 flex flex-wrap gap-2">
                                <button type="button" className={btnGhost} disabled={busyTarget !== ''} onClick={() => void runImageAction('cover', 'select')}>上传封面</button>
                                <button type="button" className={btnGhost} disabled={busyTarget !== ''} onClick={() => void runImageAction('cover', 'auto')}>自动获取</button>
                                <button type="button" className={btnGhost} disabled={busyTarget !== ''} onClick={() => void runImageAction('cover', 'clear')}>恢复默认</button>
                            </div>
                            <p className="mt-2 text-xs text-slate-500">
                                来源：{detail.coverSource || 'none'}
                                {detail.coverUpdatedAt ? ` · 更新于 ${formatDateTime(detail.coverUpdatedAt)}` : ''}
                            </p>
                        </div>
                        <div>
                            <p className="mb-2 text-xs text-slate-500">游戏图标</p>
                            <GameImage
                                gameId={detail.id}
                                name={detail.name}
                                target="icon"
                                imageVersion={`${detail.coverUpdatedAt ?? ''}${detail.iconUpdatedAt ?? ''}`}
                                className="mx-auto aspect-square w-36 rounded-xl object-cover"
                            />
                            <div className="mt-2 flex flex-wrap justify-center gap-2">
                                <button type="button" className={btnGhost} disabled={busyTarget !== ''} onClick={() => void runImageAction('icon', 'select')}>上传图标</button>
                                <button type="button" className={btnGhost} disabled={busyTarget !== ''} onClick={() => void runImageAction('icon', 'auto')}>自动获取</button>
                                <button type="button" className={btnGhost} disabled={busyTarget !== ''} onClick={() => void runImageAction('icon', 'clear')}>恢复默认</button>
                            </div>
                            <p className="mt-2 text-center text-xs text-slate-500">
                                来源：{detail.iconSource || 'none'}
                                {detail.iconUpdatedAt ? ` · 更新于 ${formatDateTime(detail.iconUpdatedAt)}` : ''}
                            </p>
                        </div>
                    </div>
                    <div className="mb-4 flex items-start justify-between gap-4">
                        <div>
                            <h3 className="text-lg font-semibold text-slate-100">{detail.name}</h3>
                            <p className="mt-1 text-xs text-slate-500">
                                本地累计 {formatDuration(detail.totalSeconds)}（{detail.totalHours} 小时）
                                {detail.steamPlaytimeSeconds > 0 ? (
                                    <span className="ml-2">· Steam {formatDuration(detail.steamPlaytimeSeconds)}</span>
                                ) : null}
                            </p>
                        </div>
                    </div>

                    <dl>
                        <Field label="分类" value={detail.category?.name ?? '未分类'} />
                        <Field label="Steam AppID" value={detail.steamAppId ? String(detail.steamAppId) : ''} />
                        <Field
                            label="标签"
                            value={detail.tagNames.length > 0 ? detail.tagNames.join('、') : '无'}
                        />
                        <Field
                            label="最后游玩日期"
                            value={
                                detail.lastPlayedAt
                                    ? formatDateTime(detail.lastPlayedAt)
                                    : '暂无记录（阶段 2 开始统计）'
                            }
                        />
                        <Field label="可执行文件路径" value={detail.exePath} />
                        <Field label="进程名" value={detail.processName} />
                        <Field label="安装目录" value={detail.installDir} />
                        <Field label="封面图片路径" value={detail.coverPath} />
                        <Field label="创建时间" value={formatDateTime(detail.createdAt)} />
                        <Field label="更新时间" value={formatDateTime(detail.updatedAt)} />
                    </dl>
                    <SaveArchivesPanel gameId={detail.id} gameName={detail.name} />
                </div>
            ) : null}
        </Modal>
    )
}