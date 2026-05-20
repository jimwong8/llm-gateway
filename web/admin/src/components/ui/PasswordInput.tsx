import { forwardRef, type InputHTMLAttributes, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '../../lib/cn'
import { Input } from './input'

type PasswordInputProps = {
  label?: string
  error?: string
} & Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>

export const PasswordInput = forwardRef<HTMLInputElement, PasswordInputProps>(
  ({ label, error, className, id, ...rest }, ref) => {
    const { t } = useTranslation()
    const [visible, setVisible] = useState(false)

    const inputId = id || (label ? label.toLowerCase().replace(/\s+/g, '-') : undefined)

    return (
      <div className={cn(className)}>
        <div style={{ position: 'relative' }}>
          <Input
            ref={ref}
            id={inputId}
            type={visible ? 'text' : 'password'}
            label={label}
            error={error}
            style={{ paddingRight: '2.75rem' }}
            {...rest}
          />
          <button
            type="button"
            onClick={() => setVisible(v => !v)}
            aria-label={visible ? t('auth.hidePassword') : t('auth.showPassword')}
            aria-pressed={visible}
            tabIndex={-1}
            style={{
              position: 'absolute',
              right: '0.5rem',
              top: label ? '1.85rem' : '0.55rem',
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              color: '#94a3b8',
              padding: '0.25rem',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            {visible ? (
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
                <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
                <line x1="1" y1="1" x2="23" y2="23" />
              </svg>
            ) : (
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                <circle cx="12" cy="12" r="3" />
              </svg>
            )}
          </button>
        </div>
      </div>
    )
  },
)

PasswordInput.displayName = 'PasswordInput'
