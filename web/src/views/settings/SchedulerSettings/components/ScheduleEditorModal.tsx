/**
 * ScheduleEditorModal Component
 * Modal for editing task schedule with user-friendly controls
 */

import { useState, useEffect, useMemo } from 'react'
import { Modal, ModalContent, ModalFooter, Button, Select, Input } from '@/components/ui'
import { ChevronDown, ChevronRight } from 'lucide-react'
import {
  cronToHuman,
  cronToReadable,
  humanToCron,
  isValidCron,
  INTERVAL_PRESETS,
  type ScheduleFrequency,
  type HumanSchedule,
} from '@/lib/utils/cron'
import type { TaskStatus } from '../SchedulerSettings.types'

interface ScheduleEditorModalProps {
  isOpen: boolean
  onClose: () => void
  task: TaskStatus | null
  onSave: (taskId: string, schedule: string) => void
  isSaving: boolean
}

const FREQUENCY_OPTIONS = [
  { value: 'interval', label: 'Interval' },
  { value: 'daily', label: 'Daily' },
  { value: 'weekly', label: 'Weekly' },
  { value: 'monthly', label: 'Monthly' },
]

const DAY_OPTIONS = [
  { value: '0', label: 'Sunday' },
  { value: '1', label: 'Monday' },
  { value: '2', label: 'Tuesday' },
  { value: '3', label: 'Wednesday' },
  { value: '4', label: 'Thursday' },
  { value: '5', label: 'Friday' },
  { value: '6', label: 'Saturday' },
]

const INTERVAL_OPTIONS = INTERVAL_PRESETS.map((preset) => ({
  value: preset.cron,
  label: preset.label,
}))

const DAY_OF_MONTH_OPTIONS = Array.from({ length: 31 }, (_, i) => ({
  value: String(i + 1),
  label: String(i + 1),
}))

const parseTimeFromDate = (date: Date): string => {
  const hours = date.getHours().toString().padStart(2, '0')
  const minutes = date.getMinutes().toString().padStart(2, '0')
  return `${hours}:${minutes}`
}

const parseTimeToDate = (timeStr: string): Date => {
  const [hours, minutes] = timeStr.split(':').map(Number)
  const date = new Date()
  date.setHours(hours || 0)
  date.setMinutes(minutes || 0)
  date.setSeconds(0)
  date.setMilliseconds(0)
  return date
}

export const ScheduleEditorModal = ({
  isOpen,
  onClose,
  task,
  onSave,
  isSaving,
}: ScheduleEditorModalProps) => {
  // Form state
  const [frequency, setFrequency] = useState<ScheduleFrequency>('daily')
  const [intervalCron, setIntervalCron] = useState<string>(INTERVAL_PRESETS[2].cron) // Every hour
  const [time, setTime] = useState('09:00')
  const [dayOfWeek, setDayOfWeek] = useState('1') // Monday
  const [dayOfMonth, setDayOfMonth] = useState('1')

  // Advanced mode
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [rawCron, setRawCron] = useState('')
  const [cronError, setCronError] = useState<string | null>(null)

  // Initialize form from task schedule
  useEffect(() => {
    if (!task) {
      return
    }

    const schedule = cronToHuman(task.schedule)
    if (!schedule) {
      // Invalid or complex cron, show advanced mode
      setShowAdvanced(true)
      setRawCron(task.schedule)
      return
    }

    setFrequency(schedule.frequency)
    setTime(parseTimeFromDate(schedule.time))
    setShowAdvanced(false)
    setRawCron(task.schedule)

    if (schedule.frequency === 'interval') {
      // Find matching preset or default to hourly
      const preset = INTERVAL_PRESETS.find(
        (p) => p.value === schedule.intervalValue && p.unit === schedule.intervalUnit
      )
      setIntervalCron(preset?.cron || INTERVAL_PRESETS[2].cron)
    } else if (schedule.frequency === 'weekly' && schedule.dayOfWeek !== undefined) {
      setDayOfWeek(String(schedule.dayOfWeek))
    } else if (schedule.frequency === 'monthly' && schedule.dayOfMonth !== undefined) {
      setDayOfMonth(String(schedule.dayOfMonth))
    } else if (schedule.frequency === 'custom') {
      setShowAdvanced(true)
    }
  }, [task])

  // Build cron from form values
  const computedCron = useMemo<string>(() => {
    if (showAdvanced) {
      return rawCron
    }

    if (frequency === 'interval') {
      return intervalCron
    }

    const schedule: HumanSchedule = {
      frequency,
      time: parseTimeToDate(time),
      dayOfWeek: frequency === 'weekly' ? parseInt(dayOfWeek) : undefined,
      dayOfMonth: frequency === 'monthly' ? parseInt(dayOfMonth) : undefined,
    }

    return humanToCron(schedule)
  }, [frequency, intervalCron, time, dayOfWeek, dayOfMonth, showAdvanced, rawCron])

  // Validate cron when in advanced mode
  useEffect(() => {
    if (showAdvanced) {
      if (!rawCron.trim()) {
        setCronError('Cron expression is required')
      } else if (!isValidCron(rawCron)) {
        setCronError('Invalid cron expression')
      } else {
        setCronError(null)
      }
    } else {
      setCronError(null)
    }
  }, [rawCron, showAdvanced])

  const handleSave = () => {
    if (!task || cronError) {
      return
    }
    onSave(task.id, computedCron)
  }

  const handleClose = () => {
    // Reset state on close
    setShowAdvanced(false)
    setCronError(null)
    onClose()
  }

  if (!task) {
    return null
  }

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title={`Edit Schedule: ${task.name}`} size="md">
      <ModalContent>
        <div className="space-y-6">
          {/* Frequency selector - only in non-advanced mode */}
          {!showAdvanced && (
            <>
              <Select
                label="Frequency"
                options={FREQUENCY_OPTIONS}
                value={frequency}
                onChange={(e) => setFrequency(e.target.value as ScheduleFrequency)}
              />

              {/* Interval presets */}
              {frequency === 'interval' && (
                <Select
                  label="Run every"
                  options={INTERVAL_OPTIONS}
                  value={intervalCron}
                  onChange={(e) => setIntervalCron(e.target.value)}
                />
              )}

              {/* Time picker for daily/weekly/monthly */}
              {frequency !== 'interval' && (
                <div>
                  <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-2">
                    Time
                  </label>
                  <input
                    type="time"
                    value={time}
                    onChange={(e) => setTime(e.target.value)}
                    className="w-full px-4 py-2.5 border rounded-lg transition-all duration-200 ease-out outline-none
                      bg-white dark:bg-white/5
                      border-neutral-200 dark:border-white/10
                      text-neutral-900 dark:text-neutral-50
                      hover:border-neutral-300 dark:hover:border-white/20
                      focus:ring-2 focus:ring-primary-500/30 focus:border-primary-500 dark:focus:ring-primary-500/20
                      dark:[color-scheme:dark]"
                  />
                </div>
              )}

              {/* Day of week for weekly */}
              {frequency === 'weekly' && (
                <Select
                  label="Day of week"
                  options={DAY_OPTIONS}
                  value={dayOfWeek}
                  onChange={(e) => setDayOfWeek(e.target.value)}
                />
              )}

              {/* Day of month for monthly */}
              {frequency === 'monthly' && (
                <Select
                  label="Day of month"
                  options={DAY_OF_MONTH_OPTIONS}
                  value={dayOfMonth}
                  onChange={(e) => setDayOfMonth(e.target.value)}
                />
              )}
            </>
          )}

          {/* Advanced mode */}
          <div className="border-t border-neutral-200 dark:border-neutral-700 pt-4">
            <button
              onClick={() => setShowAdvanced(!showAdvanced)}
              className="flex items-center gap-2 text-sm text-neutral-500 hover:text-neutral-700 dark:hover:text-neutral-300 mb-4"
            >
              {showAdvanced ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
              Advanced (Raw Cron)
            </button>

            {showAdvanced && (
              <div className="space-y-4">
                <Input
                  label="Cron Expression"
                  value={rawCron}
                  onChange={(e) => setRawCron(e.target.value)}
                  error={cronError || undefined}
                  placeholder="*/30 * * * *"
                  helperText="Format: minute hour day-of-month month day-of-week"
                />

                <div className="text-xs text-neutral-500 dark:text-neutral-400 space-y-1">
                  <p>Examples:</p>
                  <ul className="list-disc list-inside space-y-0.5 pl-2">
                    <li>
                      <code className="bg-neutral-100 dark:bg-neutral-800 px-1 rounded">*/30 * * * *</code> - Every
                      30 minutes
                    </li>
                    <li>
                      <code className="bg-neutral-100 dark:bg-neutral-800 px-1 rounded">0 */4 * * *</code> - Every 4
                      hours
                    </li>
                    <li>
                      <code className="bg-neutral-100 dark:bg-neutral-800 px-1 rounded">0 9 * * 1-5</code> - Weekdays
                      at 9 AM
                    </li>
                    <li>
                      <code className="bg-neutral-100 dark:bg-neutral-800 px-1 rounded">0 0 1 * *</code> - First of
                      each month at midnight
                    </li>
                  </ul>
                </div>
              </div>
            )}
          </div>

          {/* Preview */}
          <div className="p-4 bg-neutral-50 dark:bg-neutral-800/50 rounded-lg">
            <span className="text-sm text-neutral-500 dark:text-neutral-400 block mb-1">Schedule Preview</span>
            <span className="text-neutral-900 dark:text-neutral-100 font-medium">
              {cronToReadable(computedCron)}
            </span>
          </div>
        </div>
      </ModalContent>
      <ModalFooter>
        <Button variant="secondary" onClick={handleClose}>
          Cancel
        </Button>
        <Button variant="primary" onClick={handleSave} isLoading={isSaving} disabled={!!cronError}>
          Save Schedule
        </Button>
      </ModalFooter>
    </Modal>
  )
}
