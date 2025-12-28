import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { Badge, type BadgeColor } from '@/components/ui'
import type { ListItemProps } from './ListItem.types'

export const ListItem = ({ item, display, actions }: ListItemProps) => {
  const primaryValue = String(item[display.primaryField] || item.id || '')
  const secondaryValue = display.secondaryField
    ? item[display.secondaryField]
    : undefined

  // Filter badges that should be shown for this item
  const visibleBadges = display.badges?.filter((badge) => {
    const fieldValue = item[badge.field]
    return badge.value !== undefined ? fieldValue === badge.value : Boolean(fieldValue)
  })

  return (
    <div
      className={cn(
        'py-3 px-3 rounded-lg flex items-center justify-between gap-3',
        'bg-neutral-50 dark:bg-neutral-900/50'
      )}
    >
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className={cn('text-sm font-medium', text.primary)}>
            {primaryValue}
          </span>
          {secondaryValue !== null && secondaryValue !== undefined && (
            <span className={cn('text-xs', text.tertiary)}>
              {String(secondaryValue)}
            </span>
          )}
          {visibleBadges?.map((badge, idx) => (
            <Badge key={idx} color={badge.color as BadgeColor}>
              {badge.label}
            </Badge>
          ))}
        </div>
      </div>
      {actions && (
        <div className="flex items-center gap-1">
          {actions}
        </div>
      )}
    </div>
  )
}

ListItem.displayName = 'ListItem'
