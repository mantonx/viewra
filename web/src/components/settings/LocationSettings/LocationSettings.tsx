import { Card, CardHeader, CardContent, Button, Input, SettingToggle } from '@/components/ui'
import { useLocationSettings } from '@/lib/hooks'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { Search } from 'lucide-react'
import type { LocationSettingsProps } from './LocationSettings.types'

export const LocationSettings = ({ className }: LocationSettingsProps) => {
  const location = useLocationSettings()

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      location.search(location.searchQuery)
    }
  }

  return (
    <Card className={className}>
      <CardHeader>
        <h2 className={cn('text-lg font-semibold', text.primary)}>Location</h2>
        <p className={cn('text-sm mt-1', text.secondary)}>
          {location.enabled && location.locationName
            ? location.locationName
            : 'Used for personalized recommendations'}
        </p>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          <SettingToggle
            enabled={location.enabled}
            onChange={location.toggle}
            label="Enable for personalized suggestions"
            description="Recommendations based on time, season, weather, and local context"
            ariaLabel="Enable location"
            disabled={location.isPending}
          />

          {location.enabled && (
            <div className="mt-4">
              {location.locationName && !location.isChanging ? (
                <div className="flex items-center justify-between py-2 px-3 bg-neutral-50 dark:bg-neutral-800/50 rounded-lg">
                  <div>
                    <div className={cn('font-medium', text.primary)}>
                      {location.locationName}
                    </div>
                    <div className={cn('text-xs', text.secondary)}>{location.timezone}</div>
                  </div>
                  <Button variant="ghost" size="sm" onClick={location.startChanging}>
                    Change
                  </Button>
                </div>
              ) : (
                <div className="space-y-2 max-w-sm">
                  <div className="flex gap-2">
                    <Input
                      value={location.searchQuery}
                      onChange={(e) => location.setSearchQuery(e.target.value)}
                      placeholder="Search city..."
                      onKeyDown={handleKeyDown}
                    />
                    <Button
                      variant="secondary"
                      onClick={() => location.search(location.searchQuery)}
                      disabled={location.isSearching || !location.searchQuery.trim()}
                      isLoading={location.isSearching}
                    >
                      <Search className="w-4 h-4" />
                    </Button>
                  </div>

                  {location.searchResults.length > 0 && (
                    <div className="border rounded-lg divide-y dark:border-neutral-700 dark:divide-neutral-700">
                      {location.searchResults.map((result, idx) => (
                        <button
                          key={idx}
                          type="button"
                          onClick={() => location.selectLocation(result)}
                          className={cn(
                            'w-full px-3 py-2 text-left text-sm hover:bg-neutral-100 dark:hover:bg-neutral-800',
                            text.primary
                          )}
                        >
                          <span className="font-medium">{result.name}</span>
                          {result.admin1 && (
                            <span className={text.secondary}>, {result.admin1}</span>
                          )}
                          <span className={text.secondary}>, {result.country}</span>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
