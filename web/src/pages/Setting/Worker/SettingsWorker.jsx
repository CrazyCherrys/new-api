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
import { Banner, Button, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

const MASKED_SECRET_VALUE = '****';
const LEGACY_MASKED_SECRET_VALUE = '***';
const S3_SECRET_FIELDS = new Set([
  'worker_setting.s3_access_key',
  'worker_setting.s3_secret_key',
]);

function isMaskedSecretValue(value) {
  return value === MASKED_SECRET_VALUE || value === LEGACY_MASKED_SECRET_VALUE;
}

export default function SettingsWorker(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    'worker_setting.max_workers': 4,
    'worker_setting.user_custom_key_enabled': false,
    'worker_setting.user_custom_base_url_allowed': false,
    'worker_setting.storage_type': 'local',
    'worker_setting.local_storage_path': '',
    'worker_setting.s3_endpoint': '',
    'worker_setting.s3_bucket': '',
    'worker_setting.s3_region': '',
    'worker_setting.s3_access_key': '',
    'worker_setting.s3_secret_key': '',
    'worker_setting.s3_path_prefix': '',
    'worker_setting.s3_url_mode': 'direct',
    'worker_setting.s3_public_base_url': '',
    'worker_setting.image_timeout': 120,
    'worker_setting.video_timeout': 600,
    'worker_setting.retry_delay': 5,
    'worker_setting.max_retries': 3,
    'worker_setting.polling_interval': 5,
    'worker_setting.auto_cleanup_enabled': false,
    'worker_setting.retention_days': 30,
    'worker_setting.max_image_size': 10,
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);

  function handleFieldChange(fieldName) {
    return (value) => {
      setInputs((prev) => ({ ...prev, [fieldName]: value }));
    };
  }

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow).filter(
      (item) =>
        !(
          S3_SECRET_FIELDS.has(item.key) &&
          isMaskedSecretValue(inputs[item.key])
        ),
    );
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    const requestQueue = updateArray.map((item) => {
      let value = '';
      if (typeof inputs[item.key] === 'boolean') {
        value = String(inputs[item.key]);
      } else {
        value = String(inputs[item.key]);
      }
      return API.put('/api/option/', {
        key: item.key,
        value,
      });
    });
    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        const failedResponses = res.filter((item) => !item?.data?.success);
        if (failedResponses.length > 0) {
          failedResponses.forEach((item) => {
            if (item?.data?.message) {
              showError(item.data.message);
            }
          });
          if (!failedResponses.some((item) => item?.data?.message)) {
            showError(t('部分保存失败，请重试'));
          }
          return;
        }
        showSuccess(t('保存成功'));
        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  useEffect(() => {
    const nextInputs = { ...inputs };
    const currentInputs = {};
    for (let key in props.options) {
      if (Object.keys(nextInputs).includes(key)) {
        if (typeof nextInputs[key] === 'boolean') {
          currentInputs[key] =
            props.options[key] === 'true' || props.options[key] === true;
        } else if (typeof nextInputs[key] === 'number') {
          const parsedValue = parseInt(props.options[key], 10);
          currentInputs[key] = Number.isNaN(parsedValue)
            ? nextInputs[key]
            : parsedValue;
        } else {
          currentInputs[key] = props.options[key];
        }
      }
    }
    setInputs({ ...nextInputs, ...currentInputs });
    setInputsRow({ ...nextInputs, ...currentInputs });
    if (refForm.current) {
      refForm.current.setValues({ ...nextInputs, ...currentInputs });
    }
  }, [props.options]);

  useEffect(() => {
    if (!refForm.current) {
      return;
    }

    // Storage inputs are mounted conditionally. Re-apply the full form state
    // after the section is rendered so switching storage type does not blank
    // non-storage fields.
    refForm.current.setValues(inputs);
  }, [inputs['worker_setting.storage_type']]);

  const storageType = inputs['worker_setting.storage_type'];

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          {/* Worker 并发设置 */}
          <Form.Section text={t('Worker 并发设置')}>
            <Banner
              type='info'
              description={t(
                'Worker 数量即同时运行的最大任务数，类似于 CPU 线程数。根据服务器性能合理设置，避免过高导致资源耗尽。',
              )}
              style={{ marginBottom: 16 }}
            />
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'worker_setting.max_workers'}
                  label={t('最大 Worker 数量')}
                  extraText={t('保存后立即影响新创建的图片生成任务')}
                  min={1}
                  max={64}
                  onChange={handleFieldChange('worker_setting.max_workers')}
                />
              </Col>
            </Row>
          </Form.Section>

          <Form.Section text={t('用户自定义 Worker 设置')}>
            <Banner
              type='info'
              description={t(
                '开启后，用户可以在个人设置的 Worker设置 中填写自己的 API 密钥和 API 地址。',
              )}
              style={{ marginBottom: 16 }}
            />
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'worker_setting.user_custom_key_enabled'}
                  label={t('允许用户自定义 API 密钥')}
                  checkedText={t('开')}
                  uncheckedText={t('关')}
                  onChange={handleFieldChange(
                    'worker_setting.user_custom_key_enabled',
                  )}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'worker_setting.user_custom_base_url_allowed'}
                  label={t('允许用户自定义 API 地址')}
                  checkedText={t('开')}
                  uncheckedText={t('关')}
                  onChange={handleFieldChange(
                    'worker_setting.user_custom_base_url_allowed',
                  )}
                />
              </Col>
            </Row>
          </Form.Section>

          {/* 存储设置 */}
          <Form.Section text={t('存储设置')}>
            <Banner
              type='info'
              description={t(
                '选择生成结果的存储方式。本地存储将文件保存在服务器磁盘上，S3 对象存储支持兼容 S3 协议的存储服务。',
              )}
              style={{ marginBottom: 16 }}
            />
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Select
                  field={'worker_setting.storage_type'}
                  label={t('存储类型')}
                  extraText={t('选择文件存储方式')}
                  onChange={handleFieldChange('worker_setting.storage_type')}
                  optionList={[
                    { value: 'local', label: t('本地存储') },
                    { value: 's3', label: t('S3 对象存储') },
                  ]}
                />
              </Col>
            </Row>

            {/* 本地存储配置 */}
            {storageType === 'local' && (
              <Row gutter={16}>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.Input
                    field={'worker_setting.local_storage_path'}
                    label={t('本地存储路径')}
                    extraText={t('留空使用系统临时目录')}
                    placeholder={t('例如 /var/data/worker')}
                    onChange={handleFieldChange(
                      'worker_setting.local_storage_path',
                    )}
                    showClear
                  />
                </Col>
              </Row>
            )}

            {/* S3 对象存储配置 */}
            {storageType === 's3' && (
              <>
                <Row gutter={16}>
                  <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                    <Form.Input
                      field={'worker_setting.s3_endpoint'}
                      label={t('S3 上传端点地址')}
                      extraText={t(
                        '服务端上传、读取和删除对象时使用的 S3 兼容端点 URL，可填写 OSS 内网 Endpoint',
                      )}
                      placeholder='https://oss-cn-hongkong-internal.aliyuncs.com'
                      onChange={handleFieldChange('worker_setting.s3_endpoint')}
                      showClear
                    />
                  </Col>
                  <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                    <Form.Input
                      field={'worker_setting.s3_bucket'}
                      label={t('S3 桶名')}
                      extraText={t('存储桶名称')}
                      placeholder='my-bucket'
                      onChange={handleFieldChange('worker_setting.s3_bucket')}
                      showClear
                    />
                  </Col>
                  <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                    <Form.Input
                      field={'worker_setting.s3_region'}
                      label={t('S3 区域')}
                      extraText={t('存储桶所在区域')}
                      placeholder='us-east-1'
                      onChange={handleFieldChange('worker_setting.s3_region')}
                      showClear
                    />
                  </Col>
                </Row>
                <Row gutter={16}>
                  <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                    <Form.Input
                      field={'worker_setting.s3_access_key'}
                      label={t('S3 Access Key')}
                      extraText={t('S3 访问密钥 ID')}
                      onChange={handleFieldChange(
                        'worker_setting.s3_access_key',
                      )}
                      showClear
                    />
                  </Col>
                  <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                    <Form.Input
                      field={'worker_setting.s3_secret_key'}
                      label={t('S3 Secret Key')}
                      extraText={t('S3 访问密钥')}
                      mode={
                        isMaskedSecretValue(
                          inputs['worker_setting.s3_secret_key'],
                        )
                          ? undefined
                          : 'password'
                      }
                      onChange={handleFieldChange(
                        'worker_setting.s3_secret_key',
                      )}
                      showClear
                    />
                  </Col>
                  <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                    <Form.Input
                      field={'worker_setting.s3_path_prefix'}
                      label={t('S3 路径前缀')}
                      extraText={t('对象存储路径前缀，留空则存储在根目录')}
                      placeholder='worker/output'
                      onChange={handleFieldChange(
                        'worker_setting.s3_path_prefix',
                      )}
                      showClear
                    />
                  </Col>
                </Row>
                <Row gutter={16}>
                  <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                    <Form.Select
                      field={'worker_setting.s3_url_mode'}
                      label={t('图片访问地址模式')}
                      extraText={t(
                        '控制返回给前端的图片链接是直连对象存储，还是使用 CDN 域名',
                      )}
                      onChange={handleFieldChange('worker_setting.s3_url_mode')}
                      optionList={[
                        { value: 'direct', label: t('直连对象存储') },
                        { value: 'cdn', label: t('CDN 域名') },
                      ]}
                    />
                  </Col>
                  <Col xs={24} sm={12} md={16} lg={16} xl={16}>
                    <Form.Input
                      field={'worker_setting.s3_public_base_url'}
                      label={t('对外访问基础地址')}
                      extraText={t(
                        '当图片访问地址模式为 CDN 时填写 CDN 域名；留空时回退为直连对象存储地址',
                      )}
                      placeholder='https://img.example.com'
                      onChange={handleFieldChange(
                        'worker_setting.s3_public_base_url',
                      )}
                      showClear
                    />
                  </Col>
                </Row>
              </>
            )}
          </Form.Section>

          {/* 超时设置 */}
          <Form.Section text={t('超时设置')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'worker_setting.image_timeout'}
                  label={t('图片任务超时（秒）')}
                  extraText={t('图片生成任务的最大等待时间')}
                  min={10}
                  max={3600}
                  onChange={handleFieldChange('worker_setting.image_timeout')}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'worker_setting.video_timeout'}
                  label={t('视频任务超时（秒）')}
                  extraText={t('当前不影响 /canvas 图片生成请求')}
                  min={10}
                  max={7200}
                  onChange={handleFieldChange('worker_setting.video_timeout')}
                />
              </Col>
            </Row>
          </Form.Section>

          {/* 重试设置 */}
          <Form.Section text={t('重试设置')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'worker_setting.retry_delay'}
                  label={t('重试间隔（秒）')}
                  extraText={t('图片生成失败后等待多少秒再重试')}
                  min={1}
                  max={300}
                  onChange={handleFieldChange('worker_setting.retry_delay')}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'worker_setting.max_retries'}
                  label={t('最大重试次数')}
                  extraText={t('图片生成失败后的最大重试次数，0 表示不重试')}
                  min={0}
                  max={10}
                  onChange={handleFieldChange('worker_setting.max_retries')}
                />
              </Col>
            </Row>
          </Form.Section>

          {/* 任务管理设置 */}
          <Form.Section text={t('任务管理设置')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'worker_setting.polling_interval'}
                  label={t('轮询间隔（秒）')}
                  extraText={t(
                    '当前 /canvas 使用实时任务推送，不读取此项',
                  )}
                  min={1}
                  max={60}
                  onChange={handleFieldChange(
                    'worker_setting.polling_interval',
                  )}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'worker_setting.auto_cleanup_enabled'}
                  label={t('自动清理开关')}
                  extraText={t('是否自动清理过期的任务和文件')}
                  onChange={handleFieldChange(
                    'worker_setting.auto_cleanup_enabled',
                  )}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'worker_setting.retention_days'}
                  label={t('保留天数')}
                  extraText={t('任务和文件的保留天数，超过后自动清理')}
                  min={1}
                  max={365}
                  onChange={handleFieldChange('worker_setting.retention_days')}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'worker_setting.max_image_size'}
                  label={t('参考图片大小限制（MB）')}
                  extraText={t('单张参考图片的最大文件大小')}
                  min={1}
                  max={100}
                  onChange={handleFieldChange('worker_setting.max_image_size')}
                />
              </Col>
            </Row>
          </Form.Section>

          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存 Worker 设置')}
            </Button>
          </Row>
        </Form>
      </Spin>
    </>
  );
}
