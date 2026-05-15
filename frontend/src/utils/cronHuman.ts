/**
 * Human-readable summary for a standard 5-field cron line
 * (minute hour day-of-month month day-of-week), e.g. `0 0 * * *`.
 */
export function describeCronLine(line: string): string {
  const s = line.trim()
  if (!s) {
    return 'Run now only — no cron schedule'
  }
  const parts = s.split(/\s+/).filter((p) => p.length > 0)
  if (parts.length !== 5) {
    return 'Invalid: use five fields — minute, hour, day of month, month, weekday'
  }
  const [min, hour, dom, mon, dow] = parts

  if (min === '*' && hour === '*' && dom === '*' && mon === '*' && dow === '*') {
    return 'Every minute'
  }
  if (min === '0' && hour === '*' && dom === '*' && mon === '*' && dow === '*') {
    return 'Every hour at minute 0'
  }
  if (min === '0' && hour === '0' && dom === '*' && mon === '*' && dow === '*') {
    return 'Daily at midnight (00:00)'
  }
  if (min === '0' && hour === '12' && dom === '*' && mon === '*' && dow === '*') {
    return 'Daily at noon (12:00)'
  }

  const bits: string[] = []
  bits.push(describeMinuteField(min))
  bits.push(describeHourField(hour))
  bits.push(describeDomField(dom))
  bits.push(describeMonthField(mon))
  bits.push(describeDowField(dow))
  return capitalizeFirst(bits.join(' · '))
}

function capitalizeFirst(s: string): string {
  if (!s) return s
  return s.charAt(0).toUpperCase() + s.slice(1)
}

function describeMinuteField(part: string): string {
  if (part === '*') return 'every minute'
  if (part.startsWith('*/')) {
    const n = parseStep(part.slice(2))
    if (n) return `every ${n} minutes`
  }
  if (part.includes('-') || part.includes(',') || part.includes('/')) {
    return `minutes ${part}`
  }
  return `at minute ${part}`
}

function describeHourField(part: string): string {
  if (part === '*') return 'every hour'
  if (part.startsWith('*/')) {
    const n = parseStep(part.slice(2))
    if (n) return `every ${n} hours`
  }
  if (part.includes('-') || part.includes(',') || part.includes('/')) {
    return `hours ${part}`
  }
  return `at hour ${part} (${formatHour12(Number(part))})`
}

function describeDomField(part: string): string {
  if (part === '*') return 'every day of the month'
  if (part.startsWith('*/')) {
    const n = parseStep(part.slice(2))
    if (n) return `every ${n} days of the month`
  }
  return `on day-of-month ${part}`
}

const monthNames = [
  'January',
  'February',
  'March',
  'April',
  'May',
  'June',
  'July',
  'August',
  'September',
  'October',
  'November',
  'December',
]

function describeMonthField(part: string): string {
  if (part === '*') return 'every month'
  if (part.startsWith('*/')) {
    const n = parseStep(part.slice(2))
    if (n) return `every ${n} months`
  }
  if (/^\d+$/.test(part)) {
    const m = Number(part)
    if (m >= 1 && m <= 12) return `in ${monthNames[m - 1]}`
  }
  return `in month ${part}`
}

const dowNames = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

function describeDowField(part: string): string {
  if (part === '*') return 'any weekday'
  if (part.startsWith('*/')) {
    const n = parseStep(part.slice(2))
    if (n) return `every ${n} weekdays (0=Sun)`
  }
  if (/^\d+$/.test(part)) {
    const d = Number(part) % 7
    return `on ${dowNames[d]}`
  }
  return `weekday ${part}`
}

function parseStep(s: string): number | null {
  const n = Number.parseInt(s, 10)
  return Number.isFinite(n) && n > 0 ? n : null
}

function formatHour12(h: number): string {
  if (!Number.isFinite(h) || h < 0 || h > 23) return `${h}:00`
  const am = h < 12
  const h12 = h % 12 === 0 ? 12 : h % 12
  return `${h12}:00 ${am ? 'a.m.' : 'p.m.'}`
}
