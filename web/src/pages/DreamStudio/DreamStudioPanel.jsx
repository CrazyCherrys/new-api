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
import { Layout, Button } from '@douyinfe/semi-ui';
import { IconChevronLeft, IconChevronRight } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const { Sider, Content } = Layout;

const DreamStudioPanel = ({ state, dispatch }) => {
  const { t } = useTranslation();
  const [collapsed, setCollapsed] = useState(false);

  return (
    <Layout className='min-h-[calc(100vh-60px)]'>
      <Sider
        collapsed={collapsed}
        collapsible
        trigger={null}
        className='bg-white border-r border-gray-200'
        style={{
          width: collapsed ? 0 : 320,
          minWidth: collapsed ? 0 : 320,
          maxWidth: collapsed ? 0 : 320,
          transition: 'all 0.2s',
          overflow: 'hidden',
        }}
      >
        <div className='p-4 h-full overflow-y-auto'>
          <h3 className='text-lg font-semibold mb-4'>{t('生成参数')}</h3>
          <div className='text-sm text-gray-500'>
            {t('参数控件将在下一步实现')}
          </div>
        </div>
      </Sider>

      <Layout>
        <Content className='bg-gray-50'>
          <div className='relative h-full'>
            <Button
              icon={collapsed ? <IconChevronRight /> : <IconChevronLeft />}
              onClick={() => setCollapsed(!collapsed)}
              className='absolute left-2 top-2 z-10'
              size='small'
            />

            <div className='p-4 pt-12 max-w-6xl mx-auto'>
              <div className='bg-white rounded-lg shadow-sm p-6 mb-4'>
                <h3 className='text-lg font-semibold mb-4'>{t('提示词编辑器')}</h3>
                <div className='text-sm text-gray-500'>
                  {t('Prompt 编辑器将在后续步骤实现')}
                </div>
              </div>

              <div className='bg-white rounded-lg shadow-sm p-6 mb-4'>
                <h3 className='text-lg font-semibold mb-4'>{t('图片上传')}</h3>
                <div className='text-sm text-gray-500'>
                  {t('图片上传区域将在后续步骤实现')}
                </div>
              </div>

              <div className='bg-white rounded-lg shadow-sm p-6'>
                <h3 className='text-lg font-semibold mb-4'>{t('历史任务')}</h3>
                <div className='text-sm text-gray-500'>
                  {t('历史任务画廊将在后续步骤实现')}
                </div>
              </div>
            </div>
          </div>
        </Content>
      </Layout>

      <style jsx>{`
        @media (max-width: 768px) {
          .max-w-6xl {
            max-width: 100%;
          }
        }
      `}</style>
    </Layout>
  );
};

export default DreamStudioPanel;
