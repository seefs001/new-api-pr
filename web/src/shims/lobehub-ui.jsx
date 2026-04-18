import React from 'react';
import { Center, Flexbox } from 'react-layout-kit';
import { Typography } from '@lobehub/ui/es/index.js';

export * from '@lobehub/ui/es/index.js';
export { Center, Flexbox };

export const Text = Typography.Text;

export function TooltipGroup({ children }) {
  return <>{children}</>;
}
