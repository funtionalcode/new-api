/*
Copyright (C) 2025 QuantumNous

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

import React from 'react';
import { Tag, Space, Skeleton, DatePicker, Button } from '@douyinfe/semi-ui';
import { renderQuota } from '../../../helpers';
import CompactModeToggle from '../../common/ui/CompactModeToggle';
import { useMinimumLoadingTime } from '../../../hooks/common/useMinimumLoadingTime';

const formatAverageUseTime = (value, count) => {
  const time = Number(value || 0);
  const sampleCount = Number(count || 0);
  if (!Number.isFinite(time) || time <= 0 || sampleCount <= 0) {
    return '-';
  }
  return `${time >= 10 ? time.toFixed(1) : time.toFixed(2)} s`;
};

const LogsActions = ({
  stat,
  loadingStat,
  showStat,
  compactMode,
  setCompactMode,
  avgUseTimeDateRange,
  handleAvgUseTimeDateRangeChange,
  handleAvgUseTimeQuery,
  t,
}) => {
  const showSkeleton = useMinimumLoadingTime(loadingStat);
  const needSkeleton = !showStat || showSkeleton;

  const placeholder = (
    <Space>
      <Skeleton.Title style={{ width: 108, height: 21, borderRadius: 6 }} />
      <Skeleton.Title style={{ width: 65, height: 21, borderRadius: 6 }} />
      <Skeleton.Title style={{ width: 64, height: 21, borderRadius: 6 }} />
      <Skeleton.Title style={{ width: 320, height: 21, borderRadius: 6 }} />
    </Space>
  );

  return (
    <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
      <Skeleton loading={needSkeleton} active placeholder={placeholder}>
        <Space wrap>
          <Tag
            color='blue'
            style={{
              fontWeight: 500,
              boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
              padding: 13,
            }}
            className='!rounded-lg'
          >
            {t('消耗额度')}: {renderQuota(stat.quota)}
          </Tag>
          <Tag
            color='pink'
            style={{
              fontWeight: 500,
              boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
              padding: 13,
            }}
            className='!rounded-lg'
          >
            RPM: {stat.rpm}
          </Tag>
          <Tag
            color='white'
            style={{
              border: 'none',
              boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
              fontWeight: 500,
              padding: 13,
            }}
            className='!rounded-lg'
          >
            TPM: {stat.tpm}
          </Tag>
          <Tag
            color='green'
            style={{
              fontWeight: 500,
              boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
              padding: 13,
            }}
            className='!rounded-lg'
          >
            {t('平均耗时')}:{' '}
            {formatAverageUseTime(
              stat.avg_use_time,
              stat.avg_use_time_count,
            )}
          </Tag>
          <DatePicker
            type='dateTimeRange'
            value={avgUseTimeDateRange}
            onChange={handleAvgUseTimeDateRangeChange}
            placeholder={[t('平均耗时开始时间'), t('平均耗时结束时间')]}
            size='small'
            pure
            showClear={false}
            disabled={loadingStat}
            style={{ width: 340 }}
          />
          <Button
            type='tertiary'
            size='small'
            loading={loadingStat}
            onClick={handleAvgUseTimeQuery}
          >
            {t('查询')}
          </Button>
        </Space>
      </Skeleton>

      <CompactModeToggle
        compactMode={compactMode}
        setCompactMode={setCompactMode}
        t={t}
      />
    </div>
  );
};

export default LogsActions;
