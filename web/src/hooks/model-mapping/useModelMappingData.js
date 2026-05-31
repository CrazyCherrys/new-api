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
  const [submittedKeyword, setSubmittedKeyword] = useState('');
  const [activeModelType, setActiveModelType] = useState(1);
  const [reloadSignal, setReloadSignal] = useState(0);
  const [showEdit, setShowEdit] = useState(false);
  const [editingMapping, setEditingMapping] = useState(null);

  const loadMappings = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/model-mapping/search', {
        params: {
          keyword: submittedKeyword,
          model_type: activeModelType,
          p: (activePage - 1) * pageSize,
          page_size: pageSize,
        },
      });
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
  }, [activeModelType, activePage, pageSize, submittedKeyword]);

  const refresh = useCallback(() => {
    setReloadSignal((current) => current + 1);
  }, []);

  useEffect(() => {
    loadMappings();
  }, [loadMappings, reloadSignal]);

  const handlePageChange = (page) => {
    setActivePage(page);
  };

  const handlePageSizeChange = (size) => {
    setPageSize(size);
    setActivePage(1);
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
    setEditingMapping(mapping || { model_type: activeModelType });
    setShowEdit(true);
  };

  const closeEditModal = () => {
    setShowEdit(false);
    setEditingMapping(null);
  };

  const handleSearch = () => {
    const nextKeyword = searchKeyword.trim();
    setActivePage(1);
    if (submittedKeyword !== nextKeyword) {
      setSubmittedKeyword(nextKeyword);
      return;
    }
    setReloadSignal((current) => current + 1);
  };

  const handleModelTypeChange = (modelType) => {
    const nextModelType = Number(modelType) || 1;
    setActivePage(1);
    if (activeModelType !== nextModelType) {
      setActiveModelType(nextModelType);
      return;
    }
    setReloadSignal((current) => current + 1);
  };

  return {
    mappings,
    loading,
    activePage,
    pageSize,
    mappingCount,
    searchKeyword,
    activeModelType,
    showEdit,
    editingMapping,
    setSearchKeyword,
    handlePageChange,
    handlePageSizeChange,
    refresh,
    manageMapping,
    deleteMapping,
    openEditModal,
    closeEditModal,
    handleSearch,
    handleModelTypeChange,
  };
};
