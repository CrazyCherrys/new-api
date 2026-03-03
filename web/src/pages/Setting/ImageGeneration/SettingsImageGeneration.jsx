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

import React, { useEffect, useState, useRef } from 'react';
import { Button, Col, Form, Row, Spin, InputNumber, Input, Switch } from '@douyinfe/semi-ui';
import { showError, showSuccess, showWarning } from '../../../helpers';
import { getImageConfig, updateImageConfig } from '../../../helpers/imageApi';
import { useTranslation } from 'react-i18next';

export default function SettingsImageGeneration() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    s3_enabled: false,
    s3_endpoint: '',
    s3_bucket: '',
    s3_access_key: '',
    s3_secret_key: '',
    timeout_seconds: 300,
    max_retries: 3,
    retry_interval_seconds: 5,
    rpm_limit: 10,
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);

  async function loadConfig() {
    setLoading(true);
    try {
      const res = await getImageConfig();
      if (res.data?.success && res.data?.data) {
        const config = res.data.data;
        setInputs(config);
        setInputsRow(structuredClone(config));
        refForm.current?.setValues(config);
      }
    } catch (error) {
      showError(t('加载配置失败'));
    } finally {
      setLoading(false);
    }
  }

  async function onSubmit() {
    // 简单对比是否有修改
    if (JSON.stringify(inputs) === JSON.stringify(inputsRow)) {
      return showWarning(t('你似乎并没有修改什么'));
    }

    setLoading(true);
    try {
      const res = await updateImageConfig(inputs);
      if (res.data?.success) {
        showSuccess(t('保存成功'));
        setInputsRow(structuredClone(inputs));
      } else {
        showError(res.data?.message || t('保存失败'));
      }
    } catch (error) {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadConfig();
  }, []);

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('存储配置')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'s3_enabled'}
                  label={t('启用 S3 存储')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={(value) => {
                    setInputs({
                      ...inputs,
                      s3_enabled: value,
                    });
                  }}
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12}>
                <Form.Input
                  field={'s3_endpoint'}
                  label={t('S3 Endpoint')}
                  placeholder='https://s3.amazonaws.com'
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      s3_endpoint: value,
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12}>
                <Form.Input
                  field={'s3_bucket'}
                  label={t('S3 Bucket')}
                  placeholder='my-bucket'
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      s3_bucket: value,
                    })
                  }
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12}>
                <Form.Input
                  field={'s3_access_key'}
                  label={t('Access Key')}
                  placeholder='AKIAIOSFODNN7EXAMPLE'
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      s3_access_key: value,
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12}>
                <Form.Input
                  field={'s3_secret_key'}
                  label={t('Secret Key')}
                  mode='password'
                  placeholder='wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY'
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      s3_secret_key: value,
                    })
                  }
                />
              </Col>
            </Row>
          </Form.Section>

          <Form.Section text={t('生成参数')}>
            <Row gutter={16}>
              <Col xs={24} sm={8}>
                <Form.InputNumber
                  field={'timeout_seconds'}
                  label={t('超时时间（秒）')}
                  min={30}
                  max={600}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      timeout_seconds: value,
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={8}>
                <Form.InputNumber
                  field={'max_retries'}
                  label={t('最大重试次数')}
                  min={0}
                  max={10}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      max_retries: value,
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={8}>
                <Form.InputNumber
                  field={'retry_interval_seconds'}
                  label={t('重试间隔（秒）')}
                  min={1}
                  max={60}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      retry_interval_seconds: value,
                    })
                  }
                />
              </Col>
            </Row>
          </Form.Section>

          <Form.Section text={t('RPM 限制')}>
            <Row gutter={16}>
              <Col xs={24} sm={12}>
                <Form.InputNumber
                  field={'rpm_limit'}
                  label={t('单模型请求频率限制')}
                  min={1}
                  max={1000}
                  suffix={t('请求/分钟')}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      rpm_limit: value,
                    })
                  }
                />
              </Col>
            </Row>
          </Form.Section>

          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存图像生成设置')}
            </Button>
          </Row>
        </Form>
      </Spin>
    </>
  );
}
