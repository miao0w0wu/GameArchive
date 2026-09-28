import { useEffect, useRef } from 'react'

import { echarts, type EChartsCoreOption, type EChartsType } from '../lib/charts'

interface EChartProps {
    option: EChartsCoreOption
    /** 图表高度（像素）。 */
    height?: number
    className?: string
}

/**
 * ECharts 容器组件：负责初始化、配置更新、容器尺寸变化自适应与卸载销毁。
 */
export default function EChart({ option, height = 280, className }: EChartProps) {
    const containerRef = useRef<HTMLDivElement>(null)
    const chartRef = useRef<EChartsType | null>(null)

    useEffect(() => {
        const container = containerRef.current
        if (!container) {
            return
        }

        const chart = echarts.init(container)
        chartRef.current = chart
        const observer = new ResizeObserver(() => chart.resize())
        observer.observe(container)

        return () => {
            observer.disconnect()
            chart.dispose()
            chartRef.current = null
        }
    }, [])

    useEffect(() => {
        // notMerge = true：切换周期时完全替换配置，避免残留旧序列
        chartRef.current?.setOption(option, true)
    }, [option])

    return <div ref={containerRef} className={className} style={{ height }} />
}