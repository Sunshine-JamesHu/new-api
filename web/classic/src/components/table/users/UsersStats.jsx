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

import React, { useEffect, useState } from 'react';
import { Skeleton, Typography } from '@douyinfe/semi-ui';
import { API, renderQuota, showError } from '../../../helpers';

const UsersStats = ({ refreshKey, t }) => {
  const [loading, setLoading] = useState(true);
  const [remainingQuota, setRemainingQuota] = useState(0);

  useEffect(() => {
    let cancelled = false;

    const loadStats = async () => {
      setLoading(true);
      try {
        const res = await API.get('/api/user/stats');
        const { success, message, data } = res.data;
        if (cancelled) {
          return;
        }
        if (success) {
          setRemainingQuota(data?.remaining_quota || 0);
        } else {
          showError(message || t('Failed to load user statistics'));
        }
      } catch (error) {
        if (!cancelled) {
          showError(error.message || t('Failed to load user statistics'));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    loadStats();

    return () => {
      cancelled = true;
    };
  }, [refreshKey, t]);

  return (
    <div className='flex items-center gap-2 rounded-lg border border-solid border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] px-3 py-1.5'>
      <Typography.Text type='tertiary'>
        {t('Unconsumed Balance')}
      </Typography.Text>
      {loading ? (
        <Skeleton.Title style={{ width: 96, margin: 0 }} />
      ) : (
        <Typography.Text strong>{renderQuota(remainingQuota)}</Typography.Text>
      )}
    </div>
  );
};

export default UsersStats;
