import * as React from "react"
import { cn } from "../../lib/utils"

interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?: "default" | "secondary" | "destructive" | "outline" | "success" | "warning" | "danger" | "info"
}

const Badge = React.forwardRef<HTMLSpanElement, BadgeProps>(
  ({ className, variant = "default", ...props }, ref) => {
    return (
      <span
        ref={ref}
        className={cn(
          "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
          variant === "default" && "border-transparent bg-[var(--text-primary)] text-[var(--surface-primary)]",
          variant === "secondary" && "border-transparent bg-[var(--surface-tertiary)] text-[var(--text-secondary)]",
          variant === "destructive" && "border-[rgba(239,68,68,0.2)] bg-[rgba(239,68,68,0.1)] text-[var(--text-danger)]",
          variant === "outline" && "text-[var(--text-secondary)] border-[var(--border-default)]",
          variant === "success" && "border-[rgba(34,197,94,0.2)] bg-[rgba(34,197,94,0.1)] text-[var(--text-success)]",
          variant === "warning" && "border-[rgba(245,158,11,0.2)] bg-[rgba(245,158,11,0.1)] text-[var(--text-warning)]",
          variant === "danger" && "border-[rgba(239,68,68,0.2)] bg-[rgba(239,68,68,0.1)] text-[var(--text-danger)]",
          variant === "info" && "border-[var(--accent-blue-border)] bg-[var(--accent-blue-bg)] text-[var(--accent-blue)]",
          className
        )}
        {...props}
      />
    )
  }
)
Badge.displayName = "Badge"

export { Badge }
