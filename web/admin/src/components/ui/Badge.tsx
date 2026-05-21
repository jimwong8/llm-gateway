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
          "inline-flex items-center rounded-md border px-2.5 py-0.5 text-xs font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
          variant === "default" && "border-transparent bg-primary text-primary-foreground hover:bg-primary/80",
          variant === "secondary" && "border-transparent bg-secondary text-secondary-foreground hover:bg-secondary/80",
          variant === "destructive" && "border-transparent bg-destructive text-destructive-foreground hover:bg-destructive/80",
          variant === "outline" && "text-foreground",
          variant === "success" && "border-transparent bg-green-500 text-white hover:bg-green-500/80",
          variant === "warning" && "border-transparent bg-yellow-500 text-white hover:bg-yellow-500/80",
          variant === "danger" && "border-transparent bg-red-500 text-white hover:bg-red-500/80",
          variant === "info" && "border-transparent bg-blue-500 text-white hover:bg-blue-500/80",
          className
        )}
        {...props}
      />
    )
  }
)
Badge.displayName = "Badge"

export { Badge }