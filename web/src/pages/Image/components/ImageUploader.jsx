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
import { Upload, Typography } from '@douyinfe/semi-ui';
import { IconUpload } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { showError } from '../../../helpers';

const { Text } = Typography;

const ImageUploader = ({ value, onChange, disabled }) => {
  const { t } = useTranslation();
  const [fileList, setFileList] = useState([]);

  const handleChange = ({ fileList: newFileList }) => {
    setFileList(newFileList);

    if (newFileList.length > 0) {
      const file = newFileList[0].fileInstance;
      const reader = new FileReader();

      reader.onload = (e) => {
        onChange(e.target.result);
      };

      reader.onerror = () => {
        showError(t('图片读取失败'));
      };

      reader.readAsDataURL(file);
    } else {
      onChange(null);
    }
  };

  const handleRemove = () => {
    setFileList([]);
    onChange(null);
  };

  const beforeUpload = ({ file }) => {
    const isImage = file.type.startsWith('image/');
    if (!isImage) {
      showError(t('只能上传图片文件'));
      return false;
    }

    const isLt10M = file.size / 1024 / 1024 < 10;
    if (!isLt10M) {
      showError(t('图片大小不能超过 10MB'));
      return false;
    }

    return true;
  };

  return (
    <div className="w-full">
      <Text strong className="block mb-2">
        {t('参考图片')} <Text type="tertiary">({t('可选')})</Text>
      </Text>
      <Upload
        action=""
        fileList={fileList}
        onChange={handleChange}
        onRemove={handleRemove}
        beforeUpload={beforeUpload}
        disabled={disabled}
        accept="image/*"
        limit={1}
        draggable
        dragMainText={
          <div className="flex flex-col items-center">
            <IconUpload size="extra-large" />
            <Text className="mt-2">{t('点击上传或拖拽图片到此处')}</Text>
          </div>
        }
        dragSubText={t('支持 JPG、PNG、WebP 格式，最大 10MB')}
      />
    </div>
  );
};

export default ImageUploader;
