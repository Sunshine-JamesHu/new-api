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

import React, { useEffect, useRef, useState } from 'react';
import { Banner, Button, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import { Info } from 'lucide-react';
import {
  API,
  removeTrailingSlash,
  showError,
  showSuccess,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

export default function SettingsPaymentGatewayAlipay(props) {
  const { t } = useTranslation();
  const sectionTitle = props.hideSectionTitle
    ? undefined
    : t('支付宝官方设置');
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    AlipayAppId: '',
    AlipayPrivateKey: '',
    AlipayPublicKey: '',
    AlipayReturnUrl: '',
    AlipayPaymentMode: 'auto',
  });
  const formApiRef = useRef(null);

  useEffect(() => {
    if (props.options && formApiRef.current) {
      const currentInputs = {
        AlipayAppId: props.options.AlipayAppId || '',
        AlipayPrivateKey: props.options.AlipayPrivateKey || '',
        AlipayPublicKey: props.options.AlipayPublicKey || '',
        AlipayReturnUrl: props.options.AlipayReturnUrl || '',
        AlipayPaymentMode:
          props.options.AlipayPaymentMode === 'redirect'
            ? 'redirect'
            : 'auto',
      };

      setInputs(currentInputs);
      formApiRef.current.setValues(currentInputs);
    }
  }, [props.options]);

  const handleFormChange = (values) => {
    setInputs(values);
  };

  const submitAlipaySettings = async () => {
    setLoading(true);
    try {
      const returnUrl = removeTrailingSlash(
        (inputs.AlipayReturnUrl || '').trim(),
      );
      const options = [
        { key: 'AlipayAppId', value: (inputs.AlipayAppId || '').trim() },
        { key: 'AlipayReturnUrl', value: returnUrl },
        {
          key: 'AlipayPaymentMode',
          value:
            inputs.AlipayPaymentMode === 'redirect' ? 'redirect' : 'auto',
        },
      ];

      const privateKey = (inputs.AlipayPrivateKey || '').trim();
      const publicKey = (inputs.AlipayPublicKey || '').trim();
      if (privateKey) {
        options.push({ key: 'AlipayPrivateKey', value: privateKey });
      }
      if (publicKey) {
        options.push({ key: 'AlipayPublicKey', value: publicKey });
      }

      const results = await Promise.all(
        options.map((opt) =>
          API.put('/api/option/', {
            key: opt.key,
            value: opt.value,
          }),
        ),
      );

      const errorResults = results.filter((res) => !res.data.success);
      if (errorResults.length > 0) {
        errorResults.forEach((res) => showError(res.data.message));
      } else {
        showSuccess(t('更新成功'));
        props.refresh && props.refresh();
      }
    } catch (error) {
      showError(t('更新失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={handleFormChange}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={sectionTitle}>
          <Banner
            type='info'
            icon={<Info size={16} />}
            description={
              <div>
                <div>{t('这是支付宝官方接口，不是易支付接口。')}</div>
                <div>
                  {t('支付宝异步通知地址')}:{' '}
                  <code>{'<ServerAddress>/api/alipay/notify'}</code>
                </div>
                <div>
                  {t(
                    '桌面端默认优先展示扫码支付，未开通当面付时可切换为跳转支付。',
                  )}
                </div>
              </div>
            }
            style={{ marginBottom: 16 }}
          />

          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='AlipayAppId'
                label={t('支付宝 App ID')}
                placeholder='2021000000000000'
                autoComplete='off'
              />
            </Col>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='AlipayReturnUrl'
                label={t('支付宝同步回跳地址')}
                placeholder='https://gateway.example.com/console/log'
                extraText={t('留空则默认回到本站账单页面')}
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={24} lg={24} xl={24}>
              <Form.TextArea
                field='AlipayPrivateKey'
                label={t('支付宝应用私钥')}
                placeholder={t('留空则不更新密钥')}
                autosize={{ minRows: 4, maxRows: 8 }}
                autoComplete='new-password'
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={24} lg={24} xl={24}>
              <Form.TextArea
                field='AlipayPublicKey'
                label={t('支付宝公钥')}
                placeholder={t('留空则不更新密钥')}
                autosize={{ minRows: 4, maxRows: 8 }}
                autoComplete='new-password'
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Select
                field='AlipayPaymentMode'
                label={t('桌面端支付模式')}
                optionList={[
                  {
                    value: 'auto',
                    label: t('优先扫码，失败后跳转'),
                  },
                  {
                    value: 'redirect',
                    label: t('始终跳转支付宝收银台'),
                  },
                ]}
                extraText={t('未开通当面付时请选择跳转模式')}
              />
            </Col>
          </Row>

          <Button onClick={submitAlipaySettings} style={{ marginTop: 16 }}>
            {t('更新支付宝官方设置')}
          </Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
