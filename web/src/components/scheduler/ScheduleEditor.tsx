import { useState, useEffect } from 'react'
import DatePicker from 'react-datepicker'
import 'react-datepicker/dist/react-datepicker.css'
import { Button } from '@/components/ui'
import {
  cronToHuman,
  humanToCron,
  cronToReadable,
  isValidCron,
  type HumanSchedule,
  type ScheduleFrequency,
} from '@/lib/utils/cron'

interface ScheduleEditorProps {
  taskId: string
  taskName: string
  currentSchedule: string
  onSave: (schedule: string) => void
  onCancel: () => void
  isSubmitting?: boolean
}

export const ScheduleEditor = ({
  taskName,
  currentSchedule,
  onSave,
  onCancel,
  isSubmitting = false,
}: ScheduleEditorProps) => {
  const [advancedMode, setAdvancedMode] = useState(false)
  const [customCron, setCustomCron] = useState(currentSchedule)
  const [cronError, setCronError] = useState<string>('')

  // Initialize from current cron expression
  const initialHuman = cronToHuman(currentSchedule)
  const [frequency, setFrequency] = useState<ScheduleFrequency>(
    initialHuman?.frequency || 'daily'
  )
  const [time, setTime] = useState<Date>(initialHuman?.time || new Date())
  const [dayOfWeek, setDayOfWeek] = useState<number>(initialHuman?.dayOfWeek || 1)
  const [dayOfMonth, setDayOfMonth] = useState<number>(initialHuman?.dayOfMonth || 1)

  // Check if current schedule is complex
  useEffect(() => {
    const parsed = cronToHuman(currentSchedule)
    if (parsed?.frequency === 'custom') {
      setAdvancedMode(true)
    }
  }, [currentSchedule])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    if (advancedMode) {
      // Validate custom cron
      if (!isValidCron(customCron)) {
        setCronError('Invalid cron expression format')
        return
      }
      onSave(customCron)
    } else {
      // Convert human-readable to cron
      const humanSchedule: HumanSchedule = {
        frequency,
        time,
        ...(frequency === 'weekly' && { dayOfWeek }),
        ...(frequency === 'monthly' && { dayOfMonth }),
      }
      const cron = humanToCron(humanSchedule)
      onSave(cron)
    }
  }

  const getCurrentSchedulePreview = () => {
    if (advancedMode) {
      if (!customCron.trim()) {
        return 'No schedule set'
      }
      return isValidCron(customCron) ? cronToReadable(customCron) : customCron
    }

    const humanSchedule: HumanSchedule = {
      frequency,
      time,
      ...(frequency === 'weekly' && { dayOfWeek }),
      ...(frequency === 'monthly' && { dayOfMonth }),
    }
    const cron = humanToCron(humanSchedule)
    return cronToReadable(cron)
  }

  const weekDays = [
    { value: 0, label: 'Sunday' },
    { value: 1, label: 'Monday' },
    { value: 2, label: 'Tuesday' },
    { value: 3, label: 'Wednesday' },
    { value: 4, label: 'Thursday' },
    { value: 5, label: 'Friday' },
    { value: 6, label: 'Saturday' },
  ]

  return (
    <div
      className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
      onClick={onCancel}
    >
      <div
        className="bg-white rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <form onSubmit={handleSubmit}>
          {/* Header */}
          <div className="p-6 border-b">
            <h2 className="text-xl font-semibold">Edit Schedule</h2>
            <p className="text-sm text-gray-600 mt-1">
              Configure when "{taskName}" should run
            </p>
          </div>

          {/* Body */}
          <div className="p-6 overflow-y-auto max-h-[60vh]">
            {!advancedMode ? (
              <>
                {/* Frequency Selection */}
                <div className="mb-6">
                  <label className="block text-sm font-medium text-gray-700 mb-3">
                    How often?
                  </label>
                  <div className="grid grid-cols-3 gap-3">
                    <button
                      type="button"
                      onClick={() => setFrequency('daily')}
                      className={`
                        px-4 py-3 rounded-lg border-2 transition-colors text-center
                        ${
                          frequency === 'daily'
                            ? 'border-blue-500 bg-blue-50 text-blue-700 font-medium'
                            : 'border-gray-200 hover:border-gray-300 bg-white'
                        }
                      `}
                    >
                      Daily
                    </button>
                    <button
                      type="button"
                      onClick={() => setFrequency('weekly')}
                      className={`
                        px-4 py-3 rounded-lg border-2 transition-colors text-center
                        ${
                          frequency === 'weekly'
                            ? 'border-blue-500 bg-blue-50 text-blue-700 font-medium'
                            : 'border-gray-200 hover:border-gray-300 bg-white'
                        }
                      `}
                    >
                      Weekly
                    </button>
                    <button
                      type="button"
                      onClick={() => setFrequency('monthly')}
                      className={`
                        px-4 py-3 rounded-lg border-2 transition-colors text-center
                        ${
                          frequency === 'monthly'
                            ? 'border-blue-500 bg-blue-50 text-blue-700 font-medium'
                            : 'border-gray-200 hover:border-gray-300 bg-white'
                        }
                      `}
                    >
                      Monthly
                    </button>
                  </div>
                </div>

                {/* Day Selection for Weekly */}
                {frequency === 'weekly' && (
                  <div className="mb-6">
                    <label className="block text-sm font-medium text-gray-700 mb-3">
                      Which day?
                    </label>
                    <select
                      value={dayOfWeek}
                      onChange={(e) => setDayOfWeek(parseInt(e.target.value))}
                      className="w-full px-4 py-2 border rounded-lg"
                    >
                      {weekDays.map((day) => (
                        <option key={day.value} value={day.value}>
                          {day.label}
                        </option>
                      ))}
                    </select>
                  </div>
                )}

                {/* Day Selection for Monthly */}
                {frequency === 'monthly' && (
                  <div className="mb-6">
                    <label className="block text-sm font-medium text-gray-700 mb-3">
                      Which day of the month?
                    </label>
                    <select
                      value={dayOfMonth}
                      onChange={(e) => setDayOfMonth(parseInt(e.target.value))}
                      className="w-full px-4 py-2 border rounded-lg"
                    >
                      {Array.from({ length: 31 }, (_, i) => i + 1).map((day) => (
                        <option key={day} value={day}>
                          {day}
                          {day === 1
                            ? 'st'
                            : day === 2
                              ? 'nd'
                              : day === 3
                                ? 'rd'
                                : 'th'}
                        </option>
                      ))}
                    </select>
                  </div>
                )}

                {/* Time Selection */}
                <div className="mb-6">
                  <label className="block text-sm font-medium text-gray-700 mb-3">
                    At what time?
                  </label>
                  <DatePicker
                    selected={time}
                    onChange={(date) => date && setTime(date)}
                    showTimeSelect
                    showTimeSelectOnly
                    timeIntervals={15}
                    timeCaption="Time"
                    dateFormat="h:mm aa"
                    className="w-full px-4 py-2 border rounded-lg"
                  />
                  <p className="text-xs text-gray-500 mt-1">
                    Select the time of day the task should run
                  </p>
                </div>
              </>
            ) : (
              <>
                {/* Advanced Cron Editor */}
                <div className="mb-6">
                  <label className="block text-sm font-medium text-gray-700 mb-3">
                    Cron Expression
                  </label>
                  <input
                    type="text"
                    value={customCron}
                    onChange={(e) => {
                      setCustomCron(e.target.value)
                      setCronError('')
                    }}
                    placeholder="0 3 * * *"
                    className={`w-full px-4 py-2 border rounded-lg font-mono text-sm ${
                      cronError ? 'border-red-500' : ''
                    }`}
                    required
                  />
                  {cronError && (
                    <p className="text-xs text-red-600 mt-1">{cronError}</p>
                  )}
                  <div className="mt-2 text-xs text-gray-500">
                    <p className="font-medium mb-1">Cron format:</p>
                    <p className="font-mono">minute hour day month weekday</p>
                    <p className="mt-1">
                      Example: <span className="font-mono">0 3 * * *</span> = Daily
                      at 3:00 AM
                    </p>
                    <p className="mt-1">
                      <a
                        href="https://crontab.guru"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-blue-600 hover:underline"
                      >
                        Learn more at crontab.guru →
                      </a>
                    </p>
                  </div>
                </div>
              </>
            )}

            {/* Advanced Mode Toggle */}
            <div className="pt-4 border-t">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={advancedMode}
                  onChange={(e) => {
                    setAdvancedMode(e.target.checked)
                    setCronError('')
                  }}
                  className="rounded"
                />
                <span className="text-sm font-medium">
                  Advanced: Use custom cron expression
                </span>
              </label>
            </div>

            {/* Schedule Preview */}
            <div className="mt-6 p-4 bg-blue-50 border border-blue-200 rounded-lg">
              <h4 className="text-sm font-medium text-blue-900 mb-2">
                Schedule Preview:
              </h4>
              <p className="text-blue-800 font-medium">{getCurrentSchedulePreview()}</p>
              {!advancedMode && (
                <p className="text-xs text-blue-600 mt-1 font-mono">
                  {humanToCron({
                    frequency,
                    time,
                    ...(frequency === 'weekly' && { dayOfWeek }),
                    ...(frequency === 'monthly' && { dayOfMonth }),
                  })}
                </p>
              )}
            </div>
          </div>

          {/* Footer */}
          <div className="p-4 border-t bg-gray-50 flex justify-end gap-2">
            <Button
              type="button"
              variant="secondary"
              onClick={onCancel}
              disabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? 'Saving...' : 'Save Schedule'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}
