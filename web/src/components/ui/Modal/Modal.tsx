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
        className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50"
        onClick={onClose}
      >
        <div
          ref={ref}
          className={cn(
            'bg-white rounded-lg shadow-xl w-full max-h-[90vh] overflow-auto',
            sizes[size],
            className
          )}
          onClick={(e) => e.stopPropagation()}
          {...props}
        >
          {title && (
            <div className="flex justify-between items-start p-6 border-b border-gray-200">
              <h2 className="text-2xl font-bold">{title}</h2>
              <button
                onClick={onClose}
                className="text-gray-400 hover:text-gray-600 text-2xl leading-none"
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
    return <div ref={ref} className={cn('p-6', className)} {...props} />
  }
)

ModalContent.displayName = 'ModalContent'

export const ModalFooter = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => {
    return (
      <div
        ref={ref}
        className={cn('p-6 border-t border-gray-200 flex gap-2 justify-end', className)}
        {...props}
      />
    )
  }
)

ModalFooter.displayName = 'ModalFooter'
