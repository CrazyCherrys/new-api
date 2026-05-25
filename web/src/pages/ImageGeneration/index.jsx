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

import React, { useState, useEffect, useRef, useMemo } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Select,
  Button,
  Upload,
  Spin,
  Typography,
  Input,
  TextArea,
  SideSheet,
} from '@douyinfe/semi-ui';
import {
  IconPlus,
  IconDelete,
  IconImage,
  IconChevronUp,
  IconChevronDown,
  IconSearch,
  IconSend,
  IconMenu,
  IconVideo,
  IconSetting,
} from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import {
  getCanvasImageUiState,
  getCanvasImageSelectorVisibility,
  getReferenceImageLimit,
  modelSupportsCapability,
  modelSupportsImageEditing,
  modelSupportsMaskEditing,
} from './canvasRules';

const { Text } = Typography;

const IMAGE_CAPABILITY_GENERATION = 'image_generation';
const IMAGE_CAPABILITY_EDITING = 'image_editing';
const DEFAULT_IMAGE_CAPABILITIES = [
  IMAGE_CAPABILITY_GENERATION,
  IMAGE_CAPABILITY_EDITING,
];
const VIDEO_CAPABILITY_IMAGE_TO_VIDEO = 'image_to_video';
const VIDEO_CAPABILITY_TEXT_TO_VIDEO = 'text_to_video';
const DEFAULT_POLLING_INTERVAL_SECONDS = 5;
const DEFAULT_MAX_BATCH_TASKS = 10;
const DEFAULT_TASK_PAGE_SIZE = 21;
const TASK_PAGE_SIZE_OPTIONS = [10, 21, 50, 100];
const TASK_LIST_REQUEST_TIMEOUT_MS = 20000;
const CANVAS_PREFILL_STORAGE_KEY = 'imageGen_canvasPrefill_v1';
const DEFAULT_VIDEO_PAGE_SIZE = 20;
const MODEL_CATALOG_ALL_SERIES = 'all';

const normalizeImageCapabilities = (raw) => {
  if (Array.isArray(raw)) {
    return raw;
  }
  if (typeof raw === 'string' && raw.trim() !== '') {
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        return parsed;
      }
    } catch (e) {
      console.error('Failed to parse image capabilities:', e);
    }
  }
  return [...DEFAULT_IMAGE_CAPABILITIES];
};

const normalizeVideoCapabilities = (raw) => {
  if (Array.isArray(raw)) {
    return raw;
  }
  if (typeof raw === 'string' && raw.trim() !== '') {
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        return parsed;
      }
    } catch (e) {
      console.error('Failed to parse video capabilities:', e);
    }
  }
  return [VIDEO_CAPABILITY_IMAGE_TO_VIDEO];
};

const modelSupportsVideoCapability = (model, capability) =>
  !!model &&
  normalizeVideoCapabilities(model.video_capabilities).includes(capability);

const taskCursorPaginationSupported = (state) =>
  !!state &&
  (state.sortBy === 'created_time' || state.sortBy === 'completed_time');

const isDefaultTaskViewState = (state) =>
  !!state &&
  state.page === 1 &&
  !state.statusFilter &&
  !state.modelFilter &&
  !state.timeFilter &&
  state.sortBy === 'created_time' &&
  state.sortOrder === 'desc';

const areTasksVisuallyEquivalent = (oldTask, newTask) =>
  !!oldTask &&
  !!newTask &&
  oldTask.id === newTask.id &&
  oldTask.status === newTask.status &&
  oldTask.image_url === newTask.image_url &&
  oldTask.thumbnail_url === newTask.thumbnail_url &&
  oldTask.completed_time === newTask.completed_time &&
  oldTask.started_time === newTask.started_time &&
  oldTask.progress === newTask.progress &&
  oldTask.error_message === newTask.error_message;

const taskIsDeletable = (task) =>
  task?.status === 'success' || task?.status === 'failed';

const mergeTaskCollections = (baseTasks, incomingTasks, maxItems) => {
  const mergedById = new Map((baseTasks || []).map((task) => [task.id, task]));
  const newFrontIds = [];

  (incomingTasks || []).forEach((task) => {
    if (!task?.id) {
      return;
    }
    const existing = mergedById.get(task.id);
    mergedById.set(task.id, existing ? { ...existing, ...task } : task);
    if (!existing) {
      newFrontIds.push(task.id);
    }
  });

  const frontIdSet = new Set(newFrontIds);
  const mergedTasks = [
    ...newFrontIds.map((id) => mergedById.get(id)),
    ...(baseTasks || [])
      .map((task) => {
        if (!task?.id || frontIdSet.has(task.id)) {
          return null;
        }
        return mergedById.get(task.id) || task;
      })
      .filter(Boolean),
  ];
  if (Number.isFinite(maxItems) && maxItems > 0) {
    return mergedTasks.slice(0, maxItems);
  }
  return mergedTasks;
};

const ImageGeneration = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const isMobile = useIsMobile();

  // LocalStorage keys
  const STORAGE_KEYS = {
    GROUP: 'imageGen_selectedGroup',
    MODE: 'canvas_generation_mode',
    SERIES: 'imageGen_selectedSeries',
    MODEL: 'imageGen_selectedModel',
    ASPECT_RATIO: 'imageGen_aspectRatio',
    RESOLUTION: 'imageGen_resolution',
    QUANTITY: 'imageGen_quantity',
    VIDEO_SERIES: 'videoGen_selectedSeries',
    VIDEO_MODEL: 'videoGen_selectedModel',
    VIDEO_ASPECT_RATIO: 'videoGen_aspectRatio',
    VIDEO_RESOLUTION: 'videoGen_resolution',
    VIDEO_DURATION: 'videoGen_duration',
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
  const [groupLoading, setGroupLoading] = useState(false);
  const [groupOptions, setGroupOptions] = useState([]);
  const [selectedGroup, setSelectedGroup] = useState(() =>
    getStoredValue(STORAGE_KEYS.GROUP, ''),
  );
  const [generationMode, setGenerationMode] = useState(() =>
    getStoredValue(STORAGE_KEYS.MODE, 'image'),
  );
  const [modelSeries, setModelSeries] = useState([]);
  const [models, setModels] = useState([]);
  const [filteredModels, setFilteredModels] = useState([]);
  const [videoModelSeries, setVideoModelSeries] = useState([]);
  const [videoModels, setVideoModels] = useState([]);
  const [videoFilteredModels, setVideoFilteredModels] = useState([]);
  const [modelSearchKeyword, setModelSearchKeyword] = useState('');
  const [catalogSeriesFilter, setCatalogSeriesFilter] = useState(
    MODEL_CATALOG_ALL_SERIES,
  );
  const [imageModelsCollapsed, setImageModelsCollapsed] = useState(false);
  const [videoModelsCollapsed, setVideoModelsCollapsed] = useState(false);
  const [mobileCatalogVisible, setMobileCatalogVisible] = useState(false);
  const [composerAdvancedVisible, setComposerAdvancedVisible] = useState(false);

  const [selectedSeries, setSelectedSeries] = useState(() =>
    getStoredValue(STORAGE_KEYS.SERIES, ''),
  );
  const [selectedModel, setSelectedModel] = useState(() =>
    getStoredValue(STORAGE_KEYS.MODEL, ''),
  );
  const [selectedModelData, setSelectedModelData] = useState(null);
  const [inspiration, setInspiration] = useState('');
  const [referenceImages, setReferenceImages] = useState([]);
  const [maskImage, setMaskImage] = useState(null);
  const [aspectRatio, setAspectRatio] = useState(() =>
    getStoredValue(STORAGE_KEYS.ASPECT_RATIO, ''),
  );
  const [resolution, setResolution] = useState(() =>
    getStoredValue(STORAGE_KEYS.RESOLUTION, ''),
  );
  const [quantity, setQuantity] = useState(() =>
    getStoredNumber(STORAGE_KEYS.QUANTITY, 1),
  );
  const [generating, setGenerating] = useState(false);
  const [videoGenerating, setVideoGenerating] = useState(false);

  const [availableAspectRatios, setAvailableAspectRatios] = useState([]);
  const [availableResolutions, setAvailableResolutions] = useState([]);
  const [videoAvailableAspectRatios, setVideoAvailableAspectRatios] = useState(
    [],
  );
  const [videoAvailableResolutions, setVideoAvailableResolutions] = useState(
    [],
  );

  const [videoSelectedSeries, setVideoSelectedSeries] = useState(() =>
    getStoredValue(STORAGE_KEYS.VIDEO_SERIES, ''),
  );
  const [videoSelectedModel, setVideoSelectedModel] = useState(() =>
    getStoredValue(STORAGE_KEYS.VIDEO_MODEL, ''),
  );
  const [videoSelectedModelData, setVideoSelectedModelData] = useState(null);
  const [videoPrompt, setVideoPrompt] = useState('');
  const [videoReferenceImage, setVideoReferenceImage] = useState(null);
  const [videoAspectRatio, setVideoAspectRatio] = useState(() =>
    getStoredValue(STORAGE_KEYS.VIDEO_ASPECT_RATIO, ''),
  );
  const [videoResolution, setVideoResolution] = useState(() =>
    getStoredValue(STORAGE_KEYS.VIDEO_RESOLUTION, ''),
  );
  const [videoDuration, setVideoDuration] = useState(() =>
    getStoredNumber(STORAGE_KEYS.VIDEO_DURATION, 0),
  );

  const {
    showImageAspectRatioSelector,
    showImageResolutionSelector,
  } = getCanvasImageSelectorVisibility({
    model: selectedModelData,
    aspectRatios: availableAspectRatios,
    resolutions: availableResolutions,
  });
  const showVideoAspectRatioSelector = videoAvailableAspectRatios.length > 0;
  const showVideoResolutionSelector = videoAvailableResolutions.length > 0;

  // 任务列表相关状态
  const [tasks, setTasks] = useState([]);
  const [taskTotal, setTaskTotal] = useState(0);
  const [taskPage, setTaskPage] = useState(1);
  const [taskPageSize, setTaskPageSize] = useState(DEFAULT_TASK_PAGE_SIZE);
  const [taskHasMore, setTaskHasMore] = useState(false);
  const [taskNextCursor, setTaskNextCursor] = useState('');
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
  const [videoTasks, setVideoTasks] = useState([]);
  const [videoTaskTotal, setVideoTaskTotal] = useState(0);
  const [videoTaskPage, setVideoTaskPage] = useState(1);
  const [videoTaskPageSize, setVideoTaskPageSize] = useState(
    DEFAULT_VIDEO_PAGE_SIZE,
  );
  const [videoTaskStatusFilter, setVideoTaskStatusFilter] = useState('');
  const [videoTaskModelFilter, setVideoTaskModelFilter] = useState('');
  const [videoTaskTimeFilter, setVideoTaskTimeFilter] = useState('');
  const [videoLoadingTasks, setVideoLoadingTasks] = useState(false);
  const [videoSelectedTask, setVideoSelectedTask] = useState(null);
  const [videoTaskModalVisible, setVideoTaskModalVisible] = useState(false);
  const [videoSelectedTaskIds, setVideoSelectedTaskIds] = useState(new Set());
  const [deletingVideoTasks, setDeletingVideoTasks] = useState(false);
  const sseRef = useRef(null);
  const pollingTimerRef = useRef(null);
  const pollingIntervalRef = useRef(DEFAULT_POLLING_INTERVAL_SECONDS);
  const taskListStateRef = useRef(null);
  const taskListRequestSeqRef = useRef(0);
  const taskDetailRequestSeqRef = useRef(0);
  const drawingModelsRequestSeqRef = useRef(0);
  const loadedModelsGroupRef = useRef('');
  const taskUpdatesCompletedSinceRef = useRef(
    Math.floor(Date.now() / 1000) - 60,
  );
  const taskCursorHistoryRef = useRef(['']);
  const pendingCanvasPrefillRef = useRef(null);
  const prefillGroupFallbackNoticeShownRef = useRef(false);
  const composerComposingRef = useRef(false);
  const [maxImageSize, setMaxImageSize] = useState(10); // MB，默认 10MB
  const [userCustomWorkerKeyEnabled, setUserCustomWorkerKeyEnabled] =
    useState(false);
  const [userCustomWorkerBaseUrlAllowed, setUserCustomWorkerBaseUrlAllowed] =
    useState(false);
  const [pollingIntervalSeconds, setPollingIntervalSeconds] = useState(
    DEFAULT_POLLING_INTERVAL_SECONDS,
  );
  const [sseConnected, setSseConnected] = useState(false);
  const [isPageVisible, setIsPageVisible] = useState(() =>
    typeof document === 'undefined' ? true : !document.hidden,
  );
  const hasActiveTasks = tasks.some(
    (task) => task.status === 'pending' || task.status === 'generating',
  );
  const hasActiveVideoTasks = videoTasks.some(
    (task) => task.status === 'queued' || task.status === 'in_progress',
  );
  const canUseTaskCursorPagination = taskCursorPaginationSupported(
    taskListStateRef.current || {
      sortBy: taskSortBy,
    },
  );
  const showsReliableTaskTotal = !canUseTaskCursorPagination;
  const hasNextTaskPage = taskHasMore;

  taskListStateRef.current = {
    page: taskPage,
    pageSize: taskPageSize,
    statusFilter: taskStatusFilter,
    modelFilter: taskModelFilter,
    timeFilter: taskTimeFilter,
    sortBy: taskSortBy,
    sortOrder: taskSortOrder,
  };
  pollingIntervalRef.current = pollingIntervalSeconds;
  useEffect(() => {
    const latestCompletedTime = tasks.reduce((latest, task) => {
      const completedAt = Number(task?.completed_time) || 0;
      return completedAt > latest ? completedAt : latest;
    }, 0);
    if (latestCompletedTime > taskUpdatesCompletedSinceRef.current) {
      taskUpdatesCompletedSinceRef.current = latestCompletedTime;
    }
  }, [tasks]);

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

  const getModelDisplayName = (model) =>
    model?.display_name || model?.request_model || '';

  const modelMatchesCatalogFilters = (model) => {
    if (!model) {
      return false;
    }
    if (
      catalogSeriesFilter !== MODEL_CATALOG_ALL_SERIES &&
      model.model_series !== catalogSeriesFilter
    ) {
      return false;
    }
    const keyword = modelSearchKeyword.trim().toLowerCase();
    if (!keyword) {
      return true;
    }
    return [
      model.display_name,
      model.request_model,
      model.model_series,
      model.request_endpoint,
    ]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(keyword));
  };

  const imageCatalogModels = useMemo(
    () =>
      models
        .filter(
          (model) =>
            model.status === undefined ||
            model.status === null ||
            model.status === 1,
        )
        .filter(modelMatchesCatalogFilters),
    [models, catalogSeriesFilter, modelSearchKeyword],
  );

  const videoCatalogModels = useMemo(
    () =>
      videoModels
        .filter(
          (model) =>
            model.status === undefined ||
            model.status === null ||
            model.status === 1,
        )
        .filter(modelMatchesCatalogFilters),
    [videoModels, catalogSeriesFilter, modelSearchKeyword],
  );

  const catalogSeriesOptions = useMemo(
    () =>
      Array.from(
        new Set(
          [...modelSeries, ...videoModelSeries].filter(
            (series) => typeof series === 'string' && series.trim() !== '',
          ),
        ),
      ),
    [modelSeries, videoModelSeries],
  );

  const selectImageModelFromCatalog = (model) => {
    if (!model?.request_model) {
      return;
    }
    setGenerationMode('image');
    if (model.model_series) {
      setSelectedSeries(model.model_series);
    }
    setSelectedModel(model.request_model);
    setMobileCatalogVisible(false);
  };

  const selectVideoModelFromCatalog = (model) => {
    if (!model?.request_model) {
      return;
    }
    setGenerationMode('video');
    if (model.model_series) {
      setVideoSelectedSeries(model.model_series);
    }
    setVideoSelectedModel(model.request_model);
    setMobileCatalogVisible(false);
  };

  const buildRemoteReferenceFile = (imageUrl) => {
    if (!imageUrl) {
      return null;
    }
    return {
      uid: `remote-${Date.now()}`,
      name: 'reference-image',
      status: 'done',
      url: imageUrl,
      isRemote: true,
    };
  };

  useEffect(() => {
    loadImageGenerationGroups();
    loadVideoModels();
    loadWorkerSettings();
    connectSSE();

    return () => {
      disconnectSSE();
      stopPolling();
    };
  }, []);

  useEffect(() => {
    if (!pollingTimerRef.current) {
      return;
    }
    stopPolling();
    startPolling();
  }, [pollingIntervalSeconds]);

  useEffect(() => {
    const handleVisibilityChange = () => {
      setIsPageVisible(!document.hidden);
    };

    handleVisibilityChange();
    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, []);

  useEffect(() => {
    const prefillFromState = location.state?.canvasPrefill;
    if (prefillFromState) {
      pendingCanvasPrefillRef.current = prefillFromState;
      prefillGroupFallbackNoticeShownRef.current = false;
      try {
        sessionStorage.setItem(
          CANVAS_PREFILL_STORAGE_KEY,
          JSON.stringify({
            source: 'inspiration',
            payload: prefillFromState,
          }),
        );
      } catch (error) {
        console.error('Failed to persist canvas prefill state:', error);
      }
      navigate(location.pathname, { replace: true, state: null });
      return;
    }
    try {
      const storedPrefill = sessionStorage.getItem(CANVAS_PREFILL_STORAGE_KEY);
      if (storedPrefill) {
        const parsed = JSON.parse(storedPrefill);
        if (parsed?.source === 'inspiration' && parsed?.payload) {
          pendingCanvasPrefillRef.current = parsed.payload;
          prefillGroupFallbackNoticeShownRef.current = false;
        } else {
          sessionStorage.removeItem(CANVAS_PREFILL_STORAGE_KEY);
        }
      }
    } catch (error) {
      console.error('Failed to restore canvas prefill state:', error);
    }
  }, [location.pathname, location.state, navigate]);

  useEffect(() => {
    const searchParams = new URLSearchParams(location.search);
    const taskId = searchParams.get('task_id');
    const mode = searchParams.get('mode');
    if (!taskId) return;

    const loadTaskDetail = async () => {
      try {
        if (mode === 'video') {
          const res = await API.get(`/api/video-generation/tasks/${taskId}`);
          if (res.data.success) {
            setVideoSelectedTask(res.data.data);
            setVideoTaskModalVisible(true);
          } else {
            showError(res.data.message || t('加载任务详情失败'));
          }
          return;
        }
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
    if (generationMode === 'video') {
      loadVideoTasks();
      return;
    }
    loadTasks();
  }, [
    generationMode,
    taskPage,
    taskPageSize,
    taskStatusFilter,
    taskModelFilter,
    taskTimeFilter,
    taskSortBy,
    taskSortOrder,
    videoTaskPage,
    videoTaskPageSize,
    videoTaskStatusFilter,
    videoTaskModelFilter,
    videoTaskTimeFilter,
  ]);

  // 切换任意筛选/排序时回到第一页
  useEffect(() => {
    setTaskPage(1);
    setTaskHasMore(false);
    setTaskNextCursor('');
    taskCursorHistoryRef.current = [''];
  }, [
    taskStatusFilter,
    taskModelFilter,
    taskTimeFilter,
    taskSortBy,
    taskSortOrder,
  ]);

  useEffect(() => {
    setVideoTaskPage(1);
  }, [videoTaskStatusFilter, videoTaskModelFilter, videoTaskTimeFilter]);

  const loadImageGenerationGroups = async () => {
    setGroupLoading(true);
    try {
      const res = await API.get('/api/image-generation/groups');
      if (res.data.success) {
        const payload = res.data.data || {};
        const items = Array.isArray(payload.items) ? payload.items : [];
        const defaultGroup = payload.default_group || '';
        setGroupOptions(items);
        const availableGroups = items
          .filter((item) => item.group && item.has_available_token !== false)
          .map((item) => item.group);
        let nextGroup = getStoredValue(STORAGE_KEYS.GROUP, '');
        if (!nextGroup || !availableGroups.includes(nextGroup)) {
          const explicitDefault = items.find(
            (item) => item.is_default && item.has_available_token !== false,
          )?.group;
          nextGroup =
            explicitDefault ||
            (availableGroups.includes(defaultGroup) ? defaultGroup : '') ||
            availableGroups[0] ||
            '';
        }
        setSelectedGroup(nextGroup);
      } else {
        showError(res.data.message || t('加载分组失败'));
      }
    } catch (error) {
      showError(error.message || t('加载分组失败'));
    } finally {
      setGroupLoading(false);
    }
  };

  const loadDrawingModels = async (group) => {
    drawingModelsRequestSeqRef.current += 1;
    const requestSeq = drawingModelsRequestSeqRef.current;
    setLoading(true);
    try {
      const res = await API.get('/api/image-generation/models', {
        params: group ? { group } : undefined,
      });
      if (requestSeq !== drawingModelsRequestSeqRef.current) {
        return;
      }
      if (res.data.success) {
        const drawingModels = (res.data.data || []).map((model) => ({
          ...model,
          image_capabilities: normalizeImageCapabilities(
            model.image_capabilities,
          ),
        }));
        loadedModelsGroupRef.current = group || '';
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
      if (requestSeq !== drawingModelsRequestSeqRef.current) {
        return;
      }
      showError(error.message || t('加载模型失败'));
    } finally {
      if (requestSeq === drawingModelsRequestSeqRef.current) {
        setLoading(false);
      }
    }
  };

  useEffect(() => {
    if (!selectedGroup) {
      return;
    }
    loadDrawingModels(selectedGroup);
  }, [selectedGroup]);

  const loadVideoModels = async () => {
    try {
      const res = await API.get('/api/video-generation/models');
      if (!res.data.success) {
        showError(res.data.message || t('加载视频模型失败'));
        return;
      }
      const items = res.data.data || [];
      setVideoModels(
        items.map((item) => ({
          ...item,
          video_capabilities: normalizeVideoCapabilities(
            item.video_capabilities,
          ),
        })),
      );
      const series = Array.from(
        new Set(items.map((item) => item.model_series).filter(Boolean)),
      );
      setVideoModelSeries(series);
      setVideoSelectedSeries((prev) => {
        if (prev === 'all' || (prev && series.includes(prev))) {
          return prev;
        }
        return series[0] || 'all';
      });
    } catch (error) {
      showError(error.message || t('加载视频模型失败'));
    }
  };

  useEffect(() => {
    if (!videoSelectedModelData) {
      return;
    }
    if (
      !modelSupportsVideoCapability(
        videoSelectedModelData,
        VIDEO_CAPABILITY_IMAGE_TO_VIDEO,
      )
    ) {
      setVideoReferenceImage(null);
    }
  }, [videoSelectedModelData]);

  const loadWorkerSettings = async () => {
    try {
      const res = await API.get('/api/image-generation/settings');
      if (res.data.success && res.data.data) {
        const size = parseInt(res.data.data.max_image_size, 10);
        if (!isNaN(size) && size > 0) {
          setMaxImageSize(size);
        }
        const pollingInterval = parseInt(res.data.data.polling_interval, 10);
        if (!isNaN(pollingInterval) && pollingInterval > 0) {
          setPollingIntervalSeconds(pollingInterval);
        } else {
          setPollingIntervalSeconds(DEFAULT_POLLING_INTERVAL_SECONDS);
        }
        setUserCustomWorkerKeyEnabled(
          res.data.data.user_custom_key_enabled === true,
        );
        setUserCustomWorkerBaseUrlAllowed(
          res.data.data.user_custom_base_url_allowed === true,
        );
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

  // silent=true 时静默刷新（轮询），不会触发全局阻塞遮罩。
  // disableDuplicate=true 用于强制绕过 API 层的 in-flight GET 去重。
  const loadTasks = async (silent = false, options = {}) => {
    const { disableDuplicate = false, forceRefresh = false } = options;
    const queryState = taskListStateRef.current || {
      page: taskPage,
      pageSize: taskPageSize,
      statusFilter: taskStatusFilter,
      modelFilter: taskModelFilter,
      timeFilter: taskTimeFilter,
      sortBy: taskSortBy,
      sortOrder: taskSortOrder,
    };
    const useCursorPagination = taskCursorPaginationSupported(queryState);
    const shouldAdvanceSeq = !silent || forceRefresh;
    const requestSeq = shouldAdvanceSeq
      ? taskListRequestSeqRef.current + 1
      : taskListRequestSeqRef.current;
    if (shouldAdvanceSeq) {
      taskListRequestSeqRef.current = requestSeq;
    }
    if (!silent) setLoadingTasks(true);
    try {
      const params = {
        p: queryState.page,
        page_size: queryState.pageSize,
      };
      if (useCursorPagination) {
        const cursorHistory = taskCursorHistoryRef.current;
        params.cursor = cursorHistory[queryState.page - 1] || '';
      }
      if (queryState.statusFilter) {
        params.status = queryState.statusFilter;
      }
      if (queryState.modelFilter) {
        params.model_id = queryState.modelFilter;
      }
      const { start, end } = computeTimeRange(queryState.timeFilter);
      if (start > 0) {
        params.start_time = start;
        params.end_time = end;
      }
      if (queryState.sortBy) {
        params.sort_by = queryState.sortBy;
      }
      if (queryState.sortOrder) {
        params.sort_order = queryState.sortOrder;
      }

      const res = await API.get('/api/image-generation/tasks', {
        params,
        timeout: TASK_LIST_REQUEST_TIMEOUT_MS,
        skipErrorHandler: true,
        disableDuplicate,
      });
      if (requestSeq !== taskListRequestSeqRef.current) {
        return;
      }
      if (res.data.success) {
        const newItems = res.data.data.items || [];
        const hasTotal = Number.isFinite(res.data.data.total);
        const newTotal = hasTotal ? res.data.data.total : taskTotal;
        // 智能合并：仅当内容实际变化时才更新，避免全量替换导致卡片无效重渲染
        setTasks((prev) => {
          if (prev.length === newItems.length) {
            const unchanged = newItems.every((newTask, i) => {
              const old = prev[i];
              return areTasksVisuallyEquivalent(old, newTask);
            });
            if (unchanged) return prev;
          }
          return newItems;
        });
        if (hasTotal) {
          setTaskTotal(newTotal);
        }
        if (useCursorPagination) {
          const nextCursor = res.data.data.next_cursor || '';
          const cursorHistory = taskCursorHistoryRef.current.slice(
            0,
            queryState.page,
          );
          if (nextCursor) {
            cursorHistory[queryState.page] = nextCursor;
          }
          taskCursorHistoryRef.current = cursorHistory;
          setTaskNextCursor(nextCursor);
        } else {
          setTaskNextCursor('');
        }
        setTaskHasMore(res.data.data.has_more === true);
      } else if (!silent) {
        showError(res.data.message || t('加载任务列表失败'));
      }
    } catch (error) {
      if (requestSeq === taskListRequestSeqRef.current && !silent) {
        showError(error.message || t('加载任务列表失败'));
      }
    } finally {
      if (!silent && requestSeq === taskListRequestSeqRef.current) {
        setLoadingTasks(false);
      }
    }
  };

  const loadVideoTasks = async () => {
    setVideoLoadingTasks(true);
    try {
      const params = {
        p: videoTaskPage,
        page_size: videoTaskPageSize,
      };
      if (videoTaskStatusFilter) {
        params.status = videoTaskStatusFilter;
      }
      if (videoTaskModelFilter) {
        params.model_id = videoTaskModelFilter;
      }
      const { start, end } = computeTimeRange(videoTaskTimeFilter);
      if (start > 0) {
        params.start_time = start;
        params.end_time = end;
      }
      const res = await API.get('/api/video-generation/tasks', { params });
      if (res.data.success) {
        setVideoTasks(res.data.data.items || []);
        setVideoTaskTotal(res.data.data.total || 0);
      } else {
        showError(res.data.message || t('加载视频任务列表失败'));
      }
    } catch (error) {
      showError(error.message || t('加载视频任务列表失败'));
    } finally {
      setVideoLoadingTasks(false);
    }
  };

  const mergeTaskUpdates = (updates) => {
    if (!Array.isArray(updates) || updates.length === 0) {
      return;
    }
    setTasks((prevTasks) =>
      mergeTaskCollections(prevTasks, updates, taskPageSize),
    );
  };

  const loadTaskUpdates = async () => {
    if (!isDefaultTaskViewState(taskListStateRef.current)) {
      return;
    }
    const completedSince = Math.max(
      1,
      Number(taskUpdatesCompletedSinceRef.current) || 0,
    );

    try {
      const res = await API.get('/api/image-generation/tasks/updates', {
        params: {
          completed_since: completedSince,
          limit: Math.max(
            (taskListStateRef.current?.pageSize || taskPageSize) * 2,
            50,
          ),
        },
        timeout: TASK_LIST_REQUEST_TIMEOUT_MS,
        skipErrorHandler: true,
      });
      if (!res.data.success) {
        return;
      }
      const latestCompletedTime = (res.data.data?.items || []).reduce(
        (latest, task) => {
          const completedAt = Number(task?.completed_time) || 0;
          return completedAt > latest ? completedAt : latest;
        },
        completedSince,
      );
      if (latestCompletedTime > taskUpdatesCompletedSinceRef.current) {
        taskUpdatesCompletedSinceRef.current = latestCompletedTime;
      }
      mergeTaskUpdates(res.data.data?.items || []);
    } catch (error) {
      console.error('Failed to load task updates:', error);
    }
  };

  const handleTaskCardClick = async (task) => {
    if (!task?.id) return;

    taskDetailRequestSeqRef.current += 1;
    const requestSeq = taskDetailRequestSeqRef.current;

    setSelectedTask(task);
    setTaskModalVisible(true);

    try {
      const res = await API.get(`/api/image-generation/tasks/${task.id}`);
      if (requestSeq !== taskDetailRequestSeqRef.current) {
        return;
      }
      if (res.data.success) {
        updateTaskInList(res.data.data);
      } else {
        showError(res.data.message || t('加载任务详情失败'));
      }
    } catch (error) {
      if (requestSeq !== taskDetailRequestSeqRef.current) {
        return;
      }
      showError(error.message || t('加载任务详情失败'));
    }
  };

  const handleVideoTaskCardClick = async (task) => {
    if (!task?.id) return;
    setVideoSelectedTask(task);
    setVideoTaskModalVisible(true);
    try {
      const res = await API.get(`/api/video-generation/tasks/${task.id}`);
      if (res.data.success) {
        updateVideoTaskInList(res.data.data);
      } else {
        showError(res.data.message || t('加载视频任务详情失败'));
      }
    } catch (error) {
      showError(error.message || t('加载视频任务详情失败'));
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

  const handleVideoTaskSelect = (taskId, checked) => {
    setVideoSelectedTaskIds((prev) => {
      const next = new Set(prev);
      if (checked) {
        next.add(taskId);
      } else {
        next.delete(taskId);
      }
      return next;
    });
  };

  const handleTaskPageSizeChange = (nextPageSize) => {
    setTaskPage(1);
    setTaskPageSize(nextPageSize);
    setTaskHasMore(false);
    setTaskNextCursor('');
    taskCursorHistoryRef.current = [''];
  };

  const handleVideoTaskPageSizeChange = (nextPageSize) => {
    setVideoTaskPage(1);
    setVideoTaskPageSize(nextPageSize);
  };

  // 全选/取消全选
  const handleSelectAll = (checked) => {
    if (checked) {
      setSelectedTaskIds(new Set(tasks.map((t) => t.id)));
    } else {
      setSelectedTaskIds(new Set());
    }
  };

  const handleSelectAllVideo = (checked) => {
    if (checked) {
      setVideoSelectedTaskIds(new Set(videoTasks.map((task) => task.id)));
    } else {
      setVideoSelectedTaskIds(new Set());
    }
  };

  // 批量删除任务
  const handleBatchDelete = async () => {
    const selectedTasks = tasks.filter((task) => selectedTaskIds.has(task.id));
    const undeletableTasks = selectedTasks.filter(
      (task) => !taskIsDeletable(task),
    );
    if (selectedTaskIds.size === 0) {
      showError(t('请先选择要删除的任务'));
      return;
    }
    if (undeletableTasks.length > 0) {
      showError(t('运行中的任务暂不支持删除，请等待完成后再删除'));
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
        const deletedTaskIds = new Set(
          Array.from(selectedTaskIds).filter(
            (taskId, index) => results[index]?.status === 'fulfilled',
          ),
        );
        setTasks((prevTasks) =>
          prevTasks.filter((task) => !deletedTaskIds.has(task.id)),
        );
        setTaskTotal((prev) => Math.max(0, prev - successCount));
        if (selectedTask && deletedTaskIds.has(selectedTask.id)) {
          taskDetailRequestSeqRef.current += 1;
          setTaskModalVisible(false);
          setSelectedTask(null);
        }
        setSelectedTaskIds(new Set());
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

  const handleVideoBatchDelete = async () => {
    if (videoSelectedTaskIds.size === 0) {
      showError(t('请先选择要删除的任务'));
      return;
    }
    const selectedVideoTasks = videoTasks.filter((task) =>
      videoSelectedTaskIds.has(task.id),
    );
    if (
      selectedVideoTasks.some(
        (task) => task.status === 'queued' || task.status === 'in_progress',
      )
    ) {
      showError(t('运行中的任务暂不支持删除，请等待完成后再删除'));
      return;
    }
    setDeletingVideoTasks(true);
    try {
      const results = await Promise.allSettled(
        Array.from(videoSelectedTaskIds).map((taskId) =>
          API.delete(`/api/video-generation/tasks/${taskId}`),
        ),
      );
      const successIds = new Set(
        Array.from(videoSelectedTaskIds).filter(
          (taskId, index) => results[index]?.status === 'fulfilled',
        ),
      );
      if (successIds.size > 0) {
        showSuccess(t('成功删除 {{count}} 个任务', { count: successIds.size }));
        setVideoTasks((prev) =>
          prev.filter((task) => !successIds.has(task.id)),
        );
        setVideoTaskTotal((prev) => Math.max(0, prev - successIds.size));
        setVideoSelectedTaskIds(new Set());
      }
    } catch (error) {
      showError(error.message || t('批量删除失败'));
    } finally {
      setDeletingVideoTasks(false);
    }
  };

  // 连接 SSE
  const connectSSE = () => {
    try {
      const eventSource = new EventSource('/api/image-generation/sse', {
        withCredentials: true,
      });

      eventSource.onopen = () => {
        setSseConnected(true);
      };

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
        sseRef.current = null;
        setSseConnected(false);
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
    setSseConnected(false);
  };

  // 开始轮询
  const startPolling = () => {
    if (pollingTimerRef.current) return;

    pollingTimerRef.current = setInterval(() => {
      if (generationMode === 'video') {
        loadVideoTasks();
        return;
      }
      if (isDefaultTaskViewState(taskListStateRef.current)) {
        loadTaskUpdates();
        return;
      }
      loadTasks(true); // 静默刷新，不触发 Spin 遮罩
    }, pollingIntervalRef.current * 1000);
  };

  // ���止轮询
  const stopPolling = () => {
    if (pollingTimerRef.current) {
      clearInterval(pollingTimerRef.current);
      pollingTimerRef.current = null;
    }
  };

  useEffect(() => {
    const shouldPoll =
      !sseConnected &&
      isPageVisible &&
      (generationMode === 'video' ? hasActiveVideoTasks : hasActiveTasks);
    if (!shouldPoll) {
      stopPolling();
      return undefined;
    }

    startPolling();
    return () => stopPolling();
  }, [
    generationMode,
    hasActiveTasks,
    hasActiveVideoTasks,
    isPageVisible,
    pollingIntervalSeconds,
    sseConnected,
  ]);

  // 更新任务列表中的单个任务
  const updateTaskInList = (updatedTask) => {
    setTasks((prevTasks) => {
      const index = prevTasks.findIndex((t) => t.id === updatedTask.id);
      if (index !== -1) {
        const newTasks = [...prevTasks];
        newTasks[index] = {
          ...newTasks[index],
          ...updatedTask,
        };
        return newTasks;
      }
      if (!isDefaultTaskViewState(taskListStateRef.current)) {
        return prevTasks;
      }
      return mergeTaskCollections(prevTasks, [updatedTask], taskPageSize);
    });
    setSelectedTask((prevTask) => {
      if (!prevTask || prevTask.id !== updatedTask.id) {
        return prevTask;
      }
      return {
        ...prevTask,
        ...updatedTask,
      };
    });
  };

  const updateVideoTaskInList = (updatedTask) => {
    setVideoTasks((prev) => {
      const index = prev.findIndex((task) => task.id === updatedTask.id);
      if (index === -1) {
        if (
          videoTaskPage === 1 &&
          !videoTaskStatusFilter &&
          !videoTaskModelFilter &&
          !videoTaskTimeFilter
        ) {
          return [updatedTask, ...prev].slice(0, videoTaskPageSize);
        }
        return prev;
      }
      const next = [...prev];
      next[index] = {
        ...next[index],
        ...updatedTask,
      };
      return next;
    });
    if (
      videoTaskPage === 1 &&
      !videoTaskStatusFilter &&
      !videoTaskModelFilter &&
      !videoTaskTimeFilter &&
      !videoTasks.some((task) => task.id === updatedTask.id)
    ) {
      setVideoTaskTotal((prev) => prev + 1);
    }
    setVideoSelectedTask((prev) => {
      if (!prev || prev.id !== updatedTask.id) {
        return prev;
      }
      return {
        ...prev,
        ...updatedTask,
      };
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

  useEffect(() => {
    const prefill = pendingCanvasPrefillRef.current;
    if (!prefill || groupLoading) {
      return;
    }

    if (prefill.selected_group && prefill.selected_group !== selectedGroup) {
      const canUsePrefillGroup = groupOptions.some(
        (group) =>
          group.group === prefill.selected_group &&
          group.has_available_token !== false,
      );
      if (canUsePrefillGroup) {
        setSelectedGroup(prefill.selected_group);
        return;
      }
      if (
        prefill.selected_group &&
        !prefillGroupFallbackNoticeShownRef.current
      ) {
        showSuccess(t('原作品分组当前不可用，已切换到默认可用分组'));
        prefillGroupFallbackNoticeShownRef.current = true;
      }
    }

    if (models.length === 0) {
      return;
    }
    if (loadedModelsGroupRef.current !== selectedGroup) {
      return;
    }

    const params = (() => {
      if (!prefill.params) {
        return {};
      }
      if (typeof prefill.params === 'object') {
        return prefill.params;
      }
      try {
        return JSON.parse(prefill.params);
      } catch (error) {
        return {};
      }
    })();

    if (prefill.prompt) {
      setInspiration(prefill.prompt);
    }
    if (params.aspect_ratio) {
      setAspectRatio(params.aspect_ratio);
    }
    if (params.resolution || params.image_size || params.imageSize) {
      setResolution(params.resolution || params.image_size || params.imageSize);
    }
    if (params.n || params.quantity) {
      setQuantity(normalizeTaskCount(params.n || params.quantity));
    }
    if (prefill.mode === 'reference' && prefill.image_url) {
      const remoteReference = buildRemoteReferenceFile(prefill.image_url);
      if (remoteReference) {
        setReferenceImages([remoteReference]);
      }
    }

    const syncSeriesForModel = (model) => {
      if (model?.model_series) {
        setSelectedSeries(model.model_series);
      }
    };

    const supportsGeneration = (model) =>
      !!model && modelSupportsCapability(model, IMAGE_CAPABILITY_GENERATION);
    const supportsEditing = (model) =>
      !!model && modelSupportsCapability(model, IMAGE_CAPABILITY_EDITING);
    const exactModel = models.find(
      (model) => model.request_model === prefill.model_id,
    );
    if (prefill.mode === 'reference') {
      if (exactModel && supportsEditing(exactModel)) {
        setSelectedModel(exactModel.request_model);
        syncSeriesForModel(exactModel);
      } else {
        const fallbackEditModel = models.find((model) =>
          supportsEditing(model),
        );
        if (fallbackEditModel) {
          setSelectedModel(fallbackEditModel.request_model);
          syncSeriesForModel(fallbackEditModel);
          showSuccess(
            t('原作品模型当前不可用于参考图继续创作，已切换到可编辑模型'),
          );
        } else {
          showError(t('当前分组下没有支持参考图编辑的模型，请切换分组或模型'));
        }
      }
    } else if (exactModel && supportsGeneration(exactModel)) {
      setSelectedModel(exactModel.request_model);
      syncSeriesForModel(exactModel);
    } else {
      const fallbackGenerationModel = models.find((model) =>
        supportsGeneration(model),
      );
      if (fallbackGenerationModel) {
        setSelectedModel(fallbackGenerationModel.request_model);
        syncSeriesForModel(fallbackGenerationModel);
        showSuccess(t('原作品模型当前不可用，已切换到当前分组的可用模型'));
      } else {
        showError(t('当前分组下没有支持生图的模型，请切换分组或模型'));
      }
    }

    pendingCanvasPrefillRef.current = null;
    prefillGroupFallbackNoticeShownRef.current = false;
    try {
      sessionStorage.removeItem(CANVAS_PREFILL_STORAGE_KEY);
    } catch (error) {
      console.error('Failed to clear canvas prefill state:', error);
    }
  }, [groupLoading, groupOptions, models, selectedGroup, t]);

  useEffect(() => {
    if (videoModels.length === 0) return;
    const filtered =
      videoSelectedSeries === 'all'
        ? videoModels
        : videoModels.filter(
            (model) => model.model_series === videoSelectedSeries,
          );
    setVideoFilteredModels(filtered);
    setVideoSelectedModel((current) => {
      if (filtered.some((model) => model.request_model === current)) {
        return current;
      }
      return filtered[0]?.request_model || '';
    });
  }, [videoSelectedSeries, videoModels]);

  // 保存用户选择到 localStorage
  useEffect(() => {
    if (selectedGroup) {
      try {
        localStorage.setItem(STORAGE_KEYS.GROUP, selectedGroup);
      } catch (e) {
        console.error('Failed to save selectedGroup to localStorage:', e);
      }
    }
  }, [selectedGroup]);

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
    try {
      localStorage.setItem(STORAGE_KEYS.MODE, generationMode);
    } catch (e) {
      console.error('Failed to save generation mode:', e);
    }
  }, [generationMode]);

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
    if (videoSelectedSeries) {
      try {
        localStorage.setItem(STORAGE_KEYS.VIDEO_SERIES, videoSelectedSeries);
      } catch (e) {
        console.error('Failed to save videoSelectedSeries to localStorage:', e);
      }
    }
  }, [videoSelectedSeries]);

  useEffect(() => {
    if (videoSelectedModel) {
      try {
        localStorage.setItem(STORAGE_KEYS.VIDEO_MODEL, videoSelectedModel);
      } catch (e) {
        console.error('Failed to save videoSelectedModel to localStorage:', e);
      }
    }
  }, [videoSelectedModel]);

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
    if (videoAspectRatio) {
      try {
        localStorage.setItem(STORAGE_KEYS.VIDEO_ASPECT_RATIO, videoAspectRatio);
      } catch (e) {
        console.error('Failed to save videoAspectRatio:', e);
      }
    }
  }, [videoAspectRatio]);

  useEffect(() => {
    if (videoResolution) {
      try {
        localStorage.setItem(STORAGE_KEYS.VIDEO_RESOLUTION, videoResolution);
      } catch (e) {
        console.error('Failed to save videoResolution:', e);
      }
    }
  }, [videoResolution]);

  useEffect(() => {
    try {
      localStorage.setItem(
        STORAGE_KEYS.VIDEO_DURATION,
        String(videoDuration || 0),
      );
    } catch (e) {
      console.error('Failed to save videoDuration:', e);
    }
  }, [videoDuration]);

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

  useEffect(() => {
    if (!videoSelectedModel) {
      setVideoSelectedModelData(null);
      setVideoAvailableAspectRatios([]);
      setVideoAvailableResolutions([]);
      return;
    }
    const model = videoModels.find(
      (item) => item.request_model === videoSelectedModel,
    );
    if (!model) {
      return;
    }
    setVideoSelectedModelData(model);
    setVideoAvailableAspectRatios(model.aspect_ratios || []);
    setVideoAvailableResolutions(model.resolutions || []);
    setVideoAspectRatio((current) => {
      if (!current || !(model.aspect_ratios || []).includes(current)) {
        return model.aspect_ratios?.[0] || '';
      }
      return current;
    });
    setVideoResolution((current) => {
      if (!current || !(model.resolutions || []).includes(current)) {
        return model.resolutions?.[0] || '';
      }
      return current;
    });
    setVideoDuration((current) => {
      if (!current || !(model.duration_options || []).includes(current)) {
        return model.duration_options?.[0] || 0;
      }
      return current;
    });
  }, [videoSelectedModel, videoModels]);

  useEffect(() => {
    if (
      !selectedModelData ||
      !modelSupportsImageEditing(selectedModelData)
    ) {
      setReferenceImages([]);
      setMaskImage(null);
    }
    if (!selectedModelData || !modelSupportsMaskEditing(selectedModelData)) {
      setMaskImage(null);
    }
  }, [selectedModelData]);

  useEffect(() => {
    const limit = getReferenceImageLimit(selectedModelData);
    if (limit <= 0 || referenceImages.length <= limit) {
      return;
    }

    setReferenceImages((prev) => prev.slice(0, limit));
    showError(
      t('当前模型最多只能上传 {{count}} 张参考图', {
        count: limit,
      }),
    );
  }, [selectedModelData, referenceImages.length, t]);

  useEffect(() => {
    if (referenceImages.length === 0) {
      setMaskImage(null);
    }
  }, [referenceImages]);

  const handleImageUpload = ({ fileList }) => {
    const limit = getReferenceImageLimit(selectedModelData);
    if (limit > 0 && fileList.length > limit) {
      showError(
        t('当前模型最多只能上传 {{count}} 张参考图', {
          count: limit,
        }),
      );
      return;
    }
    setReferenceImages(fileList);
  };

  const handleMaskUpload = ({ fileList }) => {
    setMaskImage(fileList[0] || null);
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

  const handleMaskRemove = () => {
    setMaskImage(null);
  };

  const handleVideoReferenceUpload = ({ fileList }) => {
    setVideoReferenceImage(fileList[0] || null);
  };

  const handleVideoReferenceRemove = () => {
    setVideoReferenceImage(null);
  };

  const normalizeTaskCount = (value) => {
    const count = Number(value);
    if (!Number.isFinite(count)) {
      return 1;
    }
    return Math.min(DEFAULT_MAX_BATCH_TASKS, Math.max(1, Math.floor(count)));
  };

  const handleGenerate = async () => {
    const supportsImageGeneration = modelSupportsCapability(
      selectedModelData,
      IMAGE_CAPABILITY_GENERATION,
    );
    const supportsImageEditing = modelSupportsCapability(
      selectedModelData,
      IMAGE_CAPABILITY_EDITING,
    );

    if (!selectedModel) {
      showError(t('请选择模型'));
      return;
    }
    if (!selectedGroup) {
      showError(t('请选择分组'));
      return;
    }
    const selectedGroupOption = groupOptions.find(
      (item) => item.group === selectedGroup,
    );
    if (
      selectedGroupOption &&
      selectedGroupOption.has_available_token === false
    ) {
      showError(
        t('当前分组没有可用令牌，请前往 /console/token 创建或启用该分组令牌'),
      );
      return;
    }
    if (!inspiration.trim()) {
      showError(t('请输入灵感'));
      return;
    }
    if (!selectedModelData?.request_endpoint) {
      showError(t('模型配置错误：缺少 request_endpoint'));
      return;
    }
    if (showImageAspectRatioSelector && !aspectRatio) {
      showError(t('请选择宽高比'));
      return;
    }
    if (showImageResolutionSelector && !resolution) {
      showError(t('请选择分辨率'));
      return;
    }
    if (referenceImages.length > 0 && !supportsImageEditing) {
      showError(t('当前模型不支持图像编辑'));
      return;
    }
    const referenceImageLimit = getReferenceImageLimit(selectedModelData);
    if (
      referenceImageLimit > 0 &&
      referenceImages.length > referenceImageLimit
    ) {
      showError(
        t('当前模型最多只能上传 {{count}} 张参考图', {
          count: referenceImageLimit,
        }),
      );
      return;
    }
    if (maskImage && referenceImages.length === 0) {
      showError(t('请先上传参考图再添加遮罩'));
      return;
    }
    if (!supportsImageGeneration && referenceImages.length === 0) {
      showError(t('当前模型至少需要上传一张参考图'));
      return;
    }

    setGenerating(true);
    try {
      const taskCount = normalizeTaskCount(quantity);
      if (taskCount > DEFAULT_MAX_BATCH_TASKS) {
        showError(
          t('单次最多可创建 {{count}} 个任务', {
            count: DEFAULT_MAX_BATCH_TASKS,
          }),
        );
        return;
      }

      // 准备参数对象
      const params = {};

      if (aspectRatio) {
        params.aspect_ratio = aspectRatio;
      }
      if (resolution) {
        params.resolution = resolution;
      }

      // 处理参考图片
      if (supportsImageEditing && referenceImages.length > 0) {
        const imagePromises = referenceImages.map((file) => {
          if (!file.fileInstance && file.url) {
            return Promise.resolve(file.url);
          }
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
      if (modelSupportsMaskEditing(selectedModelData) && maskImage?.fileInstance) {
        params.mask = await new Promise((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = (e) => resolve(e.target.result);
          reader.onerror = reject;
          reader.readAsDataURL(maskImage.fileInstance);
        });
      }

      // UI uses inspiration wording; backend task DTO still expects prompt.
      const taskPayload = {
        model_id: selectedModel,
        group: selectedGroup,
        prompt: inspiration.trim(),
        request_endpoint: selectedModelData.request_endpoint,
        params: JSON.stringify(params),
      };

      const results = await Promise.allSettled(
        Array.from({ length: taskCount }, () =>
          API.post('/api/image-generation/tasks', taskPayload),
        ),
      );

      const createdTasks = [];
      let firstError = '';

      results.forEach((result) => {
        if (result.status === 'fulfilled' && result.value.data.success) {
          if (result.value.data.data) {
            createdTasks.push(result.value.data.data);
          }
          return;
        }

        if (!firstError) {
          firstError =
            result.status === 'fulfilled'
              ? result.value.data.message
              : result.reason?.response?.data?.message ||
                result.reason?.message;
        }
      });

      if (createdTasks.length > 0) {
        showSuccess(
          createdTasks.length === 1
            ? t('任务已创建，正在生成中...')
            : t('已创建 {{count}} 个任务，正在生成中...', {
                count: createdTasks.length,
              }),
        );
        const isDefaultTaskView =
          taskPage === 1 &&
          !taskStatusFilter &&
          !taskModelFilter &&
          !taskTimeFilter &&
          taskSortBy === 'created_time' &&
          taskSortOrder === 'desc';

        // 默认首页视图直接本地插入，避免一次整页回刷
        if (isDefaultTaskView) {
          setTasks((prevTasks) =>
            mergeTaskCollections(prevTasks, createdTasks, taskPageSize),
          );
          if (
            selectedTask &&
            createdTasks.some((task) => task.id === selectedTask.id)
          ) {
            setSelectedTask(
              createdTasks.find((task) => task.id === selectedTask.id) ||
                selectedTask,
            );
          }
        }
        setTaskTotal((prev) => prev + createdTasks.length);
        loadTasks(false, {
          disableDuplicate: true,
          forceRefresh: true,
        });
      }

      if (createdTasks.length === 0) {
        showError(firstError || t('创建任务失败'));
      } else if (createdTasks.length < taskCount) {
        showError(
          t('部分任务创建失败：已创建 {{success}} / {{total}} 个任务', {
            success: createdTasks.length,
            total: taskCount,
          }),
        );
      }
    } catch (error) {
      const serverMessage =
        error.response?.data?.message || error.message || '';
      if (
        serverMessage.includes('valid user token') ||
        serverMessage.includes('no valid token') ||
        serverMessage.includes('current group has no valid token')
      ) {
        showError(
          serverMessage.includes('current group has no valid token')
            ? t(
                '当前分组没有可用令牌，请前往 /console/token 创建或启用该分组令牌',
              )
            : userCustomWorkerKeyEnabled
              ? t('请先创建可用令牌，或检查你的自定义 Worker Key 是否可用')
              : t('请先创建一个可用令牌后再使用 /canvas'),
        );
      } else if (error.response?.data?.message) {
        showError(error.response.data.message);
      } else {
        showError(error.message || t('创建任务失败'));
      }
    } finally {
      setGenerating(false);
    }
  };

  const handleGenerateVideo = async () => {
    const supportsImageToVideo = modelSupportsVideoCapability(
      videoSelectedModelData,
      VIDEO_CAPABILITY_IMAGE_TO_VIDEO,
    );
    const supportsTextToVideo = modelSupportsVideoCapability(
      videoSelectedModelData,
      VIDEO_CAPABILITY_TEXT_TO_VIDEO,
    );

    if (!videoSelectedModel) {
      showError(t('请选择模型'));
      return;
    }
    if (!videoPrompt.trim()) {
      showError(t('请输入灵感'));
      return;
    }
    if (!videoSelectedModelData?.request_endpoint) {
      showError(t('模型配置错误：缺少 request_endpoint'));
      return;
    }
    if (!videoSelectedModelData?.duration_options?.length || !videoDuration) {
      showError(t('当前模型未配置可用时长选项'));
      return;
    }
    if (videoReferenceImage && !supportsImageToVideo) {
      showError(t('当前模型不支持图生视频'));
      return;
    }
    if (!videoReferenceImage && !supportsTextToVideo) {
      showError(t('当前模型需要上传一张参考图'));
      return;
    }

    setVideoGenerating(true);
    try {
      let base64Image = '';
      if (videoReferenceImage?.fileInstance) {
        base64Image = await new Promise((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = (e) => resolve(e.target.result);
          reader.onerror = reject;
          reader.readAsDataURL(videoReferenceImage.fileInstance);
        });
      }
      const params = {
        duration: videoDuration,
        resolution: videoResolution,
        aspect_ratio: videoAspectRatio,
      };
      if (base64Image) {
        params.reference_images = [base64Image];
      }
      const taskPayload = {
        model_id: videoSelectedModel,
        prompt: videoPrompt.trim(),
        request_endpoint: videoSelectedModelData.request_endpoint,
        params: JSON.stringify(params),
      };
      const res = await API.post('/api/video-generation/tasks', taskPayload);
      if (!res.data.success) {
        showError(res.data.message || t('创建视频任务失败'));
        return;
      }
      const newTask = res.data.data;
      showSuccess(t('视频任务已创建，正在生成中...'));
      if (
        videoTaskPage === 1 &&
        !videoTaskStatusFilter &&
        !videoTaskModelFilter &&
        !videoTaskTimeFilter
      ) {
        setVideoTasks((prev) => [newTask, ...prev].slice(0, videoTaskPageSize));
      } else {
        loadVideoTasks();
      }
      setVideoTaskTotal((prev) => prev + 1);
    } catch (error) {
      showError(
        error.response?.data?.message || error.message || t('创建视频任务失败'),
      );
    } finally {
      setVideoGenerating(false);
    }
  };

  const styles = {
    container: {
      display: 'flex',
      height: 'calc(100vh - 60px)',
      marginTop: 60,
      overflow: 'hidden',
      background: 'var(--semi-color-bg-1)',
    },
    leftPanel: {
      width: isMobile ? '100%' : 300,
      minWidth: isMobile ? 0 : 300,
      display: 'flex',
      flexDirection: 'column',
      borderRight: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
    },
    leftContent: {
      flex: 1,
      overflowY: 'auto',
      padding: 16,
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
      padding: isMobile ? '8px 12px' : '10px 20px',
      borderBottom: '1px solid var(--semi-color-border)',
      flexWrap: 'wrap',
      gap: 12,
      minHeight: 56,
      background: 'var(--semi-color-bg-0)',
    },
    rightContent: {
      flex: 1,
      minHeight: 0,
      overflow: 'hidden',
      display: 'flex',
      alignItems: 'stretch',
      justifyContent: 'stretch',
    },
    contentColumn: {
      flex: 1,
      minWidth: 0,
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
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
    catalogHeader: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 8,
      marginBottom: 14,
    },
    catalogTitle: {
      fontSize: 16,
      fontWeight: 650,
      color: 'var(--semi-color-text-0)',
    },
    catalogTools: {
      display: 'flex',
      flexDirection: 'column',
      gap: 10,
      marginBottom: 14,
    },
    modelSection: {
      marginBottom: 14,
    },
    modelSectionHeader: {
      width: '100%',
      minHeight: 36,
      border: 'none',
      background: 'transparent',
      padding: '6px 0',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      cursor: 'pointer',
      color: 'var(--semi-color-text-0)',
    },
    modelList: {
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
    },
    modelCard: {
      width: '100%',
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
      padding: '10px 12px',
      textAlign: 'left',
      cursor: 'pointer',
      transition: 'border-color 0.2s, background 0.2s',
    },
    modelCardActive: {
      borderColor: 'var(--semi-color-primary)',
      background: 'var(--semi-color-primary-light-default)',
    },
    modelCardTitle: {
      display: 'block',
      marginBottom: 5,
      color: 'var(--semi-color-text-0)',
      fontSize: 15,
      fontWeight: 650,
      lineHeight: 1.35,
    },
    modelCardMeta: {
      display: 'block',
      color: 'var(--semi-color-text-2)',
      fontSize: 12,
      lineHeight: 1.4,
      wordBreak: 'break-all',
    },
    composer: {
      flexShrink: 0,
      padding: isMobile ? '8px 10px 10px' : '12px 20px 14px',
      background: 'var(--semi-color-bg-1)',
      borderTop: '1px solid var(--semi-color-border)',
    },
    composerBox: {
      maxWidth: 1100,
      margin: '0 auto',
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
    },
    composerTop: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 10,
      padding: '0 2px',
    },
    selectedModelPill: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      minWidth: 0,
      color: 'var(--semi-color-text-0)',
      fontSize: 13,
      fontWeight: 600,
    },
    composerBody: {
      padding: 0,
    },
    textareaWrapper: {
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
      boxShadow: '0 10px 30px rgba(15, 23, 42, 0.08)',
      padding: isMobile ? '6px 8px 8px' : '8px 10px 10px',
    },
    composerInlineBar: {
      display: 'flex',
      alignItems: 'flex-end',
      justifyContent: 'space-between',
      gap: 8,
      flexWrap: 'wrap',
      paddingTop: 6,
      borderTop: '1px solid var(--semi-color-border)',
    },
    composerAssets: {
      display: 'flex',
      alignItems: 'center',
      gap: 6,
      flexWrap: 'wrap',
      minWidth: 0,
      flex: 1,
    },
    composerRightTools: {
      display: 'flex',
      alignItems: 'flex-end',
      gap: 6,
      flexWrap: 'wrap',
      justifyContent: 'flex-end',
      marginLeft: 'auto',
    },
    composerParams: {
      display: 'flex',
      gap: 6,
      alignItems: 'flex-end',
      flexWrap: 'wrap',
      justifyContent: 'flex-end',
    },
    generateIconBtn: {
      width: 36,
      height: 36,
      minWidth: 36,
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      cursor: 'pointer',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      transition: 'opacity 0.2s, background 0.2s, border-color 0.2s, color 0.2s',
    },
    addImageBtn: {
      width: 36,
      height: 36,
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
    emptyState: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 12,
      width: '100%',
      height: '100%',
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
    paramItem: {
      width: isMobile ? 88 : 104,
      flexShrink: 0,
    },
    paramLabel: {
      fontSize: 12,
      color: 'var(--semi-color-text-2)',
      marginBottom: 3,
      display: 'block',
    },
    referenceImageThumb: {
      width: 36,
      height: 36,
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
    mobileOnlyFilters: {
      display: isMobile ? 'flex' : 'none',
      alignItems: 'center',
      gap: 8,
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
    tasksGrid: {
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))',
      gap: 20,
      padding: isMobile ? 12 : 20,
      width: '100%',
      alignContent: 'start',
      flexShrink: 0,
    },
  };

  const selectedGroupHasAvailableToken = !!groupOptions.find(
    (group) =>
      group.group === selectedGroup && group.has_available_token !== false,
  );
  const imageUiState = getCanvasImageUiState({
    model: selectedModelData,
    prompt: inspiration,
    referenceImages,
    selectedGroupHasAvailableToken,
    showImageAspectRatioSelector,
    showImageResolutionSelector,
    aspectRatio,
    resolution,
  });
  const selectedModelSupportsEditing = imageUiState.supportsEditing;
  const selectedModelSupportsMaskEditing = imageUiState.supportsMaskEditing;
  const selectedModelReferenceImageLimit =
    getReferenceImageLimit(selectedModelData);
  const hasReferenceImageLimit = selectedModelReferenceImageLimit > 0;
  const referenceImageLimitReached =
    hasReferenceImageLimit &&
    referenceImages.length >= selectedModelReferenceImageLimit;
  const videoSelectedModelSupportsImageToVideo =
    !!videoSelectedModelData &&
    modelSupportsVideoCapability(
      videoSelectedModelData,
      VIDEO_CAPABILITY_IMAGE_TO_VIDEO,
    );
  const videoSelectedModelSupportsTextToVideo =
    !!videoSelectedModelData &&
    modelSupportsVideoCapability(
      videoSelectedModelData,
      VIDEO_CAPABILITY_TEXT_TO_VIDEO,
    );
  const canGenerate =
    !!selectedModel &&
    !!selectedModelData &&
    imageUiState.canGenerate;

  const canGenerateVideo =
    !!videoSelectedModel &&
    !!videoSelectedModelData &&
    !!videoPrompt.trim() &&
    !!videoSelectedModelData?.duration_options?.length &&
    ((!!videoReferenceImage && videoSelectedModelSupportsImageToVideo) ||
      (!videoReferenceImage && videoSelectedModelSupportsTextToVideo));

  const selectedGroupOption = groupOptions.find(
    (group) => group.group === selectedGroup,
  );

  const renderModelCard = (model, mode) => {
    const isVideo = mode === 'video';
    const isActive = isVideo
      ? videoSelectedModel === model.request_model
      : selectedModel === model.request_model;
    const handleClick = isVideo
      ? () => selectVideoModelFromCatalog(model)
      : () => selectImageModelFromCatalog(model);

    return (
      <button
        type='button'
        key={model.request_model}
        style={{
          ...styles.modelCard,
          ...(isActive ? styles.modelCardActive : null),
        }}
        onClick={handleClick}
      >
        <div style={styles.modelCardTitle}>{getModelDisplayName(model)}</div>
        <div style={styles.modelCardMeta}>{model.request_model}</div>
      </button>
    );
  };

  const renderModelSection = (mode, title, modelsList, collapsed, onToggle) => (
    <div style={styles.modelSection}>
      <button type='button' style={styles.modelSectionHeader} onClick={onToggle}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
          {mode === 'image' ? <IconImage size='small' /> : <IconVideo size='small' />}
          <Text strong style={{ fontSize: 14, color: 'var(--semi-color-text-0)' }}>
            {title}
          </Text>
          <Text type='tertiary' size='small'>
            {modelsList.length}
          </Text>
        </span>
        {collapsed ? <IconChevronDown /> : <IconChevronUp />}
      </button>
      {!collapsed && (
        <div style={styles.modelList}>
          {modelsList.length > 0 ? (
            modelsList.map((model) => renderModelCard(model, mode))
          ) : (
            <Text type='tertiary' size='small' style={{ padding: '4px 2px 2px' }}>
              {mode === 'image' ? t('当前分组下没有可用图片模型') : t('当前没有可用视频模型')}
            </Text>
          )}
        </div>
      )}
    </div>
  );

  const renderLeftCatalog = () => (
    <div style={styles.leftPanel}>
      <div style={styles.leftContent}>
        <Spin spinning={loading || groupLoading}>
          <div style={styles.catalogHeader}>
            <div style={{ minWidth: 0 }}>
              <div style={styles.catalogTitle}>{t('模型目录')}</div>
              <Text type='tertiary' size='small'>
                {generationMode === 'video' ? t('当前正在浏览视频模型') : t('当前正在浏览图片模型')}
              </Text>
            </div>
            {isMobile && (
              <Button
                size='small'
                type='tertiary'
                icon={<IconMenu />}
                onClick={() => setMobileCatalogVisible(true)}
              >
                {t('模型')}
              </Button>
            )}
          </div>

          <div style={styles.catalogTools}>
            <div style={styles.fieldGroup}>
              <span style={styles.label}>{t('图片分组')}</span>
              <Select
                style={{ width: '100%' }}
                value={selectedGroup}
                onChange={setSelectedGroup}
                disabled={groupLoading || groupOptions.length === 0}
                placeholder={t('请选择分组')}
              >
                {groupOptions.map((group) => (
                  <Select.Option
                    key={group.group}
                    value={group.group}
                    disabled={group.has_available_token === false}
                  >
                    {group.group}
                    {group.has_available_token === false
                      ? ` (${t('无可用令牌')})`
                      : ''}
                  </Select.Option>
                ))}
              </Select>
              {selectedGroupOption && (
                <Text
                  type={selectedGroupOption.has_available_token === false ? 'danger' : 'tertiary'}
                  size='small'
                  style={{ display: 'block', marginTop: 8 }}
                >
                  {selectedGroupOption.has_available_token === false
                    ? t('当前分组暂无可用令牌，请前往令牌管理创建或启用')
                    : t('当前分组可用令牌数：{{count}}', {
                        count: selectedGroupOption.available_token_count || 0,
                      })}
                </Text>
              )}
              {selectedGroupOption &&
                selectedGroupOption.has_available_token === false && (
                  <Button
                    size='small'
                    type='primary'
                    theme='outline'
                    style={{ marginTop: 8 }}
                    onClick={() => navigate('/console/token')}
                  >
                    {t('前往令牌管理')}
                  </Button>
                )}
            </div>

            <Input
              prefix={<IconSearch />}
              placeholder={t('搜索模型名称或系列')}
              value={modelSearchKeyword}
              onChange={setModelSearchKeyword}
              showClear
            />

            <Select
              value={catalogSeriesFilter}
              onChange={setCatalogSeriesFilter}
              disabled={catalogSeriesOptions.length === 0}
              placeholder={t('全部系列')}
            >
              <Select.Option value={MODEL_CATALOG_ALL_SERIES}>
                {t('全部系列')}
              </Select.Option>
              {catalogSeriesOptions.map((series) => (
                <Select.Option key={series} value={series}>
                  {formatModelSeries(series)}
                </Select.Option>
              ))}
            </Select>
          </div>

          {renderModelSection(
            'image',
            t('图片模型'),
            imageCatalogModels,
            imageModelsCollapsed,
            () => setImageModelsCollapsed((current) => !current),
          )}
          {renderModelSection(
            'video',
            t('视频模型'),
            videoCatalogModels,
            videoModelsCollapsed,
            () => setVideoModelsCollapsed((current) => !current),
          )}
        </Spin>
      </div>
    </div>
  );

  const renderComposerParameterField = (
    key,
    label,
    value,
    onChange,
    options,
    disabled,
  ) => (
    <div key={key} style={styles.paramItem}>
      <span style={styles.paramLabel}>{label}</span>
      <Select
        style={{ width: '100%' }}
        size='small'
        value={value}
        onChange={onChange}
        placeholder={t('请选择')}
        disabled={disabled}
      >
        {options.map((option) => (
          <Select.Option key={option.value} value={option.value}>
            {option.label}
          </Select.Option>
        ))}
      </Select>
    </div>
  );

  const renderReferenceThumb = (file, onRemove) => (
    <div key={file.uid || file.name || file.url} style={styles.referenceImageContainer}>
      <img
        src={file.url || (file.fileInstance && URL.createObjectURL(file.fileInstance))}
        alt=''
        style={styles.referenceImageThumb}
      />
      <button type='button' style={styles.removeImageBtn} onClick={onRemove}>
        <IconDelete size='extra-small' />
      </button>
    </div>
  );

  const renderComposer = () => {
    const isVideoMode = generationMode === 'video';
    const activeModel = isVideoMode ? videoSelectedModelData : selectedModelData;
    const activeModelLabel = activeModel ? getModelDisplayName(activeModel) : t('请选择模型');
    const activeModelSeries = activeModel?.model_series
      ? formatModelSeries(activeModel.model_series)
      : '';
    const activePrompt = isVideoMode ? videoPrompt : inspiration;
    const promptHasContent = activePrompt.trim().length > 0;
    const submitLoading = isVideoMode ? videoGenerating : generating;
    const submitDisabled = isVideoMode
      ? videoGenerating || !canGenerateVideo
      : generating || !canGenerate;
    const handleComposerSubmit = () => {
      if (submitDisabled) {
        return;
      }
      if (isVideoMode) {
        handleGenerateVideo();
        return;
      }
      handleGenerate();
    };
    const handleComposerKeyDown = (event) => {
      const nativeEvent = event.nativeEvent || {};
      if (
        event.key !== 'Enter' ||
        event.shiftKey ||
        composerComposingRef.current ||
        nativeEvent.isComposing ||
        event.keyCode === 229
      ) {
        return;
      }
      event.preventDefault();
      handleComposerSubmit();
    };
    const showMaskEditor =
      !isVideoMode &&
      selectedModelSupportsMaskEditing &&
      referenceImages.length > 0;
    const videoComposerParameters = [
      showVideoAspectRatioSelector &&
        renderComposerParameterField(
          'video-aspect-ratio',
          t('视频比例'),
          videoAspectRatio,
          setVideoAspectRatio,
          videoAvailableAspectRatios.map((ratio) => ({
            value: ratio,
            label: ratio,
          })),
          false,
        ),
      showVideoResolutionSelector &&
        renderComposerParameterField(
          'video-resolution',
          t('分辨率'),
          videoResolution,
          setVideoResolution,
          videoAvailableResolutions.map((res) => ({
            value: res,
            label: res,
          })),
          false,
        ),
      renderComposerParameterField(
        'video-duration',
        t('时长'),
        videoDuration,
        setVideoDuration,
        (videoSelectedModelData?.duration_options || []).map((item) => ({
          value: item,
          label: `${item}s`,
        })),
        !videoSelectedModelData?.duration_options?.length,
      ),
    ].filter(Boolean);
    const imageComposerParameters = [
      showImageAspectRatioSelector &&
        renderComposerParameterField(
          'image-aspect-ratio',
          t('生成比例'),
          aspectRatio,
          setAspectRatio,
          availableAspectRatios.map((ratio) => ({
            value: ratio,
            label: ratio,
          })),
          false,
        ),
      showImageResolutionSelector &&
        renderComposerParameterField(
          'image-resolution',
          t('分辨率'),
          resolution,
          setResolution,
          availableResolutions.map((res) => ({
            value: res,
            label: res,
          })),
          false,
        ),
      renderComposerParameterField(
        'image-quantity',
        t('生成数量'),
        quantity,
        (val) => setQuantity(normalizeTaskCount(val)),
        Array.from({ length: DEFAULT_MAX_BATCH_TASKS }, (_, index) => {
          const value = index + 1;
          return {
            value,
            label: String(value),
          };
        }),
        false,
      ),
    ].filter(Boolean);

    return (
      <div style={styles.composer}>
        <div style={styles.composerBox}>
          <div style={styles.composerTop}>
            <div style={styles.selectedModelPill}>
              {generationMode === 'video' ? <IconVideo size='small' /> : <IconImage size='small' />}
              <Text ellipsis={{ showTooltip: true }} style={{ maxWidth: 360 }}>
                {activeModelLabel}
              </Text>
              {activeModelSeries ? (
                <Text type='tertiary' size='small'>
                  {activeModelSeries}
                </Text>
              ) : null}
            </div>
            {isMobile && (
              <Button
                size='small'
                type='tertiary'
                icon={<IconMenu />}
                onClick={() => setMobileCatalogVisible(true)}
              >
                {t('模型')}
              </Button>
            )}
          </div>

          <div style={styles.composerBody}>
            <div style={styles.textareaWrapper}>
              <TextArea
                placeholder={
                  isVideoMode
                    ? t('输入视频创意描述...')
                    : t('输入创作描述...')
                }
                value={activePrompt}
                onChange={isVideoMode ? setVideoPrompt : setInspiration}
                onKeyDown={handleComposerKeyDown}
                onCompositionStart={() => {
                  composerComposingRef.current = true;
                }}
                onCompositionEnd={() => {
                  composerComposingRef.current = false;
                }}
                maxLength={5000}
                showClear
                borderless
                autosize={{ minRows: isMobile ? 3 : 4, maxRows: 10 }}
                style={{
                  border: 'none',
                  background: 'transparent',
                  resize: 'none',
                  boxShadow: 'none',
                }}
              />
              <div style={styles.composerInlineBar}>
                <div style={styles.composerAssets}>
                  {isVideoMode
                    ? videoSelectedModelSupportsImageToVideo &&
                      (videoReferenceImage ? (
                        renderReferenceThumb(videoReferenceImage, handleVideoReferenceRemove)
                      ) : null)
                    : selectedModelSupportsEditing &&
                      referenceImages.map((file) => renderReferenceThumb(file, () => handleImageRemove(file)))}

                  {isVideoMode ? (
                    videoSelectedModelSupportsImageToVideo && (
                      <Upload
                        action=''
                        accept='image/*'
                        multiple={false}
                        fileList={videoReferenceImage ? [videoReferenceImage] : []}
                        onChange={handleVideoReferenceUpload}
                        showUploadList={false}
                        beforeUpload={validateImageSize}
                      >
                        <div style={styles.addImageBtn}>
                          <IconPlus />
                        </div>
                      </Upload>
                    )
                  ) : selectedModelSupportsEditing ? (
                    <>
                      <Upload
                        action=''
                        accept='image/*'
                        multiple
                        fileList={referenceImages}
                        onChange={handleImageUpload}
                        showUploadList={false}
                        beforeUpload={validateImageSize}
                        disabled={referenceImageLimitReached}
                      >
                        <div
                          style={{
                            ...styles.addImageBtn,
                            opacity: referenceImageLimitReached ? 0.5 : 1,
                            cursor: referenceImageLimitReached ? 'not-allowed' : 'pointer',
                          }}
                        >
                          <IconPlus />
                        </div>
                      </Upload>
                      {selectedModelSupportsMaskEditing && referenceImages.length > 0 && (
                        <Button
                          size='small'
                          type='tertiary'
                          icon={<IconSetting />}
                          onClick={() => setComposerAdvancedVisible((current) => !current)}
                        >
                          {t('高级')}
                        </Button>
                      )}
                    </>
                  ) : null}
                </div>

                <div style={styles.composerRightTools}>
                  <div style={styles.composerParams}>
                    {isVideoMode ? videoComposerParameters : imageComposerParameters}
                  </div>

                  <button
                    aria-label={isVideoMode ? t('生成视频') : t('生成')}
                    style={{
                      ...styles.generateIconBtn,
                      opacity: submitDisabled ? 0.55 : 1,
                      pointerEvents: submitDisabled ? 'none' : 'auto',
                      background: promptHasContent
                        ? 'var(--semi-color-primary)'
                        : 'var(--semi-color-fill-0)',
                      borderColor: promptHasContent
                        ? 'var(--semi-color-primary)'
                        : 'var(--semi-color-border)',
                      color: promptHasContent ? '#fff' : 'var(--semi-color-text-2)',
                    }}
                    onClick={handleComposerSubmit}
                    disabled={submitDisabled}
                    type='button'
                  >
                    {submitLoading ? <Spin size='small' /> : <IconSend size='small' />}
                  </button>
                </div>
              </div>
            </div>
          </div>

          {showMaskEditor && composerAdvancedVisible && (
            <div style={{ borderTop: '1px solid var(--semi-color-border)', padding: '12px' }}>
              <Text type='tertiary' size='small' style={{ display: 'block', marginBottom: 8 }}>
                {t('遮罩会与第一张参考图一起作为标准编辑请求提交')}
              </Text>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
                {maskImage ? renderReferenceThumb(maskImage, handleMaskRemove) : null}
                <Upload
                  action=''
                  accept='image/*'
                  multiple={false}
                  fileList={maskImage ? [maskImage] : []}
                  onChange={handleMaskUpload}
                  showUploadList={false}
                  beforeUpload={validateImageSize}
                  disabled={referenceImages.length === 0}
                >
                  <div
                    style={{
                      ...styles.addImageBtn,
                      opacity: referenceImages.length === 0 ? 0.5 : 1,
                      cursor: referenceImages.length === 0 ? 'not-allowed' : 'pointer',
                    }}
                  >
                    <IconSetting size='large' />
                  </div>
                </Upload>
              </div>
            </div>
          )}
        </div>
      </div>
    );
  };

  const renderModernRightPanel = () => (
    <div style={styles.rightPanel}>
      <div style={styles.rightContent} />

      <SideSheet
        placement='left'
        visible={mobileCatalogVisible}
        onCancel={() => setMobileCatalogVisible(false)}
        width='100%'
        title={t('模型目录')}
        bodyStyle={{ padding: 0 }}
      >
        {renderLeftCatalog()}
      </SideSheet>
    </div>
  );

  return (
    <div style={styles.container}>
      {!isMobile && renderLeftCatalog()}
      <div style={styles.contentColumn}>
        {renderModernRightPanel()}
        {renderComposer()}
      </div>
    </div>
  );
};

export default ImageGeneration;
