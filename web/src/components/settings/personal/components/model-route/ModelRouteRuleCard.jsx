import React from 'react';
import {
  Button,
  Col,
  Input,
  Row,
  Select,
  Space,
  Switch,
  TagInput,
  Typography,
} from '@douyinfe/semi-ui';
import { Trash2 } from 'lucide-react';
import { endpointOptions, strategyOptions } from './modelRouteConfig';
import ModelRouteTargetList from './ModelRouteTargetList';

const ModelRouteRuleCard = ({
  t,
  rule,
  ruleIndex,
  onRemove,
  onUpdate,
  onAddTarget,
  onRemoveTarget,
  onUpdateTarget,
}) => {
  return (
    <div>
      <div className='flex justify-between items-center mb-4'>
        <Space wrap>
          <Switch
            checked={rule.enabled !== false}
            checkedText={t('开')}
            uncheckedText={t('关')}
            onChange={(value) => onUpdate(ruleIndex, { enabled: value })}
          />
          <Input
            value={rule.name || ''}
            placeholder={t('规则名称')}
            onChange={(value) => onUpdate(ruleIndex, { name: value })}
            style={{ width: 'min(260px, 52vw)' }}
            showClear
          />
        </Space>
        <Button
          type='danger'
          theme='borderless'
          icon={<Trash2 size={14} />}
          onClick={() => onRemove(ruleIndex)}
        />
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={12}>
          <Typography.Text size='small'>{t('请求模型')}</Typography.Text>
          <TagInput
            value={rule.source_models || []}
            placeholder={t('例如 auto, smart')}
            addOnBlur
            showClear
            onChange={(value) => onUpdate(ruleIndex, { source_models: value })}
            style={{ width: '100%', marginTop: 6 }}
          />
        </Col>
        <Col xs={24} md={12}>
          <Typography.Text size='small'>{t('请求端点')}</Typography.Text>
          <Select
            multiple
            filter
            value={rule.endpoints || []}
            optionList={endpointOptions}
            placeholder={t('选择请求端点')}
            onChange={(value) => onUpdate(ruleIndex, { endpoints: value })}
            style={{ width: '100%', marginTop: 6 }}
          />
        </Col>
        <Col xs={24} md={8}>
          <Typography.Text size='small'>{t('策略')}</Typography.Text>
          <Select
            value={rule.strategy || 'first'}
            optionList={strategyOptions(t)}
            onChange={(value) => onUpdate(ruleIndex, { strategy: value })}
            style={{ width: '100%', marginTop: 6 }}
          />
        </Col>
        <Col xs={24} md={16}>
          <Typography.Text size='small'>{t('真实模型')}</Typography.Text>
          <TagInput
            value={rule.target_models || []}
            placeholder={t('例如 gpt-5.5, gpt-5.5-mini')}
            addOnBlur
            showClear
            onChange={(value) => onUpdate(ruleIndex, { target_models: value })}
            style={{ width: '100%', marginTop: 6 }}
          />
        </Col>
      </Row>

      {rule.strategy === 'weighted' && (
        <ModelRouteTargetList
          t={t}
          targets={rule.targets || []}
          onAddTarget={() => onAddTarget(ruleIndex)}
          onRemoveTarget={(targetIndex) =>
            onRemoveTarget(ruleIndex, targetIndex)
          }
          onUpdateTarget={(targetIndex, patch) =>
            onUpdateTarget(ruleIndex, targetIndex, patch)
          }
        />
      )}
    </div>
  );
};

export default ModelRouteRuleCard;
