/**
 * 内联 SVG 图标集合，避免引入额外的图标依赖。
 */
import type { ReactNode, SVGProps } from 'react'

type IconProps = SVGProps<SVGSVGElement>

function Base({ children, ...props }: IconProps & { children: ReactNode }) {
    return (
        <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
            {...props}
        >
            {children}
        </svg>
    )
}

export const IconDashboard = (props: IconProps) => (
    <Base {...props}>
        <rect x="3" y="3" width="7" height="9" rx="1.5" />
        <rect x="14" y="3" width="7" height="5" rx="1.5" />
        <rect x="14" y="12" width="7" height="9" rx="1.5" />
        <rect x="3" y="16" width="7" height="5" rx="1.5" />
    </Base>
)

export const IconLibrary = (props: IconProps) => (
    <Base {...props}>
        <rect x="3" y="4" width="6" height="16" rx="1.5" />
        <rect x="11" y="4" width="4" height="16" rx="1.5" />
        <path d="m17.5 5.5 3.2 13.2" />
    </Base>
)

export const IconTags = (props: IconProps) => (
    <Base {...props}>
        <path d="M12.6 3H20a1 1 0 0 1 1 1v7.4a2 2 0 0 1-.6 1.4l-7.6 7.6a2 2 0 0 1-2.8 0l-6-6a2 2 0 0 1 0-2.8l7.6-7.6a2 2 0 0 1 1.4-.6Z" />
        <circle cx="16.5" cy="7.5" r="1.3" />
    </Base>
)

export const IconSettings = (props: IconProps) => (
    <Base {...props}>
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-2.9 1.2 2 2 0 1 1-4 0 1.7 1.7 0 0 0-2.9-1.2l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1A1.7 1.7 0 0 0 2 15a2 2 0 1 1 0-4 1.7 1.7 0 0 0 1.4-2.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1A1.7 1.7 0 0 0 9 4a2 2 0 1 1 4 0 1.7 1.7 0 0 0 2.9 1.4l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1A1.7 1.7 0 0 0 20 11a2 2 0 1 1 0 4Z" />
    </Base>
)

export const IconPlus = (props: IconProps) => (
    <Base {...props}>
        <path d="M12 5v14M5 12h14" />
    </Base>
)

export const IconSearch = (props: IconProps) => (
    <Base {...props}>
        <circle cx="11" cy="11" r="7" />
        <path d="m20 20-3.5-3.5" />
    </Base>
)

export const IconEdit = (props: IconProps) => (
    <Base {...props}>
        <path d="M12 20h9" />
        <path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z" />
    </Base>
)

export const IconTrash = (props: IconProps) => (
    <Base {...props}>
        <path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" />
        <path d="M10 11v6M14 11v6" />
    </Base>
)

export const IconClose = (props: IconProps) => (
    <Base {...props}>
        <path d="M18 6 6 18M6 6l12 12" />
    </Base>
)

export const IconGrid = (props: IconProps) => (
    <Base {...props}>
        <rect x="3" y="3" width="8" height="8" rx="1.5" />
        <rect x="13" y="3" width="8" height="8" rx="1.5" />
        <rect x="3" y="13" width="8" height="8" rx="1.5" />
        <rect x="13" y="13" width="8" height="8" rx="1.5" />
    </Base>
)

export const IconList = (props: IconProps) => (
    <Base {...props}>
        <path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01" />
    </Base>
)

export const IconInfo = (props: IconProps) => (
    <Base {...props}>
        <circle cx="12" cy="12" r="9" />
        <path d="M12 11v5M12 8h.01" />
    </Base>
)

export const IconRoute = (props: IconProps) => (
    <Base {...props}>
        <path d="M4 6h16M4 12h10M4 18h6" />
    </Base>
)