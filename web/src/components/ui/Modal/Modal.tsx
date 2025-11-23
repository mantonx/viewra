import { cn } from '@/lib/utils'
import type { HTMLAttributes } from 'react'
import { forwardRef, useEffect } from 'react'

export interface ModalProps extends HTMLAttributes<HTMLDivElement> {
  isOpen: boolean
  onClose: () => void
  title?: string
  size?: 'sm' | 'md' | 'lg' | 'xl'
}

export const Modal = forwardRef<HTMLDivElement, ModalProps>(
  ({ isOpen, onClose, title, size = 'md', className, children, ...props }, ref) => {
    useEffect(() => {
      const handleEscape = (e: KeyboardEvent) => {
        if (e.key === 'Escape' && isOpen) {
          onClose()
        }
      }

      document.addEventListener('keydown', handleEscape)
      return () => document.removeEventListener('keydown', handleEscape)
    }, [isOpen, onClose])

    useEffect(() => {
      if (isOpen) {
        document.body.style.overflow = 'hidden'
      } else {
        document.body.style.overflow = 'unset'
      }

      return () => {
        document.body.style.overflow = 'unset'
      }
    }, [isOpen])

    if (!isOpen) {
      return null
    }

    const sizes = {
      sm: 'max-w-md',
      md: 'max-w-2xl',
      lg: 'max-w-4xl',
      xl: 'max-w-6xl',
    }

    return (
      <div
        className="modal-overlay fixed inset-0 bg-black/60 dark:bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50"
        onClick={onClose}
      >
        <div
          ref={ref}
          className={cn(
            'modal-content bg-white dark:bg-neutral-900 rounded-xl shadow-2xl dark:shadow-neutral-950/50 w-full max-h-[90vh] flex flex-col overflow-hidden',
            sizes[size],
            className
          )}
          onClick={(e) => e.stopPropagation()}
          {...props}
        >
          {title && (
            <div className="flex justify-between items-start p-6 border-b border-neutral-200 dark:border-neutral-800 shrink-0">
              <h2 className="text-2xl font-bold text-neutral-900 dark:text-neutral-50">{title}</h2>
              <button
                onClick={onClose}
                className="cursor-pointer text-neutral-400 hover:text-neutral-600 dark:text-neutral-500 dark:hover:text-neutral-300 text-2xl leading-none focus:outline-none focus:ring-2 focus:ring-neutral-400 dark:focus:ring-neutral-600 rounded"
                aria-label="Close modal"
              >
                ×
              </button>
            </div>
          )}
          {children}
        </div>
      </div>
    )
  }
)

Modal.displayName = 'Modal'

export const ModalContent = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => {
    return <div ref={ref} className={cn('p-6 overflow-y-auto flex-1', className)} {...props} />
  }
)

ModalContent.displayName = 'ModalContent'

export const ModalFooter = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => {
    return (
      <div
        ref={ref}
        className={cn('p-6 border-t border-neutral-200 dark:border-neutral-800 flex gap-2 justify-end shrink-0', className)}
        {...props}
      />
    )
  }
)

ModalFooter.displayName = 'ModalFooter'
