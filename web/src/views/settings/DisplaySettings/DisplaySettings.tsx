import { useBadgePreferences } from '@/lib/hooks/useBadgePreferences'
import { useTheme } from '@/contexts'
import { useToast } from '@/lib/hooks/useToast'
import { SettingsPage } from '@/components/common'
import { SettingRow } from '@/components/settings/ui'
import type { ThemeMode } from '@/contexts/ThemeContext'

export const DisplaySettings = () => {
  const { preferences, updatePreferences } = useBadgePreferences()
  const { theme, setTheme } = useTheme()
  const toast = useToast()

  // Check if any optional badge is enabled
  const showAdvancedBadges = Object.values(preferences.optional).some((v) => v)

  const handleAdvancedToggle = (enabled: boolean) => {
    updatePreferences({
      ...preferences,
      optional: {
        resolution: enabled,
        contentRating: enabled,
        codec: enabled,
        extra: enabled,
        mediaType: enabled,
      },
    })
    toast.success(enabled ? 'Advanced badges enabled' : 'Advanced badges disabled')
  }

  const handleThemeChange = (value: string) => {
    setTheme(value as ThemeMode)
    toast.success(`Theme changed to ${value}`)
  }

  return (
    <SettingsPage>
      <SettingsPage.Header
        title="Display Settings"
        description="Customize how content is displayed"
      />

      <SettingsPage.Card>
        <div className="space-y-4">
          <SettingRow
            type="select"
            label="Theme"
            value={theme}
            onChange={handleThemeChange}
            options={[
              { value: 'dark', label: 'Dark' },
              { value: 'light', label: 'Light' },
            ]}
          />

          <SettingRow
            type="toggle"
            label="Show Metadata"
            description="Display year, rating, and other info on posters"
            value={showAdvancedBadges}
            onChange={handleAdvancedToggle}
          />
        </div>
      </SettingsPage.Card>
    </SettingsPage>
  )
}
