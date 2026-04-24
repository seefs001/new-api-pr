export const endpointOptions = [
  { label: '/v1/chat/completions', value: '/v1/chat/completions' },
  { label: '/v1/responses', value: '/v1/responses' },
  { label: '/v1/responses/compact', value: '/v1/responses/compact' },
  { label: '/v1/embeddings', value: '/v1/embeddings' },
  { label: '/v1/images/generations', value: '/v1/images/generations' },
  { label: '/v1/audio/speech', value: '/v1/audio/speech' },
  { label: '/v1/rerank', value: '/v1/rerank' },
];

export const strategyOptions = (t) => [
  { label: t('第一个'), value: 'first' },
  { label: t('随机'), value: 'random' },
  { label: t('权重'), value: 'weighted' },
];

export const createDefaultModelRouteRule = () => ({
  id: `route_${Date.now()}`,
  name: '',
  enabled: false,
  source_models: [],
  endpoints: [],
  target_models: [],
  targets: [],
  strategy: 'first',
});

export const createDefaultModelRouteTarget = () => ({
  model: '',
  weight: 1,
});
