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
import {
  Form,
  Button,
  Spin,
  Banner,
  Toast,
  Switch,
  InputNumber,
  Input,
  Select,
  TagInput,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { getImageConfig, updateImageConfig } from '../../helpers/imageApi';
import { API } from '../../helpers/api';

const AdminSettings = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [testingS3, setTestingS3] = useState(false);
  const [form] = Form.useForm();

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    setLoading(true);
    try {
      const res = await getImageConfig();
      if (res.data?.success && res.data?.data) {
        form.setValues(res.data.data);
      }
    } catch (error) {
      Toast.error(t('加载配置失败'));
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (values) => {
    setLoading(true);
    try {
      const res = await updateImageConfig(values);
      if (res.data?.success) {
        Toast.success(t('保存成功'));
      } else {
        Toast.error(res.data?.message || t('保存失败'));
      }
    } catch (error) {
      Toast.error(t('保存失败'));
    } finally {
      setLoading(false);
    }
  };

  const testS3Connection = async () => {
    const values = form.getValues();
    if (!values.s3_enabled) {
      Toast.warning(t('请先启用 S3 存储'));
      return;
    }
    if (!values.s3_endpoint || !values.s3_bucket) {
      Toast.warning(t('请填写 S3 配置信息'));
      return;
    }

    setTestingS3(true);
    try {
      const res = await API.post('/api/v1/image-tasks/config/test-s3', {
        s3_endpoint: values.s3_endpoint,
        s3_bucket: values.s3_bucket,
        s3_access_key: values.s3_access_key,
        s3_secret_key: values.s3_secret_key,
      });
      if (res.data?.success) {
        Toast.success(t('S3 连接测试成功'));
      } else {
        Toast.error(res.data?.message || t('S3 连接测试失败'));
      }
    } catch (error) {
      Toast.error(t('S3 连接测试失败'));
    } finally {
      setTestingS3(false);
    }
  };

  if (loading && !form.getValues()) {
    return (
      <div className="flex justify-center items-center h-64">
        <Spin size="large" />
      </div>
    );
  }

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <h2 className="text-2xl font-semibold mb-6">{t('图像生成管理配置')}</h2>

      <Banner
        type="info"
        description={t('配置图像生成服务的存储、参数和模型设置')}
        className="mb-6"
      />

      <Form
        form={form}
        onSubmit={handleSubmit}
        labelPosition="left"
        labelAlign="right"
        labelWidth={150}
      >
        <h3 className="text-lg font-semibold mb-4">{t('存储配置')}</h3>

        <Form.Switch
          field="s3_enabled"
          label={t('启用 S3 存储')}
          initValue={false}
        />

        <Form.Input
          field="s3_endpoint"
          label={t('S3 Endpoint')}
          placeholder="https://s3.amazonaws.com"
        />

        <Form.Input
          field="s3_bucket"
          label={t('S3 Bucket')}
          placeholder="my-bucket"
        />

        <Form.Input
          field="s3_access_key"
          label={t('Access Key')}
          placeholder="AKIAIOSFODNN7EXAMPLE"
        />

        <Form.Input
          field="s3_secret_key"
          label={t('Secret Key')}
          mode="password"
          placeholder="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
        />

        <div className="mb-6">
          <Button
            onClick={testS3Connection}
            loading={testingS3}
            disabled={loading}
          >
            {t('测试 S3 连接')}
          </Button>
        </div>

        <h3 className="text-lg font-semibold mb-4 mt-8">{t('生成参数')}</h3>

        <Form.InputNumber
          field="timeout_seconds"
          label={t('超时时间（秒）')}
          initValue={300}
          min={30}
          max={600}
        />

        <Form.InputNumber
          field="max_retries"
          label={t('最大重试次数')}
          initValue={3}
          min={0}
          max={10}
        />

        <Form.InputNumber
          field="retry_interval_seconds"
          label={t('重试间隔（秒）')}
          initValue={5}
          min={1}
          max={60}
        />

        <h3 className="text-lg font-semibold mb-4 mt-8">{t('模型管理')}</h3>

        <Form.TagInput
          field="visible_models"
          label={t('可见模型列表')}
          placeholder={t('输入模型名称后按回车')}
          initValue={['dall-e-3', 'dall-e-2', 'stable-diffusion-xl-1024-v1-0']}
        />

        <Form.Input
          field="default_model"
          label={t('默认模型')}
          initValue="dall-e-3"
          placeholder="dall-e-3"
        />

        <h3 className="text-lg font-semibold mb-4 mt-8">{t('RPM 限制')}</h3>

        <Form.InputNumber
          field="rpm_limit"
          label={t('单模型请求频率限制')}
          initValue={10}
          min={1}
          max={1000}
          suffix={t('请求/分钟')}
        />

        <div className="mt-6 flex gap-4">
          <Button
            type="primary"
            htmlType="submit"
            loading={loading}
            size="large"
          >
            {t('保存配置')}
          </Button>
          <Button
            onClick={loadConfig}
            disabled={loading}
            size="large"
          >
            {t('重置')}
          </Button>
        </div>
      </Form>
    </div>
  );
};

export default AdminSettings;
