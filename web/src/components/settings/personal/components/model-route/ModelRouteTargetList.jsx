import React from 'react';
import {
  Button,
  Input,
  InputNumber,
  Space,
  Typography,
} from '@douyinfe/semi-ui';
import { Plus, Trash2 } from 'lucide-react';

const ModelRouteTargetList = ({
  t,
  targets,
  onAddTarget,
  onRemoveTarget,
  onUpdateTarget,
}) => {
  return (
    <div className='mt-4'>
      <div className='flex justify-between items-center mb-3'>
        <Typography.Text size='small'>{t('权重目标')}</Typography.Text>
        <Button
          size='small'
          type='tertiary'
          icon={<Plus size={12} />}
          onClick={onAddTarget}
        >
          {t('新增目标')}
        </Button>
      </div>
      <div className='space-y-2'>
        {(targets || []).map((target, targetIndex) => (
          <Space
            key={targetIndex}
            align='center'
            wrap
            style={{ width: '100%' }}
          >
            <Input
              value={target.model || ''}
              placeholder={t('真实模型')}
              onChange={(value) =>
                onUpdateTarget(targetIndex, { model: value })
              }
              style={{ width: 'min(320px, 100%)' }}
              showClear
            />
            <InputNumber
              value={target.weight || 0}
              min={0}
              onChange={(value) =>
                onUpdateTarget(targetIndex, { weight: Number(value) || 0 })
              }
              style={{ width: 120 }}
            />
            <Button
              type='danger'
              theme='borderless'
              icon={<Trash2 size={14} />}
              onClick={() => onRemoveTarget(targetIndex)}
            />
          </Space>
        ))}
      </div>
    </div>
  );
};

export default ModelRouteTargetList;
