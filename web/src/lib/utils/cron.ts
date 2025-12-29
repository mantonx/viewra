/**
 * Cron utilities for converting between cron expressions and human-readable schedules
 */

export type ScheduleFrequency = 'interval' | 'daily' | 'weekly' | 'monthly' | 'custom'
export type IntervalUnit = 'minutes' | 'hours'

export interface HumanSchedule {
  frequency: ScheduleFrequency
  time: Date // Time of day (for daily/weekly/monthly)
  dayOfWeek?: number // 0-6 (Sunday-Saturday) for weekly
  dayOfMonth?: number // 1-31 for monthly
  intervalValue?: number // 15, 30, 1, 2, etc.
  intervalUnit?: IntervalUnit // 'minutes' | 'hours'
}

/**
 * Preset interval options for the schedule editor
 */
export const INTERVAL_PRESETS = [
  { value: 15, unit: 'minutes' as IntervalUnit, label: 'Every 15 minutes', cron: '*/15 * * * *' },
  { value: 30, unit: 'minutes' as IntervalUnit, label: 'Every 30 minutes', cron: '*/30 * * * *' },
  { value: 1, unit: 'hours' as IntervalUnit, label: 'Every hour', cron: '0 * * * *' },
  { value: 2, unit: 'hours' as IntervalUnit, label: 'Every 2 hours', cron: '0 */2 * * *' },
  { value: 4, unit: 'hours' as IntervalUnit, label: 'Every 4 hours', cron: '0 */4 * * *' },
  { value: 6, unit: 'hours' as IntervalUnit, label: 'Every 6 hours', cron: '0 */6 * * *' },
  { value: 12, unit: 'hours' as IntervalUnit, label: 'Every 12 hours', cron: '0 */12 * * *' },
] as const

/**
 * Parse a cron expression into a human-readable schedule
 */
export const cronToHuman = (cronExpr: string): HumanSchedule | null => {
  const parts = cronExpr.trim().split(/\s+/)
  if (parts.length !== 5) {
    return null
  }

  const [minute, hour, dayOfMonth, month, dayOfWeek] = parts

  // Create a date object for the time (using today as base)
  const time = new Date()
  time.setSeconds(0)
  time.setMilliseconds(0)

  // Check for interval patterns first

  // Every N minutes: */N * * * *
  if (minute.startsWith('*/') && hour === '*' && dayOfMonth === '*' && month === '*' && dayOfWeek === '*') {
    const intervalValue = parseInt(minute.slice(2))
    if (!isNaN(intervalValue)) {
      time.setHours(0)
      time.setMinutes(0)
      return {
        frequency: 'interval',
        time,
        intervalValue,
        intervalUnit: 'minutes',
      }
    }
  }

  // Every hour: 0 * * * * (or N * * * * where N is fixed minute)
  if (!minute.includes('*') && !minute.includes('/') && hour === '*' && dayOfMonth === '*' && month === '*' && dayOfWeek === '*') {
    const minuteNum = parseInt(minute)
    if (!isNaN(minuteNum)) {
      time.setHours(0)
      time.setMinutes(minuteNum)
      return {
        frequency: 'interval',
        time,
        intervalValue: 1,
        intervalUnit: 'hours',
      }
    }
  }

  // Every N hours: 0 */N * * * (or M */N * * * where M is fixed minute)
  if (!minute.includes('*') && hour.startsWith('*/') && dayOfMonth === '*' && month === '*' && dayOfWeek === '*') {
    const intervalValue = parseInt(hour.slice(2))
    const minuteNum = parseInt(minute)
    if (!isNaN(intervalValue) && !isNaN(minuteNum)) {
      time.setHours(0)
      time.setMinutes(minuteNum)
      return {
        frequency: 'interval',
        time,
        intervalValue,
        intervalUnit: 'hours',
      }
    }
  }

  // Parse time for non-interval schedules
  const hourNum = parseInt(hour)
  const minuteNum = parseInt(minute)
  if (!isNaN(hourNum) && !isNaN(minuteNum)) {
    time.setHours(hourNum)
    time.setMinutes(minuteNum)
  } else {
    time.setHours(0)
    time.setMinutes(0)
  }

  // Daily schedule: "30 14 * * *" (2:30 PM daily)
  if (dayOfMonth === '*' && month === '*' && dayOfWeek === '*' && !hour.includes('/') && !hour.includes('*')) {
    return {
      frequency: 'daily',
      time,
    }
  }

  // Weekly schedule: "0 9 * * 1" (9 AM every Monday)
  if (dayOfMonth === '*' && month === '*' && dayOfWeek !== '*' && !hour.includes('/') && !hour.includes('*')) {
    return {
      frequency: 'weekly',
      time,
      dayOfWeek: parseInt(dayOfWeek),
    }
  }

  // Monthly schedule: "0 3 1 * *" (3 AM on the 1st of each month)
  if (dayOfMonth !== '*' && month === '*' && dayOfWeek === '*' && !hour.includes('/') && !hour.includes('*')) {
    return {
      frequency: 'monthly',
      time,
      dayOfMonth: parseInt(dayOfMonth),
    }
  }

  // Custom/complex schedule
  return {
    frequency: 'custom',
    time,
  }
}

/**
 * Convert a human-readable schedule to a cron expression
 */
export const humanToCron = (schedule: HumanSchedule): string => {
  const minute = schedule.time.getMinutes()
  const hour = schedule.time.getHours()

  switch (schedule.frequency) {
    case 'interval':
      if (schedule.intervalUnit === 'minutes') {
        return `*/${schedule.intervalValue} * * * *`
      } else if (schedule.intervalUnit === 'hours') {
        if (schedule.intervalValue === 1) {
          return `${minute} * * * *`
        }
        return `${minute} */${schedule.intervalValue} * * *`
      }
      return `${minute} ${hour} * * *` // Fallback to daily

    case 'daily':
      return `${minute} ${hour} * * *`

    case 'weekly':
      if (schedule.dayOfWeek === undefined) {
        throw new Error('Day of week required for weekly schedule')
      }
      return `${minute} ${hour} * * ${schedule.dayOfWeek}`

    case 'monthly':
      if (schedule.dayOfMonth === undefined) {
        throw new Error('Day of month required for monthly schedule')
      }
      return `${minute} ${hour} ${schedule.dayOfMonth} * *`

    case 'custom':
      // For custom, we just use daily as fallback
      return `${minute} ${hour} * * *`

    default:
      return `${minute} ${hour} * * *`
  }
}

/**
 * Format a cron expression as human-readable text
 */
export const cronToReadable = (cronExpr: string): string => {
  const schedule = cronToHuman(cronExpr)
  if (!schedule) {
    return cronExpr
  }

  const timeStr = schedule.time.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  })

  const days = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

  switch (schedule.frequency) {
    case 'interval':
      if (schedule.intervalUnit === 'minutes') {
        return `Every ${schedule.intervalValue} minutes`
      } else if (schedule.intervalUnit === 'hours') {
        if (schedule.intervalValue === 1) {
          return 'Every hour'
        }
        return `Every ${schedule.intervalValue} hours`
      }
      return cronExpr

    case 'daily':
      return `Daily at ${timeStr}`

    case 'weekly': {
      const dayName = days[schedule.dayOfWeek ?? 0]
      return `Every ${dayName} at ${timeStr}`
    }

    case 'monthly': {
      const suffix = getDaySuffix(schedule.dayOfMonth ?? 1)
      return `Monthly on the ${schedule.dayOfMonth}${suffix} at ${timeStr}`
    }

    case 'custom':
      return cronExpr // Show the raw cron for complex schedules

    default:
      return cronExpr
  }
}

const getDaySuffix = (day: number): string => {
  if (day >= 11 && day <= 13) {
    return 'th'
  }
  switch (day % 10) {
    case 1:
      return 'st'
    case 2:
      return 'nd'
    case 3:
      return 'rd'
    default:
      return 'th'
  }
}

/**
 * Validate a cron expression format
 */
export const isValidCron = (cronExpr: string): boolean => {
  const parts = cronExpr.trim().split(/\s+/)
  if (parts.length !== 5) {
    return false
  }

  const [minute, hour, dayOfMonth, month, dayOfWeek] = parts

  // Basic validation
  const isValidPart = (part: string, min: number, max: number) => {
    if (part === '*') {
      return true
    }
    if (part.startsWith('*/')) {
      const step = parseInt(part.slice(2))
      return !isNaN(step) && step > 0
    }
    if (part.includes('/')) {
      const [range, step] = part.split('/')
      if (range === '*') {
        return !isNaN(parseInt(step))
      }
      // Handle range/step like 0-59/5
      const [start, end] = range.split('-').map(Number)
      return !isNaN(start) && !isNaN(end) && !isNaN(parseInt(step))
    }
    if (part.includes(',')) {
      return part.split(',').every((p) => {
        const num = parseInt(p)
        return !isNaN(num) && num >= min && num <= max
      })
    }
    if (part.includes('-')) {
      const [start, end] = part.split('-').map(Number)
      return !isNaN(start) && !isNaN(end) && start >= min && end <= max
    }
    const num = parseInt(part)
    return !isNaN(num) && num >= min && num <= max
  }

  return (
    isValidPart(minute, 0, 59) &&
    isValidPart(hour, 0, 23) &&
    isValidPart(dayOfMonth, 1, 31) &&
    isValidPart(month, 1, 12) &&
    isValidPart(dayOfWeek, 0, 6)
  )
}

/**
 * Get a short description for a cron expression (for compact display)
 */
export const cronToShortDescription = (cronExpr: string): string => {
  const schedule = cronToHuman(cronExpr)
  if (!schedule) {
    return cronExpr
  }

  switch (schedule.frequency) {
    case 'interval':
      if (schedule.intervalUnit === 'minutes') {
        return `Every ${schedule.intervalValue}m`
      }
      if (schedule.intervalValue === 1) {
        return 'Hourly'
      }
      return `Every ${schedule.intervalValue}h`

    case 'daily':
      return `Daily ${formatTimeShort(schedule.time)}`

    case 'weekly': {
      const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
      return `${days[schedule.dayOfWeek ?? 0]} ${formatTimeShort(schedule.time)}`
    }

    case 'monthly':
      return `Monthly ${schedule.dayOfMonth}${getDaySuffix(schedule.dayOfMonth ?? 1)}`

    default:
      return cronExpr
  }
}

const formatTimeShort = (date: Date): string => {
  return date.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  })
}

/**
 * Find the matching preset for a cron expression
 */
export const findIntervalPreset = (
  cronExpr: string
): (typeof INTERVAL_PRESETS)[number] | undefined => {
  return INTERVAL_PRESETS.find((preset) => preset.cron === cronExpr)
}
