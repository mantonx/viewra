import { useMemo } from 'react'
import { DropdownSelector, type DropdownOption } from '../DropdownSelector'
import type { SpeedSelectorProps } from './SpeedSelector.types'

const SPEED_OPTIONS = [
  { value: 0.25, label: '0.25x', sublabel: 'Very slow' },
  { value: 0.5, label: '0.5x', sublabel: 'Slow' },
  { value: 0.75, label: '0.75x', sublabel: 'Slower' },
  { value: 1, label: '1x', sublabel: 'Normal' },
  { value: 1.25, label: '1.25x', sublabel: 'Faster' },
  { value: 1.5, label: '1.5x', sublabel: 'Fast' },
  { value: 1.75, label: '1.75x', sublabel: 'Very fast' },
  { value: 2, label: '2x', sublabel: 'Double speed' },
]

export const SpeedSelector = ({ playbackSpeed, onSpeedChange }: SpeedSelectorProps) => {
  const options: DropdownOption<number>[] = useMemo(() => {
    return SPEED_OPTIONS.map((opt) => ({
      value: opt.value,
      label: opt.label,
      sublabel: opt.sublabel,
      isSelected: playbackSpeed === opt.value,
    }))
  }, [playbackSpeed])

  const displayText = playbackSpeed === 1 ? '1x' : `${playbackSpeed}x`

  const speedIcon = (
    <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
      <path d="M13 2.05v2.02c3.95.49 7 3.85 7 7.93 0 3.21-1.92 6-4.72 7.28L13 17v5l5-5-2.28-2.28C18.03 13.22 20 10.3 20 7c0-4.42-3.58-8-8-8-.34 0-.67.03-1 .07v2.02c.33-.04.66-.07 1-.07 3.31 0 6 2.69 6 6 0 2.61-1.67 4.83-4 5.65V10h-2v7.93c-3.95-.49-7-3.85-7-7.93 0-3.31 2.69-6 6-6V2.05c-.34-.03-.67-.05-1-.05-4.42 0-8 3.58-8 8s3.58 8 8 8c.34 0 .67-.02 1-.05V2.05z" />
    </svg>
  )

  return (
    <DropdownSelector
      displayText={displayText}
      icon={speedIcon}
      title="Playback Speed"
      subtitle="Adjust video playback rate"
      options={options}
      onSelect={onSpeedChange}
      minButtonWidth="70px"
      panelWidth="w-52"
      ariaLabel="Playback speed"
    />
  )
}

export default SpeedSelector
