import { cn } from "../../lib/utils"

interface TabItem {
  key: string
  label: string
}

interface SimpleTabsProps {
  tabs: TabItem[]
  activeKey: string
  onChange: (key: string) => void
  className?: string
}

export function SimpleTabs({ tabs, activeKey, onChange, className }: SimpleTabsProps) {
  return (
    <div className={cn("simple-tabs", className)} role="tablist">
      {tabs.map((tab) => (
        <button
          key={tab.key}
          role="tab"
          aria-selected={activeKey === tab.key}
          className={cn("simple-tabs__tab", activeKey === tab.key && "simple-tabs__tab--active")}
          onClick={() => onChange(tab.key)}
        >
          {tab.label}
        </button>
      ))}
    </div>
  )
}

interface TabPanelProps {
  label: string
  children: React.ReactNode
  className?: string
}

export function TabPanel({ children, className }: TabPanelProps) {
  return <div className={cn("simple-tabs__panel", className)}>{children}</div>
}
