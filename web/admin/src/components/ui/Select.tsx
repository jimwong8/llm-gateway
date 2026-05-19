import { forwardRef, type SelectHTMLAttributes } from 'react'
import { cn } from '../../lib/cn'
import styles from './Select.module.css'

type SelectOption = {
  value: string
  label: string
}

type SelectProps = {
  label?: string
  error?: string
  options: SelectOption[]
  placeholder?: string
} & Omit<SelectHTMLAttributes<HTMLSelectElement>, 'children'>

export const Select = forwardRef<HTMLSelectElement, SelectProps>(
  ({ label, error, options, placeholder, className, id, ...rest }, ref) => {
    const selectId = id || (label ? label.toLowerCase().replace(/\s+/g, '-') : undefined)
    const errorId = selectId ? `${selectId}-error` : undefined

    return (
      <div className={cn(styles.wrapper, className)}>
        {label ? (
          <label htmlFor={selectId} className={styles.label}>
            {label}
          </label>
        ) : null}
        <select
          ref={ref}
          id={selectId}
          className={cn(styles.select, error ? styles['has-error'] : undefined)}
          aria-invalid={!!error || undefined}
          aria-describedby={error ? errorId : undefined}
          {...rest}
        >
          {placeholder ? (
            <option value="" disabled>
              {placeholder}
            </option>
          ) : null}
          {options.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
        {error ? (
          <p id={errorId} className={styles.error} role="alert">
            {error}
          </p>
        ) : null}
      </div>
    )
  },
)

Select.displayName = 'Select'
