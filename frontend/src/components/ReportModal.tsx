import { useEffect, useMemo, useRef, useState } from 'react'

import { DeleteReport, ExportReportMarkdown, GetReport } from '../../wailsjs/go/main/App'
import type { models } from '../../wailsjs/go/models'
import { exportElementAsImage, imageExportName } from '../lib/exportImage'
import { formatDateTime } from '../lib/format'
import { renderMarkdown } from '../lib/markdown'
import { btnDanger, btnGhost, btnPrimary } from '../lib/ui'
import { extractError, useAppStore } from '../stores/appStore'
import ConfirmDialog from './ConfirmDialog'
import Modal from './Modal'

interface ReportModalProps {
    /** 报告 ID，打开时按 ID 拉取正文。 */
    reportId: number
    onClose: () => void
    onDeleted?: () => void
}

const SOURCE_LABELS: Record<string, string> = {
    ai: 'AI 生成',
    import: '外部导入',
    manual: '手动新建',
}

/** 报告详情弹窗：查看 Markdown 正文，并支持导出 Markdown / 图片与删除。 */
export default function ReportModal({ reportId, onClose, onDeleted }: ReportModalProps) {
    const notify = useAppStore((state) => state.notify)
    const loadReports = useAppStore((state) => state.loadReports)

    const contentRef = useRef<HTMLDivElement>(null)
    const [report, setReport] = useState<models.Report | null>(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [busy, setBusy] = useState(false)
    const [confirmDelete, setConfirmDelete] = useState(false)

    useEffect(() => {
        let cancelled = false
        setLoading(true)
        GetReport(reportId)
            .then((data) => {
                if (!cancelled) {
                    setReport(data)
                    setError('')
                }
            })
            .catch((reason: unknown) => {
                if (!cancelled) {
                    const message = extractError(reason)
                    setError(message)
                    notify('error', `加载报告失败：${message}`)
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
    }, [reportId, notify])

    const html = useMemo(() => renderMarkdown(report?.content ?? ''), [report?.content])

    const exportMarkdown = async () => {
        if (!report) return
        setBusy(true)
        try {
            const path = await ExportReportMarkdown(report.id)
            if (path) {
                notify('success', `报告已导出：${path}`)
            }
        } catch (reason) {
            notify('error', `导出报告失败：${extractError(reason)}`)
        } finally {
            setBusy(false)
        }
    }

    const exportImage = async () => {
        const element = contentRef.current
        if (!element || !report) return
        setBusy(true)
        try {
            const result = await exportElementAsImage(element, imageExportName(`report-${report.id}`))
            if (result.saved) {
                notify('success', `图片已导出：${result.path}`)
            }
        } catch (reason) {
            notify('error', `导出图片失败：${extractError(reason)}`)
        } finally {
            setBusy(false)
        }
    }

    const remove = async () => {
        if (!report) return
        try {
            await DeleteReport(report.id)
            notify('success', '报告已删除')
            await loadReports()
            onDeleted?.()
            onClose()
        } catch (reason) {
            notify('error', `删除报告失败：${extractError(reason)}`)
        }
    }

    return (
        <>
            <Modal
                title={report?.title ?? '报告详情'}
                onClose={onClose}
                widthClass="max-w-3xl"
                footer={
                    <>
                        <button
                            type="button"
                            className={`${btnDanger} mr-auto`}
                            onClick={() => setConfirmDelete(true)}
                            disabled={busy || !report}
                        >
                            删除报告
                        </button>
                        <button
                            type="button"
                            className={btnGhost}
                            onClick={() => void exportImage()}
                            disabled={busy || !report}
                        >
                            导出图片
                        </button>
                        <button
                            type="button"
                            className={btnGhost}
                            onClick={() => void exportMarkdown()}
                            disabled={busy || !report}
                        >
                            导出 Markdown
                        </button>
                        <button type="button" className={btnPrimary} onClick={onClose} disabled={busy}>
                            关闭
                        </button>
                    </>
                }
            >
                {loading ? (
                    <div className="py-10 text-center text-sm text-slate-500">正在加载报告…</div>
                ) : error ? (
                    <div className="rounded-xl border border-rose-500/40 bg-rose-500/10 px-4 py-3 text-sm text-rose-200">
                        {error}
                    </div>
                ) : (
                    <div ref={contentRef} className="rounded-xl bg-slate-950/40 p-5">
                        <p className="mb-1 text-xs text-slate-500">
                            {SOURCE_LABELS[report?.source ?? ''] ?? '报告'}
                            {report?.periodType ? ` · ${report.periodType}` : ''}
                            {report?.createdAt ? ` · ${formatDateTime(report.createdAt)}` : ''}
                        </p>
                        <div className="markdown-body" dangerouslySetInnerHTML={{ __html: html }} />
                    </div>
                )}
            </Modal>

            {confirmDelete ? (
                <ConfirmDialog
                    title="删除报告"
                    message={<>确定要删除「{report?.title}」吗？删除后无法恢复。</>}
                    onCancel={() => setConfirmDelete(false)}
                    onConfirm={remove}
                />
            ) : null}
        </>
    )
}