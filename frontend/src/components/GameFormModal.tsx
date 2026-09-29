import { useState } from 'react'

import { models } from '../../wailsjs/go/models'
import { btnGhost, btnPrimary, inputClass, labelClass } from '../lib/ui'
import { useAppStore } from '../stores/appStore'
import Modal from './Modal'

interface GameFormModalProps {
    /** 传入表示编辑，缺省表示新增。 */
    game?: models.Game | null
    onClose: () => void
}

/** 新增 / 编辑游戏表单。 */
export default function GameFormModal({ game, onClose }: GameFormModalProps) {
    const categories = useAppStore((state) => state.categories)
    const tags = useAppStore((state) => state.tags)
    const saveGame = useAppStore((state) => state.saveGame)

    const [name, setName] = useState(game?.name ?? '')
    const [exePath, setExePath] = useState(game?.exePath ?? '')
    const [processName, setProcessName] = useState(game?.processName ?? '')
    const [installDir, setInstallDir] = useState(game?.installDir ?? '')
    const [coverPath, setCoverPath] = useState(game?.coverPath ?? '')
    const [categoryId, setCategoryId] = useState(
        game?.categoryId ? String(game.categoryId) : '',
    )
    const [tagIds, setTagIds] = useState<number[]>(() =>
        (game?.tags ?? []).filter((tag) => tag.id > 0).map((tag) => tag.id),
    )
    const [error, setError] = useState('')
    const [busy, setBusy] = useState(false)

    const toggleTag = (tagId: number) => {
        setTagIds((prev) =>
            prev.includes(tagId) ? prev.filter((id) => id !== tagId) : [...prev, tagId],
        )
    }

    const handleSubmit = async () => {
        const trimmedName = name.trim()
        if (trimmedName === '') {
            setError('请填写游戏名称')
            return
        }

        setError('')
        setBusy(true)
        const ok = await saveGame(
            {
                name: trimmedName,
                exePath: exePath.trim(),
                processName: processName.trim(),
                installDir: installDir.trim(),
                coverPath: coverPath.trim(),
                categoryId: categoryId === '' ? null : Number(categoryId),
                tagIds,
            },
            game?.id,
        )
        setBusy(false)
        if (ok) {
            onClose()
        }
    }

    return (
        <Modal
            title={game ? '编辑游戏' : '添加游戏'}
            onClose={onClose}
            footer={
                <>
                    <button type="button" className={btnGhost} onClick={onClose} disabled={busy}>
                        取消
                    </button>
                    <button
                        type="button"
                        className={btnPrimary}
                        onClick={handleSubmit}
                        disabled={busy}
                    >
                        {busy ? '保存中…' : '保存'}
                    </button>
                </>
            }
        >
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="sm:col-span-2">
                    <label className={labelClass} htmlFor="game-name">
                        游戏名称 <span className="text-rose-400">*</span>
                    </label>
                    <input
                        id="game-name"
                        className={inputClass}
                        value={name}
                        onChange={(event) => setName(event.target.value)}
                        placeholder="例如：星露谷物语"
                        autoFocus
                    />
                </div>

                <div className="sm:col-span-2">
                    <label className={labelClass} htmlFor="game-exe">
                        可执行文件路径
                    </label>
                    <input
                        id="game-exe"
                        className={inputClass}
                        value={exePath}
                        onChange={(event) => setExePath(event.target.value)}
                        placeholder="D:\Games\Stardew Valley\Stardew Valley.exe"
                    />
                    <p className="mt-1 text-xs text-slate-500">
                        进程监控会优先按该路径匹配游戏进程，其次使用进程名。
                    </p>
                </div>

                <div>
                    <label className={labelClass} htmlFor="game-process">
                        进程名
                    </label>
                    <input
                        id="game-process"
                        className={inputClass}
                        value={processName}
                        onChange={(event) => setProcessName(event.target.value)}
                        placeholder="例如：Stardew Valley.exe"
                    />
                </div>

                <div>
                    <label className={labelClass} htmlFor="game-category">
                        分类
                    </label>
                    <select
                        id="game-category"
                        className={inputClass}
                        value={categoryId}
                        onChange={(event) => setCategoryId(event.target.value)}
                    >
                        <option value="">未分类</option>
                        {categories.map((category) => (
                            <option key={category.id} value={category.id}>
                                {category.name}
                            </option>
                        ))}
                    </select>
                </div>

                <div className="sm:col-span-2">
                    <label className={labelClass} htmlFor="game-install-dir">
                        安装目录
                    </label>
                    <input
                        id="game-install-dir"
                        className={inputClass}
                        value={installDir}
                        onChange={(event) => setInstallDir(event.target.value)}
                        placeholder="D:\Games\Stardew Valley"
                    />
                </div>

                <div className="sm:col-span-2">
                    <label className={labelClass} htmlFor="game-cover">
                        封面图片路径
                    </label>
                    <input
                        id="game-cover"
                        className={inputClass}
                        value={coverPath}
                        onChange={(event) => setCoverPath(event.target.value)}
                        placeholder="C:\Pictures\stardew.png"
                    />
                    <p className="mt-1 text-xs text-slate-500">
                        可填写已有图片路径；也可在游戏详情中上传图片或使用自动获取。
                    </p>
                </div>

                <div className="sm:col-span-2">
                    <span className={labelClass}>标签（可多选）</span>
                    {tags.length === 0 ? (
                        <p className="text-xs text-slate-500">
                            还没有标签，可先在「分类与标签」页面创建。
                        </p>
                    ) : (
                        <div className="flex flex-wrap gap-2">
                            {tags.map((tag) => {
                                const active = tagIds.includes(tag.id)
                                return (
                                    <button
                                        key={tag.id}
                                        type="button"
                                        onClick={() => toggleTag(tag.id)}
                                        className="rounded-full border px-3 py-1 text-xs font-medium transition"
                                        style={{
                                            borderColor: active ? tag.color : '#334155',
                                            backgroundColor: active ? `${tag.color}33` : 'transparent',
                                            color: active ? '#f1f5f9' : '#94a3b8',
                                        }}
                                    >
                                        {tag.name}
                                    </button>
                                )
                            })}
                        </div>
                    )}
                </div>
            </div>

            {error ? (
                <p className="mt-4 rounded-lg border border-rose-500/40 bg-rose-500/10 px-3 py-2 text-xs text-rose-200">
                    {error}
                </p>
            ) : null}
        </Modal>
    )
}