import { useEffect, useState } from 'react'
import type { CSSProperties } from 'react'

import { GetGameImageData } from '../../wailsjs/go/main/App'
import { initialOf } from '../lib/format'

interface GameImageProps {
    gameId: number
    name: string
    target: 'cover' | 'icon'
    className?: string
    imageVersion?: string
    style?: CSSProperties
}

/** 从 Go 侧读取本地缓存图片，自动按列表 / 卡片优先级回退。 */
export default function GameImage({ gameId, name, target, className = '', imageVersion = '', style }: GameImageProps) {
    const [src, setSrc] = useState('')

    useEffect(() => {
        let cancelled = false
        setSrc('')
        GetGameImageData(gameId, target)
            .then((data) => {
                if (!cancelled) setSrc(data)
            })
            .catch(() => {
                if (!cancelled) setSrc('')
            })
        return () => {
            cancelled = true
        }
    }, [gameId, target, imageVersion])

    return src ? (
        <img src={src} alt={`${name}${target === 'cover' ? '封面' : '图标'}`} className={className} style={style} />
    ) : (
        <span
            aria-label={`${name}默认占位图`}
            className={`flex items-center justify-center bg-gradient-to-br from-indigo-500/30 to-fuchsia-500/30 font-semibold text-slate-100 ${className}`}
            style={style}
        >
            {initialOf(name)}
        </span>
    )
}
