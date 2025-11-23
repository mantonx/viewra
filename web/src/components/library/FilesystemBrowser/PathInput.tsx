import { useState } from 'react'
import { Button } from '@/components/ui/Button/Button'
import type { PathInputProps } from './PathInput.types'

const PathInput = ({ onNavigate, isLoading }: PathInputProps) => {
  const [showInput, setShowInput] = useState(false)
  const [pathInput, setPathInput] = useState('')
  const [pathInputError, setPathInputError] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    e.stopPropagation()

    const trimmedPath = pathInput.trim()

    if (!trimmedPath) {
      setPathInputError('Please enter a path')
      return
    }

    if (!trimmedPath.startsWith('/')) {
      setPathInputError('Path must start with /')
      return
    }

    onNavigate(trimmedPath)
    setPathInput('')
    setPathInputError('')
    setShowInput(false)
  }

  const handleCancel = () => {
    setShowInput(false)
    setPathInput('')
    setPathInputError('')
  }

  if (!showInput) {
    return (
      <div className="mb-4">
        <button
          onClick={() => setShowInput(true)}
          className="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 hover:underline"
        >
          Jump to path...
        </button>
      </div>
    )
  }

  return (
    <form onSubmit={handleSubmit} className="mb-4">
      <div className="flex gap-2">
        <div className="flex-1">
          <input
            type="text"
            value={pathInput}
            onChange={(e) => {
              setPathInput(e.target.value)
              setPathInputError('')
            }}
            placeholder="Type a path (e.g., /home/user/Videos)"
            disabled={isLoading}
            autoFocus
            className="w-full px-3 py-2 border border-neutral-300 dark:border-neutral-700 rounded-md text-sm bg-white dark:bg-neutral-900 text-neutral-900 dark:text-neutral-50 placeholder:text-neutral-500 dark:placeholder:text-neutral-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent disabled:bg-neutral-100 dark:disabled:bg-neutral-900 disabled:cursor-not-allowed"
          />
          {pathInputError && (
            <p className="mt-1 text-sm text-red-600 dark:text-red-400">{pathInputError}</p>
          )}
        </div>
        <Button
          type="submit"
          variant="secondary"
          size="sm"
          disabled={isLoading || !pathInput.trim()}
        >
          Go
        </Button>
        <Button
          type="button"
          variant="secondary"
          size="sm"
          onClick={handleCancel}
        >
          Cancel
        </Button>
      </div>
    </form>
  )
}

export { PathInput }
