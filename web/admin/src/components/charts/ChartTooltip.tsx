import type { TooltipProps } from 'recharts'

interface ChartTooltipProps extends TooltipProps<any, string> {
  title?: string
  valueFormatter?: (value: any) => string
}

export function ChartTooltip({ active, payload, label, title, valueFormatter }: ChartTooltipProps) {
  if (!active || !payload || payload.length === 0) return null

  return (
    <div
      style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        borderRadius: 'var(--radius-md)',
        padding: '0.75rem 1rem',
        boxShadow: 'var(--shadow-lg)',
        minWidth: '150px',
      }}
      role="tooltip"
    >
      {title && (
        <div style={{ fontSize: '0.75rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>
          {title}
        </div>
      )}
      {label !== undefined && (
        <div style={{ fontSize: '0.8rem', color: 'var(--text-tertiary)', marginBottom: '0.5rem' }}>
          {label}
        </div>
      )}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.35rem' }}>
        {(payload as Array<{ color?: string; name?: string; value?: any }>)?.map((entry, index) => (
          <div key={index} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.8rem' }}>
            <span
              style={{
                width: '10px',
                height: '10px',
                borderRadius: '2px',
                backgroundColor: entry.color || '#3b82f6',
                flexShrink: 0,
              }}
            />
            <span style={{ color: 'var(--text-secondary)' }}>{entry.name}:</span>
            <span style={{ color: 'var(--text-primary)', fontWeight: 500, marginLeft: 'auto' }}>
              {valueFormatter ? valueFormatter(entry.value) : String(entry.value ?? '')}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}
