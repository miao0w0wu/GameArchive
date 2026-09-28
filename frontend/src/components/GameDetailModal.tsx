import { useEffect, useState } from 'react'

import { GetGameDetail } from '../../wailsjs/go/main/App'
import { services } from '../../wailsjs/go/models'
import { formatDateTime, formatDuration } from '../lib/format'
import { extractError } from '../stores/appStore'
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

    useEffect(() => {
        let cancelled = false

        GetGameDetail(gameId)
            .then((result) => {
                if (!cancelled) {
                    setDetail(result)
                }
            })
            .catch((err: unknown) => {
                if (!cancelled) {
                    setError(extractError(err))
                }
            })

        return () => {
            cancelled = true
        }
    }, [gameId])

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
                    <div className="mb-4 flex items-start justify-between gap-4">
                        <div>
                            <h3 className="text-lg font-semibold text-slate-100">{detail.name}</h3>
                            <p className="mt-1 text-xs text-slate-500">
                                累计时长 {formatDuration(detail.totalSeconds)}（{detail.totalHours} 小时）
                            </p>
                        </div>
                    </div>

                    <dl>
                        <Field label="分类" value={detail.category?.name ?? '未分类'} />
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