import { useState } from 'react'

export interface HoverPlayButtonProps {
  /**
   * Whether the parent card/item is being hovered
   */
  isParentHovered: boolean

  /**
   * Icon type - determines which icon to show
   */
  iconType?: 'play' | 'view'

  /**
   * Size variant for the button
   */
  size?: 'small' | 'medium' | 'large'

  /**
   * Border radius style - matches the container
   */
  rounded?: 'normal' | 'full'
}

/**
 * HoverPlayButton - Reusable centered play/view button with transparent hover states
 *
 * Features:
 * - Appears when parent is hovered
 * - Button starts at 50% opacity with 20% backdrop
 * - Increases to 100% opacity with 60% backdrop when button itself is hovered
 * - Supports different sizes and icon types
 */
export const HoverPlayButton = ({
  isParentHovered,
  iconType = 'play',
  size = 'medium',
  rounded = 'normal',
}: HoverPlayButtonProps) => {
  const [isButtonHovered, setIsButtonHovered] = useState(false)

  // Size configurations
  const sizeConfig = {
    small: {
      button: 'w-10 h-10',
      icon: 'w-4 h-4',
    },
    medium: {
      button: 'w-16 h-16',
      icon: 'w-6 h-6',
    },
    large: {
      button: 'w-20 h-20',
      icon: 'w-8 h-8',
    },
  }

  const config = sizeConfig[size]
  const roundedClass = rounded === 'full' ? 'rounded-full' : 'rounded'

  if (!isParentHovered) {
    return null
  }

  return (
    <div className={`absolute inset-0 flex items-center justify-center transition-all duration-200 ${roundedClass}`}>
      {/* Semi-transparent backdrop */}
      <div
        className={`absolute inset-0 bg-black transition-opacity duration-200 ${roundedClass} ${
          isButtonHovered ? 'opacity-60' : 'opacity-20'
        }`}
      />

      {/* Play/View button */}
      <div
        className={`relative ${config.button} rounded-full flex items-center justify-center shadow-lg transition-all duration-200 ${
          isButtonHovered ? 'bg-rose-500' : 'bg-rose-500/50'
        }`}
        onMouseEnter={() => setIsButtonHovered(true)}
        onMouseLeave={() => setIsButtonHovered(false)}
      >
        {iconType === 'play' ? (
          <svg
            className={`${config.icon} text-white ml-0.5`}
            fill="currentColor"
            viewBox="0 0 20 20"
          >
            <path d="M6.3 2.841A1.5 1.5 0 004 4.11V15.89a1.5 1.5 0 002.3 1.269l9.344-5.89a1.5 1.5 0 000-2.538L6.3 2.84z" />
          </svg>
        ) : (
          <svg
            className={`${config.icon} text-white`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M9 5l7 7-7 7"
            />
          </svg>
        )}
      </div>
    </div>
  )
}
