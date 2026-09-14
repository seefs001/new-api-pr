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
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen, within } from '@testing-library/react'
import { expect, it } from 'vitest'

import type { ModelRanking, RankingMover } from '../../types'
import { ModelLeaderboard } from '../model-leaderboard'
import { PulseSection } from '../pulse-section'

const variants = ['default', 'compact', 'up', 'down'] as const
const cases = [
  {
    name: 'missing icon',
    model: 'codex-auto-review',
    icon: undefined,
    expected: 'OpenAI',
  },
  {
    name: 'empty icon',
    model: 'codex-mini-latest',
    icon: '',
    expected: 'OpenAI',
  },
  { name: 'blank icon', model: 'wan2.2-t2v', icon: ' \t ', expected: 'Wan' },
  {
    name: 'configured icon',
    model: 'codex-auto-review',
    icon: ' Claude.Color ',
    expected: 'Claude',
  },
  {
    name: 'configured custom icon',
    model: 'codex-auto-review',
    icon: 'Wan',
    expected: 'Wan',
  },
  {
    name: 'unknown model without an icon',
    model: 'custom-model',
    icon: undefined,
    expected: '?',
  },
]

it.each(
  variants.flatMap((variant) =>
    cases.map((testCase) => ({ ...testCase, variant }))
  )
)(
  'shows $expected for $name in the $variant ranking',
  async ({ model, icon, expected, variant }) => {
    const row: ModelRanking = {
      rank: 1,
      model_name: model,
      vendor: 'Configured vendor',
      vendor_icon: icon,
      category: 'all',
      total_tokens: 1000,
      share: 1,
      growth_pct: 0,
    }
    const mover: RankingMover = {
      model_name: row.model_name,
      vendor: row.vendor,
      vendor_icon: row.vendor_icon,
      rank_delta: variant === 'down' ? -1 : 1,
      current_rank: 1,
      growth_pct: 0,
    }
    const content =
      variant === 'up' || variant === 'down' ? (
        <PulseSection
          movers={variant === 'up' ? [mover] : []}
          droppers={variant === 'down' ? [mover] : []}
        />
      ) : (
        <ModelLeaderboard rows={[row]} variant={variant} />
      )
    const router = createRouter({
      routeTree: createRootRoute({ component: () => content }),
      history: createMemoryHistory({ initialEntries: ['/'] }),
    })
    await router.load()
    render(<RouterProvider router={router} />)

    const item = within(await screen.findByRole('listitem'))
    expect(item.getByRole('link', { name: model })).toHaveAttribute(
      'href',
      `/pricing/${model}`
    )
    expect(item.getByRole('link', { name: 'configured vendor' })).toBeVisible()
    if (expected === 'Wan') {
      expect(item.getByRole('presentation')).toHaveAttribute(
        'src',
        expect.stringContaining('wan.png')
      )
    } else if (expected === '?') {
      expect(item.getByText('?')).toBeVisible()
    } else {
      expect(item.getByTitle(expected)).toBeInTheDocument()
      expect(item.queryByText('?')).not.toBeInTheDocument()
    }
  }
)
