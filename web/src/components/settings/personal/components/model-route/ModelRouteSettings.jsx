import React from 'react';
import { Button, Typography } from '@douyinfe/semi-ui';
import { Plus } from 'lucide-react';
import {
  createDefaultModelRouteRule,
  createDefaultModelRouteTarget,
} from './modelRouteConfig';
import ModelRouteRuleCard from './ModelRouteRuleCard';

const ModelRouteSettings = ({ t, rules, onChange }) => {
  const routeRules = Array.isArray(rules) ? rules : [];

  const updateRules = (nextRules) => {
    onChange(nextRules);
  };

  const updateRule = (index, patch) => {
    updateRules(
      routeRules.map((rule, ruleIndex) =>
        ruleIndex === index ? { ...rule, ...patch } : rule,
      ),
    );
  };

  const addRule = () => {
    updateRules([...routeRules, createDefaultModelRouteRule()]);
  };

  const removeRule = (index) => {
    updateRules(routeRules.filter((_, ruleIndex) => ruleIndex !== index));
  };

  const updateTarget = (ruleIndex, targetIndex, patch) => {
    const rule = routeRules[ruleIndex];
    const targets = Array.isArray(rule.targets) ? rule.targets : [];
    updateRule(ruleIndex, {
      targets: targets.map((target, index) =>
        index === targetIndex ? { ...target, ...patch } : target,
      ),
    });
  };

  const addTarget = (ruleIndex) => {
    const rule = routeRules[ruleIndex];
    const targets = Array.isArray(rule.targets) ? rule.targets : [];
    updateRule(ruleIndex, {
      targets: [...targets, createDefaultModelRouteTarget()],
    });
  };

  const removeTarget = (ruleIndex, targetIndex) => {
    const rule = routeRules[ruleIndex];
    const targets = Array.isArray(rule.targets) ? rule.targets : [];
    updateRule(ruleIndex, {
      targets: targets.filter((_, index) => index !== targetIndex),
    });
  };

  return (
    <div className='py-4'>
      <div className='flex justify-between items-center mb-4'>
        <Typography.Text type='secondary' size='small'>
          {t('按请求模型和端点改写为真实模型，计费和日志按真实模型计算')}
        </Typography.Text>
        <Button type='primary' icon={<Plus size={14} />} onClick={addRule}>
          {t('新增规则')}
        </Button>
      </div>

      {routeRules.length === 0 ? (
        <div
          className='border rounded-xl p-4 text-sm'
          style={{
            borderColor: 'var(--semi-color-border)',
            color: 'var(--semi-color-text-2)',
            backgroundColor: 'var(--semi-color-fill-0)',
          }}
        >
          {t('暂无模型路由规则')}
        </div>
      ) : (
        <div className='space-y-4'>
          {routeRules.map((rule, ruleIndex) => (
            <ModelRouteRuleCard
              key={rule.id || ruleIndex}
              t={t}
              rule={rule}
              ruleIndex={ruleIndex}
              onRemove={removeRule}
              onUpdate={updateRule}
              onAddTarget={addTarget}
              onRemoveTarget={removeTarget}
              onUpdateTarget={updateTarget}
            />
          ))}
        </div>
      )}
    </div>
  );
};

export default ModelRouteSettings;
