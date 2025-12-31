import { Card, Button } from '@/components/ui'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { Database, Server, Check } from 'lucide-react'
import type { DatabaseDriver } from '../types'

type Props = {
  currentDriver: DatabaseDriver
  selectedDriver: DatabaseDriver | null
  onSelect: (driver: DatabaseDriver) => void
  onNext: () => void
  onCancel: () => void
}

export const StepChooseTarget = ({
  currentDriver,
  selectedDriver,
  onSelect,
  onNext,
  onCancel,
}: Props) => {
  const options: { driver: DatabaseDriver; title: string; description: string; bestFor: string }[] =
    [
      {
        driver: 'sqlite',
        title: 'SQLite',
        description: 'Self-contained, zero configuration',
        bestFor: 'Single user, local deployments',
      },
      {
        driver: 'postgres',
        title: 'PostgreSQL',
        description: 'Separate server, robust and scalable',
        bestFor: 'Multi-user, Kubernetes, production',
      },
    ]

  return (
    <div className="space-y-6">
      <div>
        <p className={cn('text-sm', text.secondary)}>
          Current Database:{' '}
          <span className={cn('font-medium', text.primary)}>
            {currentDriver === 'postgres' ? 'PostgreSQL' : 'SQLite'}
          </span>
        </p>
      </div>

      <div>
        <p className={cn('text-sm font-medium mb-4', text.primary)}>Choose your target database:</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {options.map((option) => {
            const isSelected = selectedDriver === option.driver
            const isCurrent = currentDriver === option.driver

            return (
              <Card
                key={option.driver}
                variant={isSelected ? 'default' : 'glass'}
                className={cn(
                  'relative cursor-pointer transition-all duration-200',
                  'hover:border-primary-500/50 dark:hover:border-primary-400/50',
                  isSelected && 'border-primary-500 dark:border-primary-400 ring-2 ring-primary-500/20'
                )}
                onClick={() => onSelect(option.driver)}
              >
                <div className="p-5">
                  <div className="flex items-start gap-4">
                    <div
                      className={cn(
                        'p-2.5 rounded-xl',
                        isSelected
                          ? 'bg-primary-500 text-white'
                          : 'bg-neutral-100 dark:bg-neutral-800 text-neutral-600 dark:text-neutral-400'
                      )}
                    >
                      {option.driver === 'postgres' ? (
                        <Server className="w-5 h-5" />
                      ) : (
                        <Database className="w-5 h-5" />
                      )}
                    </div>
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <h3 className={cn('font-semibold', text.primary)}>{option.title}</h3>
                        {isCurrent && (
                          <span
                            className={cn(
                              'px-2 py-0.5 text-xs font-medium rounded-full',
                              'bg-neutral-200 dark:bg-neutral-700',
                              text.secondary
                            )}
                          >
                            current
                          </span>
                        )}
                      </div>
                      <p className={cn('text-sm mt-1', text.secondary)}>{option.description}</p>
                      <p className={cn('text-xs mt-2', text.tertiary)}>
                        Best for: {option.bestFor}
                      </p>
                    </div>
                    {isSelected && (
                      <div className="absolute top-3 right-3">
                        <Check className="w-5 h-5 text-primary-500" />
                      </div>
                    )}
                  </div>
                </div>
              </Card>
            )
          })}
        </div>
      </div>

      {selectedDriver === currentDriver && (
        <div
          className={cn(
            'p-4 rounded-xl',
            'bg-amber-50 dark:bg-amber-500/10',
            'border border-amber-200 dark:border-amber-500/20',
            'text-amber-700 dark:text-amber-400 text-sm'
          )}
        >
          You're already using {currentDriver === 'postgres' ? 'PostgreSQL' : 'SQLite'}. Select a
          different database type to migrate.
        </div>
      )}

      <div className="flex justify-end gap-3 pt-4">
        <Button variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
        <Button onClick={onNext} disabled={!selectedDriver || selectedDriver === currentDriver}>
          Next
        </Button>
      </div>
    </div>
  )
}
