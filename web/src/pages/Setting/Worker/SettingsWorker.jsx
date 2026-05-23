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
import {
  normalizeWorkerSettingInputs,
  WORKER_SETTING_DEFAULTS,
  getWorkerSettingEffectiveDisplayInputs,
} from './workerSettingSchema';

const MASKED_SECRET_VALUE = '****';
const LEGACY_MASKED_SECRET_VALUE = '***';
const S3_SECRET_FIELDS = new Set([
  'worker_setting.s3_access_key',
  'worker_setting.s3_secret_key',
  'worker_setting.result_s3_access_key',
  'worker_setting.result_s3_secret_key',
  'worker_setting.reference_s3_access_key',
  'worker_setting.reference_s3_secret_key',
]);

function isMaskedSecretValue(value) {
  return value === MASKED_SECRET_VALUE || value === LEGACY_MASKED_SECRET_VALUE;
}

function renderStorageConfigSection({
  t,
  storageType,
  onChange,
  inputs,
  effectiveValues,
  typeField,
  localPathField,
  endpointField,
  bucketField,
  regionField,
  accessKeyField,
  secretKeyField,
  pathPrefixField,
  urlModeField,
  publicBaseURLField,
  localPathLabel,
}) {
  return (
    <>
      <Row gutter={16}>
        <Col xs={24} sm={12} md={8} lg={8} xl={8}>
          <Form.Select
            field={typeField}
            label={t('存储类型')}
            extraText={t('选择文件存储方式')}
            onChange={onChange(typeField)}
            optionList={[
              { value: 'local', label: t('本地存储') },
              { value: 's3', label: t('S3 对象存储') },
            ]}
          />
        </Col>
      </Row>

	      {storageType === 'local' && (
	        <Row gutter={16}>
	          <Col xs={24} sm={12} md={8} lg={8} xl={8}>
	            <Form.Input
	              field={localPathField}
	              label={localPathLabel}
	              extraText={
	                !inputs[localPathField] && effectiveValues?.localPath
	                  ? t('当前生效值：{{value}}；留空使用系统临时目录', {
	                      value: effectiveValues.localPath,
	                    })
	                  : t('留空使用系统临时目录')
	              }
	              placeholder={t('例如 /var/data/worker')}
	              onChange={onChange(localPathField)}
	              showClear
	            />
          </Col>
        </Row>
      )}

      {storageType === 's3' && (
        <>
          <Row gutter={16}>
	            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
	              <Form.Input
	                field={endpointField}
	                label={t('S3 上传端点地址')}
	                extraText={t(
	                  !inputs[endpointField] && effectiveValues?.endpoint
	                    ? '服务端上传、读取和删除对象时使用的 S3 兼容端点 URL，可填写 OSS 内网 Endpoint。当前生效值：{{value}}'
	                    : '服务端上传、读取和删除对象时使用的 S3 兼容端点 URL，可填写 OSS 内网 Endpoint',
	                  { value: effectiveValues?.endpoint || '' },
	                )}
	                placeholder='https://oss-cn-hongkong-internal.aliyuncs.com'
	                onChange={onChange(endpointField)}
	                showClear
              />
            </Col>
	            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
	              <Form.Input
	                field={bucketField}
	                label={t('S3 桶名')}
	                extraText={
	                  !inputs[bucketField] && effectiveValues?.bucket
	                    ? t('存储桶名称；当前生效值：{{value}}', {
	                        value: effectiveValues.bucket,
	                      })
	                    : t('存储桶名称')
	                }
	                placeholder='my-bucket'
	                onChange={onChange(bucketField)}
	                showClear
	              />
            </Col>
	            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
	              <Form.Input
	                field={regionField}
	                label={t('S3 区域')}
	                extraText={
	                  !inputs[regionField] && effectiveValues?.region
	                    ? t('存储桶所在区域；当前生效值：{{value}}', {
	                        value: effectiveValues.region,
	                      })
	                    : t('存储桶所在区域')
	                }
	                placeholder='us-east-1'
	                onChange={onChange(regionField)}
	                showClear
	              />
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.Input
                field={accessKeyField}
                label={t('S3 Access Key')}
                extraText={t('S3 访问密钥 ID')}
                onChange={onChange(accessKeyField)}
                showClear
              />
            </Col>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.Input
                field={secretKeyField}
                label={t('S3 Secret Key')}
                extraText={t('S3 访问密钥')}
                mode={
                  isMaskedSecretValue(inputs[secretKeyField])
                    ? undefined
                    : 'password'
                }
                onChange={onChange(secretKeyField)}
                showClear
              />
            </Col>
	            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
	              <Form.Input
	                field={pathPrefixField}
	                label={t('S3 路径前缀')}
	                extraText={
	                  !inputs[pathPrefixField] && effectiveValues?.pathPrefix
	                    ? t('对象存储路径前缀，留空则存储在根目录；当前生效值：{{value}}', {
	                        value: effectiveValues.pathPrefix,
	                      })
	                    : t('对象存储路径前缀，留空则存储在根目录')
	                }
	                placeholder='worker/output'
	                onChange={onChange(pathPrefixField)}
	                showClear
	              />
            </Col>
          </Row>
          <Row gutter={16}>
	            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
	              <Form.Select
	                field={urlModeField}
	                label={t('图片访问地址模式')}
	                extraText={t(
	                  !inputs[urlModeField] && effectiveValues?.urlMode
	                    ? '控制返回给前端的图片链接是直连对象存储，还是使用 CDN 域名。当前生效值：{{value}}'
	                    : '控制返回给前端的图片链接是直连对象存储，还是使用 CDN 域名',
	                  { value: effectiveValues?.urlMode || '' },
	                )}
	                onChange={onChange(urlModeField)}
	                optionList={[
                  { value: 'direct', label: t('直连对象存储') },
                  { value: 'cdn', label: t('CDN 域名') },
                ]}
              />
            </Col>
	            <Col xs={24} sm={12} md={16} lg={16} xl={16}>
	              <Form.Input
	                field={publicBaseURLField}
	                label={t('对外访问基础地址')}
	                extraText={t(
	                  !inputs[publicBaseURLField] && effectiveValues?.publicBaseURL
	                    ? '当图片访问地址模式为 CDN 时填写 CDN 域名；留空时回退为直连对象存储地址。当前生效值：{{value}}'
	                    : '当图片访问地址模式为 CDN 时填写 CDN 域名；留空时回退为直连对象存储地址',
	                  { value: effectiveValues?.publicBaseURL || '' },
	                )}
	                placeholder='https://img.example.com'
	                onChange={onChange(publicBaseURLField)}
                showClear
              />
            </Col>
          </Row>
        </>
      )}
    </>
  );
}

export default function SettingsWorker(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState(WORKER_SETTING_DEFAULTS);
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
    const normalizedInputs = normalizeWorkerSettingInputs(props.options);
    setInputs(normalizedInputs);
    setInputsRow(normalizedInputs);
    if (refForm.current) {
      refForm.current.setValues(normalizedInputs);
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
  }, [
    inputs['worker_setting.storage_type'],
    inputs['worker_setting.result_storage_type'],
    inputs['worker_setting.reference_storage_type'],
  ]);

  const resultStorageType =
    getWorkerSettingEffectiveDisplayInputs(inputs).resultStorageType;
  const referenceStorageType =
    getWorkerSettingEffectiveDisplayInputs(inputs).referenceStorageType;
  const effectiveStorageInputs = getWorkerSettingEffectiveDisplayInputs(inputs);

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

          <Form.Section text={t('结果图存储设置')}>
            <Banner
              type='info'
              description={t(
                '控制生图结果和缩略图的存储位置，可单独选择本地磁盘或 S3 对象存储。留空的结果图字段会继续继承旧的通用存储字段。',
              )}
              style={{ marginBottom: 16 }}
            />
            {renderStorageConfigSection({
              t,
              storageType: resultStorageType,
              onChange: handleFieldChange,
              inputs,
              effectiveValues: {
                localPath: effectiveStorageInputs.resultLocalStoragePath,
                endpoint: effectiveStorageInputs.resultS3Endpoint,
                bucket: effectiveStorageInputs.resultS3Bucket,
                region: effectiveStorageInputs.resultS3Region,
                pathPrefix: effectiveStorageInputs.resultS3PathPrefix,
                urlMode: effectiveStorageInputs.resultS3URLMode,
                publicBaseURL: effectiveStorageInputs.resultS3PublicBaseURL,
              },
              typeField: 'worker_setting.result_storage_type',
              localPathField: 'worker_setting.result_local_storage_path',
              endpointField: 'worker_setting.result_s3_endpoint',
              bucketField: 'worker_setting.result_s3_bucket',
              regionField: 'worker_setting.result_s3_region',
              accessKeyField: 'worker_setting.result_s3_access_key',
              secretKeyField: 'worker_setting.result_s3_secret_key',
              pathPrefixField: 'worker_setting.result_s3_path_prefix',
              urlModeField: 'worker_setting.result_s3_url_mode',
              publicBaseURLField: 'worker_setting.result_s3_public_base_url',
              localPathLabel: t('结果图本地存储路径'),
            })}
          </Form.Section>

          <Form.Section text={t('参考图存储设置')}>
            <Banner
              type='info'
              description={t(
                '控制参考图和遮罩图的存储位置，可与结果图存储策略分开配置。留空的参考图字段会继承结果图存储配置，而不是单独保存一份相同值。',
              )}
              style={{ marginBottom: 16 }}
            />
            {renderStorageConfigSection({
              t,
              storageType: referenceStorageType,
              onChange: handleFieldChange,
              inputs,
              effectiveValues: {
                localPath: effectiveStorageInputs.referenceLocalStoragePath,
                endpoint: effectiveStorageInputs.referenceS3Endpoint,
                bucket: effectiveStorageInputs.referenceS3Bucket,
                region: effectiveStorageInputs.referenceS3Region,
                pathPrefix: effectiveStorageInputs.referenceS3PathPrefix,
                urlMode: effectiveStorageInputs.referenceS3URLMode,
                publicBaseURL: effectiveStorageInputs.referenceS3PublicBaseURL,
              },
              typeField: 'worker_setting.reference_storage_type',
              localPathField: 'worker_setting.reference_local_storage_path',
              endpointField: 'worker_setting.reference_s3_endpoint',
              bucketField: 'worker_setting.reference_s3_bucket',
              regionField: 'worker_setting.reference_s3_region',
              accessKeyField: 'worker_setting.reference_s3_access_key',
              secretKeyField: 'worker_setting.reference_s3_secret_key',
              pathPrefixField: 'worker_setting.reference_s3_path_prefix',
              urlModeField: 'worker_setting.reference_s3_url_mode',
              publicBaseURLField: 'worker_setting.reference_s3_public_base_url',
              localPathLabel: t('参考图本地存储路径'),
            })}
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'worker_setting.reference_auto_cleanup_enabled'}
                  label={t('启用参考图自动清理')}
                  checkedText={t('开')}
                  uncheckedText={t('关')}
                  onChange={handleFieldChange(
                    'worker_setting.reference_auto_cleanup_enabled',
                  )}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'worker_setting.reference_retention_days'}
                  label={t('参考图保留天数')}
                  extraText={t('仅清理无引用的旧参考图资产')}
                  min={1}
                  max={365}
                  onChange={handleFieldChange(
                    'worker_setting.reference_retention_days',
                  )}
                />
              </Col>
            </Row>
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
                    '当前仅在 /canvas 实时推送失败时，作为后备轮询间隔使用',
                  )}
                  min={1}
                  max={60}
                  onChange={handleFieldChange(
                    'worker_setting.polling_interval',
                  )}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'worker_setting.inspiration_page_cache_ttl'}
                  label={t('灵感后续页缓存（秒）')}
                  extraText={t('控制 /inspiration 非首页游标页缓存时长，0 表示关闭缓存')}
                  min={0}
                  max={86400}
                  onChange={handleFieldChange(
                    'worker_setting.inspiration_page_cache_ttl',
                  )}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'worker_setting.inspiration_first_page_cache_ttl'}
                  label={t('灵感首页缓存（秒）')}
                  extraText={t('控制 /inspiration 首页缓存时长，0 表示关闭缓存')}
                  min={0}
                  max={86400}
                  onChange={handleFieldChange(
                    'worker_setting.inspiration_first_page_cache_ttl',
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
