import { useMemo, useState } from 'react'

import type { models } from '../../wailsjs/go/models'
import ConfirmDialog from '../components/ConfirmDialog'
import { IconEdit, IconPlus, IconTrash } from '../components/Icons'
import { COLOR_PRESETS } from '../lib/format'
import { btnGhost, btnIcon, btnPrimary, cardClass, inputClass, labelClass } from '../lib/ui'
import { useAppStore } from '../stores/appStore'

/** 颜色选择器：预设色 + 系统取色器。 */
function ColorPicker({ value, onChange }: { value: string; onChange: (color: string) => void }) {
    return (
        <div className="flex flex-wrap items-center gap-2">
            {COLOR_PRESETS.map((preset) => (
                <button
                    key={preset}
                    type="button"
                    onClick={() => onChange(preset)}
                    className={`h-6 w-6 rounded-full border-2 transition ${
                        value.toLowerCase() === preset ? 'border-slate-100' : 'border-transparent'
                    }`}
                    style={{ backgroundColor: preset }}
                    aria-label={`选择颜色 ${preset}`}
                />
            ))}
            <input
                type="color"
                value={value}
                onChange={(event) => onChange(event.target.value)}
                className="h-7 w-10 cursor-pointer rounded border border-slate-700 bg-transparent"
                aria-label="自定义颜色"
            />
            <span className="text-xs text-slate-500">{value}</span>
        </div>
    )
}

/** 分类与标签管理。 */
export default function CategoriesPage() {
    const categories = useAppStore((state) => state.categories)
    const tags = useAppStore((state) => state.tags)
    const games = useAppStore((state) => state.games)
    const saveCategory = useAppStore((state) => state.saveCategory)
    const removeCategory = useAppStore((state) => state.removeCategory)
    const saveTag = useAppStore((state) => state.saveTag)
    const removeTag = useAppStore((state) => state.removeTag)

    // 分类表单
    const [catName, setCatName] = useState('')
    const [catColor, setCatColor] = useState(COLOR_PRESETS[6])
    const [catOrder, setCatOrder] = useState('0')
    const [editingCategory, setEditingCategory] = useState<models.Category | null>(null)
    const [catError, setCatError] = useState('')
    const [catBusy, setCatBusy] = useState(false)

    // 标签表单
    const [tagName, setTagName] = useState('')
    const [tagColor, setTagColor] = useState(COLOR_PRESETS[0])
    const [editingTag, setEditingTag] = useState<models.Tag | null>(null)
    const [tagError, setTagError] = useState('')
    const [tagBusy, setTagBusy] = useState(false)

    const [deleteCategory, setDeleteCategory] = useState<models.Category | null>(null)
    const [deleteTag, setDeleteTag] = useState<models.Tag | null>(null)

    const gameCountByCategory = useMemo(() => {
        const counter = new Map<number, number>()
        for (const game of games) {
            if (game.categoryId) {
                counter.set(game.categoryId, (counter.get(game.categoryId) ?? 0) + 1)
            }
        }
        return counter
    }, [games])

    const tagUsage = useMemo(() => {
        const counter = new Map<number, number>()
        for (const game of games) {
            for (const tag of game.tags ?? []) {
                counter.set(tag.id, (counter.get(tag.id) ?? 0) + 1)
            }
        }
        return counter
    }, [games])

    const resetCategoryForm = () => {
        setEditingCategory(null)
        setCatName('')
        setCatColor(COLOR_PRESETS[6])
        setCatOrder('0')
        setCatError('')
    }

    const submitCategory = async () => {
        if (catName.trim() === '') {
            setCatError('请填写分类名称')
            return
        }
        const order = Number(catOrder)
        setCatBusy(true)
        const ok = await saveCategory(
            {
                name: catName.trim(),
                color: catColor,
                sortOrder: Number.isFinite(order) ? order : 0,
            },
            editingCategory?.id,
        )
        setCatBusy(false)
        if (ok) {
            resetCategoryForm()
        }
    }

    const resetTagForm = () => {
        setEditingTag(null)
        setTagName('')
        setTagColor(COLOR_PRESETS[0])
        setTagError('')
    }

    const submitTag = async () => {
        if (tagName.trim() === '') {
            setTagError('请填写标签名称')
            return
        }
        setTagBusy(true)
        const ok = await saveTag({ name: tagName.trim(), color: tagColor }, editingTag?.id)
        setTagBusy(false)
        if (ok) {
            resetTagForm()
        }
    }

    return (
        <div className="mx-auto max-w-6xl px-8 py-8">
            <header className="mb-6">
                <h1 className="text-xl font-semibold text-slate-100">分类与标签</h1>
                <p className="mt-1 text-sm text-slate-500">
                    分类是游戏的唯一归属，标签可以多选；删除分类后相关游戏会变为「未分类」。
                </p>
            </header>

            <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
                {/* 分类 */}
                <section className={`${cardClass} p-5`}>
                    <h2 className="text-sm font-semibold text-slate-200">
                        分类（{categories.length}）
                    </h2>

                    <div className="mt-4 space-y-3 rounded-xl border border-slate-800 bg-slate-950/40 p-4">
                        <div>
                            <label className={labelClass} htmlFor="cat-name">
                                分类名称
                            </label>
                            <input
                                id="cat-name"
                                className={inputClass}
                                value={catName}
                                onChange={(event) => setCatName(event.target.value)}
                                placeholder="例如：动作"
                            />
                        </div>
                        <div>
                            <label className={labelClass}>颜色</label>
                            <ColorPicker value={catColor} onChange={setCatColor} />
                        </div>
                        <div>
                            <label className={labelClass} htmlFor="cat-order">
                                排序值（越小越靠前）
                            </label>
                            <input
                                id="cat-order"
                                type="number"
                                className={inputClass}
                                value={catOrder}
                                onChange={(event) => setCatOrder(event.target.value)}
                            />
                        </div>

                        {catError ? <p className="text-xs text-rose-300">{catError}</p> : null}

                        <div className="flex gap-2">
                            <button
                                type="button"
                                className={btnPrimary}
                                onClick={submitCategory}
                                disabled={catBusy}
                            >
                                {editingCategory ? (
                                    <IconEdit width={15} height={15} />
                                ) : (
                                    <IconPlus width={15} height={15} />
                                )}
                                {catBusy ? '保存中…' : editingCategory ? '保存修改' : '添加分类'}
                            </button>
                            {editingCategory ? (
                                <button type="button" className={btnGhost} onClick={resetCategoryForm}>
                                    取消编辑
                                </button>
                            ) : null}
                        </div>
                    </div>

                    <ul className="mt-4 divide-y divide-slate-800/70">
                        {categories.length === 0 ? (
                            <li className="py-6 text-center text-sm text-slate-500">还没有分类</li>
                        ) : (
                            categories.map((category) => (
                                <li key={category.id} className="flex items-center gap-3 py-3">
                                    <span
                                        className="h-3 w-3 shrink-0 rounded-full"
                                        style={{ backgroundColor: category.color }}
                                    />
                                    <span className="min-w-0 flex-1">
                                        <span className="block truncate text-sm text-slate-200">
                                            {category.name}
                                        </span>
                                        <span className="block text-xs text-slate-500">
                                            排序 {category.sortOrder} · {(gameCountByCategory.get(category.id) ?? 0)} 个游戏
                                        </span>
                                    </span>
                                    <button
                                        type="button"
                                        className={btnIcon}
                                        aria-label="编辑分类"
                                        onClick={() => {
                                            setEditingCategory(category)
                                            setCatName(category.name)
                                            setCatColor(category.color || COLOR_PRESETS[6])
                                            setCatOrder(String(category.sortOrder))
                                            setCatError('')
                                        }}
                                    >
                                        <IconEdit width={15} height={15} />
                                    </button>
                                    <button
                                        type="button"
                                        className={`${btnIcon} hover:border-rose-500/50 hover:text-rose-300`}
                                        aria-label="删除分类"
                                        onClick={() => setDeleteCategory(category)}
                                    >
                                        <IconTrash width={15} height={15} />
                                    </button>
                                </li>
                            ))
                        )}
                    </ul>
                </section>

                {/* 标签 */}
                <section className={`${cardClass} p-5`}>
                    <h2 className="text-sm font-semibold text-slate-200">标签（{tags.length}）</h2>

                    <div className="mt-4 space-y-3 rounded-xl border border-slate-800 bg-slate-950/40 p-4">
                        <div>
                            <label className={labelClass} htmlFor="tag-name">
                                标签名称
                            </label>
                            <input
                                id="tag-name"
                                className={inputClass}
                                value={tagName}
                                onChange={(event) => setTagName(event.target.value)}
                                placeholder="例如：联机"
                            />
                        </div>
                        <div>
                            <label className={labelClass}>颜色</label>
                            <ColorPicker value={tagColor} onChange={setTagColor} />
                        </div>

                        {tagError ? <p className="text-xs text-rose-300">{tagError}</p> : null}

                        <div className="flex gap-2">
                            <button
                                type="button"
                                className={btnPrimary}
                                onClick={submitTag}
                                disabled={tagBusy}
                            >
                                {editingTag ? (
                                    <IconEdit width={15} height={15} />
                                ) : (
                                    <IconPlus width={15} height={15} />
                                )}
                                {tagBusy ? '保存中…' : editingTag ? '保存修改' : '添加标签'}
                            </button>
                            {editingTag ? (
                                <button type="button" className={btnGhost} onClick={resetTagForm}>
                                    取消编辑
                                </button>
                            ) : null}
                        </div>
                    </div>

                    <ul className="mt-4 divide-y divide-slate-800/70">
                        {tags.length === 0 ? (
                            <li className="py-6 text-center text-sm text-slate-500">还没有标签</li>
                        ) : (
                            tags.map((tag) => (
                                <li key={tag.id} className="flex items-center gap-3 py-3">
                                    <span
                                        className="rounded-full px-2.5 py-1 text-xs font-medium"
                                        style={{ backgroundColor: `${tag.color}33`, color: tag.color }}
                                    >
                                        {tag.name}
                                    </span>
                                    <span className="flex-1 text-xs text-slate-500">
                                        {tagUsage.get(tag.id) ?? 0} 个游戏使用
                                    </span>
                                    <button
                                        type="button"
                                        className={btnIcon}
                                        aria-label="编辑标签"
                                        onClick={() => {
                                            setEditingTag(tag)
                                            setTagName(tag.name)
                                            setTagColor(tag.color || COLOR_PRESETS[0])
                                            setTagError('')
                                        }}
                                    >
                                        <IconEdit width={15} height={15} />
                                    </button>
                                    <button
                                        type="button"
                                        className={`${btnIcon} hover:border-rose-500/50 hover:text-rose-300`}
                                        aria-label="删除标签"
                                        onClick={() => setDeleteTag(tag)}
                                    >
                                        <IconTrash width={15} height={15} />
                                    </button>
                                </li>
                            ))
                        )}
                    </ul>
                </section>
            </div>

            {deleteCategory ? (
                <ConfirmDialog
                    title="删除分类"
                    message={
                        <>
                            确定要删除分类「{deleteCategory.name}」吗？
                            <br />
                            该分类下的游戏会变为「未分类」，游戏本身不会被删除。
                        </>
                    }
                    onCancel={() => setDeleteCategory(null)}
                    onConfirm={async () => {
                        const target = deleteCategory
                        setDeleteCategory(null)
                        if (target) {
                            await removeCategory(target.id)
                        }
                    }}
                />
            ) : null}

            {deleteTag ? (
                <ConfirmDialog
                    title="删除标签"
                    message={
                        <>
                            确定要删除标签「{deleteTag.name}」吗？
                            <br />
                            该标签与所有游戏的关联会一并移除。
                        </>
                    }
                    onCancel={() => setDeleteTag(null)}
                    onConfirm={async () => {
                        const target = deleteTag
                        setDeleteTag(null)
                        if (target) {
                            await removeTag(target.id)
                        }
                    }}
                />
            ) : null}
        </div>
    )
}