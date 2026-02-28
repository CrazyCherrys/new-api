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
import { Tabs, TabPane } from '@douyinfe/semi-ui';
import { IconImage, IconHistogram, IconGallery } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import GenerationView from './views/GenerationView';

const ImagePage = () => {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState('generation');

  return (
    <div className="w-full h-full mt-[60px] px-4 py-4">
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        type="line"
        size="large"
      >
        <TabPane
          tab={
            <span className="flex items-center gap-2">
              <IconImage />
              {t('图片生成')}
            </span>
          }
          itemKey="generation"
        >
          <div className="mt-4">
            <GenerationView />
          </div>
        </TabPane>

        <TabPane
          tab={
            <span className="flex items-center gap-2">
              <IconHistogram />
              {t('生成历史')}
            </span>
          }
          itemKey="history"
        >
          <div className="mt-4 flex items-center justify-center h-64">
            <div className="text-center text-gray-400">
              {t('生成历史功能即将推出')}
            </div>
          </div>
        </TabPane>

        <TabPane
          tab={
            <span className="flex items-center gap-2">
              <IconGallery />
              {t('图片库')}
            </span>
          }
          itemKey="gallery"
        >
          <div className="mt-4 flex items-center justify-center h-64">
            <div className="text-center text-gray-400">
              {t('图片库功能即将推出')}
            </div>
          </div>
        </TabPane>
      </Tabs>
    </div>
  );
};

export default ImagePage;
