import { useAppStore } from '../stores/appStore'
import { IconClose, IconInfo } from './Icons'

const kindClass: Record<string, string> = {
    success: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-200',
    error: 'border-rose-500/40 bg-rose-500/10 text-rose-200',
    info: 'border-sky-500/40 bg-sky-500/10 text-sky-200',
}

/** 右下角浮层提示。 */
export default function Toaster() {
    const toasts = useAppStore((state) => state.toasts)
    const dismissToast = useAppStore((state) => state.dismissToast)

    if (toasts.length === 0) {
        return null
    }

    return (
        <div className="pointer-events-none fixed right-5 bottom-5 z-50 flex w-80 flex-col gap-2">
            {toasts.map((toast) => (
                <div
                    key={toast.id}
                    className={`pointer-events-auto flex items-start gap-3 rounded-xl border px-4 py-3 text-sm shadow-lg shadow-black/30 backdrop-blur ${
                        kindClass[toast.kind] ?? kindClass.info
                    }`}
                >
                    <IconInfo className="mt-0.5 shrink-0" />
                    <span className="flex-1 leading-relaxed break-all">{toast.message}</span>
                    <button
                        type="button"
                        onClick={() => dismissToast(toast.id)}
                        className="shrink-0 opacity-70 transition hover:opacity-100"
                        aria-label="关闭提示"
                    >
                        <IconClose width={14} height={14} />
                    </button>
                </div>
            ))}
        </div>
    )
}