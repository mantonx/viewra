import { createContext, useEffect, useState, type ReactNode } from 'react'
import type { ThemeMode } from '@/styles/tokens'

export interface ThemeContextType {
  theme: ThemeMode
  setTheme: (theme: ThemeMode) => void
  toggleTheme: () => void
}

export const ThemeContext = createContext<ThemeContextType | undefined>(undefined)

interface ThemeProviderProps {
  children: ReactNode
  defaultTheme?: ThemeMode
  storageKey?: string
}

const ThemeProvider = ({
  children,
  defaultTheme = 'light',
  storageKey = 'viewra-theme',
}: ThemeProviderProps) => {
  const [theme, setThemeState] = useState<ThemeMode>(() => {
    // Try to get theme from localStorage
    if (typeof window !== 'undefined') {
      const stored = localStorage.getItem(storageKey) as ThemeMode | null
      if (stored === 'light' || stored === 'dark') {
        return stored
      }
      // Check system preference
      if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
        return 'dark'
      }
    }
    return defaultTheme
  })

  useEffect(() => {
    const root = window.document.documentElement
    root.classList.remove('light', 'dark')
    root.classList.add(theme)
  }, [theme])

  const setTheme = (newTheme: ThemeMode) => {
    localStorage.setItem(storageKey, newTheme)
    setThemeState(newTheme)
  }

  const toggleTheme = () => {
    setTheme(theme === 'light' ? 'dark' : 'light')
  }

  const value = {
    theme,
    setTheme,
    toggleTheme,
  }

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}

export { ThemeProvider }
