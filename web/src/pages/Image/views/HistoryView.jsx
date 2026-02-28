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

import React, { useState, useEffect, useCallback } from 'react';
import {
  Card,
  Select,
  Input,
  Space,
  Pagination,
  Spin,
  Button,
  Toast,
} from '@douyinfe/semi-ui';
import { IconSearch, IconRefresh } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';
import { useTaskPolling } from '../../../hooks/useTaskPolling';
import TaskTable from '../components/TaskTable';

const HistoryView = ({ onRegenerate }) => {
  const { t } = useTranslation();

  const [tasks, setTasks] = useState([]);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({
    status: 'all',
    model: 'all',
    search: '',
  });
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 10,
    total: 0,
  });

  // 获取活跃任务 ID（pending 和 running 状态）
  const activeTaskIds = tasks
    .filter((task) => task.status === 'pending' || task.status === 'running')
    .map((task) => task.id);

  // 任务更新回调
  const handleTaskUpdate = useCallback((updatedTask) => {
    setTasks((prevTasks) =>
      prevTasks.map((task) =>
        task.id === updatedTask.id ? { ...task, ...updatedTask } : task
      )
    );
  }, []);

  // 任务完成回调
  const handleTaskComplete = useCallback(
    (completedTask) => {
      if (completedTask.status === 'succeeded') {
        Toast.success({
          content: t('任务 #{{id}} 已完成', { id: completedTask.id }),
          duration: 3,
        });
      } else if (completedTask.status === 'failed') {
        Toast.error({
          content: t('任务 #{{id}} 失败', { id: completedTask.id }),
          duration: 3,
        });
      }
    },
    [t]
  );

  // 启用轮询
  useTaskPolling(activeTaskIds, handleTaskUpdate, handleTaskComplete, {
    enabled: activeTaskIds.length > 0,
  });

  // 获取任务列表
  const fetchTasks = useCallback(async () => {
    setLoading(true);
    try {
      const params = {
        page: pagination.current,
        page_size: pagination.pageSize,
      };

      if (filters.status !== 'all') {
        params.status = filters.status;
      }

      if (filters.model !== 'all') {
        params.model = filters.model;
      }

      if (filters.search.trim()) {
        params.search = filters.search.trim();
      }

      const res = await API.get('/api/image/tasks', { params });
      const { success, message, data } = res.data;

      if (success) {
        setTasks(data.tasks || []);
        setPagination((prev) => ({
          ...prev,
          total: data.total || 0,
        }));
      } else {
        showError(message || t('获取任务列表失败'));
      }
    } catch (error) {
      console.error('Fetch tasks error:', error);
      showError(t('获取任务列表失败，请重试'));
    } finally {
      setLoading(false);
    }
  }, [pagination.current, pagination.pageSize, filters, t]);

  // 初始加载和过滤变化时重新获取
  useEffect(() => {
    fetchTasks();
  }, [fetchTasks]);

  // 删除任务
  const handleDelete = async (taskId) => {
    try {
      const res = await API.delete(`/api/image/task/${taskId}`);
      const { success, message } = res.data;

      if (success) {
        showSuccess(t('任务删除成功'));
        fetchTasks();
      } else {
        showError(message || t('任务删除失败'));
      }
    } catch (error) {
      console.error('Delete task error:', error);
      showError(t('任务删除失败，请重试'));
    }
  };

  // 处理过滤器变化
  const handleFilterChange = (field, value) => {
    setFilters((prev) => ({
      ...prev,
      [field]: value,
    }));
    setPagination((prev) => ({
      ...prev,
      current: 1, // 重置到第一页
    }));
  };

  // 处理分页变化
  const handlePageChange = (page) => {
    setPagination((prev) => ({
      ...prev,
      current: page,
    }));
  };

  // 刷新列表
  const handleRefresh = () => {
    fetchTasks();
  };

  return (
    <div className="w-full h-full">
      <Card>
        {/* 过滤器 */}
        <Space spacing="medium" wrap style={{ marginBottom: '16px' }}>
          <Select
            value={filters.status}
            onChange={(value) => handleFilterChange('status', value)}
            style={{ width: 150 }}
            placeholder={t('选择状态')}
          >
            <Select.Option value="all">{t('全部状态')}</Select.Option>
            <Select.Option value="pending">{t('等待中')}</Select.Option>
            <Select.Option value="running">{t('生成中')}</Select.Option>
            <Select.Option value="succeeded">{t('已完成')}</Select.Option>
            <Select.Option value="failed">{t('失败')}</Select.Option>
          </Select>

          <Select
            value={filters.model}
            onChange={(value) => handleFilterChange('model', value)}
            style={{ width: 200 }}
            placeholder={t('选择模型')}
          >
            <Select.Option value="all">{t('全部模型')}</Select.Option>
            <Select.Option value="dall-e-3">DALL-E 3</Select.Option>
            <Select.Option value="dall-e-2">DALL-E 2</Select.Option>
            <Select.Option value="stable-diffusion">
              Stable Diffusion
            </Select.Option>
            <Select.Option value="midjourney">Midjourney</Select.Option>
          </Select>

          <Input
            prefix={<IconSearch />}
            placeholder={t('搜索提示词')}
            value={filters.search}
            onChange={(value) => handleFilterChange('search', value)}
            style={{ width: 250 }}
          />

          <Button
            icon={<IconRefresh />}
            onClick={handleRefresh}
            loading={loading}
          >
            {t('刷新')}
          </Button>
        </Space>

        {/* 任务表格 */}
        <Spin spinning={loading}>
          <TaskTable
            tasks={tasks}
            loading={loading}
            onRegenerate={onRegenerate}
            onDelete={handleDelete}
          />
        </Spin>

        {/* 分页 */}
        {pagination.total > 0 && (
          <div
            style={{
              marginTop: '16px',
              display: 'flex',
              justifyContent: 'center',
            }}
          >
            <Pagination
              total={pagination.total}
              currentPage={pagination.current}
              pageSize={pagination.pageSize}
              onPageChange={handlePageChange}
              showSizeChanger
              pageSizeOpts={[10, 20, 50, 100]}
              onPageSizeChange={(size) => {
                setPagination((prev) => ({
                  ...prev,
                  pageSize: size,
                  current: 1,
                }));
              }}
            />
          </div>
        )}
      </Card>
    </div>
  );
};

export default HistoryView;
