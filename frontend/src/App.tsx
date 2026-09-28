import { lazy, Suspense, useEffect, type ComponentType } from 'react'

import Sidebar from './components/Sidebar'
import Toaster from './components/Toaster'
import CategoriesPage from './pages/CategoriesPage'
import Dashboard from './pages/Dashboard'
import GameLibrary from './pages/GameLibrary'
import SettingsPage from './pages/SettingsPage'
import { useAppStore, type Route } from './stores/appStore'
import { EventsOn } from '../wailsjs/runtime/runtime'
import type { services } from '../wailsjs/go/models'

// 统计报告页依赖 ECharts，单独分包按需加载，避免拖慢应用启动
const StatsPage = lazy(() => import('./pages/StatsPage'))

const PAGES: Record<Route, ComponentType> = {
    dashboard: Dashboard,
    library: GameLibrary,
    stats: StatsPage,
    categories: CategoriesPage,
    settings: SettingsPage,
}

export default function App() {
    const route = useAppStore((state) => state.route)
    const loading = useAppStore((state) => state.loading)
    const bootstrap = useAppStore((state) => state.bootstrap)
    const refreshTrackedGames = useAppStore((state) => state.refreshTrackedGames)
        const refreshDashboardStats = useAppStore((state) => state.refreshDashboardStats)

        useEffect(() => {
            void bootstrap()
        }, [bootstrap])

        useEffect(() => {
            const unsubscribe = EventsOn('tracker:update', (status: services.MonitorStatus) => {
                useAppStore.setState({ monitorStatus: status })
                void refreshTrackedGames()
                // 游玩时长有变化时同步刷新仪表盘的统计卡片
                void refreshDashboardStats()
            })
            return unsubscribe
        }, [refreshTrackedGames, refreshDashboardStats])

    const Page = PAGES[route]

    return (
        <div className="flex h-screen w-screen overflow-hidden">
            <Sidebar />
            <main className="flex-1 overflow-y-auto bg-slate-950">
                {loading ? (
                    <div className="flex h-full items-center justify-center text-sm text-slate-500">
                        正在加载数据…
                    </div>
                ) : (
                                    <Suspense
                                        fallback={
                                            <div className="flex h-full items-center justify-center text-sm text-slate-500">
                                                正在加载统计图表…
                                            </div>
                                        }
                                    >
                                        <Page />
                                    </Suspense>
                                )}
            </main>
            <Toaster />
        </div>
    )
}