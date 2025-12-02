import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { useBadgePreferences } from '@/lib/hooks/useBadgePreferences'
import { useTheme } from '@/contexts'
import { Card, CardHeader, CardContent, SettingToggle } from '@/components/ui'
import { PageHeader } from '@/components/common'
import { useToast } from '@/lib/hooks/useToast'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { BadgePreferences } from '@/lib/types/preferences'

const DisplaySettings = () => {
  const { preferences, updatePreferences } = useBadgePreferences()
  const { theme, setTheme } = useTheme()
  const [showAdvancedBadges, setShowAdvancedBadges] = useState(() => {
    // If any optional badge is enabled, show advanced
    return Object.values(preferences.optional).some(v => v)
  })
  const toast = useToast()

  const handleAdvancedToggle = (enabled: boolean) => {
    setShowAdvancedBadges(enabled)

    // Enable/disable all optional badges at once
    const updated: BadgePreferences = {
      ...preferences,
      optional: {
        resolution: enabled,
        contentRating: enabled,
        codec: enabled,
        extra: enabled,
        mediaType: enabled,
      },
    }

    updatePreferences(updated)
    toast.success(enabled ? 'Advanced badges enabled' : 'Advanced badges disabled')
  }

  const handleThemeToggle = (isDark: boolean) => {
    const newTheme = isDark ? 'dark' : 'light'
    setTheme(newTheme)
    toast.success(`Theme changed to ${newTheme}`)
  }

  return (
    <div className="p-8 page-enter">
      <PageHeader
        title="Display Settings"
        description="Customize the appearance and display preferences for your media library"
      />

      {/* Theme Settings */}
      <Card className="mt-6">
        <CardHeader>
          <h2 className={cn('text-lg font-semibold', text.primary)}>
            Theme
          </h2>
          <p className={cn('text-sm mt-1', text.secondary)}>
            Choose how the application looks
          </p>
        </CardHeader>

        <CardContent>
          <SettingToggle
            enabled={theme === 'dark'}
            onChange={handleThemeToggle}
            label="Dark Mode"
            description={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
            ariaLabel="Dark mode"
          />
        </CardContent>
      </Card>

      {/* Badge Settings */}
      <Card className="mt-6">
        <CardHeader>
          <h2 className={cn('text-lg font-semibold', text.primary)}>
            Media Information
          </h2>
          <p className={cn('text-sm mt-1', text.secondary)}>
            Control what information is displayed on media cards
          </p>
        </CardHeader>

        <CardContent>
          <SettingToggle
            enabled={showAdvancedBadges}
            onChange={handleAdvancedToggle}
            label="Show Advanced Badges"
            description="Display technical details like resolution, codec, and content rating"
            ariaLabel="Show advanced badges"
            previewContent={
              <div className="flex gap-1.5 flex-wrap">
                <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
                  4K
                </span>
                <span className="px-2 py-1 text-xs font-semibold bg-green-600 text-white rounded">
                  H265
                </span>
                <span className="px-2 py-1 text-xs font-semibold bg-neutral-800 dark:bg-neutral-700 bg-opacity-90 text-white rounded border border-neutral-600 dark:border-neutral-500">
                  PG-13
                </span>
              </div>
            }
          />
        </CardContent>
      </Card>
    </div>
  )
}

export const Route = createFileRoute('/_layout/settings/display')({
  component: DisplaySettings,
})
