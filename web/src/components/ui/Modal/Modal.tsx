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
        className="modal-overlay fixed inset-0 bg-black/30 dark:bg-black/50 backdrop-blur-sm flex items-center justify-center p-4 z-50"
        onClick={onClose}
      >
        <div
          ref={ref}
          className={cn(
            // Base styles
            'modal-content w-full max-h-[90vh] flex flex-col overflow-hidden rounded-xl',
            // Glass effect - more transparent to match app aesthetic
            'bg-white/70 dark:bg-white/[0.06]',
            'border border-neutral-200/50 dark:border-white/15',
            'shadow-2xl dark:shadow-black/50',
            'backdrop-blur-2xl',
            // Animation
            'animate-in',
            sizes[size],
            className
          )}
          onClick={(e) => e.stopPropagation()}
          {...props}
        >
          {title && (
            <div className="flex justify-between items-start p-6 border-b border-neutral-200/50 dark:border-white/5 shrink-0">
              <h2 className="text-xl font-semibold text-neutral-900 dark:text-neutral-50 font-display tracking-tight">{title}</h2>
              <button
                onClick={onClose}
                className="cursor-pointer p-1 -m-1 text-neutral-400 hover:text-neutral-600 dark:text-neutral-500 dark:hover:text-neutral-300 rounded-lg transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-primary-500/30"
                aria-label="Close modal"
              >
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
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
        className={cn('p-6 border-t border-neutral-200/50 dark:border-white/5 flex gap-3 justify-end shrink-0', className)}
        {...props}
      />
    )
  }
)

ModalFooter.displayName = 'ModalFooter'
