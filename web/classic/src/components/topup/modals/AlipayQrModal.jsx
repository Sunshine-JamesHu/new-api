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
import { Modal, Typography } from '@douyinfe/semi-ui';
import { QRCodeSVG } from 'qrcode.react';

const { Text } = Typography;

const AlipayQrModal = ({ t, qrCode, onCancel }) => {
  return (
    <Modal
      title={t('支付宝扫码支付')}
      visible={!!qrCode}
      footer={null}
      onCancel={onCancel}
      size='small'
      centered
    >
      <div className='flex flex-col items-center gap-4 py-2'>
        <Text type='secondary'>
          {t('请使用支付宝扫描二维码完成支付。')}
        </Text>
        <div
          style={{
            background: '#fff',
            border: '1px solid var(--semi-color-border)',
            borderRadius: 8,
            padding: 16,
          }}
        >
          <QRCodeSVG value={qrCode} size={220} />
        </div>
      </div>
    </Modal>
  );
};

export default AlipayQrModal;
