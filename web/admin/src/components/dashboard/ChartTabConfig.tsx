import {
  BarChart3,
  Layers,
  Zap,
  Link2,
  Timer,
  ShieldAlert,
  type LucideIcon,
} from 'lucide-react'

type ChartTab = 'tokens' | 'models' | 'cache' | 'channels' | 'latency' | 'errorRate'

export type ChartTabConfig = {
  key: ChartTab
  label: string
  icon: LucideIcon
}

export const CHART_TAB_CONFIG: ChartTabConfig[] = [
  { key: 'tokens', label: 'Token 趋势', icon: BarChart3 },
  { key: 'models', label: '模型分布', icon: Layers },
  { key: 'cache', label: '缓存命中', icon: Zap },
  { key: 'channels', label: '渠道状态', icon: Link2 },
  { key: 'latency', label: '延迟趋势', icon: Timer },
  { key: 'errorRate', label: '错误率', icon: ShieldAlert },
]

export type { ChartTab }
