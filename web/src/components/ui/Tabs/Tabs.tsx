import { cn } from '@/lib/utils'
import { border, text } from '@/styles/semantic'
import type { TabPanelProps, TabsProps } from './Tabs.types'

const variantStyles = {
  default: {
    container: 'border-b',
    tab: 'px-4 py-2 -mb-px border-b-2 border-transparent',
    active: 'border-blue-500 text-blue-600 dark:text-blue-400',
    inactive: 'hover:border-neutral-300 dark:hover:border-neutral-600',
  },
  underline: {
    container: 'border-b',
    tab: 'px-4 py-2 -mb-px border-b-2 border-transparent',
    active: 'border-blue-500 text-blue-600 dark:text-blue-400 font-medium',
    inactive: 'hover:text-neutral-700 dark:hover:text-neutral-300',
  },
  pills: {
    container: 'gap-2',
    tab: 'px-3 py-1.5 rounded-md',
    active: 'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300',
    inactive: 'hover:bg-neutral-100 dark:hover:bg-neutral-800',
  },
}

const sizeStyles = {
  sm: 'text-sm',
  md: 'text-base',
  lg: 'text-lg',
}

const Tabs = ({
  tabs,
  activeTab,
  onTabChange,
  variant = 'default',
  size = 'md',
  className,
}: TabsProps) => {
  const styles = variantStyles[variant]

  return (
    <div className={cn('flex', border.secondary, styles.container, className)} role="tablist">
      {tabs.map((tab) => {
        const isActive = tab.id === activeTab
        return (
          <button
            key={tab.id}
            role="tab"
            aria-selected={isActive}
            aria-controls={`tabpanel-${tab.id}`}
            id={`tab-${tab.id}`}
            disabled={tab.disabled}
            onClick={() => onTabChange(tab.id)}
            className={cn(
              'cursor-pointer transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2',
              sizeStyles[size],
              styles.tab,
              isActive ? styles.active : cn(text.secondary, styles.inactive),
              tab.disabled && 'opacity-50 cursor-not-allowed'
            )}
          >
            <span className="flex items-center gap-2">
              {tab.label}
              {tab.badge !== undefined && (
                <span
                  className={cn(
                    'inline-flex items-center justify-center min-w-[1.25rem] px-1.5 py-0.5 text-xs font-medium rounded-full',
                    isActive
                      ? 'bg-blue-500 text-white'
                      : 'bg-neutral-200 dark:bg-neutral-700 text-neutral-600 dark:text-neutral-300'
                  )}
                >
                  {tab.badge}
                </span>
              )}
            </span>
          </button>
        )
      })}
    </div>
  )
}

const TabPanel = ({ children, isActive, className }: TabPanelProps) => {
  if (!isActive) {
    return null
  }

  return (
    <div role="tabpanel" className={cn(className)}>
      {children}
    </div>
  )
}

export type { Tab, TabPanelProps, TabsProps } from './Tabs.types'
export { TabPanel, Tabs }
