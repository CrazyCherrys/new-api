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
import { useTranslation } from 'react-i18next';
import CardPro from '../../common/ui/CardPro';
import ModelMappingTable from './ModelMappingTable';
import ModelMappingActions from './ModelMappingActions';
import ModelMappingFilters from './ModelMappingFilters';
import { useModelMappingData } from '../../../hooks/model-mapping/useModelMappingData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import EditModelMappingModal from './modals/EditModelMappingModal';
import { createCardProPagination } from '../../../helpers/utils';

const ModelMappingPage = () => {
  const { t } = useTranslation();
  const mappingData = useModelMappingData();
  const isMobile = useIsMobile();

  return (
    <>
      <EditModelMappingModal
        visible={mappingData.showEdit}
        handleClose={mappingData.closeEditModal}
        editingMapping={mappingData.editingMapping}
        refresh={mappingData.refresh}
      />
      <CardPro
        type='type3'
        searchArea={<ModelMappingFilters {...mappingData} />}
        paginationArea={createCardProPagination({
          currentPage: mappingData.activePage,
          pageSize: mappingData.pageSize,
          total: mappingData.mappingCount,
          onPageChange: mappingData.handlePageChange,
          onPageSizeChange: mappingData.handlePageSizeChange,
          isMobile: isMobile,
          t: t,
        })}
        t={t}
      >
        <ModelMappingTable {...mappingData} />
      </CardPro>
    </>
  );
};

export default ModelMappingPage;
