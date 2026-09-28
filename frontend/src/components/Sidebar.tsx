import type { ReactNode } from 'react'

import { useAppStore, type Route } from '../stores/appStore'
import { IconChart, IconDashboard, IconLibrary, IconSettings, IconTags } from './Icons'

interface NavItem {
    route: Route
    label: string
    description: string
    icon: ReactNode
}

const NAV_ITEMS: NavItem[] = [
    { route: 'dashboard', label: '仪表盘', description: '总览', icon: <IconDashboard /> },
    { route: 'library', label: '游戏库', description: '游戏增删改查', icon: <IconLibrary /> },
    { route: 'stats', label: '统计报告', description: '年 / 月 / 周分析', icon: <IconChart /> },
    { route: 'categories', label: '分类与标签', description: '分类 / 标签管理', icon: <IconTags /> },
    { route: 'settings', label: '设置', description: '数据目录', icon: <IconSettings /> },
]

/** 左侧导航栏。 */
export default function Sidebar() {
    const route = useAppStore((state) => state.route)
    const navigate = useAppStore((state) => state.navigate)
    const games = useAppStore((state) => state.games)
    const categories = useAppStore((state) => state.categories)
    const appInfo = useAppStore((state) => state.appInfo)

    const badge = (item: Route): number | null => {
        if (item === 'library') {
            return games.length
        }
        if (item === 'categories') {
            return categories.length
        }
        return null
    }

    return (
        <aside className="flex w-60 shrink-0 flex-col border-r border-slate-800 bg-slate-900/70">
            <div className="flex items-center gap-3 px-5 py-5">
                <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500 to-fuchsia-500 text-lg font-bold text-white">
                    游
                </div>
                <div>
                    <p className="text-sm font-semibold text-slate-100">游戏档案</p>
                    <p className="text-xs text-slate-500">游玩时长统计</p>
                </div>
            </div>

            <nav className="flex-1 space-y-1 px-3 py-2">
                {NAV_ITEMS.map((item) => {
                    const active = route === item.route
                    const count = badge(item.route)

                    return (
                        <button
                            key={item.route}
                            type="button"
                            onClick={() => navigate(item.route)}
                            className={`flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left transition ${
                                active
                                    ? 'bg-indigo-500/15 text-indigo-200'
                                    : 'text-slate-400 hover:bg-slate-800/60 hover:text-slate-200'
                            }`}
                        >
                            <span className={active ? 'text-indigo-300' : 'text-slate-500'}>
                                {item.icon}
                            </span>
                            <span className="flex-1">
                                <span className="block text-sm font-medium">{item.label}</span>
                                <span className="block text-xs text-slate-500">{item.description}</span>
                            </span>
                            {count !== null && count > 0 ? (
                                <span className="rounded-full bg-slate-800 px-2 py-0.5 text-xs text-slate-300">
                                    {count}
                                </span>
                            ) : null}
                        </button>
                    )
                })}
            </nav>

            <div className="border-t border-slate-800 px-5 py-4 text-xs text-slate-500">
                <p>版本 {appInfo?.version ?? '0.1.0'}</p>
                <p className="mt-1 leading-relaxed text-slate-600">
                                                    阶段 4：AI 报告 / 图片导出
                </p>
            </div>
        </aside>
    )
}