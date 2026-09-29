import { useEffect, useState } from 'react'

import {
    GetSteamSettings,
    GetSteamSyncStatus,
    SaveSteamSettings,
    SyncSteamLibrary,
} from '../../wailsjs/go/main/App'
import { services } from '../../wailsjs/go/models'
import { btnGhost, btnPrimary, cardClass, inputClass, labelClass } from '../lib/ui'
import { formatDuration } from '../lib/format'
import { extractError, useAppStore } from '../stores/appStore'

function formatDateTime(value?: string | number | Date) {
    if (!value) return '—'
    const date = value instanceof Date ? value : new Date(value)
    return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString()
}

/** Steam 设置卡片：管理本机 API 凭据、手动同步和最近一次同步状态。 */
export default function SteamSettingsCard() {
    const notify = useAppStore((state) => state.notify)
    const refreshAll = useAppStore((state) => state.refreshAll)
    const [apiKey, setApiKey] = useState('')
    const [steamId, setSteamId] = useState('')
    const [status, setStatus] = useState<services.SteamSyncStatus | null>(null)
    const [showKey, setShowKey] = useState(false)
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [syncing, setSyncing] = useState(false)
    const [error, setError] = useState('')

    useEffect(() => {
        let cancelled = false
        Promise.all([GetSteamSettings(), GetSteamSyncStatus()])
            .then(([config, syncStatus]) => {
                if (cancelled) return
                setApiKey(config.apiKey ?? '')
                setSteamId(config.steamId ?? '')
                setStatus(syncStatus)
            })
            .catch((reason: unknown) => {
                if (!cancelled) setError(extractError(reason))
            })
            .finally(() => {
                if (!cancelled) setLoading(false)
            })
        return () => {
            cancelled = true
        }
    }, [])

    const save = async () => {
        setSaving(true)
        setError('')
        try {
            await SaveSteamSettings({
                apiKey: apiKey.trim(),
                steamId: steamId.trim(),
            })
            notify('success', 'Steam 配置已保存')
        } catch (reason) {
            const message = extractError(reason)
            setError(message)
            notify('error', `保存 Steam 配置失败：${message}`)
        } finally {
            setSaving(false)
        }
    }

    const sync = async () => {
        setSyncing(true)
        setError('')
        let operation = '保存配置'
        try {
            await SaveSteamSettings({
                apiKey: apiKey.trim(),
                steamId: steamId.trim(),
            })
            operation = '同步游戏库'
            const result: services.SteamSyncResult = await SyncSteamLibrary()
            setStatus(services.SteamSyncStatus.createFrom({
                lastAttemptAt: String(result.syncedAt),
                lastError: '',
                lastResult: result,
            }))
            notify('success', `Steam 同步完成：匹配 ${result.matchedByAppId + result.matchedByName} 个，新增 ${result.importedGames} 个`)
            await refreshAll()
        } catch (reason) {
            const message = extractError(reason)
            setError(message)
            try {
                setStatus(await GetSteamSyncStatus())
            } catch (statusReason) {
                setError(`${message}；读取同步状态失败：${extractError(statusReason)}`)
            }
            notify('error', `Steam ${operation}失败：${message}`)
        } finally {
            setSyncing(false)
        }
    }

    const busy = loading || saving || syncing

    return (
        <section className={`${cardClass} mt-5 p-5`}>
            <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                    <h2 className="text-sm font-semibold text-slate-200">Steam 平台数据</h2>
                    <p className="mt-1 text-xs text-slate-500">
                        保存 Steam Web API Key 与 SteamID64 后，可手动同步游戏库和累计游玩时长。
                    </p>
                </div>
                <div className="flex gap-2">
                    <button type="button" className={btnGhost} onClick={() => void sync()} disabled={busy}>
                        {syncing ? '同步中…' : '立即同步'}
                    </button>
                    <button type="button" className={btnPrimary} onClick={() => void save()} disabled={busy}>
                        {saving ? '保存中…' : '保存配置'}
                    </button>
                </div>
            </div>

            <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                    <label className={labelClass} htmlFor="steam-api-key">Steam API Key</label>
                    <div className="flex gap-2">
                        <input
                            id="steam-api-key"
                            type={showKey ? 'text' : 'password'}
                            className={`${inputClass} font-mono text-xs`}
                            value={apiKey}
                            onChange={(event) => setApiKey(event.target.value)}
                            autoComplete="off"
                            disabled={loading}
                        />
                        <button type="button" className={btnGhost} onClick={() => setShowKey((current) => !current)} disabled={loading}>
                            {showKey ? '隐藏' : '显示'}
                        </button>
                    </div>
                </div>
                <div>
                    <label className={labelClass} htmlFor="steam-id">SteamID64</label>
                    <input
                        id="steam-id"
                        className={`${inputClass} font-mono text-xs`}
                        value={steamId}
                        onChange={(event) => setSteamId(event.target.value)}
                        placeholder="7656119xxxxxxxxxx"
                        inputMode="numeric"
                        autoComplete="off"
                        disabled={loading}
                    />
                </div>
            </div>

            <p className="mt-3 rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2 text-xs text-amber-200/90">
                凭据保存在本机数据库中。Steam 资料库隐私设置需允许查看游戏详情；Steam 游玩时长独立于本地进程监控时长保存，不会重复累加。
            </p>

            <div className="mt-4 rounded-xl bg-slate-950/50 px-4 py-3">
                <p className="text-xs text-slate-400">
                    最近尝试：<span className="text-slate-300">{formatDateTime(status?.lastAttemptAt)}</span>
                </p>
                {status?.lastResult ? (
                    <div className="mt-2 grid grid-cols-2 gap-2 text-xs sm:grid-cols-4">
                        <p className="text-slate-400">获取游戏 <span className="text-slate-200">{status.lastResult.fetchedGames}</span></p>
                        <p className="text-slate-400">按 AppID 匹配 <span className="text-slate-200">{status.lastResult.matchedByAppId}</span></p>
                        <p className="text-slate-400">按名称匹配 <span className="text-slate-200">{status.lastResult.matchedByName}</span></p>
                        <p className="text-slate-400">新增记录 <span className="text-slate-200">{status.lastResult.importedGames}</span></p>
                        <p className="col-span-2 text-slate-400 sm:col-span-4">
                            Steam 总游玩时长：<span className="text-slate-200">{formatDuration(status.lastResult.totalPlaytimeSeconds)}</span>
                            <span className="ml-2 text-slate-500">· {formatDateTime(status.lastResult.syncedAt)}</span>
                        </p>
                    </div>
                ) : (
                    <p className="mt-2 text-xs text-slate-500">尚无成功的同步记录。</p>
                )}
                {status?.lastError ? (
                    <p className="mt-2 text-xs text-rose-300">上次同步失败：{status.lastError}</p>
                ) : null}
            </div>

            {error ? (
                <p className="mt-3 rounded-lg border border-rose-500/40 bg-rose-500/10 px-3 py-2 text-xs text-rose-200">
                    {error}
                </p>
            ) : null}
        </section>
    )
}
