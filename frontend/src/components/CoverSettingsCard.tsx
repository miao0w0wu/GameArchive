import { useEffect, useState } from 'react'

import { GetCoverSettings, SaveCoverSettings } from '../../wailsjs/go/main/App'
import { btnPrimary, cardClass, inputClass, labelClass } from '../lib/ui'
import { extractError, useAppStore } from '../stores/appStore'
import { services } from '../../wailsjs/go/models'

/** 在线图片来源与本地封面缓存配置。 */
export default function CoverSettingsCard() {
    const notify = useAppStore((state) => state.notify)
    const [config, setConfig] = useState<services.CoverSettings | null>(null)
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [showKeys, setShowKeys] = useState(false)
    const [error, setError] = useState('')

    useEffect(() => {
        let cancelled = false
        GetCoverSettings()
            .then((settings) => {
                if (!cancelled) setConfig(settings)
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

    const update = (patch: Partial<services.CoverSettings>) => {
        setConfig((current) => current ? services.CoverSettings.createFrom({ ...current, ...patch }) : current)
    }

    const save = async () => {
        if (!config) return
        setSaving(true)
        setError('')
        try {
            await SaveCoverSettings(config)
            notify('success', '封面获取设置已保存')
        } catch (reason) {
            const message = extractError(reason)
            setError(message)
            notify('error', `保存封面获取设置失败：${message}`)
        } finally {
            setSaving(false)
        }
    }

    return (
        <section className={`${cardClass} mt-5 p-5`}>
            <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                    <h2 className="text-sm font-semibold text-slate-200">游戏封面与图标</h2>
                    <p className="mt-1 text-xs text-slate-500">自动查找本地图片，再依次尝试 Steam、SteamGridDB 和 RAWG。</p>
                </div>
                <button type="button" className={btnPrimary} onClick={() => void save()} disabled={loading || saving || !config}>
                    {saving ? '保存中…' : '保存设置'}
                </button>
            </div>
            {config ? (
                <>
                    <label className="mt-4 flex items-center gap-2 text-sm text-slate-300">
                        <input
                            type="checkbox"
                            checked={config.onlineEnabled}
                            onChange={(event) => update({ onlineEnabled: event.target.checked })}
                            disabled={loading}
                            className="accent-indigo-500"
                        />
                        启用在线图片搜索与下载
                    </label>
                    <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
                        <div>
                            <label className={labelClass} htmlFor="steamgriddb-key">SteamGridDB API Key（可选）</label>
                            <input
                                id="steamgriddb-key"
                                type={showKeys ? 'text' : 'password'}
                                className={`${inputClass} font-mono text-xs`}
                                value={config.steamGridDBKey}
                                onChange={(event) => update({ steamGridDBKey: event.target.value })}
                                autoComplete="off"
                            />
                        </div>
                        <div>
                            <label className={labelClass} htmlFor="rawg-key">RAWG API Key（可选）</label>
                            <input
                                id="rawg-key"
                                type={showKeys ? 'text' : 'password'}
                                className={`${inputClass} font-mono text-xs`}
                                value={config.rawgKey}
                                onChange={(event) => update({ rawgKey: event.target.value })}
                                autoComplete="off"
                            />
                        </div>
                    </div>
                    <button type="button" className="mt-3 text-xs text-indigo-300 hover:text-indigo-200" onClick={() => setShowKeys((value) => !value)}>
                        {showKeys ? '隐藏密钥' : '显示密钥'}
                    </button>
                    <p className="mt-3 text-xs text-slate-500">
                        图片缓存目录：封面 <span className="break-all font-mono">{config.cacheDir}</span>
                        <span className="mx-2">·</span>
                        图标 <span className="break-all font-mono">{config.iconCacheDir}</span>
                    </p>
                    <p className="mt-2 text-xs text-amber-200/80">
                        API Key 保存在本机数据库。在线图片搜索默认启用；图片获取失败不会影响游戏库操作。
                    </p>
                </>
            ) : null}
            {error ? <p className="mt-3 text-xs text-rose-300">{error}</p> : null}
        </section>
    )
}
