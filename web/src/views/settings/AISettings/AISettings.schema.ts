import { z } from 'zod'

export const aiSettingsSchema = z.object({
  enabled: z.boolean(),
  embeddingProvider: z.string(),
  chatProvider: z.string(),
})

export type AISettingsValues = z.infer<typeof aiSettingsSchema>

export const AI_SETTINGS_DEFAULT_VALUES: AISettingsValues = {
  enabled: false,
  embeddingProvider: '',
  chatProvider: '',
}
