/**
 * ECharts 按需注册：只引入「统计报告」页用到的图表与组件，控制打包体积。
 *
 * 所有图表统一使用暗色主题配色，避免每个页面重复设置。
 */
import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import * as echarts from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([
    LineChart,
    BarChart,
    PieChart,
    GridComponent,
    TooltipComponent,
    LegendComponent,
    CanvasRenderer,
])

export { echarts }
export type { EChartsCoreOption, EChartsType } from 'echarts/core'

/** 图表主色，与界面主色一致。 */
export const chartColors = ['#6366f1', '#22c55e', '#f97316', '#0ea5e9', '#ec4899', '#a855f7']

/** 坐标轴文字颜色。 */
export const chartTextColor = '#94a3b8'
/** 坐标轴与网格线颜色。 */
export const chartLineColor = '#334155'
/** 图表 tooltip 的暗色背景。 */
export const chartTooltipStyle = {
    backgroundColor: '#0f172a',
    borderColor: '#334155',
    textStyle: { color: '#e2e8f0', fontSize: 12 },
}

/** 把秒转换为小时（保留两位小数），用于图表数值。 */
export function toHours(seconds: number): number {
    return Math.round(((seconds || 0) / 3600) * 100) / 100
}