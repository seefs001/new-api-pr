import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Collapse,
  Input,
  Space,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { Braces, ListFilter, Plus } from 'lucide-react';
import {
  createDefaultModelRouteRule,
  createDefaultModelRouteTarget,
} from './modelRouteConfig';
import ModelRouteRuleCard from './ModelRouteRuleCard';

const { Text } = Typography;

const getRuleKey = (rule, index) => rule.id || `rule-${index}`;

const stringifyRuleValue = (value) => {
  if (Array.isArray(value)) {
    return value.filter(Boolean).join(', ');
  }
  return '';
};

const formatRuleSummary = (rule) => {
  const sources = stringifyRuleValue(rule.source_models) || '*';
  const targets =
    stringifyRuleValue(rule.target_models) ||
    stringifyRuleValue((rule.targets || []).map((target) => target.model)) ||
    '-';
  return `${sources} -> ${targets}`;
};

const ruleMatchesQuery = (rule, query) => {
  const trimmedQuery = query.trim().toLowerCase();
  if (!trimmedQuery) {
    return true;
  }

  const haystack = [
    rule.name,
    rule.strategy,
    ...(rule.source_models || []),
    ...(rule.endpoints || []),
    ...(rule.target_models || []),
    ...(rule.targets || []).map((target) => target.model),
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();

  return haystack.includes(trimmedQuery);
};

const RuleHeader = ({ t, rule, index }) => {
  return (
    <div className='flex flex-col gap-2 pr-2'>
      <div className='flex items-center gap-2 flex-wrap'>
        <Tag color={rule.enabled === false ? 'grey' : 'green'}>
          {rule.enabled === false ? t('已关闭') : t('已启用')}
        </Tag>
        <Text strong>{rule.name || `${t('规则')} ${index + 1}`}</Text>
        <Text type='tertiary' size='small'>
          {rule.strategy || 'first'}
        </Text>
      </div>
      <Text type='secondary' size='small' ellipsis={{ showTooltip: true }}>
        {formatRuleSummary(rule)}
      </Text>
    </div>
  );
};

const ModelRouteSettings = ({ t, rules, onChange }) => {
  const routeRules = Array.isArray(rules) ? rules : [];
  const [mode, setMode] = useState('visual');
  const [query, setQuery] = useState('');
  const [jsonValue, setJsonValue] = useState('');
  const [jsonError, setJsonError] = useState('');
  const [activeKeys, setActiveKeys] = useState([]);

  const enabledCount = routeRules.filter(
    (rule) => rule.enabled !== false,
  ).length;

  const filteredRules = useMemo(
    () =>
      routeRules
        .map((rule, index) => ({ rule, index }))
        .filter(({ rule }) => ruleMatchesQuery(rule, query)),
    [routeRules, query],
  );

  useEffect(() => {
    if (mode === 'json') {
      setJsonValue(JSON.stringify(routeRules, null, 2));
      setJsonError('');
    }
  }, [mode, routeRules]);

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
    const rule = createDefaultModelRouteRule();
    updateRules([...routeRules, rule]);
    setActiveKeys([getRuleKey(rule, routeRules.length)]);
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

  const applyJson = () => {
    try {
      const parsed = JSON.parse(jsonValue);
      if (!Array.isArray(parsed)) {
        setJsonError(t('JSON 必须是规则数组'));
        return;
      }
      updateRules(parsed);
      setJsonError('');
      setMode('visual');
    } catch (error) {
      setJsonError(error.message || t('JSON 格式不正确'));
    }
  };

  return (
    <div className='py-4'>
      <div className='flex flex-col gap-3 mb-4'>
        <div className='flex justify-between items-start gap-3 flex-wrap'>
          <div>
            <Text type='secondary' size='small' className='block'>
              {t('按请求模型和端点改写为真实模型，计费和日志按真实模型计算')}
            </Text>
            <Space className='mt-2' wrap>
              <Tag>{`${t('规则')}: ${routeRules.length}`}</Tag>
              <Tag color='green'>{`${t('启用')}: ${enabledCount}`}</Tag>
            </Space>
          </div>
          <Space wrap>
            <Button
              type={mode === 'visual' ? 'primary' : 'tertiary'}
              icon={<ListFilter size={14} />}
              onClick={() => setMode('visual')}
            >
              {t('规则视图')}
            </Button>
            <Button
              type={mode === 'json' ? 'primary' : 'tertiary'}
              icon={<Braces size={14} />}
              onClick={() => setMode('json')}
            >
              {t('JSON 编辑')}
            </Button>
            <Button type='primary' icon={<Plus size={14} />} onClick={addRule}>
              {t('新增规则')}
            </Button>
          </Space>
        </div>
      </div>

      {mode === 'json' ? (
        <div>
          <TextArea
            value={jsonValue}
            autosize={{ minRows: 14, maxRows: 28 }}
            onChange={(value) => setJsonValue(value)}
            placeholder='[]'
            showClear
          />
          {jsonError && (
            <Text type='danger' size='small' className='mt-2 block'>
              {jsonError}
            </Text>
          )}
          <div className='flex justify-end gap-2 mt-3'>
            <Button type='tertiary' onClick={() => setMode('visual')}>
              {t('取消')}
            </Button>
            <Button type='primary' onClick={applyJson}>
              {t('应用 JSON')}
            </Button>
          </div>
        </div>
      ) : routeRules.length === 0 ? (
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
        <div>
          <Input
            value={query}
            prefix={<ListFilter size={14} />}
            placeholder={t('搜索规则、模型或端点')}
            onChange={setQuery}
            showClear
            className='mb-3'
          />
          {filteredRules.length === 0 ? (
            <div
              className='border rounded-xl p-4 text-sm'
              style={{
                borderColor: 'var(--semi-color-border)',
                color: 'var(--semi-color-text-2)',
                backgroundColor: 'var(--semi-color-fill-0)',
              }}
            >
              {t('没有匹配的规则')}
            </div>
          ) : (
            <Collapse
              keepDOM
              activeKey={activeKeys}
              onChange={(nextActiveKeys) => {
                const keys = Array.isArray(nextActiveKeys)
                  ? nextActiveKeys
                  : [nextActiveKeys];
                setActiveKeys(keys.filter(Boolean));
              }}
            >
              {filteredRules.map(({ rule, index }) => (
                <Collapse.Panel
                  key={getRuleKey(rule, index)}
                  itemKey={getRuleKey(rule, index)}
                  header={<RuleHeader t={t} rule={rule} index={index} />}
                >
                  <ModelRouteRuleCard
                    t={t}
                    rule={rule}
                    ruleIndex={index}
                    onRemove={removeRule}
                    onUpdate={updateRule}
                    onAddTarget={addTarget}
                    onRemoveTarget={removeTarget}
                    onUpdateTarget={updateTarget}
                  />
                </Collapse.Panel>
              ))}
            </Collapse>
          )}
        </div>
      )}
    </div>
  );
};

export default ModelRouteSettings;
