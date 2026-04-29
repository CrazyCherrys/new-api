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

import React, { useState, useEffect, useRef } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Select,
  Button,
  Upload,
  Spin,
  Typography,
  Image,
  InputNumber,
  TextArea,
  Pagination,
  Empty,
} from '@douyinfe/semi-ui';
import {
  IconPlus,
  IconDelete,
  IconClock,
  IconImage,
  IconBolt,
  IconChevronUp,
  IconChevronDown,
  IconExternalOpen,
} from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../helpers';
import ImageGenerationTaskCard from '../../components/ImageGenerationTaskCard';
import ImageGenerationTaskModal from '../../components/ImageGenerationTaskModal';

const { Text } = Typography;

const ImageGeneration = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();

  // LocalStorage keys
  const STORAGE_KEYS = {
    SERIES: 'imageGen_selectedSeries',
    MODEL: 'imageGen_selectedModel',
    ASPECT_RATIO: 'imageGen_aspectRatio',
    RESOLUTION: 'imageGen_resolution',
    QUANTITY: 'imageGen_quantity',
  };

  // 从 localStorage 读取初始值的辅助函数
  const getStoredValue = (key, defaultValue) => {
    try {
      const stored = localStorage.getItem(key);
      return stored !== null ? stored : defaultValue;
    } catch (e) {
      return defaultValue;
    }
  };

  const getStoredNumber = (key, defaultValue) => {
    try {
      const stored = localStorage.getItem(key);
      if (stored !== null) {
        const num = parseInt(stored, 10);
        return isNaN(num) ? defaultValue : num;
      }
      return defaultValue;
    } catch (e) {
      return defaultValue;
    }
  };

  const [loading, setLoading] = useState(false);
  const [modelSeries, setModelSeries] = useState([]);
  const [models, setModels] = useState([]);
  const [filteredModels, setFilteredModels] = useState([]);

  const [selectedSeries, setSelectedSeries] = useState(() =>
    getStoredValue(STORAGE_KEYS.SERIES, ''),
  );
  const [selectedModel, setSelectedModel] = useState(() =>
    getStoredValue(STORAGE_KEYS.MODEL, ''),
  );
  const [selectedModelData, setSelectedModelData] = useState(null);
  const [prompt, setPrompt] = useState('');
  const [referenceImages, setReferenceImages] = useState([]);
  const [aspectRatio, setAspectRatio] = useState(() =>
    getStoredValue(STORAGE_KEYS.ASPECT_RATIO, ''),
  );
  const [resolution, setResolution] = useState(() =>
    getStoredValue(STORAGE_KEYS.RESOLUTION, ''),
  );
  const [quantity, setQuantity] = useState(() =>
    getStoredNumber(STORAGE_KEYS.QUANTITY, 1),
  );
  const [generatedImages, setGeneratedImages] = useState([]);
  const [generating, setGenerating] = useState(false);

  const [availableAspectRatios, setAvailableAspectRatios] = useState([]);
  const [availableResolutions, setAvailableResolutions] = useState([]);

  // 任务列表相关状态
  const [tasks, setTasks] = useState([]);
  const [taskTotal, setTaskTotal] = useState(0);
  const [taskPage, setTaskPage] = useState(1);
  const [taskPageSize, setTaskPageSize] = useState(20);
  const [taskStatusFilter, setTaskStatusFilter] = useState(''); // '' | 'pending' | 'generating' | 'success' | 'failed'
  const [taskModelFilter, setTaskModelFilter] = useState(''); // '' | model_id
  const [taskTimeFilter, setTaskTimeFilter] = useState(''); // '' | 'today' | 'last7d' | 'last30d' | 'thisMonth'
  const [taskSortBy, setTaskSortBy] = useState('created_time'); // 'created_time' | 'completed_time' | 'status'
  const [taskSortOrder, setTaskSortOrder] = useState('desc'); // 'desc' | 'asc'
  const [selectedTask, setSelectedTask] = useState(null);
  const [taskModalVisible, setTaskModalVisible] = useState(false);
  const [loadingTasks, setLoadingTasks] = useState(false);
  const [selectedTaskIds, setSelectedTaskIds] = useState(new Set());
  const [deletingTasks, setDeletingTasks] = useState(false);
  const sseRef = useRef(null);
  const pollingTimerRef = useRef(null);
  const [maxImageSize, setMaxImageSize] = useState(10); // MB，默认 10MB

  const formatModelSeries = (series) => {
    if (!series) return '';

    const seriesMap = {
      openai: 'OpenAI',
      gemini: 'Gemini',
      claude: 'Claude',
      grok: 'Grok',
      deepseek: 'DeepSeek',
      qwen: 'Qwen',
      glm: 'GLM',
      hunyuan: 'Hunyuan',
      doubao: 'Doubao',
      spark: 'Spark',
      baichuan: 'Baichuan',
      minimax: 'Minimax',
      moonshot: 'Moonshot',
      yi: 'Yi',
      chatglm: 'ChatGLM',
      ernie: 'ERNIE',
      wenxin: 'Wenxin',
      tongyi: 'Tongyi',
      azure: 'Azure',
      aws: 'AWS',
      cohere: 'Cohere',
      anthropic: 'Anthropic',
      mistral: 'Mistral',
      llama: 'Llama',
      palm: 'PaLM',
      bard: 'Bard',
      midjourney: 'Midjourney',
      dalle: 'OpenAI',
      'stable-diffusion': 'Stable Diffusion',
      flux: 'Flux',
      suno: 'Suno',
    };

    return (
      seriesMap[series.toLowerCase()] ||
      series.charAt(0).toUpperCase() + series.slice(1)
    );
  };

  useEffect(() => {
    loadDrawingModels();
    loadTasks();
    loadWorkerSettings();
    connectSSE();

    return () => {
      disconnectSSE();
      stopPolling();
    };
  }, []);

  useEffect(() => {
    const taskId = new URLSearchParams(location.search).get('task_id');
    if (!taskId) return;

    const loadTaskDetail = async () => {
      try {
        const res = await API.get(`/api/image-generation/tasks/${taskId}`);
        if (res.data.success) {
          setSelectedTask(res.data.data);
          setTaskModalVisible(true);
        } else {
          showError(res.data.message || t('加载任务详情失败'));
        }
      } catch (error) {
        showError(error.message || t('加载任务详情失败'));
      }
    };

    loadTaskDetail();
  }, [location.search, t]);

  useEffect(() => {
    loadTasks();
  }, [
    taskPage,
    taskPageSize,
    taskStatusFilter,
    taskModelFilter,
    taskTimeFilter,
    taskSortBy,
    taskSortOrder,
  ]);

  // 切换任意筛选/排序时回到第一页
  useEffect(() => {
    setTaskPage(1);
  }, [
    taskStatusFilter,
    taskModelFilter,
    taskTimeFilter,
    taskSortBy,
    taskSortOrder,
  ]);

  const loadDrawingModels = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/image-generation/models');
      if (res.data.success) {
        const drawingModels = res.data.data || [];
        setModels(drawingModels);

        const seriesSet = new Set();
        drawingModels.forEach((model) => {
          if (model.model_series) {
            seriesSet.add(model.model_series);
          }
        });
        const seriesList = Array.from(seriesSet);
        setModelSeries(seriesList);
        setSelectedSeries((prev) => {
          if (prev === 'all' || (prev && seriesList.includes(prev))) {
            return prev;
          }
          return seriesList[0] || 'all';
        });
      } else {
        showError(res.data.message || t('加载模型失败'));
      }
    } catch (error) {
      showError(error.message || t('加载模型失败'));
    } finally {
      setLoading(false);
    }
  };

  const loadWorkerSettings = async () => {
    try {
      const res = await API.get('/api/image-generation/settings');
      if (res.data.success && res.data.data) {
        const size = parseInt(res.data.data.max_image_size, 10);
        if (!isNaN(size) && size > 0) {
          setMaxImageSize(size);
        }
      }
    } catch (error) {
      // 静默失败，使用默认值
      console.error('Failed to load worker settings:', error);
    }
  };

  // 根据时间范围预设计算 start/end 时间戳（秒）
  const computeTimeRange = (preset) => {
    if (!preset) return { start: 0, end: 0 };
    const now = new Date();
    const end = Math.floor(now.getTime() / 1000);
    let startDate;
    switch (preset) {
      case 'today':
        startDate = new Date(now.getFullYear(), now.getMonth(), now.getDate());
        break;
      case 'last7d':
        startDate = new Date(now.getTime() - 7 * 24 * 3600 * 1000);
        break;
      case 'last30d':
        startDate = new Date(now.getTime() - 30 * 24 * 3600 * 1000);
        break;
      case 'thisMonth':
        startDate = new Date(now.getFullYear(), now.getMonth(), 1);
        break;
      default:
        return { start: 0, end: 0 };
    }
    return { start: Math.floor(startDate.getTime() / 1000), end };
  };

  // silent=true 时静默刷新（轮询），不触发 loadingTasks，不显示 Spin 遮罩
  const loadTasks = async (silent = false) => {
    if (!silent) setLoadingTasks(true);
    try {
      const params = {
        page: taskPage,
        page_size: taskPageSize,
      };
      if (taskStatusFilter) {
        params.status = taskStatusFilter;
      }
      if (taskModelFilter) {
        params.model_id = taskModelFilter;
      }
      const { start, end } = computeTimeRange(taskTimeFilter);
      if (start > 0) {
        params.start_time = start;
        params.end_time = end;
      }
      if (taskSortBy) {
        params.sort_by = taskSortBy;
      }
      if (taskSortOrder) {
        params.sort_order = taskSortOrder;
      }

      const res = await API.get('/api/image-generation/tasks', { params });
      if (res.data.success) {
        const newItems = res.data.data.items || [];
        const newTotal = res.data.data.total || 0;
        // 智能合并：仅当内容实际变化时才更新，避免全量替换导致卡片无效重渲染
        setTasks((prev) => {
          if (prev.length === newItems.length) {
            const unchanged = newItems.every((newTask, i) => {
              const old = prev[i];
              return (
                old &&
                old.id === newTask.id &&
                old.status === newTask.status &&
                old.image_url === newTask.image_url &&
                old.error_message === newTask.error_message
              );
            });
            if (unchanged) return prev;
          }
          return newItems;
        });
        setTaskTotal(newTotal);
      } else if (!silent) {
        showError(res.data.message || t('加载任务列表失败'));
      }
    } catch (error) {
      if (!silent) showError(error.message || t('加载任务列表失败'));
    } finally {
      if (!silent) setLoadingTasks(false);
    }
  };

  // 处理任务选择
  const handleTaskSelect = (taskId, checked) => {
    setSelectedTaskIds((prev) => {
      const newSet = new Set(prev);
      if (checked) {
        newSet.add(taskId);
      } else {
        newSet.delete(taskId);
      }
      return newSet;
    });
  };

  // 全选/取消全选
  const handleSelectAll = (checked) => {
    if (checked) {
      setSelectedTaskIds(new Set(tasks.map((t) => t.id)));
    } else {
      setSelectedTaskIds(new Set());
    }
  };

  // 批量删除任务
  const handleBatchDelete = async () => {
    if (selectedTaskIds.size === 0) {
      showError(t('请先选择要删除的任务'));
      return;
    }

    setDeletingTasks(true);
    try {
      const deletePromises = Array.from(selectedTaskIds).map((taskId) =>
        API.delete(`/api/image-generation/tasks/${taskId}`),
      );

      const results = await Promise.allSettled(deletePromises);
      const successCount = results.filter(
        (r) => r.status === 'fulfilled',
      ).length;
      const failCount = results.filter((r) => r.status === 'rejected').length;

      if (successCount > 0) {
        showSuccess(t('成功删除 {{count}} 个任务', { count: successCount }));
        setSelectedTaskIds(new Set());
        loadTasks();
      }

      if (failCount > 0) {
        showError(t('删除失败 {{count}} 个任务', { count: failCount }));
      }
    } catch (error) {
      showError(error.message || t('批量删除失败'));
    } finally {
      setDeletingTasks(false);
    }
  };

  // 连接 SSE
  const connectSSE = () => {
    try {
      const eventSource = new EventSource('/api/image-generation/sse');

      eventSource.addEventListener('task_update', (e) => {
        try {
          const data = JSON.parse(e.data);
          updateTaskInList(data);
        } catch (err) {
          console.error('Failed to parse SSE data:', err);
        }
      });

      eventSource.onerror = () => {
        console.log('SSE connection error, falling back to polling');
        eventSource.close();
        startPolling();
      };

      sseRef.current = eventSource;
    } catch (error) {
      console.error('Failed to connect SSE:', error);
      startPolling();
    }
  };

  // 断开 SSE
  const disconnectSSE = () => {
    if (sseRef.current) {
      sseRef.current.close();
      sseRef.current = null;
    }
  };

  // 开始轮询
  const startPolling = () => {
    if (pollingTimerRef.current) return;

    pollingTimerRef.current = setInterval(() => {
      loadTasks(true); // 静默刷新，不触发 Spin 遮罩
    }, 5000); // 5秒轮询
  };

  // ���止轮询
  const stopPolling = () => {
    if (pollingTimerRef.current) {
      clearInterval(pollingTimerRef.current);
      pollingTimerRef.current = null;
    }
  };

  // 更新任务列表中的单个任务
  const updateTaskInList = (updatedTask) => {
    setTasks((prevTasks) => {
      const index = prevTasks.findIndex((t) => t.id === updatedTask.id);
      if (index !== -1) {
        const newTasks = [...prevTasks];
        newTasks[index] = updatedTask;
        return newTasks;
      }
      // 如果是新任务，添加到列表开头
      return [updatedTask, ...prevTasks];
    });
  };

  useEffect(() => {
    // 模型列表尚未加载时跳过，避免以空列表覆盖从 localStorage 恢复的选择
    if (models.length === 0) return;
    const isModelEnabled = (model) =>
      model.status === undefined || model.status === null || model.status === 1;
    let filtered = [];
    if (selectedSeries === 'all') {
      filtered = models.filter(isModelEnabled);
    } else if (selectedSeries) {
      filtered = models.filter(
        (model) =>
          model.model_series === selectedSeries && isModelEnabled(model),
      );
    }
    setFilteredModels(filtered);
    setSelectedModel((current) => {
      if (filtered.some((model) => model.request_model === current)) {
        return current;
      }
      return filtered[0]?.request_model || '';
    });
    if (filtered.length === 0) {
      setAvailableAspectRatios([]);
      setAvailableResolutions([]);
    }
  }, [selectedSeries, models]);

  // 保存用户选择到 localStorage
  useEffect(() => {
    if (selectedSeries) {
      try {
        localStorage.setItem(STORAGE_KEYS.SERIES, selectedSeries);
      } catch (e) {
        console.error('Failed to save selectedSeries to localStorage:', e);
      }
    }
  }, [selectedSeries]);

  useEffect(() => {
    if (selectedModel) {
      try {
        localStorage.setItem(STORAGE_KEYS.MODEL, selectedModel);
      } catch (e) {
        console.error('Failed to save selectedModel to localStorage:', e);
      }
    }
  }, [selectedModel]);

  useEffect(() => {
    if (aspectRatio) {
      try {
        localStorage.setItem(STORAGE_KEYS.ASPECT_RATIO, aspectRatio);
      } catch (e) {
        console.error('Failed to save aspectRatio to localStorage:', e);
      }
    }
  }, [aspectRatio]);

  useEffect(() => {
    if (resolution) {
      try {
        localStorage.setItem(STORAGE_KEYS.RESOLUTION, resolution);
      } catch (e) {
        console.error('Failed to save resolution to localStorage:', e);
      }
    }
  }, [resolution]);

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEYS.QUANTITY, quantity.toString());
    } catch (e) {
      console.error('Failed to save quantity to localStorage:', e);
    }
  }, [quantity]);

  useEffect(() => {
    if (selectedModel) {
      const model = models.find((m) => m.request_model === selectedModel);
      if (model) {
        setSelectedModelData(model);

        if (model.aspect_ratios) {
          try {
            const ratios = JSON.parse(model.aspect_ratios);
            setAvailableAspectRatios(ratios);
            // 只在当前 aspectRatio 为空或不在新���表中时才重置
            setAspectRatio((current) => {
              if (!current || !ratios.includes(current)) {
                return ratios.length > 0 ? ratios[0] : '';
              }
              return current;
            });
          } catch (e) {
            setAvailableAspectRatios([]);
            setAspectRatio('');
          }
        } else {
          setAvailableAspectRatios([]);
          setAspectRatio('');
        }

        if (model.resolutions) {
          try {
            const resolutions = JSON.parse(model.resolutions);
            setAvailableResolutions(resolutions);
            // 只在当前 resolution 为空或不在新列表中时才重置
            setResolution((current) => {
              if (!current || !resolutions.includes(current)) {
                return resolutions.length > 0 ? resolutions[0] : '';
              }
              return current;
            });
          } catch (e) {
            setAvailableResolutions([]);
            setResolution('');
          }
        } else {
          setAvailableResolutions([]);
          setResolution('');
        }
      }
    } else {
      setSelectedModelData(null);
      setAvailableAspectRatios([]);
      setAvailableResolutions([]);
      setAspectRatio('');
      setResolution('');
    }
  }, [selectedModel, models]);

  const handleImageUpload = ({ fileList }) => {
    setReferenceImages(fileList);
  };

  const validateImageSize = (file) => {
    const fileSizeMB = file.size / 1024 / 1024;
    if (fileSizeMB > maxImageSize) {
      showError(
        t('图片文件过大：{{size}}MB，最大允许 {{max}}MB', {
          size: fileSizeMB.toFixed(2),
          max: maxImageSize,
        }),
      );
      return false;
    }
    return true;
  };

  const handleImageRemove = (file) => {
    setReferenceImages(referenceImages.filter((img) => img.uid !== file.uid));
  };

  const handleGenerate = async () => {
    if (!selectedModel) {
      showError(t('请选择模型'));
      return;
    }
    if (!prompt.trim()) {
      showError(t('请输入提示词'));
      return;
    }
    if (!selectedModelData?.request_endpoint) {
      showError(t('模型配置错误：缺少 request_endpoint'));
      return;
    }

    setGenerating(true);
    try {
      // 准备参数对象
      const params = {
        quantity: quantity,
      };

      if (aspectRatio) {
        params.aspect_ratio = aspectRatio;
      }
      if (resolution) {
        params.resolution = resolution;
      }

      // 处理参考图片
      if (referenceImages.length > 0) {
        const imagePromises = referenceImages.map((file) => {
          return new Promise((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = (e) => resolve(e.target.result);
            reader.onerror = reject;
            reader.readAsDataURL(file.fileInstance);
          });
        });
        const base64Images = await Promise.all(imagePromises);
        params.reference_images = base64Images;
      }

      // 创建任务（使用后端期望的字段名）
      const taskPayload = {
        model_id: selectedModel,
        prompt: prompt.trim(),
        request_endpoint: selectedModelData.request_endpoint,
        params: JSON.stringify(params),
      };

      const res = await API.post('/api/image-generation/tasks', taskPayload);
      if (res.data.success) {
        showSuccess(t('任务已创建，正在生成中...'));
        // 立即将新任务添加到列表开头，避免等待 loadTasks() 的延迟
        const newTask = res.data.data;
        if (newTask) {
          setTasks((prevTasks) => [newTask, ...prevTasks]);
          setTaskTotal((prev) => prev + 1);
        }
        // 同时刷新任务列表以确保数据一致性
        loadTasks();
      } else {
        showError(res.data.message || t('创建任务失败'));
      }
    } catch (error) {
      if (error.response?.data?.message) {
        showError(error.response.data.message);
      } else {
        showError(error.message || t('创建任务失败'));
      }
    } finally {
      setGenerating(false);
    }
  };

  const styles = {
    container: {
      display: 'flex',
      height: 'calc(100vh - 60px)',
      marginTop: 60,
      overflow: 'hidden',
    },
    leftPanel: {
      width: 320,
      minWidth: 320,
      display: 'flex',
      flexDirection: 'column',
      borderRight: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
    },
    leftContent: {
      flex: 1,
      overflowY: 'auto',
      padding: '20px 16px 0',
    },
    leftBottom: {
      padding: '16px',
      borderTop: '1px solid var(--semi-color-border)',
    },
    rightPanel: {
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      background: 'var(--semi-color-bg-1)',
      overflow: 'hidden',
    },
    rightTopBar: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '10px 20px',
      borderBottom: '1px solid var(--semi-color-border)',
      flexWrap: 'wrap',
      gap: 12,
      minHeight: 56,
    },
    rightContent: {
      flex: 1,
      overflow: 'auto',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    selectionBar: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '8px 20px',
      borderBottom: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
      gap: 12,
      flexWrap: 'wrap',
    },
    label: {
      display: 'block',
      fontSize: 13,
      fontWeight: 500,
      color: 'var(--semi-color-text-0)',
      marginBottom: 6,
    },
    fieldGroup: {
      marginBottom: 16,
    },
    generateBtn: {
      width: '100%',
      height: 44,
      borderRadius: 10,
      border: 'none',
      cursor: 'pointer',
      fontSize: 15,
      fontWeight: 600,
      color: '#fff',
      background:
        'linear-gradient(135deg, #e8593c 0%, #d4a843 50%, #5a9e6f 100%)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 6,
      transition: 'opacity 0.2s',
      marginTop: 12,
    },
    addImageBtn: {
      width: 48,
      height: 48,
      borderRadius: 8,
      border: '1px dashed var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      cursor: 'pointer',
      color: 'var(--semi-color-text-2)',
      transition: 'border-color 0.2s',
    },
    textareaWrapper: {
      position: 'relative',
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
      padding: 0,
    },
    charCount: {
      fontSize: 12,
      color: 'var(--semi-color-text-3)',
      padding: '4px 12px 8px',
      textAlign: 'left',
    },
    emptyState: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: 12,
    },
    emptyIcon: {
      width: 64,
      height: 64,
      borderRadius: '50%',
      background: 'rgba(232, 89, 60, 0.12)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    paramRow: {
      display: 'flex',
      gap: 12,
      alignItems: 'flex-end',
    },
    paramItem: {
      flex: 1,
    },
    paramLabel: {
      fontSize: 12,
      color: 'var(--semi-color-text-2)',
      marginBottom: 4,
      display: 'block',
    },
    referenceImageThumb: {
      width: 48,
      height: 48,
      borderRadius: 8,
      objectFit: 'cover',
      border: '1px solid var(--semi-color-border)',
    },
    referenceImageContainer: {
      position: 'relative',
      display: 'inline-block',
    },
    removeImageBtn: {
      position: 'absolute',
      top: -6,
      right: -6,
      width: 18,
      height: 18,
      borderRadius: '50%',
      background: 'var(--semi-color-danger)',
      border: 'none',
      cursor: 'pointer',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: '#fff',
      fontSize: 10,
      padding: 0,
    },
    filterGroup: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      flexWrap: 'wrap',
    },
    filterLabel: {
      fontSize: 13,
      color: 'var(--semi-color-text-2)',
    },
    imagesGrid: {
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
      gap: 16,
      padding: 16,
      width: '100%',
      alignContent: 'start',
    },
    tasksGrid: {
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))',
      gap: 20,
      padding: 20,
      width: '100%',
      flex: 1,
      alignContent: 'start',
      overflowY: 'auto',
    },
  };

  const renderLeftPanel = () => (
    <div style={styles.leftPanel}>
      <div style={styles.leftContent}>
        <Spin spinning={loading}>
          <div style={styles.fieldGroup}>
            <span style={styles.label}>{t('模型系列')}</span>
            <Select
              style={{ width: '100%' }}
              value={selectedSeries}
              onChange={setSelectedSeries}
              disabled={modelSeries.length === 0}
            >
              <Select.Option value='all'>{t('全部系列')}</Select.Option>
              {modelSeries.map((series) => (
                <Select.Option key={series} value={series}>
                  {formatModelSeries(series)}
                </Select.Option>
              ))}
            </Select>
          </div>

          <div style={styles.fieldGroup}>
            <span style={styles.label}>{t('模型')}</span>
            <Select
              style={{ width: '100%' }}
              value={selectedModel}
              onChange={setSelectedModel}
              disabled={filteredModels.length === 0}
              filter
              placeholder={t('请选择模型')}
            >
              {filteredModels.map((model) => (
                <Select.Option
                  key={model.request_model}
                  value={model.request_model}
                >
                  {model.display_name || model.request_model}
                </Select.Option>
              ))}
            </Select>
          </div>

          <div style={styles.fieldGroup}>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                marginBottom: 6,
              }}
            >
              <span style={styles.label}>{t('描述你的创意')}</span>
              <span
                style={{
                  fontSize: 12,
                  color: 'var(--semi-color-primary)',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 4,
                }}
              >
                <IconBolt size='small' />
                {t('AI优化')}
              </span>
            </div>
            <div style={styles.textareaWrapper}>
              <TextArea
                placeholder={t('描述你的创意...')}
                value={prompt}
                onChange={setPrompt}
                maxCount={5000}
                showClear
                autosize={{ minRows: 6, maxRows: 12 }}
                style={{
                  border: 'none',
                  background: 'transparent',
                  resize: 'none',
                }}
              />
              <div style={styles.charCount}>{prompt.length}/5000</div>
            </div>
          </div>

          <div style={styles.fieldGroup}>
            <span style={styles.label}>{t('参考图像')}</span>
            <div
              style={{
                display: 'flex',
                gap: 8,
                flexWrap: 'wrap',
                alignItems: 'center',
              }}
            >
              {referenceImages.map((file, idx) => (
                <div
                  key={file.uid || idx}
                  style={styles.referenceImageContainer}
                >
                  <img
                    src={
                      file.url ||
                      (file.fileInstance &&
                        URL.createObjectURL(file.fileInstance))
                    }
                    alt=''
                    style={styles.referenceImageThumb}
                  />
                  <button
                    style={styles.removeImageBtn}
                    onClick={() => handleImageRemove(file)}
                  >
                    <IconDelete size='extra-small' />
                  </button>
                </div>
              ))}
              <Upload
                action=''
                accept='image/*'
                multiple
                fileList={referenceImages}
                onChange={handleImageUpload}
                showUploadList={false}
                beforeUpload={validateImageSize}
              >
                <div style={styles.addImageBtn}>
                  <IconPlus size='large' />
                </div>
              </Upload>
            </div>
          </div>
        </Spin>
      </div>

      <div style={styles.leftBottom}>
        <div style={styles.paramRow}>
          <div style={styles.paramItem}>
            <span style={styles.paramLabel}>{t('生成比例')}</span>
            <Select
              style={{ width: '100%' }}
              value={aspectRatio}
              onChange={setAspectRatio}
              size='default'
              disabled={availableAspectRatios.length === 0}
              placeholder={t('请选择')}
            >
              {availableAspectRatios.map((ratio) => (
                <Select.Option key={ratio} value={ratio}>
                  {ratio}
                </Select.Option>
              ))}
            </Select>
          </div>
          <div style={styles.paramItem}>
            <span style={styles.paramLabel}>{t('分辨率')}</span>
            <Select
              style={{ width: '100%' }}
              value={resolution}
              onChange={setResolution}
              size='default'
              disabled={availableResolutions.length === 0}
              placeholder={t('请选择')}
            >
              {availableResolutions.map((res) => (
                <Select.Option key={res} value={res}>
                  {res}
                </Select.Option>
              ))}
            </Select>
          </div>
          <div style={styles.paramItem}>
            <span style={styles.paramLabel}>{t('生成数量')}</span>
            <InputNumber
              min={1}
              max={4}
              value={quantity}
              onChange={(val) => setQuantity(val || 1)}
              style={{ width: '100%' }}
            />
          </div>
        </div>

        <button
          style={{
            ...styles.generateBtn,
            opacity: generating || !selectedModel || !prompt.trim() ? 0.6 : 1,
            pointerEvents:
              generating || !selectedModel || !prompt.trim() ? 'none' : 'auto',
          }}
          onClick={handleGenerate}
          disabled={generating || !selectedModel || !prompt.trim()}
        >
          {generating ? (
            <Spin size='small' />
          ) : (
            <>
              <IconImage size='small' />
              {t('生成')}
            </>
          )}
        </button>
      </div>
    </div>
  );

  const renderHistoryContent = () => (
    <Spin spinning={loadingTasks} style={{ width: '100%', height: '100%' }}>
      {tasks.length > 0 ? (
        <div
          style={{
            width: '100%',
            height: '100%',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          <div style={styles.tasksGrid}>
            {tasks.map((task) => (
              <ImageGenerationTaskCard
                key={task.id}
                task={task}
                selected={selectedTaskIds.has(task.id)}
                onSelectChange={handleTaskSelect}
                onClick={() => {
                  setSelectedTask(task);
                  setTaskModalVisible(true);
                }}
              />
            ))}
          </div>
          {taskTotal > taskPageSize && (
            <div
              style={{
                padding: '16px',
                textAlign: 'center',
                borderTop: '1px solid var(--semi-color-border)',
              }}
            >
              <Pagination
                total={taskTotal}
                currentPage={taskPage}
                pageSize={taskPageSize}
                onPageChange={setTaskPage}
                showSizeChanger
                onPageSizeChange={setTaskPageSize}
                pageSizeOpts={[10, 20, 50, 100]}
              />
            </div>
          )}
        </div>
      ) : (
        <div style={styles.emptyState}>
          <div style={styles.emptyIcon}>
            <IconClock
              size='extra-large'
              style={{ color: '#e8593c', fontSize: 28 }}
            />
          </div>
          <Text
            strong
            style={{ fontSize: 16, color: 'var(--semi-color-text-0)' }}
          >
            {t('暂无生成记录')}
          </Text>
          <Text
            type='tertiary'
            style={{ fontSize: 13, textAlign: 'center', maxWidth: 320 }}
          >
            {t('完成一次生成后，这里会保留你的创作历史记录。')}
          </Text>
        </div>
      )}
    </Spin>
  );

  const renderRightPanel = () => (
    <div style={styles.rightPanel}>
      {/* 顶部栏：左侧标题 + 右侧筛选 */}
      <div style={styles.rightTopBar}>
        <Text
          strong
          style={{ fontSize: 15, color: 'var(--semi-color-text-0)' }}
        >
          {t('生成记录')}
        </Text>

        <div style={styles.filterGroup}>
          <Button
            size='small'
            type='tertiary'
            icon={<IconExternalOpen />}
            onClick={() => navigate('/console/assets')}
          >
            {t('查看资产仓库')}
          </Button>

          <span style={styles.filterLabel}>{t('状态')}</span>
          <Select
            size='small'
            value={taskStatusFilter}
            onChange={setTaskStatusFilter}
            style={{ width: 110 }}
          >
            <Select.Option value=''>{t('全部')}</Select.Option>
            <Select.Option value='pending'>{t('等待中')}</Select.Option>
            <Select.Option value='generating'>{t('生成中')}</Select.Option>
            <Select.Option value='success'>{t('已完成')}</Select.Option>
            <Select.Option value='failed'>{t('失败')}</Select.Option>
          </Select>

          <span style={styles.filterLabel}>{t('模型')}</span>
          <Select
            size='small'
            value={taskModelFilter}
            onChange={setTaskModelFilter}
            style={{ width: 140 }}
            filter
            placeholder={t('全部')}
          >
            <Select.Option value=''>{t('全部')}</Select.Option>
            {models
              .filter((m) => m.status === 1)
              .map((m) => (
                <Select.Option key={m.request_model} value={m.request_model}>
                  {m.display_name || m.request_model}
                </Select.Option>
              ))}
          </Select>

          <span style={styles.filterLabel}>{t('时间')}</span>
          <Select
            size='small'
            value={taskTimeFilter}
            onChange={setTaskTimeFilter}
            style={{ width: 110 }}
          >
            <Select.Option value=''>{t('全部')}</Select.Option>
            <Select.Option value='today'>{t('今天')}</Select.Option>
            <Select.Option value='last7d'>{t('近 7 天')}</Select.Option>
            <Select.Option value='last30d'>{t('近 30 天')}</Select.Option>
            <Select.Option value='thisMonth'>{t('本月')}</Select.Option>
          </Select>

          <Select
            size='small'
            value={taskSortBy}
            onChange={setTaskSortBy}
            style={{ width: 120 }}
          >
            <Select.Option value='created_time'>
              {t('排序：创建时间')}
            </Select.Option>
            <Select.Option value='completed_time'>
              {t('排序：完成时间')}
            </Select.Option>
            <Select.Option value='status'>{t('排序：状态')}</Select.Option>
          </Select>

          <Button
            size='small'
            type='tertiary'
            icon={
              taskSortOrder === 'desc' ? <IconChevronDown /> : <IconChevronUp />
            }
            onClick={() =>
              setTaskSortOrder(taskSortOrder === 'desc' ? 'asc' : 'desc')
            }
          >
            {taskSortOrder === 'desc' ? t('降序') : t('升序')}
          </Button>
        </div>
      </div>

      {/* 选择/批量操作条 */}
      {(selectedTaskIds.size > 0 || tasks.length > 0) && (
        <div style={styles.selectionBar}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <Text type='tertiary' size='small'>
              {t('共')} {taskTotal} {t('个任务')}
            </Text>
            {selectedTaskIds.size > 0 && (
              <Text type='tertiary' size='small'>
                ({t('已选择')} {selectedTaskIds.size} {t('个')})
              </Text>
            )}
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            {selectedTaskIds.size > 0 ? (
              <>
                <Button
                  size='small'
                  type='tertiary'
                  onClick={() => handleSelectAll(false)}
                >
                  {t('取消选择')}
                </Button>
                <Button
                  size='small'
                  type='danger'
                  icon={<IconDelete />}
                  loading={deletingTasks}
                  onClick={handleBatchDelete}
                >
                  {t('删除选中')}
                </Button>
              </>
            ) : (
              <Button
                size='small'
                type='tertiary'
                onClick={() => handleSelectAll(true)}
              >
                {t('全选')}
              </Button>
            )}
          </div>
        </div>
      )}

      <div style={styles.rightContent}>{renderHistoryContent()}</div>

      <ImageGenerationTaskModal
        visible={taskModalVisible}
        onClose={() => {
          setTaskModalVisible(false);
          setSelectedTask(null);
        }}
        task={selectedTask}
        onRetrySuccess={(newTask) => {
          updateTaskInList(newTask);
        }}
        onDeleted={(deletedId) => {
          setTasks((prev) => prev.filter((t) => t.id !== deletedId));
          setTaskTotal((prev) => Math.max(0, prev - 1));
          setSelectedTaskIds((prev) => {
            const next = new Set(prev);
            next.delete(deletedId);
            return next;
          });
        }}
      />
    </div>
  );

  return (
    <div style={styles.container}>
      {renderLeftPanel()}
      {renderRightPanel()}
    </div>
  );
};

export default ImageGeneration;
