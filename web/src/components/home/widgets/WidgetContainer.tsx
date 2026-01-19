import { cn } from '@/lib/utils'
import { SearchHero } from './SearchHero'
import { MediaRow } from './MediaRow'
import { ContinueRow } from './ContinueRow'
import { WidgetType, type HomeSection, type SearchHeroData, type MediaRowData, type TrendingRowData, type ContinueWatchingData } from './widget.types'

interface WidgetContainerProps {
  section: HomeSection
  className?: string
}

/**
 * WidgetContainer - Renders a widget based on its type
 *
 * Maps widget types to their corresponding components and passes
 * the appropriate data structure.
 */
export const WidgetContainer = ({ section, className }: WidgetContainerProps) => {
  switch (section.type) {
    case WidgetType.SearchHero:
      return (
        <SearchHero
          data={section.data as SearchHeroData}
          className={className}
        />
      )

    case WidgetType.MediaRow:
    case WidgetType.FeaturedRow:
      return (
        <MediaRow
          data={section.data as MediaRowData | TrendingRowData}
          className={className}
        />
      )

    case WidgetType.ContinueRow:
      return (
        <ContinueRow
          data={section.data as ContinueWatchingData}
          className={className}
        />
      )

    default:
      // Unknown widget type - log and skip
      console.warn(`Unknown widget type: ${section.type}`)
      return null
  }
}

interface WidgetSectionProps {
  sections: HomeSection[]
  location: 'homepage-top' | 'homepage-sections'
  className?: string
}

/**
 * WidgetSection - Renders all widgets for a specific location
 *
 * Filters sections by location and renders them in priority order.
 */
export const WidgetSection = ({ sections, location, className }: WidgetSectionProps) => {
  // Filter sections by location and sort by priority (highest first)
  const locationSections = sections
    .filter((s) => s.location === location)
    .sort((a, b) => b.priority - a.priority)

  if (locationSections.length === 0) {
    return null
  }

  return (
    <div className={cn('space-y-8', className)}>
      {locationSections.map((section) => (
        <WidgetContainer key={section.id} section={section} />
      ))}
    </div>
  )
}
