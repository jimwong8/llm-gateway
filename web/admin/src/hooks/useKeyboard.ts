import { useEffect } from 'react'

interface KeyBinding {
  key: string
  ctrl?: boolean
  shift?: boolean
  alt?: boolean
  meta?: boolean
  handler: (e: KeyboardEvent) => void
  preventDefault?: boolean
}

export function useKeyboard(bindings: KeyBinding[], enabled: boolean = true) {
  useEffect(() => {
    if (!enabled) return

    const listener = (e: KeyboardEvent) => {
      for (const binding of bindings) {
        if (
          e.key.toLowerCase() === binding.key.toLowerCase() &&
          !!binding.ctrl === e.ctrlKey &&
          !!binding.shift === e.shiftKey &&
          !!binding.alt === e.altKey &&
          !!binding.meta === e.metaKey
        ) {
          if (binding.preventDefault !== false) {
            e.preventDefault()
          }
          binding.handler(e)
          return
        }
      }
    }

    window.addEventListener('keydown', listener)
    return () => window.removeEventListener('keydown', listener)
  }, [bindings, enabled])
}
