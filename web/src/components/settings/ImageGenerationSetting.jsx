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

import React from 'react';
import { Card, Spin } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { showError } from '../../helpers';
import { getImageConfig } from '../../helpers/imageApi';
import SettingsImageGenerationBase from '../../pages/Setting/ImageGeneration/SettingsImageGenerationBase';
import SettingsImageGenerationModelManagement from '../../pages/Setting/ImageGeneration/SettingsImageGenerationModelManagement';
import {
  defaultImageGenerationInputs,
  transformToFrontend,
} from '../../pages/Setting/ImageGeneration/shared';

const ImageGenerationSetting = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = React.useState(false);
  const [inputs, setInputs] = React.useState(defaultImageGenerationInputs);

  const onRefresh = React.useCallback(async () => {
    setLoading(true);
    try {
      const res = await getImageConfig();
      if (res.data?.success && res.data?.data) {
        setInputs(transformToFrontend(res.data.data));
      }
    } catch (error) {
      showError(t('加载配置失败'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    onRefresh();
  }, [onRefresh]);

  return (
    <Spin spinning={loading} size='large'>
      <Card style={{ marginTop: '10px' }}>
        <SettingsImageGenerationBase options={inputs} refresh={onRefresh} />
      </Card>
      <Card style={{ marginTop: '10px' }}>
        <SettingsImageGenerationModelManagement
          options={inputs}
          refresh={onRefresh}
        />
      </Card>
    </Spin>
  );
};

export default ImageGenerationSetting;
