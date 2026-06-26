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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Modal,
  Typography,
  Input,
  InputNumber,
  Tabs,
  TabPane,
} from '@douyinfe/semi-ui';
import { CreditCard } from 'lucide-react';
import {
  displayAmountToQuota,
  quotaToDisplayAmount,
} from '../../../helpers/quota';

const TransferModal = ({
  t,
  openTransfer,
  transfer,
  handleTransferCancel,
  userState,
  renderQuota,
  getQuotaPerUnit,
  transferAmount,
  setTransferAmount,
}) => {
  const quotaPerUnit = getQuotaPerUnit();
  const [transferMode, setTransferMode] = useState('amount');
  const [displayAmount, setDisplayAmount] = useState(
    quotaToDisplayAmount(quotaPerUnit),
  );

  const quotaToTransfer = useMemo(() => {
    if (transferMode === 'amount') {
      return displayAmountToQuota(displayAmount);
    }
    return Number(transferAmount || 0);
  }, [displayAmount, transferAmount, transferMode]);

  useEffect(() => {
    if (!openTransfer) {
      return;
    }
    setTransferMode('amount');
    setDisplayAmount(quotaToDisplayAmount(quotaPerUnit));
    setTransferAmount(quotaPerUnit);
  }, [openTransfer, quotaPerUnit, setTransferAmount]);

  const handleOk = () => {
    setTransferAmount(quotaToTransfer);
    transfer(quotaToTransfer);
  };

  return (
    <Modal
      title={
        <div className='flex items-center'>
          <CreditCard className='mr-2' size={18} />
          {t('Transfer Rewards')}
        </div>
      }
      visible={openTransfer}
      onOk={handleOk}
      onCancel={handleTransferCancel}
      maskClosable={false}
      centered
    >
      <div className='space-y-4'>
        <div>
          <Typography.Text strong className='block mb-2'>
            {t('Available Rewards')}
          </Typography.Text>
          <Input
            value={renderQuota(userState?.user?.aff_quota)}
            disabled
            className='!rounded-lg'
          />
        </div>
        <div>
          <Tabs
            type='button'
            activeKey={transferMode}
            onChange={setTransferMode}
            className='mb-3'
          >
            <TabPane tab={t('By amount')} itemKey='amount' />
            <TabPane tab={t('By tokens')} itemKey='tokens' />
          </Tabs>

          <Typography.Text strong className='block mb-2'>
            {transferMode === 'amount'
              ? t('Transfer Amount')
              : t('Transfer Tokens')}{' '}
            · {t('Minimum:')} {renderQuota(quotaPerUnit)}
          </Typography.Text>
          <InputNumber
            min={
              transferMode === 'amount'
                ? quotaToDisplayAmount(quotaPerUnit)
                : quotaPerUnit
            }
            max={
              transferMode === 'amount'
                ? quotaToDisplayAmount(userState?.user?.aff_quota || 0)
                : userState?.user?.aff_quota || 0
            }
            value={transferMode === 'amount' ? displayAmount : transferAmount}
            onChange={(value) => {
              if (transferMode === 'amount') {
                setDisplayAmount(value);
                return;
              }
              setTransferAmount(value);
            }}
            className='w-full !rounded-lg'
          />
          <Typography.Text type='tertiary' size='small' className='block mt-2'>
            {t('Will transfer')}: {renderQuota(quotaToTransfer)} (
            {Number(quotaToTransfer || 0).toLocaleString()} {t('tokens')})
          </Typography.Text>
        </div>
      </div>
    </Modal>
  );
};

export default TransferModal;
