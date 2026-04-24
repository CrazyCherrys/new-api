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

import { useState, useEffect, useCallback } from 'react';
import { API, showError, showSuccess } from '../../helpers';

export const useModelMappingData = () => {
  const [mappings, setMappings] = useState([]);
  const [loading, setLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [mappingCount, setMappingCount] = useState(0);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searchModelType, setSearchModelType] = useState(0);
  const [showEdit, setShowEdit] = useState(false);
  const [editingMapping, setEditingMapping] = useState(null);

  const loadMappings = useCallback(
    async (startIdx) => {
      setLoading(true);
      try {
        let url = '';
        if (searchKeyword || searchModelType > 0) {
          url = `/api/model-mapping/search?keyword=${searchKeyword}&model_type=${searchModelType}`;
        } else {
          url = '/api/model-mapping/';
        }
        url += `&p=${startIdx}&page_size=${pageSize}`;

        const res = await API.get(url);
        const { success, message, data } = res.data;
        if (success) {
          setMappings(data.items || []);
          setMappingCount(data.total || 0);
        } else {
          showError(message);
        }
      } catch (error) {
        showError(error.message);
      } finally {
        setLoading(false);
      }
    },
    [searchKeyword, searchModelType, pageSize]
  );

  useEffect(() => {
    loadMappings(0);
  }, [loadMappings]);

  const refresh = useCallback(() => {
    loadMappings((activePage - 1) * pageSize);
  }, [activePage, pageSize, loadMappings]);

  const handlePageChange = (page) => {
    setActivePage(page);
    loadMappings((page - 1) * pageSize);
  };

  const handlePageSizeChange = (size) => {
    setPageSize(size);
    setActivePage(1);
    loadMappings(0);
  };

  const manageMapping = async (id, action, value) => {
    try {
      const res = await API.post('/api/model-mapping/manage', {
        id,
        action,
        value,
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess('操作成功');
        await refresh();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
  };

  const deleteMapping = async (id) => {
    try {
      const res = await API.delete(`/api/model-mapping/${id}`);
      const { success, message } = res.data;
      if (success) {
        showSuccess('删除成功');
        await refresh();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
  };

  const openEditModal = (mapping = null) => {
    setEditingMapping(mapping);
    setShowEdit(true);
  };

  const closeEditModal = () => {
    setShowEdit(false);
    setEditingMapping(null);
  };

  const handleSearch = () => {
    setActivePage(1);
    loadMappings(0);
  };

  return {
    mappings,
    loading,
    activePage,
    pageSize,
    mappingCount,
    searchKeyword,
    searchModelType,
    showEdit,
    editingMapping,
    setSearchKeyword,
    setSearchModelType,
    handlePageChange,
    handlePageSizeChange,
    refresh,
    manageMapping,
    deleteMapping,
    openEditModal,
    closeEditModal,
    handleSearch,
  };
};
