/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { CodexUsageHistoryLimit } from '../api'

export type CodexUsageChartRow = {
  timestamp: number
  [key: string]: number | null
}

export type CodexUsageChartSeries = {
  dataKey: string
  limitKey: string
  windowKey: string
  windowSeconds: number | null
}

export type CodexUsageChartData = {
  rows: CodexUsageChartRow[]
  series: CodexUsageChartSeries[]
}

export function buildCodexUsageChartData(
  limits: CodexUsageHistoryLimit[]
): CodexUsageChartData {
  const series = limits.map((limit, index) => ({
    dataKey: `series${index}`,
    limitKey: limit.limit_key,
    windowKey: limit.window_key,
    windowSeconds: limit.window_seconds,
  }))
  const rowsByTimestamp = new Map<number, CodexUsageChartRow>()

  for (const limit of limits) {
    for (const point of limit.points) {
      if (!rowsByTimestamp.has(point.ts)) {
        const row: CodexUsageChartRow = { timestamp: point.ts }
        for (const item of series) row[item.dataKey] = null
        rowsByTimestamp.set(point.ts, row)
      }
    }
  }

  limits.forEach((limit, index) => {
    const dataKey = series[index].dataKey
    for (const point of limit.points) {
      const row = rowsByTimestamp.get(point.ts)
      if (row) row[dataKey] = point.used_percent
    }
  })

  return {
    rows: [...rowsByTimestamp.values()].sort(
      (left, right) => left.timestamp - right.timestamp
    ),
    series,
  }
}
