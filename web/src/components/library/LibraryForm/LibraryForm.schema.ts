import { z } from 'zod'

export const libraryFormSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  path: z.string().min(1, 'Path is required'),
  type: z.enum(['movies', 'tv', 'music']),
})

export type LibraryFormValues = z.infer<typeof libraryFormSchema>
