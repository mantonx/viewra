import { createFileRoute } from '@tanstack/react-router'
import { Home } from '@/views/home'

export const Route = createFileRoute('/_layout/')({
  component: Home,
})
