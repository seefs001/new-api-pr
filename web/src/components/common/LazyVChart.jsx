import React, { Suspense } from 'react';

const VChartLazy = React.lazy(() =>
  import('@visactor/react-vchart').then((module) => ({
    default: module.VChart,
  })),
);

const fallbackStyle = {
  width: '100%',
  height: '100%',
  minHeight: '100%',
  borderRadius: '8px',
  backgroundColor: 'var(--semi-color-fill-0)',
};

const LazyVChart = ({ fallback, ...props }) => {
  return (
    <Suspense fallback={fallback || <div style={fallbackStyle} />}>
      <VChartLazy {...props} />
    </Suspense>
  );
};

export default LazyVChart;
