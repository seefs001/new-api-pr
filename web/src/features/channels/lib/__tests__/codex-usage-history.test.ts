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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { CodexUsageHistoryLimit } from '../../api'
import { buildCodexUsageChartData } from '../codex-usage-history'

describe('Codex usage history chart data', () => {
  test('preserves missing samples so the chart can break the line', () => {
    const limits: CodexUsageHistoryLimit[] = [
      {
        limit_type: 'base',
        limit_key: 'base',
        window_key: 'duration:18000',
        window_seconds: 18_000,
        points: [
          {
            ts: 1_700_000_000,
            used_percent: 20,
            min_used_percent: 10,
            max_used_percent: 20,
            sample_count: 2,
          },
          {
            ts: 1_700_003_600,
            used_percent: null,
            min_used_percent: null,
            max_used_percent: null,
            sample_count: 0,
          },
        ],
      },
    ]

    const result = buildCodexUsageChartData(limits)

    assert.deepEqual(result.rows, [
      { timestamp: 1_700_000_000, series0: 20 },
      { timestamp: 1_700_003_600, series0: null },
    ])
    assert.deepEqual(result.series, [
      {
        dataKey: 'series0',
        limitKey: 'base',
        windowSeconds: 18_000,
        windowKey: 'duration:18000',
      },
    ])
  })

  test('merges multiple windows by timestamp without inventing values', () => {
    const limits: CodexUsageHistoryLimit[] = [
      {
        limit_type: 'base',
        limit_key: 'base',
        window_key: 'duration:18000',
        window_seconds: 18_000,
        points: [
          {
            ts: 1,
            used_percent: 10,
            min_used_percent: 10,
            max_used_percent: 10,
            sample_count: 1,
          },
        ],
      },
      {
        limit_type: 'base',
        limit_key: 'base',
        window_key: 'duration:604800',
        window_seconds: 604_800,
        points: [
          {
            ts: 1,
            used_percent: 70,
            min_used_percent: 70,
            max_used_percent: 70,
            sample_count: 1,
          },
        ],
      },
    ]

    const result = buildCodexUsageChartData(limits)

    assert.deepEqual(result.rows, [{ timestamp: 1, series0: 10, series1: 70 }])
  })
})
