import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react'
import { cn } from '../../lib/cn'
import styles from './Button.module.css'

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'outline' | 'ghost'
type ButtonSize = 'sm' | 'md' | 'lg'

type ButtonProps = {
  variant?: ButtonVariant
  size?: ButtonSize
  loading?: boolean
  children: ReactNode
} & ButtonHTMLAttributes<HTMLButtonElement>

const variantClass: Record<ButtonVariant, string> = {
  primary: styles['variant-primary'],
  secondary: styles['variant-secondary'],
  danger: styles['variant-danger'],
  outline: styles['variant-outline'],
  ghost: styles['variant-ghost'],
}

const sizeClass: Record<ButtonSize, string> = {
  sm: styles['size-sm'],
  md: styles['size-md'],
  lg: styles['size-lg'],
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', loading = false, className, disabled, children, ...rest }, ref) => {
    const isDisabled = disabled || loading

    return (
      <button
        ref={ref}
        className={cn(styles.btn, variantClass[variant], sizeClass[size], className)}
        disabled={isDisabled}
        aria-busy={loading || undefined}
        {...rest}
      >
        {loading ? (
          <span className={styles.spinner} aria-hidden="true">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
              <circle cx="12" cy="12" r="10" strokeDasharray="31.4 31.4" strokeLinecap="round" />
            </svg>
          </span>
        ) : null}
        {children}
      </button>
    )
  },
)

Button.displayName = 'Button'
