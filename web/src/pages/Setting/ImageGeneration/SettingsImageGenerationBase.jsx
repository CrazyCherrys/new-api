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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Button, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { showError, showSuccess, showWarning } from '../../../helpers';
import { updateImageConfig } from '../../../helpers/imageApi';
import {
  baseConfigKeys,
  defaultImageGenerationInputs,
  pickFields,
  transformToBackend,
} from './shared';

export default function SettingsImageGenerationBase({ options, refresh }) {
  const { t } = useTranslation();
  const refForm = useRef();
  const [loading, setLoading] = useState(false);

  const baseOptions = useMemo(
    () =>
      pickFields(
        {
          ...defaultImageGenerationInputs,
          ...options,
        },
        baseConfigKeys,
      ),
    [options],
  );

  const [inputs, setInputs] = useState(baseOptions);
  const [inputsRow, setInputsRow] = useState(baseOptions);

  useEffect(() => {
    setInputs(baseOptions);
    setInputsRow(structuredClone(baseOptions));
    refForm.current?.setValues(baseOptions);
  }, [baseOptions]);

  async function onSubmit() {
    if (JSON.stringify(inputs) === JSON.stringify(inputsRow)) {
      return showWarning(t('你似乎并没有修改什么'));
    }

    setLoading(true);
    try {
      const merged = {
        ...defaultImageGenerationInputs,
        ...options,
        ...inputs,
      };
      const res = await updateImageConfig(transformToBackend(merged));
      if (res.data?.success) {
        showSuccess(t('保存成功'));
        setInputsRow(structuredClone(inputs));
        await refresh?.();
      } else {
        showError(res.data?.message || t('保存失败'));
      }
    } catch (error) {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  }

  const isS3Enabled = inputs.s3_enabled;

  return (
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
                    storage_type: value ? 's3' : 'local',
                  });
                }}
              />
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} sm={12}>
              <Form.Input
                field={'storage_local_path'}
                label={t('本地存储路径')}
                placeholder='./data/images'
                disabled={isS3Enabled}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    storage_local_path: value,
                  })
                }
              />
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} sm={12}>
              <Form.Input
                field={'storage_s3_endpoint'}
                label={t('S3 Endpoint')}
                placeholder='https://s3.amazonaws.com'
                disabled={!isS3Enabled}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    storage_s3_endpoint: value,
                  })
                }
              />
            </Col>
            <Col xs={24} sm={12}>
              <Form.Input
                field={'storage_s3_bucket'}
                label={t('S3 Bucket')}
                placeholder='my-bucket'
                disabled={!isS3Enabled}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    storage_s3_bucket: value,
                  })
                }
              />
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} sm={12}>
              <Form.Input
                field={'storage_s3_region'}
                label={t('S3 区域')}
                placeholder='us-east-1'
                disabled={!isS3Enabled}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    storage_s3_region: value,
                  })
                }
              />
            </Col>
            <Col xs={24} sm={12}>
              <Form.Input
                field={'storage_s3_path_prefix'}
                label={t('S3 路径前缀')}
                placeholder='generated-images'
                disabled={!isS3Enabled}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    storage_s3_path_prefix: value,
                  })
                }
              />
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} sm={12}>
              <Form.Input
                field={'storage_s3_access_key'}
                label={t('Access Key')}
                placeholder='AKIAIOSFODNN7EXAMPLE'
                disabled={!isS3Enabled}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    storage_s3_access_key: value,
                  })
                }
              />
            </Col>
            <Col xs={24} sm={12}>
              <Form.Input
                field={'storage_s3_secret_key'}
                label={t('Secret Key')}
                mode='password'
                placeholder='wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY'
                disabled={!isS3Enabled}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    storage_s3_secret_key: value,
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
                field={'image_timeout_seconds'}
                label={t('超时时间（秒）')}
                min={30}
                max={600}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    image_timeout_seconds: value,
                  })
                }
              />
            </Col>
            <Col xs={24} sm={8}>
              <Form.InputNumber
                field={'image_max_retry_attempts'}
                label={t('最大重试次数')}
                min={0}
                max={10}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    image_max_retry_attempts: value,
                  })
                }
              />
            </Col>
            <Col xs={24} sm={8}>
              <Form.InputNumber
                field={'image_retry_interval_seconds'}
                label={t('重试间隔（秒）')}
                min={1}
                max={60}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    image_retry_interval_seconds: value,
                  })
                }
              />
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} sm={12}>
              <Form.InputNumber
                field={'image_worker_count'}
                label={t('Worker 数量')}
                min={1}
                max={10}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    image_worker_count: value,
                  })
                }
              />
            </Col>
            <Col xs={24} sm={12}>
              <Form.InputNumber
                field={'image_stale_after_minutes'}
                label={t('僵尸任务判定时间（分钟）')}
                min={1}
                max={60}
                onChange={(value) =>
                  setInputs({
                    ...inputs,
                    image_stale_after_minutes: value,
                  })
                }
              />
            </Col>
          </Row>
        </Form.Section>

        <Form.Section text={t('限流配置')}>
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
  );
}
