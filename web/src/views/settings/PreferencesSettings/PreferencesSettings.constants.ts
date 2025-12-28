import { Play, Palette } from 'lucide-react'
import { createElement } from 'react'
import type { PreferencesCategoryConfig } from './PreferencesSettings.types'

export const PREFERENCES_CATEGORIES: PreferencesCategoryConfig[] = [
  {
    key: 'playback',
    label: 'Playback',
    description: 'Control how media plays',
    icon: createElement(Play, { className: 'w-5 h-5' }),
  },
  {
    key: 'ui',
    label: 'Appearance',
    description: 'Customize the look and feel',
    icon: createElement(Palette, { className: 'w-5 h-5' }),
  },
]
