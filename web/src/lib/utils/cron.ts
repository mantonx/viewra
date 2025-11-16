/**
 * Cron utilities for converting between cron expressions and human-readable schedules
 */

export type ScheduleFrequency = 'daily' | 'weekly' | 'monthly' | 'custom'

export interface HumanSchedule {
  frequency: ScheduleFrequency
  time: Date // Time of day
  dayOfWeek?: number // 0-6 (Sunday-Saturday) for weekly
  dayOfMonth?: number // 1-31 for monthly
}

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
  time.setHours(parseInt(hour) || 0)
  time.setMinutes(parseInt(minute) || 0)
  time.setSeconds(0)
  time.setMilliseconds(0)

  // Daily schedule: "30 14 * * *" (2:30 PM daily)
  if (dayOfMonth === '*' && month === '*' && dayOfWeek === '*') {
    return {
      frequency: 'daily',
      time,
    }
  }

  // Weekly schedule: "0 9 * * 1" (9 AM every Monday)
  if (dayOfMonth === '*' && month === '*' && dayOfWeek !== '*') {
    return {
      frequency: 'weekly',
      time,
      dayOfWeek: parseInt(dayOfWeek),
    }
  }

  // Monthly schedule: "0 3 1 * *" (3 AM on the 1st of each month)
  if (dayOfMonth !== '*' && month === '*' && dayOfWeek === '*') {
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
    if (part === '*') {return true}
    if (part.includes('/')) {
      const [, step] = part.split('/')
      return !isNaN(parseInt(step))
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
