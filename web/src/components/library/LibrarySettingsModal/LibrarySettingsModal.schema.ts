import { z } from 'zod'

export const librarySettingsSchema = z.object({
  monitoring_enabled: z.boolean(),
  polling_interval_minutes: z
    .number()
    .min(1, 'Must be at least 1 minute')
    .max(1440, 'Cannot exceed 24 hours'),
  debounce_seconds: z
    .number()
    .min(1, 'Must be at least 1 second')
    .max(300, 'Cannot exceed 5 minutes'),
})

export type LibrarySettingsForm = z.infer<typeof librarySettingsSchema>
