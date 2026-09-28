import { useEffect, type ReactNode } from 'react'

import { IconClose } from './Icons'

interface ModalProps {
    title: string
    onClose: () => void
    children: ReactNode
    footer?: ReactNode
    /** 弹窗最大宽度类名，默认 max-w-2xl。 */
    widthClass?: string
}

/** 通用弹窗：支持 Esc 关闭、点击遮罩关闭。 */
export default function Modal({
    title,
    onClose,
    children,
    footer,
    widthClass = 'max-w-2xl',
}: ModalProps) {
    useEffect(() => {
        const handler = (event: KeyboardEvent) => {
            if (event.key === 'Escape') {
                onClose()
            }
        }
        window.addEventListener('keydown', handler)
        return () => window.removeEventListener('keydown', handler)
    }, [onClose])

    return (
        <div
            className="fixed inset-0 z-40 flex items-center justify-center bg-slate-950/70 p-6 backdrop-blur-sm"
            onMouseDown={onClose}
        >
            <div
                className={`flex max-h-[88vh] w-full ${widthClass} flex-col overflow-hidden rounded-2xl border border-slate-800 bg-slate-900 shadow-2xl shadow-black/40`}
                onMouseDown={(event) => event.stopPropagation()}
            >
                <header className="flex items-center justify-between border-b border-slate-800 px-5 py-4">
                    <h2 className="text-base font-semibold text-slate-100">{title}</h2>
                    <button
                        type="button"
                        onClick={onClose}
                        className="rounded-lg p-1.5 text-slate-400 transition hover:bg-slate-800 hover:text-slate-100"
                        aria-label="关闭"
                    >
                        <IconClose />
                    </button>
                </header>

                <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>

                {footer ? (
                    <footer className="flex justify-end gap-3 border-t border-slate-800 bg-slate-950/40 px-5 py-4">
                        {footer}
                    </footer>
                ) : null}
            </div>
        </div>
    )
}