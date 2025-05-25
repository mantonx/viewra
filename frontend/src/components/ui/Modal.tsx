import React, { useEffect } from 'react';
import { cn } from '@/lib/utils';
import { X } from '@/components/ui/icons';

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  children: React.ReactNode;
  title?: string;
  showCloseButton?: boolean;
  className?: string;
  overlayClassName?: string;
  contentClassName?: string;
  size?: 'sm' | 'md' | 'lg' | 'xl' | 'full';
  closeOnOverlayClick?: boolean;
  closeOnEscape?: boolean;
}

const Modal: React.FC<ModalProps> = ({
  isOpen,
  onClose,
  children,
  title,
  showCloseButton = true,
  className,
  overlayClassName,
  contentClassName,
  size = 'md',
  closeOnOverlayClick = true,
  closeOnEscape = true,
}) => {
  // Handle escape key
  useEffect(() => {
    if (!isOpen || !closeOnEscape) return;

    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };

    document.addEventListener('keydown', handleEscape);
    return () => document.removeEventListener('keydown', handleEscape);
  }, [isOpen, closeOnEscape, onClose]);

  // Prevent body scroll when modal is open
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = 'unset';
    }

    return () => {
      document.body.style.overflow = 'unset';
    };
  }, [isOpen]);

  if (!isOpen) return null;

  const handleOverlayClick = (e: React.MouseEvent) => {
    if (closeOnOverlayClick && e.target === e.currentTarget) {
      onClose();
    }
  };

  const sizeClasses = {
    sm: 'max-w-sm',
    md: 'max-w-md',
    lg: 'max-w-2xl',
    xl: 'max-w-4xl',
    full: 'max-w-full max-h-full w-full h-full',
  };

  return (
    <div
      className={cn(
        'fixed inset-0 z-[9999] flex items-center justify-center',
        'bg-black/80 backdrop-blur-sm',
        'animate-fade-in',
        overlayClassName
      )}
      onClick={handleOverlayClick}
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        zIndex: 9999,
      }}
    >
      <div
        className={cn(
          'relative w-full bg-slate-900 rounded-xl shadow-2xl',
          'animate-zoom-in',
          // Only add margins and max-height for non-full modals
          size !== 'full' && 'my-4 mx-4 max-h-[calc(100vh-2rem)] overflow-auto',
          sizeClasses[size],
          className
        )}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        {(title || showCloseButton) && (
          <div className="flex items-center justify-between p-6 border-b border-slate-700">
            {title && <h2 className="text-xl font-semibold text-white">{title}</h2>}
            {showCloseButton && (
              <button
                onClick={onClose}
                className="p-2 text-slate-400 hover:text-white hover:bg-slate-800 rounded-lg transition-colors"
                aria-label="Close modal"
              >
                <X className="w-5 h-5" />
              </button>
            )}
          </div>
        )}

        {/* Content */}
        <div className={cn('p-6', contentClassName)}>{children}</div>
      </div>
    </div>
  );
};

export default Modal;
