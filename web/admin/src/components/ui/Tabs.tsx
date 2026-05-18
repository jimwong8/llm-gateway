import { type ReactNode } from 'react'
import { cn } from '../../lib/cn'
import styles from './Tabs.module.css'

type Tab = {
  key: string
  label: string
}

type TabsProps = {
  tabs: Tab[]
  activeKey: string
  onChange: (key: string) => void
  className?: string
  children?: ReactNode
}

export function Tabs({ tabs, activeKey, onChange, className, children }: TabsProps) {
  return (
    <div className={cn(styles.wrapper, className)}>
      <div className={styles.tabStrip} role="tablist" aria-orientation="horizontal">
        {tabs.map((tab) => {
          const isActive = tab.key === activeKey
          return (
            <button
              key={tab.key}
              type="button"
              role="tab"
              aria-selected={isActive}
              className={cn(styles.tab, isActive ? styles.active : undefined)}
              onClick={() => onChange(tab.key)}
            >
              {tab.label}
            </button>
          )
        })}
      </div>
      {children ? (
        <div
          role="tabpanel"
          aria-label={tabs.find((t) => t.key === activeKey)?.label}
          className={styles.panel}
        >
          {children}
        </div>
      ) : null}
    </div>
  )
}
