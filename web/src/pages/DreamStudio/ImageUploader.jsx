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
import { Upload, Typography, Toast } from '@douyinfe/semi-ui';
import { IconUpload } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const ImageUploader = ({ onImageChange, disabled = false }) => {
  const { t } = useTranslation();
  const [fileList, setFileList] = useState([]);

  const handleBeforeUpload = ({ file }) => {
    const isImage = file.type.startsWith('image/');
    const isValidType = ['image/jpeg', 'image/png', 'image/webp'].includes(file.type);
    const isLt5M = file.size / 1024 / 1024 < 5;

    if (!isImage || !isValidType) {
      Toast.error(t('只支持 JPG、PNG、WebP 格式的图片'));
      return false;
    }

    if (!isLt5M) {
      Toast.error(t('图片大小不能超过 5MB'));
      return false;
    }

    const reader = new FileReader();
    reader.onload = (e) => {
      const base64 = e.target.result;
      onImageChange(base64);
    };
    reader.readAsDataURL(file);

    return false;
  };

  const handleRemove = () => {
    setFileList([]);
    onImageChange(null);
  };

  const handleChange = ({ fileList: newFileList }) => {
    setFileList(newFileList);
  };

  return (
    <div className={disabled ? 'opacity-50 pointer-events-none' : ''}>
      <Typography.Text strong className='text-sm mb-2 block'>
        {t('上传图片')}
      </Typography.Text>
      <Upload
        action=''
        fileList={fileList}
        beforeUpload={handleBeforeUpload}
        onRemove={handleRemove}
        onChange={handleChange}
        accept='image/jpeg,image/png,image/webp'
        limit={1}
        draggable
        dragMainText={t('点击上传或拖拽图片到此处')}
        dragSubText={t('支持 JPG、PNG、WebP 格式，大小不超过 5MB')}
        disabled={disabled}
        listType='picture'
        className='w-full'
      >
        <div className='flex flex-col items-center justify-center py-8 border-2 border-dashed border-gray-300 rounded-lg hover:border-blue-400 transition-colors cursor-pointer'>
          <IconUpload size='extra-large' className='text-gray-400 mb-2' />
          <Typography.Text className='text-sm text-gray-600'>
            {t('点击上传或拖拽图片到此处')}
          </Typography.Text>
          <Typography.Text className='text-xs text-gray-400 mt-1'>
            {t('支持 JPG、PNG、WebP 格式，大小不超过 5MB')}
          </Typography.Text>
        </div>
      </Upload>
    </div>
  );
};

export default ImageUploader;
