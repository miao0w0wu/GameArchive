import { useEffect, useState } from 'react'

import { GetAISettings, SaveAISettings, TestAIConnection } from '../../wailsjs/go/main/App'
import { services } from '../../wailsjs/go/models'
import { btnGhost, btnPrimary, cardClass, inputClass, labelClass } from '../lib/ui'
import { extractError, useAppStore } from '../stores/appStore'

/** AI 配置卡片：OpenAI 兼容接口的地址、API Key 与模型，支持测试连接。 */
export default function AISettingsCard() {
    const notify = useAppStore((state) => state.notify)

    const [baseUrl, setBaseUrl] = useState('')
    const [apiKey, setApiKey] = useState('')
    const [model, setModel] = useState('')
    const [showKey, setShowKey] = useState(false)
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [testing, setTesting] = useState(false)
    const [error, setError] = useState('')
    const [testResult, setTestResult] = useState('')

    useEffect(() => {
        let cancelled = false
        GetAISettings()
            .then((config: services.AIConfig) => {
                if (cancelled) return
                setBaseUrl(config.baseUrl ?? '')
                setApiKey(config.apiKey ?? '')
                setModel(config.model ?? '')
            })
            .catch((reason: unknown) => {
                if (!cancelled) {
                    setError(extractError(reason))
                }
            })
            .finally(() => {
                if (!cancelled) {
                    setLoading(false)
                }
            })
        return () => {
            cancelled = true
        }
    }, [])

    const payload = (): services.AIConfig =>
        services.AIConfig.createFrom({
            baseUrl: baseUrl.trim(),
            apiKey: apiKey.trim(),
            model: model.trim(),
        })

    const save = async () => {
        setSaving(true)
        setError('')
        setTestResult('')
        try {
            await SaveAISettings(payload())
            notify('success', 'AI 配置已保存')
        } catch (reason) {
            const message = extractError(reason)
            setError(message)
            notify('error', `保存 AI 配置失败：${message}`)
        } finally {
            setSaving(false)
        }
    }

    const test = async () => {
        setTesting(true)
        setError('')
        setTestResult('')
        try {
            // 先保存再测试，保证测试用的就是当前表单里的配置
            await SaveAISettings(payload())
            await TestAIConnection()
            setTestResult('连接成功，接口可用')
            notify('success', 'AI 接口连接成功')
        } catch (reason) {
            const message = extractError(reason)
            setError(message)
            notify('error', `连接测试失败：${message}`)
        } finally {
            setTesting(false)
        }
    }

    const busy = saving || testing

    return (
        <section className={`${cardClass} p-5`}>
            <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                    <h2 className="text-sm font-semibold text-slate-200">AI 报告配置</h2>
                    <p className="mt-1 text-xs text-slate-500">
                        默认使用 DeepSeek OpenAI 兼容接口；也可以填写其它兼容服务地址和模型。
                    </p>
                </div>
                <div className="flex gap-2">
                    <button type="button" className={btnGhost} onClick={() => void test()} disabled={busy || loading}>
                        {testing ? '测试中…' : '测试连接'}
                    </button>
                    <button type="button" className={btnPrimary} onClick={() => void save()} disabled={busy || loading}>
                        {saving ? '保存中…' : '保存配置'}
                    </button>
                </div>
            </div>

            <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="sm:col-span-2">
                    <label className={labelClass} htmlFor="ai-base-url">
                        接口地址
                    </label>
                    <input
                        id="ai-base-url"
                        className={`${inputClass} font-mono text-xs`}
                        value={baseUrl}
                        onChange={(event) => setBaseUrl(event.target.value)}
                        placeholder="https://api.deepseek.com"
                        disabled={loading}
                    />
                </div>

                <div>
                    <label className={labelClass} htmlFor="ai-model">
                        模型
                    </label>
                    <input
                        id="ai-model"
                        className={`${inputClass} font-mono text-xs`}
                        value={model}
                        onChange={(event) => setModel(event.target.value)}
                        placeholder="deepseek-chat"
                        disabled={loading}
                    />
                </div>

                <div>
                    <label className={labelClass} htmlFor="ai-api-key">
                        API Key
                    </label>
                    <div className="flex gap-2">
                        <input
                            id="ai-api-key"
                            type={showKey ? 'text' : 'password'}
                            className={`${inputClass} font-mono text-xs`}
                            value={apiKey}
                            onChange={(event) => setApiKey(event.target.value)}
                            placeholder="sk-..."
                            autoComplete="off"
                            disabled={loading}
                        />
                        <button
                            type="button"
                            className={btnGhost}
                            onClick={() => setShowKey((current) => !current)}
                            disabled={loading}
                        >
                            {showKey ? '隐藏' : '显示'}
                        </button>
                    </div>
                </div>
            </div>

            <p className="mt-3 rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2 text-xs text-amber-200/90">
                密钥保存在本机数据库（settings 表）中；测试连接和生成报告时会发送到上面配置的接口。
                生成报告会同时发送所选周期的统计数据，请勿在共享电脑上保存密钥。
            </p>

            {error ? (
                <p className="mt-3 rounded-lg border border-rose-500/40 bg-rose-500/10 px-3 py-2 text-xs text-rose-200">
                    {error}
                </p>
            ) : null}
            {testResult ? (
                <p className="mt-3 rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-3 py-2 text-xs text-emerald-200">
                    {testResult}
                </p>
            ) : null}
        </section>
    )
}