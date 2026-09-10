type SkeletonProps = {
  width?: string
  height?: string
  count?: number
  className?: string
}

export function Skeleton({ width = '100%', height = '1rem', count = 1, className = '' }: SkeletonProps) {
  return (
    <>
      {Array.from({ length: count }).map((_, i) => (
        <div
          key={i}
          className={`skeleton ${className}`.trim()}
          style={{ width, height }}
          aria-hidden="true"
        />
      ))}
    </>
  )
}

export function TableSkeleton({ rows = 5, cols = 4 }: { rows?: number; cols?: number }) {
  return (
    <div className="skeleton-table">
      {Array.from({ length: rows }).map((_, r) => (
        <div key={r} className="skeleton-table__row">
          {Array.from({ length: cols }).map((_, c) => (
            <Skeleton key={c} height="1.2rem" />
          ))}
        </div>
      ))}
    </div>
  )
}
