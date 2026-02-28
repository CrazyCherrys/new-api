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

import React, { useState } from 'react';
import { Card, Button, Space, Toast } from '@douyinfe/semi-ui';
import { IconSetting } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import ModelSelector from '../components/ModelSelector';
import PromptArea from '../components/PromptArea';
import ParameterSidebar from '../components/ParameterSidebar';

const GenerationView = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();

  const [formState, setFormState] = useState({
    model: 'dall-e-3',
    prompt: '',
    resolution: '1024',
    aspectRatio: '1:1',
    referenceImage: null,
    count: 1,
  });

  const [loading, setLoading] = useState(false);
  const [sidebarVisible, setSidebarVisible] = useState(false);

  const handleFieldChange = (field, value) => {
    setFormState((prev) => ({
      ...prev,
      [field]: value,
    }));
  };

  const handleSubmit = async () => {
    if (!formState.prompt.trim()) {
      showError(t('请输入提示词'));
      return;
    }

    if (!formState.model) {
      showError(t('请选择模型'));
      return;
    }

    setLoading(true);

    try {
      const requestData = {
        model: formState.model,
        prompt: formState.prompt,
        n: formState.count,
        size: `${formState.resolution}x${formState.resolution}`,
        response_format: 'url',
      };

      if (formState.referenceImage) {
        requestData.image = formState.referenceImage;
      }

      const res = await API.post('/api/image/generations', requestData);
      const { success, message, data } = res.data;

      if (success) {
        showSuccess(t('图片生成成功'));
        // TODO: 处理生成的图片数据
        console.log('Generated images:', data);
      } else {
        showError(message || t('图片生成失败'));
      }
    } catch (error) {
      console.error('Image generation error:', error);
      showError(error.message || t('图片生成失败，请重试'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="w-full h-full">
      <div className="flex flex-col lg:flex-row gap-4 h-full">
        {/* 左侧主内容区 */}
        <div className="flex-1 flex flex-col gap-4">
          <Card className="w-full">
            <Space vertical spacing="large" className="w-full">
              <ModelSelector
                value={formState.model}
                onChange={(value) => handleFieldChange('model', value)}
                disabled={loading}
              />

              <PromptArea
                value={formState.prompt}
                onChange={(value) => handleFieldChange('prompt', value)}
                disabled={loading}
              />
            </Space>
          </Card>

          {/* 移动端参数按钮 */}
          {isMobile && (
            <Button
              theme="solid"
              type="primary"
              size="large"
              icon={<IconSetting />}
              onClick={() => setSidebarVisible(true)}
              block
            >
              {t('设置参数')}
            </Button>
          )}

          {/* 生成结果展示区 */}
          <Card className="flex-1">
            <div className="flex items-center justify-center h-full min-h-[300px]">
              <div className="text-center text-gray-400">
                {loading ? t('生成中...') : t('生成的图片将显示在这里')}
              </div>
            </div>
          </Card>
        </div>

        {/* 右侧参数栏 */}
        {!isMobile && (
          <div className="w-full lg:w-80 xl:w-96">
            <ParameterSidebar
              resolution={formState.resolution}
              aspectRatio={formState.aspectRatio}
              referenceImage={formState.referenceImage}
              count={formState.count}
              onResolutionChange={(value) => handleFieldChange('resolution', value)}
              onAspectRatioChange={(value) => handleFieldChange('aspectRatio', value)}
              onReferenceImageChange={(value) => handleFieldChange('referenceImage', value)}
              onCountChange={(value) => handleFieldChange('count', value)}
              onSubmit={handleSubmit}
              loading={loading}
              disabled={!formState.prompt.trim() || !formState.model}
              isMobile={false}
            />
          </div>
        )}

        {/* 移动端侧边栏 */}
        {isMobile && (
          <ParameterSidebar
            resolution={formState.resolution}
            aspectRatio={formState.aspectRatio}
            referenceImage={formState.referenceImage}
            count={formState.count}
            onResolutionChange={(value) => handleFieldChange('resolution', value)}
            onAspectRatioChange={(value) => handleFieldChange('aspectRatio', value)}
            onReferenceImageChange={(value) => handleFieldChange('referenceImage', value)}
            onCountChange={(value) => handleFieldChange('count', value)}
            onSubmit={handleSubmit}
            loading={loading}
            disabled={!formState.prompt.trim() || !formState.model}
            isMobile={true}
            visible={sidebarVisible}
            onClose={() => setSidebarVisible(false)}
          />
        )}
      </div>
    </div>
  );
};

export default GenerationView;
