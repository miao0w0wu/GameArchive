import type { ReactNode } from 'react'

import { cardClass } from '../lib/ui'

interface StatCardProps {
    label: string
    value: string
    hint?: string
    icon?: ReactNode
    accent?: string
}

/** 仪表盘统计卡片。 */
export default function StatCard({ label, value, hint, icon, accent = '#6366f1' }: StatCardProps) {
    return (
        <div className={`${cardClass} p-5`}>
            <div className="flex items-center justify-between">
                <span className="text-xs font-medium tracking-wide text-slate-400">{label}</span>
                {icon ? (
                    <span
                        className="flex h-8 w-8 items-center justify-center rounded-lg"
                        style={{ backgroundColor: `${accent}22`, color: accent }}
                    >
                        {icon}
                    </span>
                ) : null}
            </div>
            <p className="mt-3 text-2xl font-semibold text-slate-100">{value}</p>
            {hint ? <p className="mt-1 text-xs text-slate-500">{hint}</p> : null}
        </div>
    )
}