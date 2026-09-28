/**
 * 全局状态（Zustand）：页面路由、游戏/分类/标签数据、提示消息。
 *
 * 所有数据都通过 Wails 生成的绑定方法从 Go 侧读取，前端不做持久化。
 */
import { create } from 'zustand'

import {
    AddCategory,
    AddGame,
    AddTag,
    DeleteCategory,
    DeleteGame,
    DeleteTag,
    GetAppInfo,
    GetDashboardStats,
    GetMonitorStatus,
    ListCategories,
    ListGames,
    ListReports,
    ListTags,
    UpdateCategory,
    UpdateGame,
    UpdateTag,
    StartMonitor,
    StopMonitor,
} from '../../wailsjs/go/main/App'
import { main, models, services } from '../../wailsjs/go/models'

/** 应用内页面标识。 */
export type Route = 'dashboard' | 'library' | 'stats' | 'categories' | 'settings'

export type ToastKind = 'success' | 'error' | 'info'

export interface Toast {
    id: number
    kind: ToastKind
    message: string
}

/** 新增 / 编辑游戏的表单数据。 */
export interface GameInput {
    name: string
    exePath: string
    processName: string
    installDir: string
    coverPath: string
    categoryId: number | null
    tagIds: number[]
}

export interface CategoryInput {
    name: string
    color: string
    sortOrder: number
}

export interface TagInput {
    name: string
    color: string
}

/** 把 Wails 抛出的错误转换成可展示的文案。 */
export function extractError(error: unknown): string {
    if (typeof error === 'string') {
        return error
    }
    if (error instanceof Error) {
        return error.message
    }
    return error ? String(error) : '未知错误'
}

let toastSeq = 0

interface AppState {
    route: Route
    /** 首次加载中。 */
    loading: boolean
    /** 刷新中（用于按钮禁用等）。 */
    refreshing: boolean
    games: models.Game[]
    categories: models.Category[]
    tags: models.Tag[]
    /** 报告列表（不含正文）。 */
    reports: models.Report[]
    appInfo: main.AppInfo | null
    monitorStatus: services.MonitorStatus | null
    /** 仪表盘汇总统计（总时长、今日 / 本周 / 本月、Top 游戏、分类占比）。 */
    dashboardStats: services.StatsOverview | null
    toasts: Toast[]

    navigate: (route: Route) => void
    notify: (kind: ToastKind, message: string) => void
    dismissToast: (id: number) => void

    bootstrap: () => Promise<void>
    refreshAll: () => Promise<void>
    refreshTrackedGames: () => Promise<void>
    refreshDashboardStats: () => Promise<void>
    loadReports: () => Promise<void>
    setMonitorEnabled: (enabled: boolean) => Promise<void>

    saveGame: (input: GameInput, id?: number) => Promise<boolean>
    removeGame: (id: number) => Promise<void>

    saveCategory: (input: CategoryInput, id?: number) => Promise<boolean>
    removeCategory: (id: number) => Promise<void>

    saveTag: (input: TagInput, id?: number) => Promise<boolean>
    removeTag: (id: number) => Promise<void>
}

export const useAppStore = create<AppState>((set, get) => ({
    route: 'dashboard',
    loading: true,
    refreshing: false,
    games: [],
    categories: [],
    tags: [],
    reports: [],
    appInfo: null,
    monitorStatus: null,
    dashboardStats: null,
    toasts: [],

    navigate: (route) => set({ route }),

    notify: (kind, message) => {
        const toast: Toast = { id: (toastSeq += 1), kind, message }
        set((state) => ({ toasts: [...state.toasts, toast] }))
        // 提示自动消失，避免堆积
        window.setTimeout(() => get().dismissToast(toast.id), 3600)
    },

    dismissToast: (id) =>
        set((state) => ({ toasts: state.toasts.filter((toast) => toast.id !== id) })),

    bootstrap: async () => {
        set({ loading: true })
        await get().refreshAll()
        set({ loading: false })
    },

    refreshAll: async () => {
        set({ refreshing: true })
        try {
            const [games, categories, tags, reports, appInfo, monitorStatus, dashboardStats] =
                await Promise.all([
                    ListGames(),
                    ListCategories(),
                    ListTags(),
                    ListReports(),
                    GetAppInfo(),
                    GetMonitorStatus(),
                    GetDashboardStats(),
                ])
            set({ games, categories, tags, reports, appInfo, monitorStatus, dashboardStats })
        } catch (error) {
            get().notify('error', `加载数据失败：${extractError(error)}`)
        } finally {
            set({ refreshing: false })
        }
    },

    refreshTrackedGames: async () => {
        try {
            const games = await ListGames()
            set({ games })
        } catch (error) {
            get().notify('error', `刷新游玩时长失败：${extractError(error)}`)
        }
    },

    refreshDashboardStats: async () => {
        try {
            const dashboardStats = await GetDashboardStats()
            set({ dashboardStats })
        } catch (error) {
            get().notify('error', `刷新统计数据失败：${extractError(error)}`)
        }
    },

    loadReports: async () => {
        try {
            const reports = await ListReports()
            set({ reports })
        } catch (error) {
            get().notify('error', `加载报告列表失败：${extractError(error)}`)
        }
    },

    setMonitorEnabled: async (enabled) => {
        try {
            if (enabled) {
                await StartMonitor()
                get().notify('success', '进程监控已开启')
            } else {
                await StopMonitor()
                get().notify('success', '进程监控已暂停，当前会话已收尾')
            }
            await get().refreshAll()
        } catch (error) {
            get().notify('error', `更新监控状态失败：${extractError(error)}`)
            await get().refreshAll()
        }
    },

    saveGame: async (input, id) => {
        try {
            const payload = models.Game.createFrom({
                id: id ?? 0,
                name: input.name,
                exePath: input.exePath,
                processName: input.processName,
                installDir: input.installDir,
                coverPath: input.coverPath,
                categoryId: input.categoryId ?? undefined,
                // 只传 id，Go 侧按 id 关联已存在的标签
                tags: input.tagIds.map((tagId) => ({ id: tagId })),
            })

            if (id) {
                await UpdateGame(payload)
                get().notify('success', '游戏已更新')
            } else {
                await AddGame(payload)
                get().notify('success', '游戏已添加')
            }
            await get().refreshAll()
            return true
        } catch (error) {
            get().notify('error', extractError(error))
            return false
        }
    },

    removeGame: async (id) => {
        try {
            await DeleteGame(id)
            get().notify('success', '游戏已删除')
            await get().refreshAll()
        } catch (error) {
            get().notify('error', extractError(error))
        }
    },

    saveCategory: async (input, id) => {
        try {
            const payload = models.Category.createFrom({
                id: id ?? 0,
                name: input.name,
                color: input.color,
                sortOrder: input.sortOrder,
            })

            if (id) {
                await UpdateCategory(payload)
                get().notify('success', '分类已更新')
            } else {
                await AddCategory(payload)
                get().notify('success', '分类已添加')
            }
            await get().refreshAll()
            return true
        } catch (error) {
            get().notify('error', extractError(error))
            return false
        }
    },

    removeCategory: async (id) => {
        try {
            await DeleteCategory(id)
            get().notify('success', '分类已删除，相关游戏已变为未分类')
            await get().refreshAll()
        } catch (error) {
            get().notify('error', extractError(error))
        }
    },

    saveTag: async (input, id) => {
        try {
            const payload = models.Tag.createFrom({
                id: id ?? 0,
                name: input.name,
                color: input.color,
            })

            if (id) {
                await UpdateTag(payload)
                get().notify('success', '标签已更新')
            } else {
                await AddTag(payload)
                get().notify('success', '标签已添加')
            }
            await get().refreshAll()
            return true
        } catch (error) {
            get().notify('error', extractError(error))
            return false
        }
    },

    removeTag: async (id) => {
        try {
            await DeleteTag(id)
            get().notify('success', '标签已删除')
            await get().refreshAll()
        } catch (error) {
            get().notify('error', extractError(error))
        }
    },
}))