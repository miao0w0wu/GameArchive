import { useState, type ReactNode } from 'react'

import { btnDanger, btnGhost } from '../lib/ui'
import Modal from './Modal'

interface ConfirmDialogProps {
    title: string
    message: ReactNode
    confirmText?: string
    onConfirm: () => void | Promise<void>
    onCancel: () => void
}

/** 危险操作确认弹窗（删除游戏 / 分类 / 标签）。 */
export default function ConfirmDialog({
    title,
    message,
    confirmText = '确认删除',
    onConfirm,
    onCancel,
}: ConfirmDialogProps) {
    const [busy, setBusy] = useState(false)

    const handleConfirm = async () => {
        setBusy(true)
        try {
            await onConfirm()
        } finally {
            setBusy(false)
        }
    }

    return (
        <Modal
            title={title}
            onClose={onCancel}
            widthClass="max-w-md"
            footer={
                <>
                    <button type="button" className={btnGhost} onClick={onCancel} disabled={busy}>
                        取消
                    </button>
                    <button
                        type="button"
                        className={btnDanger}
                        onClick={handleConfirm}
                        disabled={busy}
                    >
                        {busy ? '处理中…' : confirmText}
                    </button>
                </>
            }
        >
            <div className="text-sm leading-relaxed text-slate-300">{message}</div>
        </Modal>
    )
}