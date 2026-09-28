import { useEffect, type ComponentType } from 'react'

import Sidebar from './components/Sidebar'
import Toaster from './components/Toaster'
import CategoriesPage from './pages/CategoriesPage'
import Dashboard from './pages/Dashboard'
import GameLibrary from './pages/GameLibrary'
import SettingsPage from './pages/SettingsPage'
import { useAppStore, type Route } from './stores/appStore'

const PAGES: Record<Route, ComponentType> = {
    dashboard: Dashboard,
    library: GameLibrary,
    categories: CategoriesPage,
    settings: SettingsPage,
}

export default function App() {
    const route = useAppStore((state) => state.route)
    const loading = useAppStore((state) => state.loading)
    const bootstrap = useAppStore((state) => state.bootstrap)

    useEffect(() => {
        void bootstrap()
    }, [bootstrap])

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
                    <Page />
                )}
            </main>
            <Toaster />
        </div>
    )
}