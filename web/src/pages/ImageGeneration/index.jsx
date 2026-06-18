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

import React, {
  useState,
  useEffect,
  useRef,
  useMemo,
  useCallback,
} from 'react';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Brain } from 'lucide-react';
import {
  Select,
  Dropdown,
  Button,
  Upload,
  Spin,
  Typography,
  TextArea,
  SideSheet,
  Empty,
  Checkbox,
  Tag,
  Popconfirm,
  Tooltip,
} from '@douyinfe/semi-ui';
import {
  IconDelete,
  IconImage,
  IconChevronLeft,
  IconChevronDown,
  IconSend,
  IconMenu,
  IconVideo,
  IconSetting,
  IconCommentStroked,
  IconArchive,
  IconRefresh,
  IconDownload,
  IconClock,
  IconPlayCircle,
  IconRealSizeStroked,
  IconExternalOpen,
  IconMore,
  IconSidebar,
  IconCopy,
  IconPlus,
  IconEdit,
} from '@douyinfe/semi-icons';
import {
  API,
  copy,
  authHeader,
  getUserIdFromLocalStorage,
  showError,
  showInfo,
  showSuccess,
} from '../../helpers';
import {
  extractCanvasChatAttachments,
  getCanvasChatUploadVisibility,
  getCanvasChatWebSearchVisibility,
  normalizeCanvasChatCapabilities,
} from '../../helpers/canvasChat';
import { getLobeHubIcon } from '../../helpers/render';
import { CanvasModelSeriesIcon } from '../../helpers/modelSeries';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import ImageGenerationTaskCard from '../../components/ImageGenerationTaskCard';
import ImageGenerationTaskModal from '../../components/ImageGenerationTaskModal';
import PlayableVideo from '../../components/PlayableVideo';
import VideoGenerationTaskCard from '../../components/VideoGenerationTaskCard';
import VideoGenerationTaskModal from '../../components/VideoGenerationTaskModal';
import MarkdownRenderer from '../../components/common/markdown/MarkdownRenderer';
import {
  getCanvasImageUiState,
  getCanvasImageSelectorVisibility,
  getReferenceImageLimit,
  modelSupportsCapability,
  modelSupportsImageEditing,
  modelSupportsMaskEditing,
} from './canvasRules';
import {
  areCanvasChatReasoningUiStatesEqual,
  createCanvasChatReasoningUiState,
  extractCanvasChatReasoning,
  getCanvasChatReasoningAutoCollapseRemainingMs,
  getCanvasChatReasoningDurationMs,
  shouldAutoCollapseCanvasChatReasoning,
  shouldHideCanvasChatAssistantText,
  syncCanvasChatReasoningUiState,
} from './canvasChatReasoning';
import {
  CANVAS_RENDERABLE_IMAGE_BATCH,
  getRenderableCanvasMessages,
} from './canvasMessageBatches';
import {
  applyCanvasTaskUpdates,
  buildCanvasChatRetryPromptMap,
  buildCanvasMessageIndexes,
  mergeCanvasLatestTimelinePage,
  mergeCanvasMessagesById,
  removeCanvasMessagesByRequestId,
  replaceCanvasMessagesByRequestId,
  sortCanvasMessagesByCreated,
  updateCanvasMessageById,
  updateCanvasMessagesByTask,
  upsertCanvasMessages,
} from './canvasMessageTimeline';
import {
  getDisplayedCanvasMessages,
  getVisibleCanvasSession,
  getVisibleCanvasSessionId,
} from './canvasSessionVisibility';
import {
  getChatRouteSyncAction,
  getRouteSelectionSyncAction,
} from './canvasSessionRouting';
import './canvas-theme.css';

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
const CANVAS_MODE_CHAT = 'chat';
const CANVAS_MODE_IMAGE = 'image';
const CANVAS_MODE_VIDEO = 'video';
const CANVAS_MODES = [CANVAS_MODE_CHAT, CANVAS_MODE_IMAGE, CANVAS_MODE_VIDEO];
const DEFAULT_CANVAS_SESSION_RECENT_PAGE_SIZE = 20;
const MAX_CANVAS_SESSION_RECENT_PAGE_SIZE = 100;
const DEFAULT_CANVAS_MESSAGE_PAGE_SIZE = 100;
const DEFAULT_ASSET_PAGE_SIZE = 24;
const DEFAULT_CHAT_TEMPERATURE = '0.7';
const DEFAULT_CHAT_CONTEXT_COUNT = '8';
const DEFAULT_CHAT_SUMMARY_TRIGGER_MESSAGES = '8';
const DEFAULT_CHAT_SUMMARY_RECENT_MESSAGES = '8';
const DEFAULT_CHAT_WEB_SEARCH_ENABLED = false;
const CHAT_SUMMARY_STRATEGY_OPTIONS = ['0', '4', '8', '12', '16', '24', '32'];
const CHAT_MODEL_OPTIONS_CACHE_PREFIX = 'canvas_chat_model_options_v1';
const CANVAS_AUTO_FOLLOW_BOTTOM_THRESHOLD = 96;
const CANVAS_HISTORY_AUTOLOAD_TOP_THRESHOLD = 72;
const CANVAS_CHAT_STREAMING_NOTICE_THROTTLE_MS = 1200;
const CANVAS_CHAT_SUPPORTED_FILE_ACCEPT =
  '.pdf,.txt,text/plain,application/pdf';
const CANVAS_CHAT_SUPPORTED_FILE_MIME_TYPES = new Set([
  'application/pdf',
  'text/plain',
]);
const CANVAS_CHAT_IMAGE_FILE_NAME_PATTERN =
  /\.(apng|avif|bmp|gif|heic|heif|ico|jpe?g|png|svg|tiff?|webp)$/i;

const sortCanvasSessionsByRecent = (a, b) => {
  if (!!a?.pinned !== !!b?.pinned) {
    return a?.pinned ? -1 : 1;
  }
  const updatedDiff =
    (Number(b?.updated_time) || 0) - (Number(a?.updated_time) || 0);
  if (updatedDiff !== 0) {
    return updatedDiff;
  }
  return (Number(b?.id) || 0) - (Number(a?.id) || 0);
};

const getCanvasSessionIdentifier = (session) => {
  const publicId = String(session?.public_id || '').trim();
  if (publicId) {
    return publicId;
  }
  return normalizeComparableId(session?.id).trim();
};

const canvasSessionMatchesIdentifier = (session, identifier) => {
  const target = String(identifier || '').trim();
  if (!target) {
    return false;
  }
  return (
    getCanvasSessionIdentifier(session) === target ||
    normalizeComparableId(session?.id) === target
  );
};

const buildCanvasSessionApiPath = (sessionOrIdentifier, suffix = '') => {
  const identifier =
    typeof sessionOrIdentifier === 'object'
      ? getCanvasSessionIdentifier(sessionOrIdentifier)
      : String(sessionOrIdentifier || '').trim();
  return `/api/canvas/sessions/${encodeURIComponent(identifier)}${suffix}`;
};

const buildCanvasSessionPath = (sessionOrIdentifier = '') => {
  const identifier =
    typeof sessionOrIdentifier === 'object'
      ? getCanvasSessionIdentifier(sessionOrIdentifier)
      : String(sessionOrIdentifier || '').trim();
  return identifier ? `/canvas/${encodeURIComponent(identifier)}` : '/canvas';
};

const normalizeCanvasSessionRecord = (
  session,
  fallbackMode = CANVAS_MODE_IMAGE,
) => {
  if (!session?.id && !session?.public_id) {
    return null;
  }
  const normalizedMode = CANVAS_MODES.includes(session.mode)
    ? session.mode
    : fallbackMode;
  return {
    ...session,
    mode: normalizedMode,
    public_id: String(session?.public_id || '').trim(),
    session_identifier: getCanvasSessionIdentifier(session),
    web_search_enabled: !!session?.web_search_enabled,
  };
};

const mergeCanvasSessionItems = (existing = [], incoming = []) => {
  const merged = new Map();
  [...existing, ...incoming].forEach((session) => {
    const normalized = normalizeCanvasSessionRecord(session);
    if (!normalized) {
      return;
    }
    merged.set(getCanvasSessionIdentifier(normalized), normalized);
  });
  return Array.from(merged.values()).sort(sortCanvasSessionsByRecent);
};

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

const isDefaultVideoTaskViewState = (state) =>
  !!state &&
  state.page === 1 &&
  !state.statusFilter &&
  !state.modelFilter &&
  !state.timeFilter;

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

const areVideoTasksVisuallyEquivalent = (oldTask, newTask) =>
  !!oldTask &&
  !!newTask &&
  oldTask.id === newTask.id &&
  oldTask.status === newTask.status &&
  oldTask.thumbnail_url === newTask.thumbnail_url &&
  oldTask.result_url === newTask.result_url &&
  oldTask.video_url === newTask.video_url &&
  oldTask.completed_time === newTask.completed_time &&
  oldTask.started_time === newTask.started_time &&
  oldTask.progress === newTask.progress &&
  oldTask.fail_reason === newTask.fail_reason;

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

const normalizeComparableId = (value) => {
  if (value === undefined || value === null) {
    return '';
  }
  return String(value);
};

const buildComparableIdSet = (values) => {
  const list =
    values instanceof Set ? Array.from(values) : [].concat(values || []);
  return new Set(
    list.map(normalizeComparableId).filter((value) => value !== ''),
  );
};

const parseCanvasTaskParams = (params) => {
  if (!params) {
    return {};
  }
  if (typeof params === 'object' && !Array.isArray(params)) {
    return params;
  }
  if (typeof params !== 'string') {
    return {};
  }
  try {
    const parsed = JSON.parse(params);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? parsed
      : {};
  } catch {
    return {};
  }
};

const getCanvasMessageTaskType = (message) =>
  message?.task_type ||
  (message?.video_task
    ? 'video_generation'
    : message?.image_task
      ? 'image_generation'
      : '');

const getCanvasMessagePromptText = (message) =>
  String(
    message?.prompt ||
      message?.image_task?.prompt ||
      message?.video_task?.prompt ||
      '',
  ).trim();

const normalizeAspectRatioText = (value) => {
  const text = String(value || '').trim();
  if (!text) {
    return '';
  }
  const match = text.match(/^(\d+(?:\.\d+)?)\s*[:/xX×]\s*(\d+(?:\.\d+)?)$/);
  if (!match) {
    return '';
  }
  return `${match[1]} / ${match[2]}`;
};

const buildAspectRatioTextFromDimensions = (width, height) => {
  const normalizedWidth = Number(width);
  const normalizedHeight = Number(height);
  if (
    !Number.isFinite(normalizedWidth) ||
    !Number.isFinite(normalizedHeight) ||
    normalizedWidth <= 0 ||
    normalizedHeight <= 0
  ) {
    return '';
  }
  return `${Math.round(normalizedWidth)} / ${Math.round(normalizedHeight)}`;
};

const normalizeImageResolutionTier = (value) => {
  const text = String(value || '')
    .trim()
    .toUpperCase();
  return ['1K', '2K', '4K'].includes(text) ? text : '';
};

const normalizeImageAspectRatioValue = (value) => {
  const text = String(value || '').trim();
  if (!text) {
    return '';
  }
  if (text.toLowerCase() === 'auto') {
    return 'auto';
  }
  return text
    .toLowerCase()
    .replace(/×/g, 'x')
    .replace(/\s+/g, '')
    .replace(/x/g, ':');
};

const roundImageSizeToMultiple = (value, multiple = 16) =>
  Math.max(multiple, Math.round(value / multiple) * multiple);

const floorImageSizeToMultiple = (value, multiple = 16) =>
  Math.max(multiple, Math.floor(value / multiple) * multiple);

const ceilImageSizeToMultiple = (value, multiple = 16) =>
  Math.max(multiple, Math.ceil(value / multiple) * multiple);

const normalizeImagePreviewDimensions = (rawWidth, rawHeight) => {
  const multiple = 16;
  const maxEdge = 3840;
  const maxAspect = 3;
  const minPixels = 655360;
  const maxPixels = 8294400;
  let width = roundImageSizeToMultiple(rawWidth, multiple);
  let height = roundImageSizeToMultiple(rawHeight, multiple);

  const scaleToFit = (scale) => {
    width = floorImageSizeToMultiple(width * scale, multiple);
    height = floorImageSizeToMultiple(height * scale, multiple);
  };
  const scaleToFill = (scale) => {
    width = ceilImageSizeToMultiple(width * scale, multiple);
    height = ceilImageSizeToMultiple(height * scale, multiple);
  };

  for (let index = 0; index < 4; index += 1) {
    const edge = Math.max(width, height);
    if (edge > maxEdge) {
      scaleToFit(maxEdge / edge);
    }

    if (width / height > maxAspect) {
      width = floorImageSizeToMultiple(height * maxAspect, multiple);
    } else if (height / width > maxAspect) {
      height = floorImageSizeToMultiple(width * maxAspect, multiple);
    }

    const pixels = width * height;
    if (pixels > maxPixels) {
      scaleToFit(Math.sqrt(maxPixels / pixels));
    } else if (pixels < minPixels) {
      scaleToFill(Math.sqrt(minPixels / pixels));
    }
  }

  return { width, height };
};

const parseImageAspectRatioPair = (ratio) => {
  const match = String(ratio || '')
    .trim()
    .match(/^(\d+(?:\.\d+)?)\s*[:xX×]\s*(\d+(?:\.\d+)?)$/);
  if (!match) {
    return null;
  }
  const width = Number(match[1]);
  const height = Number(match[2]);
  if (
    !Number.isFinite(width) ||
    !Number.isFinite(height) ||
    width <= 0 ||
    height <= 0
  ) {
    return null;
  }
  return { width, height };
};

const parseImageSizePair = (size) => {
  const match = String(size || '')
    .trim()
    .match(/^(\d+)\s*[xX×]\s*(\d+)$/);
  if (!match) {
    return null;
  }
  const width = Number(match[1]);
  const height = Number(match[2]);
  if (
    !Number.isFinite(width) ||
    !Number.isFinite(height) ||
    width <= 0 ||
    height <= 0
  ) {
    return null;
  }
  return { width, height };
};

const getImageDimensionPreview = (resolutionValue, aspectRatioValue) => {
  const explicitSize = parseImageSizePair(resolutionValue);
  if (explicitSize) {
    return {
      width: String(explicitSize.width),
      height: String(explicitSize.height),
    };
  }

  const normalizedRatio = normalizeImageAspectRatioValue(aspectRatioValue);
  if (normalizedRatio === 'auto') {
    return { width: 'auto', height: 'auto' };
  }

  const tier = normalizeImageResolutionTier(resolutionValue);
  if (!tier) {
    return { width: 'auto', height: 'auto' };
  }

  const ratio = parseImageAspectRatioPair(normalizedRatio || '1:1');
  if (!ratio) {
    return { width: 'auto', height: 'auto' };
  }

  const isSquare = ratio.width === ratio.height;
  if (isSquare) {
    const side = tier === '4K' ? 3840 : tier === '2K' ? 2048 : 1024;
    const normalized = normalizeImagePreviewDimensions(side, side);
    return {
      width: String(normalized.width),
      height: String(normalized.height),
    };
  }

  if (tier === '1K') {
    const shortSide = 1024;
    if (ratio.width > ratio.height) {
      return {
        width: String(
          roundImageSizeToMultiple((shortSide * ratio.width) / ratio.height),
        ),
        height: String(shortSide),
      };
    }
    return {
      width: String(shortSide),
      height: String(
        roundImageSizeToMultiple((shortSide * ratio.height) / ratio.width),
      ),
    };
  }

  const longSide = tier === '4K' ? 3840 : 2048;
  let rawWidth = longSide;
  let rawHeight = longSide;
  if (ratio.width > ratio.height) {
    rawHeight = roundImageSizeToMultiple(
      (longSide * ratio.height) / ratio.width,
    );
  } else {
    rawWidth = roundImageSizeToMultiple(
      (longSide * ratio.width) / ratio.height,
    );
  }
  const normalized = normalizeImagePreviewDimensions(rawWidth, rawHeight);
  return {
    width: String(normalized.width),
    height: String(normalized.height),
  };
};

const collectTaskReferenceValues = (raw) => {
  const values = [];
  const append = (item) => {
    if (!item) {
      return;
    }
    if (typeof item === 'string') {
      const trimmed = item.trim();
      if (trimmed) {
        values.push(trimmed);
      }
      return;
    }
    if (typeof item === 'object') {
      append(item.url || item.image_url || item.thumbnail_url);
    }
  };

  if (Array.isArray(raw)) {
    raw.forEach(append);
  } else {
    append(raw);
  }

  return Array.from(new Set(values));
};

const ImageGeneration = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { sessionId: routeSessionId = '' } = useParams();
  const isMobile = useIsMobile();

  useEffect(() => {
    document.body.classList.add('canvas-theme-route');
    return () => {
      document.body.classList.remove('canvas-theme-route');
    };
  }, []);

  // LocalStorage keys
  const STORAGE_KEYS = {
    GROUP: 'imageGen_selectedGroup',
    MODE: 'canvas_generation_mode',
    CHAT_MODEL: 'canvas_chat_model',
    CHAT_TEMPERATURE: 'canvas_chat_temperature',
    CHAT_CONTEXT: 'canvas_chat_context',
    CHAT_WEB_SEARCH: 'canvas_chat_web_search',
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

  function getChatModelOptionsCacheKey() {
    let userId = '-1';
    try {
      userId = String(getUserIdFromLocalStorage() ?? '-1').trim() || '-1';
    } catch (e) {
      userId = '-1';
    }
    return `${CHAT_MODEL_OPTIONS_CACHE_PREFIX}:${userId}`;
  }

  function normalizeCanvasChatModelRecords(items) {
    return (Array.isArray(items) ? items : [])
      .map((item) => normalizeCanvasChatModelRecord(item))
      .filter(Boolean);
  }

  function getCachedChatModels() {
    try {
      const cached = localStorage.getItem(getChatModelOptionsCacheKey());
      if (!cached) {
        return [];
      }
      return normalizeCanvasChatModelRecords(JSON.parse(cached));
    } catch (e) {
      return [];
    }
  }

  function cacheChatModels(items) {
    try {
      const normalizedItems = normalizeCanvasChatModelRecords(items);
      localStorage.setItem(
        getChatModelOptionsCacheKey(),
        JSON.stringify(normalizedItems),
      );
    } catch (e) {
      console.error('Failed to cache chat models:', e);
    }
  }

  const [loading, setLoading] = useState(false);
  const [groupLoading, setGroupLoading] = useState(false);
  const [groupOptions, setGroupOptions] = useState([]);
  const [selectedGroup, setSelectedGroup] = useState(() =>
    getStoredValue(STORAGE_KEYS.GROUP, ''),
  );
  const [generationMode, setGenerationMode] = useState(CANVAS_MODE_CHAT);
  const [models, setModels] = useState([]);
  const [filteredModels, setFilteredModels] = useState([]);
  const [videoModels, setVideoModels] = useState([]);
  const [videoFilteredModels, setVideoFilteredModels] = useState([]);
  const [mobileTaskbarVisible, setMobileTaskbarVisible] = useState(false);
  const [desktopSidebarCollapsed, setDesktopSidebarCollapsed] = useState(false);
  const [composerAdvancedVisible, setComposerAdvancedVisible] = useState(false);
  const [activeDropdownKey, setActiveDropdownKey] = useState('');
  const [assetLibraryVisible, setAssetLibraryVisible] = useState(false);
  const [canvasAssets, setCanvasAssets] = useState([]);
  const [canvasAssetsLoading, setCanvasAssetsLoading] = useState(false);
  const [canvasAssetsError, setCanvasAssetsError] = useState('');
  const [canvasAssetsTotal, setCanvasAssetsTotal] = useState(0);
  const [canvasAssetPage, setCanvasAssetPage] = useState(1);
  const [selectedAssetPreview, setSelectedAssetPreview] = useState(null);
  const [selectedCanvasImagePreview, setSelectedCanvasImagePreview] =
    useState(null);
  const [selectedCanvasAssetIds, setSelectedCanvasAssetIds] = useState(
    new Set(),
  );
  const [assetBatchDeleting, setAssetBatchDeleting] = useState(false);

  const [chatModels, setChatModels] = useState(() => getCachedChatModels());
  const [chatModel, setChatModel] = useState(() =>
    getStoredValue(STORAGE_KEYS.CHAT_MODEL, ''),
  );
  const [chatPrompt, setChatPrompt] = useState('');
  const [chatImageAttachment, setChatImageAttachment] = useState(null);
  const [chatFileAttachment, setChatFileAttachment] = useState(null);
  const [chatComposerDragActive, setChatComposerDragActive] = useState(false);
  const [chatTemperature, setChatTemperature] = useState(() =>
    getStoredValue(STORAGE_KEYS.CHAT_TEMPERATURE, DEFAULT_CHAT_TEMPERATURE),
  );
  const [chatContext, setChatContext] = useState(() =>
    getStoredValue(STORAGE_KEYS.CHAT_CONTEXT, DEFAULT_CHAT_CONTEXT_COUNT),
  );
  const [chatWebSearchEnabled, setChatWebSearchEnabled] = useState(
    () =>
      getStoredValue(
        STORAGE_KEYS.CHAT_WEB_SEARCH,
        DEFAULT_CHAT_WEB_SEARCH_ENABLED ? 'true' : 'false',
      ) === 'true',
  );
  const [chatStreaming, setChatStreaming] = useState(false);
  const [chatStreamRenderVersion, setChatStreamRenderVersion] = useState(0);
  const [chatSessionSettingsVisible, setChatSessionSettingsVisible] =
    useState(false);
  const [chatSessionSettingsSessionId, setChatSessionSettingsSessionId] =
    useState(null);
  const [chatSessionSettingsDraft, setChatSessionSettingsDraft] =
    useState(null);
  const [chatSessionSettingsSaving, setChatSessionSettingsSaving] =
    useState(false);
  const [canvasSessions, setCanvasSessions] = useState({
    [CANVAS_MODE_CHAT]: [],
    [CANVAS_MODE_IMAGE]: [],
    [CANVAS_MODE_VIDEO]: [],
  });
  const [recentCanvasSessions, setRecentCanvasSessions] = useState({
    items: [],
    expanded: true,
    initialLoading: false,
    loadingMore: false,
    hasMore: true,
    offset: 0,
    error: '',
  });
  const [selectedCanvasSessionIds, setSelectedCanvasSessionIds] = useState({
    [CANVAS_MODE_CHAT]: null,
    [CANVAS_MODE_IMAGE]: null,
    [CANVAS_MODE_VIDEO]: null,
  });
  const [deletingCanvasSession, setDeletingCanvasSession] = useState(false);
  const [canvasMessagesSessionId, setCanvasMessagesSessionId] = useState(null);
  const [canvasMessages, setCanvasMessages] = useState([]);
  const [canvasAutoFollowEnabled, setCanvasAutoFollowEnabled] = useState(true);
  const [canvasMessagesLoading, setCanvasMessagesLoading] = useState(false);
  const [canvasMessagesLoadingMore, setCanvasMessagesLoadingMore] =
    useState(false);
  const [canvasMessagesError, setCanvasMessagesError] = useState('');
  const [canvasMessagesHasMore, setCanvasMessagesHasMore] = useState(false);
  const [canvasMessagesNextCursor, setCanvasMessagesNextCursor] = useState('');
  const [hoveredSidebarNavKey, setHoveredSidebarNavKey] = useState('');
  const [hoveredCanvasSessionId, setHoveredCanvasSessionId] = useState(null);
  const [hoveredCanvasChatMessageId, setHoveredCanvasChatMessageId] =
    useState(null);
  const [
    activeCanvasChatActionTooltipKey,
    setActiveCanvasChatActionTooltipKey,
  ] = useState('');
  const [canvasChatCopiedMessageId, setCanvasChatCopiedMessageId] =
    useState(null);
  const [
    canvasChatReasoningUiStateByMessageId,
    setCanvasChatReasoningUiStateByMessageId,
  ] = useState({});
  const routeSessionIdNormalized = String(routeSessionId || '').trim();

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

  const { showImageAspectRatioSelector, showImageResolutionSelector } =
    getCanvasImageSelectorVisibility({
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
  const [taskListError, setTaskListError] = useState('');
  const [selectedTaskIds, setSelectedTaskIds] = useState(new Set());
  const [deletingTasks, setDeletingTasks] = useState(false);
  const [videoTasks, setVideoTasks] = useState([]);
  const [videoTaskTotal, setVideoTaskTotal] = useState(0);
  const [videoTaskPage, setVideoTaskPage] = useState(1);
  const [videoTaskPageSize, setVideoTaskPageSize] = useState(
    DEFAULT_VIDEO_PAGE_SIZE,
  );
  const [videoTaskHasMore, setVideoTaskHasMore] = useState(false);
  const [videoTaskNextCursor, setVideoTaskNextCursor] = useState('');
  const [videoTaskStatusFilter, setVideoTaskStatusFilter] = useState('');
  const [videoTaskModelFilter, setVideoTaskModelFilter] = useState('');
  const [videoTaskTimeFilter, setVideoTaskTimeFilter] = useState('');
  const [videoLoadingTasks, setVideoLoadingTasks] = useState(false);
  const [videoTaskListError, setVideoTaskListError] = useState('');
  const [videoSelectedTask, setVideoSelectedTask] = useState(null);
  const [videoTaskModalVisible, setVideoTaskModalVisible] = useState(false);
  const [videoSelectedTaskIds, setVideoSelectedTaskIds] = useState(new Set());
  const [deletingVideoTasks, setDeletingVideoTasks] = useState(false);
  const [selectedCanvasMessageId, setSelectedCanvasMessageId] = useState(null);
  const canvasMessageViewportRef = useRef(null);
  const canvasAutoFollowEnabledRef = useRef(true);
  const canvasMessagesRef = useRef([]);
  const canvasMessageDetailCacheRef = useRef(new Map());
  const canvasMessageDetailRequestSeqRef = useRef(new Map());
  const canvasMessageClientRequestSeqRef = useRef(0);
  const recentCanvasSessionsRequestSeqRef = useRef(0);
  const sseRef = useRef(null);
  const pollingTimerRef = useRef(null);
  const pollingIntervalRef = useRef(DEFAULT_POLLING_INTERVAL_SECONDS);
  const taskListStateRef = useRef(null);
  const videoTaskListStateRef = useRef(null);
  const taskListRequestSeqRef = useRef(0);
  const videoTaskListRequestSeqRef = useRef(0);
  const taskDetailRequestSeqRef = useRef(0);
  const drawingModelsRequestSeqRef = useRef(0);
  const loadedModelsGroupRef = useRef('');
  const canvasMessagesRequestSeqRef = useRef(0);
  const canvasMessagesSessionIdRef = useRef(null);
  const canvasMessageIndexesRef = useRef(buildCanvasMessageIndexes([]));
  const taskUpdatesCompletedSinceRef = useRef(
    Math.floor(Date.now() / 1000) - 60,
  );
  const videoTaskUpdatesCompletedSinceRef = useRef(
    Math.floor(Date.now() / 1000) - 60,
  );
  const taskCursorHistoryRef = useRef(['']);
  const videoTaskCursorHistoryRef = useRef(['']);
  const pendingCanvasPrefillRef = useRef(null);
  const prefillGroupFallbackNoticeShownRef = useRef(false);
  const composerComposingRef = useRef(false);
  const chatStreamAbortRef = useRef(null);
  const chatStreamingMessageIdRef = useRef(null);
  const chatStreamingSessionIdRef = useRef(null);
  const chatAttachmentUploadInputRef = useRef(null);
  const chatComposerDragDepthRef = useRef(0);
  const canvasChatCopiedMessageTimerRef = useRef(null);
  const canvasChatReasoningAutoCollapseTimersRef = useRef(new Map());
  const previousRouteSessionIdRef = useRef(routeSessionIdNormalized);
  const chatStreamingNoticeAtRef = useRef(0);
  const chatModelsRequestRef = useRef(null);
  const chatModelsRequestNotifyRef = useRef(false);
  const selectedCanvasSessionRef = useRef(null);
  const generationModeRef = useRef(generationMode);
  const selectedCanvasSessionIdsRef = useRef(selectedCanvasSessionIds);
  const routeSyncInFlightRef = useRef(false);
  const routeSessionLookupRequestSeqRef = useRef(0);
  const routeSessionLookupTargetRef = useRef('');
  const canvasImagePreviewRequestSeqRef = useRef(0);
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
  const selectedCanvasSessionId = selectedCanvasSessionIds[generationMode];
  const selectedChatSessionId = String(
    selectedCanvasSessionIds[CANVAS_MODE_CHAT] || '',
  ).trim();
  const findCanvasSessionByIdentifier = useCallback(
    (sessionId, mode = '') => {
      const target = String(sessionId || '').trim();
      if (!target) {
        return null;
      }
      const normalizedMode = CANVAS_MODES.includes(mode) ? mode : '';
      if (normalizedMode) {
        const directMatch = (canvasSessions[normalizedMode] || []).find(
          (item) => canvasSessionMatchesIdentifier(item, target),
        );
        if (directMatch) {
          return directMatch;
        }
        const recentMatch = recentCanvasSessions.items.find(
          (item) =>
            item.mode === normalizedMode &&
            canvasSessionMatchesIdentifier(item, target),
        );
        if (recentMatch) {
          return recentMatch;
        }
      }
      const anyRecentMatch = recentCanvasSessions.items.find((item) =>
        canvasSessionMatchesIdentifier(item, target),
      );
      if (anyRecentMatch) {
        return anyRecentMatch;
      }
      for (const canvasMode of CANVAS_MODES) {
        const match = (canvasSessions[canvasMode] || []).find((item) =>
          canvasSessionMatchesIdentifier(item, target),
        );
        if (match) {
          return match;
        }
      }
      return null;
    },
    [canvasSessions, recentCanvasSessions.items],
  );
  const visibleCanvasSessionId = getVisibleCanvasSessionId({
    generationMode,
    routeSessionId: routeSessionIdNormalized,
    selectedCanvasSessionId,
    canvasModes: CANVAS_MODES,
    findCanvasSessionByIdentifier,
  });
  const selectedCanvasSession = getVisibleCanvasSession({
    generationMode,
    routeSessionId: routeSessionIdNormalized,
    selectedCanvasSessionId,
    findCanvasSessionByIdentifier,
    canvasModes: CANVAS_MODES,
  });
  useEffect(() => {
    selectedCanvasSessionRef.current = selectedCanvasSession;
  }, [selectedCanvasSession]);
  const displayedCanvasMessages = useMemo(
    () =>
      getDisplayedCanvasMessages({
        visibleSessionId: visibleCanvasSessionId,
        canvasMessagesSessionId,
        canvasMessages,
      }),
    [canvasMessages, canvasMessagesSessionId, visibleCanvasSessionId],
  );
  const renderableCanvasMessages = useMemo(
    () => getRenderableCanvasMessages(displayedCanvasMessages),
    [displayedCanvasMessages],
  );
  const chatContextDividerIndex = useMemo(() => {
    if (
      generationMode !== CANVAS_MODE_CHAT ||
      renderableCanvasMessages.length === 0
    ) {
      return -1;
    }
    const clearContextMessageId = Number(
      selectedCanvasSession?.clear_context_message_id,
    );
    if (!Number.isFinite(clearContextMessageId) || clearContextMessageId <= 0) {
      return -1;
    }
    const firstPostClearIndex = renderableCanvasMessages.findIndex(
      (message) => {
        const numericMessageId = Number(message?.id);
        return (
          !Number.isFinite(numericMessageId) ||
          numericMessageId > clearContextMessageId
        );
      },
    );
    return firstPostClearIndex >= 0
      ? firstPostClearIndex
      : renderableCanvasMessages.length;
  }, [
    generationMode,
    renderableCanvasMessages,
    selectedCanvasSession?.clear_context_message_id,
  ]);
  const canvasChatRetryPromptByMessageId = useMemo(
    () => buildCanvasChatRetryPromptMap(displayedCanvasMessages),
    [displayedCanvasMessages],
  );
  const displayedCanvasReasoningMessages = useMemo(
    () =>
      generationMode === CANVAS_MODE_CHAT
        ? displayedCanvasMessages.reduce((result, message) => {
            if (message?.role !== 'assistant' || !message?.id) {
              return result;
            }
            const display = extractCanvasChatReasoning(message);
            if (!display.hasReasoning) {
              return result;
            }
            result.push(message);
            return result;
          }, [])
        : [],
    [displayedCanvasMessages, generationMode],
  );
  canvasMessagesRef.current = canvasMessages;
  const isCurrentCanvasMessageSession = (sessionId) =>
    String(canvasMessagesSessionIdRef.current || '') ===
    String(sessionId || '');
  const ensureCanvasMessagesSessionForSend = (sessionId) => {
    const normalizedSessionId = String(sessionId || '').trim();
    if (!normalizedSessionId) {
      return;
    }
    canvasMessagesRequestSeqRef.current += 1;
    if (isCurrentCanvasMessageSession(normalizedSessionId)) {
      setCanvasMessagesLoading(false);
      setCanvasMessagesLoadingMore(false);
      return;
    }
    canvasMessagesSessionIdRef.current = normalizedSessionId;
    setCanvasMessagesSessionId(normalizedSessionId);
    setCanvasMessages([]);
    setCanvasMessagesHasMore(false);
    setCanvasMessagesNextCursor('');
    setCanvasMessagesError('');
    setCanvasMessagesLoading(false);
    setCanvasMessagesLoadingMore(false);
  };
  const setCanvasAutoFollowEnabledState = (nextValue) => {
    canvasAutoFollowEnabledRef.current = nextValue;
    setCanvasAutoFollowEnabled((current) =>
      current === nextValue ? current : nextValue,
    );
  };
  const isCanvasViewportNearBottom = (container) => {
    if (!container) {
      return true;
    }
    const remainingDistance = Math.max(
      0,
      container.scrollHeight - container.scrollTop - container.clientHeight,
    );
    return remainingDistance <= CANVAS_AUTO_FOLLOW_BOTTOM_THRESHOLD;
  };
  const syncCanvasAutoFollowState = (container) => {
    const nextValue = isCanvasViewportNearBottom(container);
    if (canvasAutoFollowEnabledRef.current !== nextValue) {
      setCanvasAutoFollowEnabledState(nextValue);
    }
    return nextValue;
  };
  const scrollCanvasViewportToBottom = ({
    behavior = 'smooth',
    enableAutoFollow = true,
  } = {}) => {
    const container = canvasMessageViewportRef.current;
    if (!container) {
      return;
    }
    if (enableAutoFollow) {
      setCanvasAutoFollowEnabledState(true);
    }
    const nextTop = Math.max(
      0,
      container.scrollHeight - container.clientHeight,
    );
    if (typeof container.scrollTo === 'function') {
      container.scrollTo({ top: nextTop, behavior });
      return;
    }
    container.scrollTop = nextTop;
  };
  const focusCanvasChatComposerInput = () => {
    window.requestAnimationFrame(() => {
      const textarea =
        document.querySelector("[data-canvas-prompt-input='chat'] textarea") ||
        document.querySelector("[data-canvas-prompt-input='chat']");
      if (!textarea?.focus) {
        return;
      }
      textarea.focus();
      const selectionLength =
        typeof textarea.value === 'string' ? textarea.value.length : 0;
      textarea.setSelectionRange?.(selectionLength, selectionLength);
    });
  };
  const showChatStreamingNotice = () => {
    const now = Date.now();
    if (
      now - chatStreamingNoticeAtRef.current <
      CANVAS_CHAT_STREAMING_NOTICE_THROTTLE_MS
    ) {
      return;
    }
    chatStreamingNoticeAtRef.current = now;
    showInfo(t('当前回复仍在生成，请先停止后再发送下一条消息'));
  };
  const syncCanvasRouteToSession = (sessionOrIdentifier, options = {}) => {
    const identifier =
      typeof sessionOrIdentifier === 'object'
        ? getCanvasSessionIdentifier(sessionOrIdentifier)
        : String(sessionOrIdentifier || '').trim();
    const replace = options.replace !== false;
    const nextPath = buildCanvasSessionPath(identifier);
    const currentPath = `${location.pathname}${location.search}`;
    const nextLocation = `${nextPath}${location.search}`;
    routeSyncInFlightRef.current = true;
    if (currentPath === nextLocation) {
      window.setTimeout(() => {
        routeSyncInFlightRef.current = false;
      }, 0);
      return;
    }
    navigate(nextPath, {
      replace,
      state: location.state,
    });
    window.setTimeout(() => {
      routeSyncInFlightRef.current = false;
    }, 0);
  };
  const syncCanvasRouteToBlank = (options = {}) => {
    const replace = options.replace !== false;
    const currentPath = `${location.pathname}${location.search}`;
    if (currentPath === '/canvas') {
      return;
    }
    routeSyncInFlightRef.current = true;
    navigate('/canvas', {
      replace,
      state: location.state,
    });
    window.setTimeout(() => {
      routeSyncInFlightRef.current = false;
    }, 0);
  };
  const updateCurrentCanvasSessionModel = async (mode, modelId) => {
    const normalizedMode = CANVAS_MODES.includes(mode) ? mode : generationMode;
    const sessionId = selectedCanvasSessionIds[normalizedMode];
    const session = findCanvasSessionByIdentifier(sessionId, normalizedMode);
    const sessionIdentifier = getCanvasSessionIdentifier(session);
    if (!sessionIdentifier) {
      return;
    }
    const currentModel = String(modelId || '').trim();
    setCanvasSessionsForMode(normalizedMode, (prev) =>
      prev.map((item) =>
        canvasSessionMatchesIdentifier(item, sessionIdentifier)
          ? { ...item, current_model: currentModel }
          : item,
      ),
    );
    try {
      const res = await API.patch(buildCanvasSessionApiPath(session), {
        current_model: currentModel,
      });
      if (res.data.success && res.data.data) {
        updateCanvasSessionInState(res.data.data);
      } else {
        showError(res.data.message || t('更新会话模型失败'));
      }
    } catch (error) {
      showError(error.message || t('更新会话模型失败'));
    }
  };

  const updateCurrentCanvasChatSessionConfig = async (
    updates,
    errorMessage,
  ) => {
    const sessionId = selectedCanvasSessionIds[CANVAS_MODE_CHAT];
    const session = findCanvasSessionByIdentifier(sessionId, CANVAS_MODE_CHAT);
    const sessionIdentifier = getCanvasSessionIdentifier(session);
    if (!sessionIdentifier || !updates || Object.keys(updates).length === 0) {
      return;
    }
    setCanvasSessionsForMode(CANVAS_MODE_CHAT, (prev) =>
      prev.map((item) =>
        canvasSessionMatchesIdentifier(item, sessionIdentifier)
          ? { ...item, ...updates }
          : item,
      ),
    );
    try {
      const res = await API.patch(buildCanvasSessionApiPath(session), updates);
      if (res.data.success && res.data.data) {
        updateCanvasSessionInState(res.data.data);
      } else {
        showError(res.data.message || errorMessage);
      }
    } catch (error) {
      showError(error.message || errorMessage);
    }
  };

  useEffect(() => {
    generationModeRef.current = generationMode;
  }, [generationMode]);

  useEffect(() => {
    selectedCanvasSessionIdsRef.current = selectedCanvasSessionIds;
  }, [selectedCanvasSessionIds]);

  useEffect(() => {
    if (!chatSessionSettingsVisible || !chatSessionSettingsSessionId) {
      return;
    }
    const session = findCanvasSessionByIdentifier(chatSessionSettingsSessionId);
    if (!session) {
      closeCanvasChatSessionSettings();
    }
  }, [
    canvasSessions,
    chatSessionSettingsSessionId,
    chatSessionSettingsVisible,
    recentCanvasSessions.items,
  ]);

  useEffect(() => {
    setCanvasAutoFollowEnabledState(true);
  }, [generationMode, selectedCanvasSessionId]);

  useEffect(() => {
    const container = canvasMessageViewportRef.current;
    if (!container || !canvasAutoFollowEnabledRef.current) {
      return;
    }
    container.scrollTop = container.scrollHeight;
  }, [
    canvasAutoFollowEnabled,
    canvasMessagesSessionId,
    displayedCanvasMessages.length,
    chatStreamRenderVersion,
    generationMode,
  ]);

  useEffect(() => () => stopChatStream({ syncUI: false }), []);

  useEffect(
    () => () => {
      if (canvasChatCopiedMessageTimerRef.current) {
        window.clearTimeout(canvasChatCopiedMessageTimerRef.current);
        canvasChatCopiedMessageTimerRef.current = null;
      }
      canvasChatReasoningAutoCollapseTimersRef.current.forEach((timerId) => {
        window.clearTimeout(timerId);
      });
      canvasChatReasoningAutoCollapseTimersRef.current.clear();
    },
    [],
  );

  useEffect(() => {
    if (!chatStreaming) {
      return;
    }
    const streamingSessionId = chatStreamingSessionIdRef.current;
    if (!streamingSessionId) {
      return;
    }
    const selectedChatSessionId = selectedCanvasSessionIds[CANVAS_MODE_CHAT];
    if (
      generationMode !== CANVAS_MODE_CHAT ||
      selectedChatSessionId !== streamingSessionId
    ) {
      stopChatStream({ syncUI: false });
    }
  }, [chatStreaming, generationMode, selectedCanvasSessionIds]);

  useEffect(() => {
    setSelectedCanvasMessageId(null);
    setHoveredCanvasChatMessageId(null);
    setHoveredCanvasSessionId(null);
    setActiveCanvasChatActionTooltipKey('');
    setCanvasChatCopiedMessageId(null);
    if (canvasChatCopiedMessageTimerRef.current) {
      window.clearTimeout(canvasChatCopiedMessageTimerRef.current);
      canvasChatCopiedMessageTimerRef.current = null;
    }
    canvasChatReasoningAutoCollapseTimersRef.current.forEach((timerId) => {
      window.clearTimeout(timerId);
    });
    canvasChatReasoningAutoCollapseTimersRef.current.clear();
    setCanvasChatReasoningUiStateByMessageId({});
  }, [selectedCanvasSessionId, generationMode]);

  useEffect(() => {
    if (generationMode !== CANVAS_MODE_CHAT) {
      return;
    }

    const nextReasoningMessages = new Map();
    displayedCanvasReasoningMessages.forEach((message) => {
      nextReasoningMessages.set(String(message.id), message);
    });

    setCanvasChatReasoningUiStateByMessageId((prev) => {
      let changed = false;
      let next = prev;
      const now = Date.now();

      nextReasoningMessages.forEach((message, messageId) => {
        const previousState = prev[messageId] || null;
        const syncedState = syncCanvasChatReasoningUiState({
          message,
          previousState,
          hasReasoning: true,
          isLiveMessage:
            chatStreaming &&
            String(chatStreamingSessionIdRef.current || '') ===
              String(canvasMessagesSessionId || '') &&
            String(chatStreamingMessageIdRef.current || '') === messageId,
          now,
        });

        if (
          syncedState &&
          !areCanvasChatReasoningUiStatesEqual(previousState, syncedState)
        ) {
          if (!changed) {
            next = {
              ...prev,
            };
            changed = true;
          }
          next[messageId] = syncedState;
        }
      });

      Object.keys(prev).forEach((messageId) => {
        if (nextReasoningMessages.has(String(messageId))) {
          return;
        }
        if (!changed) {
          next = {
            ...prev,
          };
          changed = true;
        }
        delete next[messageId];
      });

      return changed ? next : prev;
    });
  }, [
    canvasMessagesSessionId,
    chatStreaming,
    displayedCanvasReasoningMessages,
    generationMode,
  ]);

  useEffect(() => {
    Object.entries(canvasChatReasoningUiStateByMessageId).forEach(
      ([messageId, reasoningUiState]) => {
        const timerId =
          canvasChatReasoningAutoCollapseTimersRef.current.get(messageId);
        const remainingMs =
          getCanvasChatReasoningAutoCollapseRemainingMs(reasoningUiState);

        if (remainingMs === null) {
          if (timerId) {
            window.clearTimeout(timerId);
            canvasChatReasoningAutoCollapseTimersRef.current.delete(messageId);
          }
          return;
        }

        if (timerId) {
          return;
        }

        const nextTimerId = window.setTimeout(() => {
          canvasChatReasoningAutoCollapseTimersRef.current.delete(messageId);
          setCanvasChatReasoningUiStateByMessageId((prev) => {
            const currentState = prev[messageId];
            if (!shouldAutoCollapseCanvasChatReasoning(currentState)) {
              return prev;
            }
            return {
              ...prev,
              [messageId]: {
                ...currentState,
                isExpanded: false,
                hasAutoCollapsed: true,
              },
            };
          });
        }, remainingMs);

        canvasChatReasoningAutoCollapseTimersRef.current.set(
          messageId,
          nextTimerId,
        );
      },
    );

    canvasChatReasoningAutoCollapseTimersRef.current.forEach(
      (timerId, messageId) => {
        if (canvasChatReasoningUiStateByMessageId[messageId]) {
          return;
        }
        window.clearTimeout(timerId);
        canvasChatReasoningAutoCollapseTimersRef.current.delete(messageId);
      },
    );
  }, [canvasChatReasoningUiStateByMessageId]);

  taskListStateRef.current = {
    page: taskPage,
    pageSize: taskPageSize,
    statusFilter: taskStatusFilter,
    modelFilter: taskModelFilter,
    timeFilter: taskTimeFilter,
    sortBy: taskSortBy,
    sortOrder: taskSortOrder,
  };
  videoTaskListStateRef.current = {
    page: videoTaskPage,
    pageSize: videoTaskPageSize,
    statusFilter: videoTaskStatusFilter,
    modelFilter: videoTaskModelFilter,
    timeFilter: videoTaskTimeFilter,
  };
  pollingIntervalRef.current = pollingIntervalSeconds;

  useEffect(() => {
    canvasMessageIndexesRef.current = buildCanvasMessageIndexes(canvasMessages);
  }, [canvasMessages]);

  const taskMatchesTimeFilter = (task, timeFilter) => {
    if (!timeFilter) {
      return true;
    }
    const createdTime = Number(task?.created_time) || 0;
    if (createdTime <= 0) {
      return false;
    }
    const { start, end } = computeTimeRange(timeFilter);
    return start > 0 && createdTime >= start && (!end || createdTime <= end);
  };

  const imageTaskMatchesCurrentTaskFilters = (task) => {
    if (!task) {
      return false;
    }
    if (taskStatusFilter && task.status !== taskStatusFilter) {
      return false;
    }
    if (taskModelFilter && task.model_id !== taskModelFilter) {
      return false;
    }
    return taskMatchesTimeFilter(task, taskTimeFilter);
  };

  useEffect(() => {
    const latestCompletedTime = tasks.reduce((latest, task) => {
      const completedAt = Number(task?.completed_time) || 0;
      return completedAt > latest ? completedAt : latest;
    }, 0);
    if (latestCompletedTime > taskUpdatesCompletedSinceRef.current) {
      taskUpdatesCompletedSinceRef.current = latestCompletedTime;
    }
  }, [tasks]);

  const enabledImageModels = useMemo(
    () =>
      models.filter(
        (model) =>
          model.status === undefined ||
          model.status === null ||
          model.status === 1,
      ),
    [models],
  );

  const enabledVideoModels = useMemo(
    () =>
      videoModels.filter(
        (model) =>
          model.status === undefined ||
          model.status === null ||
          model.status === 1,
      ),
    [videoModels],
  );

  const chatModelOptionMap = useMemo(() => {
    const nextMap = new Map();
    chatModels.forEach((item) => {
      const requestModel = String(item?.request_model || '').trim();
      if (requestModel) {
        nextMap.set(requestModel, item);
      }
    });
    return nextMap;
  }, [chatModels]);

  function getCanvasModelDisplayName(item) {
    const displayName = String(item?.display_name || '').trim();
    if (displayName) {
      return displayName;
    }
    return String(item?.request_model || '').trim();
  }

  const getChatModelDisplayText = (item) => getCanvasModelDisplayName(item);

  const buildUnavailableChatModelOption = (requestModel, reasonText) => ({
    request_model: String(requestModel || '').trim(),
    display_name: String(requestModel || '').trim(),
    model_series: '',
    request_endpoint: '',
    description: '',
    vendor_icon: '',
    chat_capabilities: [],
    usable: false,
    unavailable_reason: reasonText || t('当前会话模型，现不可用'),
    available_groups: [],
  });

  const buildChatModelOptionLabel = (item) => {
    const text = getChatModelDisplayText(item);
    const unavailableReason =
      item?.usable === false
        ? String(item?.unavailable_reason || t('当前不可用'))
        : '';
    return (
      <span style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
        <span>{text || t('未命名模型')}</span>
        {unavailableReason ? (
          <span
            style={{
              fontSize: 12,
              color: 'var(--canvas-text-muted)',
            }}
          >
            {unavailableReason}
          </span>
        ) : null}
      </span>
    );
  };

  const getChatModelsWithPreservedCurrent = (currentModel, reasonText) => {
    const normalizedCurrentModel = String(currentModel || '').trim();
    if (!normalizedCurrentModel) {
      return chatModels;
    }
    if (chatModelOptionMap.has(normalizedCurrentModel)) {
      return chatModels;
    }
    return [
      buildUnavailableChatModelOption(normalizedCurrentModel, reasonText),
      ...chatModels,
    ];
  };

  const activeChatModelOption = useMemo(() => {
    const normalizedChatModel = String(chatModel || '').trim();
    if (!normalizedChatModel) {
      return null;
    }
    const existing = chatModelOptionMap.get(normalizedChatModel);
    if (existing) {
      return existing;
    }
    const isCurrentSessionModel =
      normalizedChatModel ===
      String(selectedCanvasSession?.current_model || '').trim();
    return buildUnavailableChatModelOption(
      normalizedChatModel,
      isCurrentSessionModel
        ? t('当前会话模型，现不可用')
        : t('当前选择的模型，现不可用'),
    );
  }, [chatModel, chatModelOptionMap, selectedCanvasSession?.current_model, t]);

  useEffect(() => {
    const uploadVisibility = getCanvasChatUploadVisibility(
      activeChatModelOption,
    );
    if (!uploadVisibility.showImageUpload) {
      setChatImageAttachment(null);
    }
    if (!uploadVisibility.showFileUpload) {
      setChatFileAttachment(null);
    }
    if (!getCanvasChatWebSearchVisibility(activeChatModelOption)) {
      setChatWebSearchEnabled(false);
    }
  }, [activeChatModelOption]);

  useEffect(() => {
    if (generationMode === CANVAS_MODE_CHAT) {
      return;
    }
    chatComposerDragDepthRef.current = 0;
    setChatComposerDragActive(false);
  }, [generationMode]);

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
    loadChatModels({ silent: true });
    connectSSE();

    return () => {
      disconnectSSE();
      stopPolling();
    };
  }, []);

  useEffect(() => {
    if (generationMode !== CANVAS_MODE_CHAT || chatModels.length > 0) {
      return;
    }
    loadChatModels();
  }, [generationMode, chatModels.length]);

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
      setGenerationMode(CANVAS_MODE_IMAGE);
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
          setGenerationMode(CANVAS_MODE_IMAGE);
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
            setGenerationMode(CANVAS_MODE_VIDEO);
            setVideoTaskModalVisible(true);
          } else {
            showError(res.data.message || t('加载任务详情失败'));
          }
          return;
        }
        const res = await API.get(`/api/image-generation/tasks/${taskId}`);
        if (res.data.success) {
          setSelectedTask(res.data.data);
          setGenerationMode(CANVAS_MODE_IMAGE);
          setTaskModalVisible(false);
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
    if (generationMode === CANVAS_MODE_VIDEO) {
      loadVideoTasks();
      return;
    }
    if (generationMode === CANVAS_MODE_IMAGE) {
      loadTasks();
    }
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

  useEffect(() => {
    loadRecentCanvasSessions({ reset: true });
  }, []);

  useEffect(() => {
    const nextRouteId = routeSessionIdNormalized;
    routeSessionLookupTargetRef.current = nextRouteId;
    const previousRouteId = previousRouteSessionIdRef.current;
    const routeSelectionAction = getRouteSelectionSyncAction({
      routeSessionId: nextRouteId,
      previousRouteSessionId: previousRouteId,
      selectedSessionIdsByMode: selectedCanvasSessionIdsRef.current,
      findCanvasSessionByIdentifier,
      getCanvasSessionIdentifier,
      canvasModes: CANVAS_MODES,
      defaultMode: CANVAS_MODE_IMAGE,
    });
    previousRouteSessionIdRef.current = nextRouteId;
    if (routeSelectionAction.type === 'noop') {
      return;
    }
    if (routeSelectionAction.type === 'clear-selection') {
      setSelectedCanvasSessionIds((prev) => {
        if (
          prev[CANVAS_MODE_CHAT] === null &&
          prev[CANVAS_MODE_IMAGE] === null &&
          prev[CANVAS_MODE_VIDEO] === null
        ) {
          return prev;
        }
        return {
          ...prev,
          [CANVAS_MODE_CHAT]: null,
          [CANVAS_MODE_IMAGE]: null,
          [CANVAS_MODE_VIDEO]: null,
        };
      });
      return;
    }
    if (routeSyncInFlightRef.current) {
      return;
    }
    if (routeSelectionAction.type === 'select-route-session') {
      const { sessionIdentifier, sessionMode = CANVAS_MODE_IMAGE } =
        routeSelectionAction;
      setGenerationMode((current) =>
        current === sessionMode ? current : sessionMode,
      );
      setSelectedCanvasSessionIds((prev) => {
        if (prev[sessionMode] === sessionIdentifier) {
          return prev;
        }
        return {
          ...prev,
          [sessionMode]: sessionIdentifier,
        };
      });
      return;
    }

    const requestSeq = ++routeSessionLookupRequestSeqRef.current;
    (async () => {
      const loadedSession = await loadCanvasSessionByRouteId(nextRouteId);
      if (
        requestSeq !== routeSessionLookupRequestSeqRef.current ||
        routeSessionLookupTargetRef.current !== nextRouteId
      ) {
        return;
      }
      if (!loadedSession) {
        syncCanvasRouteToBlank({ replace: true });
        return;
      }
      const sessionIdentifier = getCanvasSessionIdentifier(loadedSession);
      const sessionMode = CANVAS_MODES.includes(loadedSession.mode)
        ? loadedSession.mode
        : CANVAS_MODE_IMAGE;
      setGenerationMode(sessionMode);
      setSelectedCanvasSessionIds((prev) => {
        if (prev[sessionMode] === sessionIdentifier) {
          return prev;
        }
        return {
          ...prev,
          [sessionMode]: sessionIdentifier,
        };
      });
    })();
  }, [findCanvasSessionByIdentifier, routeSessionIdNormalized]);

  useEffect(() => {
    const targetSessionId = visibleCanvasSessionId;
    if (!targetSessionId) {
      canvasMessagesRequestSeqRef.current += 1;
      canvasMessagesSessionIdRef.current = null;
      setCanvasMessagesSessionId(null);
      setCanvasMessages([]);
      setCanvasMessagesHasMore(false);
      setCanvasMessagesNextCursor('');
      setCanvasMessagesError('');
      setCanvasMessagesLoading(false);
      setCanvasMessagesLoadingMore(false);
      return;
    }
    loadCanvasMessages(targetSessionId);
  }, [visibleCanvasSessionId]);

  useEffect(() => {
    if (generationMode !== CANVAS_MODE_CHAT || routeSyncInFlightRef.current) {
      return;
    }
    const chatRouteAction = getChatRouteSyncAction({
      generationMode,
      chatMode: CANVAS_MODE_CHAT,
      routeSessionId: routeSessionIdNormalized,
      selectedChatSessionId,
      findCanvasSessionByIdentifier,
    });
    if (chatRouteAction.type === 'sync-route-to-selected-chat') {
      syncCanvasRouteToSession(chatRouteAction.sessionId, { replace: true });
      return;
    }
    if (chatRouteAction.type === 'sync-route-to-blank') {
      syncCanvasRouteToBlank({ replace: true });
    }
  }, [
    findCanvasSessionByIdentifier,
    generationMode,
    routeSessionIdNormalized,
    selectedChatSessionId,
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
    setVideoTaskHasMore(false);
    setVideoTaskNextCursor('');
    videoTaskCursorHistoryRef.current = [''];
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

        const seriesList = Array.from(
          new Set(
            drawingModels.map((model) => model.model_series).filter(Boolean),
          ),
        );
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

  const loadChatModels = async ({ silent = false } = {}) => {
    if (chatModelsRequestRef.current) {
      if (!silent) {
        chatModelsRequestNotifyRef.current = true;
      }
      return chatModelsRequestRef.current;
    }

    chatModelsRequestNotifyRef.current = !silent;
    const request = (async () => {
      try {
        const res = await API.get('/api/canvas/chat-models');
        if (!res.data.success) {
          if (chatModelsRequestNotifyRef.current) {
            showError(res.data.message || t('加载聊天模型失败'));
          }
          return [];
        }
        const items = normalizeCanvasChatModelRecords(res.data.data);
        setChatModels(items);
        cacheChatModels(items);
        setChatModel((current) => {
          const normalizedCurrent = String(current || '').trim();
          const sessionCurrentModel =
            selectedCanvasSessionRef.current?.mode === CANVAS_MODE_CHAT
              ? String(
                  selectedCanvasSessionRef.current?.current_model || '',
                ).trim()
              : '';
          if (sessionCurrentModel) {
            return sessionCurrentModel;
          }
          if (
            normalizedCurrent &&
            items.some((item) => item?.request_model === normalizedCurrent)
          ) {
            return normalizedCurrent;
          }
          const firstUsable = items.find((item) => item?.usable !== false);
          return firstUsable?.request_model || items[0]?.request_model || '';
        });
        return items;
      } catch (error) {
        if (chatModelsRequestNotifyRef.current) {
          showError(error.message || t('加载聊天模型失败'));
        }
        return [];
      } finally {
        chatModelsRequestRef.current = null;
        chatModelsRequestNotifyRef.current = false;
      }
    })();
    chatModelsRequestRef.current = request;
    return request;
  };

  const loadCanvasAssets = async (nextPage = 1) => {
    setCanvasAssetsLoading(true);
    setCanvasAssetsError('');
    try {
      const res = await API.get('/api/image-generation/assets', {
        params: {
          p: nextPage,
          page_size: DEFAULT_ASSET_PAGE_SIZE,
          sort_by: 'created_time',
          sort_order: 'desc',
        },
      });
      if (!res.data.success) {
        const message = res.data.message || t('加载资产失败');
        setCanvasAssetsError(message);
        showError(message);
        return;
      }
      const data = res.data.data || {};
      setCanvasAssets(data.items || []);
      setCanvasAssetsTotal(data.total || 0);
      setCanvasAssetPage(data.page || nextPage);
      setSelectedCanvasAssetIds(new Set());
    } catch (error) {
      const message = error.message || t('加载资产失败');
      setCanvasAssetsError(message);
      showError(message);
    } finally {
      setCanvasAssetsLoading(false);
    }
  };

  const setCanvasSessionsForMode = (mode, updater) => {
    setCanvasSessions((prev) => ({
      ...prev,
      [mode]:
        typeof updater === 'function'
          ? updater(prev[mode] || [])
          : updater || [],
    }));
  };

  const mergeCanvasSessionsIntoState = (sessions) => {
    if (!sessions?.length) {
      return;
    }
    const grouped = {
      [CANVAS_MODE_CHAT]: [],
      [CANVAS_MODE_IMAGE]: [],
      [CANVAS_MODE_VIDEO]: [],
    };
    sessions.forEach((session) => {
      const normalized = normalizeCanvasSessionRecord(session);
      if (!normalized) {
        return;
      }
      grouped[normalized.mode].push(normalized);
    });
    setCanvasSessions((prev) => {
      const next = { ...prev };
      CANVAS_MODES.forEach((mode) => {
        if (grouped[mode].length === 0) {
          return;
        }
        next[mode] = mergeCanvasSessionItems(prev[mode] || [], grouped[mode]);
      });
      return next;
    });
  };

  const upsertCanvasSessionInCollections = (session) => {
    const normalized = normalizeCanvasSessionRecord(session);
    if (!normalized) {
      return;
    }
    mergeCanvasSessionsIntoState([normalized]);
    setRecentCanvasSessions((prev) => ({
      ...prev,
      items: mergeCanvasSessionItems(prev.items, [normalized]),
    }));
  };

  const removeCanvasSessionFromCollections = (session) => {
    const sessionIdentifier = getCanvasSessionIdentifier(session);
    if (!sessionIdentifier) {
      return;
    }
    const normalizedMode = CANVAS_MODES.includes(session.mode)
      ? session.mode
      : CANVAS_MODE_IMAGE;
    setCanvasSessionsForMode(normalizedMode, (prev) =>
      prev.filter(
        (item) => !canvasSessionMatchesIdentifier(item, sessionIdentifier),
      ),
    );
    setRecentCanvasSessions((prev) => ({
      ...prev,
      items: prev.items.filter(
        (item) => !canvasSessionMatchesIdentifier(item, sessionIdentifier),
      ),
    }));
  };

  const syncCanvasSessionSelectionDefaults = (sessions) => {
    if (!sessions?.length) {
      return;
    }
    if (routeSessionIdNormalized) {
      return;
    }
  };

  const loadRecentCanvasSessions = async (options = {}) => {
    const reset = !!options.reset;
    const silent = !!options.silent;
    const nextOffset = reset
      ? 0
      : Math.max(0, recentCanvasSessions.offset || 0);
    const requestedLimit = Number.parseInt(options.limit, 10);
    const limit =
      Number.isFinite(requestedLimit) && requestedLimit > 0
        ? Math.min(requestedLimit, MAX_CANVAS_SESSION_RECENT_PAGE_SIZE)
        : DEFAULT_CANVAS_SESSION_RECENT_PAGE_SIZE;
    const requestSeq = recentCanvasSessionsRequestSeqRef.current + 1;
    recentCanvasSessionsRequestSeqRef.current = requestSeq;
    setRecentCanvasSessions((prev) => ({
      ...prev,
      initialLoading: reset ? !silent : prev.initialLoading,
      loadingMore: reset ? false : !silent,
      error: reset && !silent ? '' : prev.error,
    }));
    try {
      const res = await API.get('/api/canvas/sessions', {
        params: {
          limit,
          offset: nextOffset,
        },
      });
      if (requestSeq !== recentCanvasSessionsRequestSeqRef.current) {
        return [];
      }
      if (!res.data.success) {
        const message = res.data.message || t('加载会话失败');
        setRecentCanvasSessions((prev) => ({
          ...prev,
          initialLoading: false,
          loadingMore: false,
          error: message,
        }));
        if (!silent) {
          showError(message);
        }
        return [];
      }
      const items = Array.isArray(res.data.data?.items)
        ? res.data.data.items
            .map((session) => normalizeCanvasSessionRecord(session))
            .filter(Boolean)
        : [];
      mergeCanvasSessionsIntoState(items);
      syncCanvasSessionSelectionDefaults(items);
      setRecentCanvasSessions((prev) => ({
        ...prev,
        items: reset ? items : mergeCanvasSessionItems(prev.items, items),
        hasMore: Boolean(res.data.data?.has_more),
        offset: nextOffset + items.length,
        initialLoading: false,
        loadingMore: false,
        error: '',
      }));
      return items;
    } catch (error) {
      if (requestSeq !== recentCanvasSessionsRequestSeqRef.current) {
        return [];
      }
      const message = error.message || t('加载会话失败');
      setRecentCanvasSessions((prev) => ({
        ...prev,
        initialLoading: false,
        loadingMore: false,
        error: message,
      }));
      if (!silent) {
        showError(message);
      }
      return [];
    }
  };

  const refreshRecentCanvasSessions = async (options = {}) => {
    const loadedCount =
      Number(recentCanvasSessions.offset) > 0
        ? Number(recentCanvasSessions.offset)
        : DEFAULT_CANVAS_SESSION_RECENT_PAGE_SIZE;
    return loadRecentCanvasSessions({
      reset: true,
      silent: options.silent !== false,
      limit: loadedCount,
    });
  };

  const createCanvasSession = async (mode = generationMode, title = '') => {
    const normalizedMode = CANVAS_MODES.includes(mode) ? mode : generationMode;
    const payload = {
      mode: normalizedMode,
      title,
    };
    if (normalizedMode === CANVAS_MODE_CHAT) {
      const chatTemperatureValue = Number(chatTemperature);
      const chatContextValue = Number(chatContext);
      payload.current_model = chatModel;
      if (Number.isFinite(chatTemperatureValue)) {
        payload.chat_temperature = chatTemperatureValue;
      }
      if (Number.isFinite(chatContextValue)) {
        payload.chat_context_count = chatContextValue;
      }
      payload.web_search_enabled =
        !!chatWebSearchEnabled &&
        getCanvasChatWebSearchVisibility(activeChatModelOption);
    }
    const res = await API.post('/api/canvas/sessions', payload);
    if (!res.data.success) {
      throw new Error(res.data.message || t('创建会话失败'));
    }
    const session = res.data.data;
    const sessionIdentifier = getCanvasSessionIdentifier(session);
    upsertCanvasSessionInCollections(session);
    setSelectedCanvasSessionIds((prev) => ({
      ...prev,
      [normalizedMode]: sessionIdentifier,
    }));
    canvasMessagesRequestSeqRef.current += 1;
    canvasMessagesSessionIdRef.current = sessionIdentifier;
    setCanvasMessagesSessionId(sessionIdentifier);
    setCanvasMessages([]);
    setCanvasMessagesHasMore(false);
    setCanvasMessagesNextCursor('');
    setCanvasMessagesError('');
    setCanvasMessagesLoading(false);
    setCanvasMessagesLoadingMore(false);
    setMobileTaskbarVisible(false);
    if (normalizedMode === CANVAS_MODE_CHAT || !routeSessionIdNormalized) {
      syncCanvasRouteToSession(session, { replace: true });
    }
    return session;
  };

  const ensureCanvasSession = async (mode = generationMode) => {
    const existingId = selectedCanvasSessionIds[mode];
    const existing = findCanvasSessionByIdentifier(existingId, mode);
    if (existing) {
      return existing;
    }
    return createCanvasSession(mode);
  };

  const loadCanvasMessages = async (sessionId, options = {}) => {
    if (!sessionId) {
      canvasMessagesRequestSeqRef.current += 1;
      canvasMessagesSessionIdRef.current = null;
      setCanvasMessagesSessionId(null);
      setCanvasMessages([]);
      setCanvasMessagesHasMore(false);
      setCanvasMessagesNextCursor('');
      setCanvasMessagesLoadingMore(false);
      return [];
    }
    const requestSeq = canvasMessagesRequestSeqRef.current + 1;
    const isSameSession = canvasMessagesSessionIdRef.current === sessionId;
    canvasMessagesRequestSeqRef.current = requestSeq;
    canvasMessagesSessionIdRef.current = sessionId;
    setCanvasMessagesSessionId(sessionId);
    setCanvasMessagesLoadingMore(false);
    if (!isSameSession) {
      setCanvasMessages([]);
      setCanvasMessagesHasMore(false);
      setCanvasMessagesNextCursor('');
      setCanvasMessagesError('');
    }
    if (!options.silent) {
      setCanvasMessagesLoading(true);
    }
    try {
      const res = await API.get(
        buildCanvasSessionApiPath(sessionId, '/messages'),
        {
          params: {
            limit: DEFAULT_CANVAS_MESSAGE_PAGE_SIZE,
          },
        },
      );
      if (
        requestSeq !== canvasMessagesRequestSeqRef.current ||
        !isCurrentCanvasMessageSession(sessionId)
      ) {
        return [];
      }
      if (!res.data.success) {
        const message = res.data.message || t('加载消息失败');
        setCanvasMessagesError(message);
        if (!options.silent) {
          showError(message);
        }
        return [];
      }
      const page = res.data.data || {};
      const messages = Array.isArray(page.items) ? page.items : [];
      const mergeResult =
        isSameSession && canvasMessagesRef.current.length > 0
          ? mergeCanvasLatestTimelinePage(canvasMessagesRef.current, messages)
          : {
              messages: messages.slice().sort(sortCanvasMessagesByCreated),
              hasOlderLoadedHistory: false,
            };
      setCanvasMessages(mergeResult.messages);
      if (!mergeResult.hasOlderLoadedHistory) {
        setCanvasMessagesHasMore(page.has_more === true);
        setCanvasMessagesNextCursor(page.next_cursor || '');
      }
      setCanvasMessagesError('');
      syncSelectedTaskFromCanvasMessages(mergeResult.messages);
      return mergeResult.messages;
    } catch (error) {
      if (
        requestSeq !== canvasMessagesRequestSeqRef.current ||
        !isCurrentCanvasMessageSession(sessionId)
      ) {
        return [];
      }
      const message = error.message || t('加载消息失败');
      setCanvasMessagesError(message);
      if (!options.silent) {
        showError(message);
      }
      return [];
    } finally {
      if (
        !options.silent &&
        requestSeq === canvasMessagesRequestSeqRef.current &&
        isCurrentCanvasMessageSession(sessionId)
      ) {
        setCanvasMessagesLoading(false);
      }
    }
  };

  const loadCanvasSessionByRouteId = async (identifier) => {
    const targetIdentifier = String(identifier || '').trim();
    if (!targetIdentifier) {
      return null;
    }
    try {
      const res = await API.get(buildCanvasSessionApiPath(targetIdentifier));
      if (!res.data.success || !res.data.data) {
        return null;
      }
      const session = normalizeCanvasSessionRecord(res.data.data);
      if (!session) {
        return null;
      }
      upsertCanvasSessionInCollections(session);
      return session;
    } catch (error) {
      return null;
    }
  };

  const loadOlderCanvasMessages = async (sessionId) => {
    if (
      !sessionId ||
      canvasMessagesLoadingMore ||
      !canvasMessagesHasMore ||
      !canvasMessagesNextCursor
    ) {
      return [];
    }
    const container = canvasMessageViewportRef.current;
    const previousScrollTop = container?.scrollTop || 0;
    const previousScrollHeight = container?.scrollHeight || 0;
    setCanvasMessagesLoadingMore(true);
    try {
      const res = await API.get(
        buildCanvasSessionApiPath(sessionId, '/messages'),
        {
          params: {
            limit: DEFAULT_CANVAS_MESSAGE_PAGE_SIZE,
            cursor: canvasMessagesNextCursor,
          },
        },
      );
      if (!isCurrentCanvasMessageSession(sessionId)) {
        return [];
      }
      if (!res.data.success) {
        throw new Error(res.data.message || t('加载消息失败'));
      }
      const page = res.data.data || {};
      const items = Array.isArray(page.items) ? page.items : [];
      setCanvasMessages((prev) => mergeCanvasMessagesById(prev, items));
      setCanvasMessagesHasMore(page.has_more === true);
      setCanvasMessagesNextCursor(page.next_cursor || '');
      if (
        generationMode === CANVAS_MODE_CHAT &&
        container &&
        items.length > 0
      ) {
        window.requestAnimationFrame(() => {
          const activeContainer = canvasMessageViewportRef.current;
          if (!activeContainer || !isCurrentCanvasMessageSession(sessionId)) {
            return;
          }
          activeContainer.scrollTop = Math.max(
            0,
            activeContainer.scrollHeight -
              previousScrollHeight +
              previousScrollTop,
          );
        });
      }
      return items;
    } catch (error) {
      if (!isCurrentCanvasMessageSession(sessionId)) {
        return [];
      }
      showError(error.message || t('加载消息失败'));
      return [];
    } finally {
      if (isCurrentCanvasMessageSession(sessionId)) {
        setCanvasMessagesLoadingMore(false);
      }
    }
  };

  const appendCanvasMessagesForSession = (sessionId, messages) => {
    if (
      !sessionId ||
      !messages?.length ||
      !isCurrentCanvasMessageSession(sessionId)
    ) {
      return;
    }
    setCanvasMessages((prev) => mergeCanvasMessagesById(prev, messages));
  };

  const bumpChatStreamRenderVersion = () => {
    setChatStreamRenderVersion((current) => current + 1);
  };

  const upsertCanvasMessagesForSession = (sessionId, messages) => {
    if (
      !sessionId ||
      !messages?.length ||
      !isCurrentCanvasMessageSession(sessionId)
    ) {
      return;
    }
    setCanvasMessages((prev) =>
      upsertCanvasMessages(prev, messages, canvasMessageIndexesRef.current),
    );
  };

  const appendCanvasChatDeltaForSession = (
    sessionId,
    messageId,
    delta,
    reasoningDelta,
  ) => {
    if (!sessionId || !messageId || !isCurrentCanvasMessageSession(sessionId)) {
      return;
    }
    if (!delta && !reasoningDelta) {
      return;
    }
    setCanvasMessages((prev) =>
      updateCanvasMessageById(
        prev,
        messageId,
        (message) => ({
          ...message,
          prompt: `${String(message?.prompt || '')}${delta || ''}`,
          reasoning_content: `${String(
            message?.reasoning_content || '',
          )}${reasoningDelta || ''}`,
        }),
        canvasMessageIndexesRef.current,
      ),
    );
  };

  const replaceCanvasMessagesForSession = (
    sessionId,
    requestId,
    messages,
    options = {},
  ) => {
    if (
      !sessionId ||
      !requestId ||
      !messages?.length ||
      !isCurrentCanvasMessageSession(sessionId)
    ) {
      return;
    }
    const nextMessages = messages.map((message) => ({
      ...message,
      client_request_id: requestId,
      canvas_aspect_ratio:
        message.canvas_aspect_ratio || options.canvasAspectRatio || '',
    }));
    setCanvasMessages((prev) => {
      return replaceCanvasMessagesByRequestId(prev, requestId, nextMessages);
    });
  };

  const generateCanvasClientRequestId = () => {
    canvasMessageClientRequestSeqRef.current += 1;
    return `canvas-client-${Date.now()}-${canvasMessageClientRequestSeqRef.current}`;
  };

  const removeCanvasMessagesForRequest = (sessionId, requestId) => {
    if (!sessionId || !requestId || !isCurrentCanvasMessageSession(sessionId)) {
      return;
    }
    setCanvasMessages((prev) =>
      removeCanvasMessagesByRequestId(prev, requestId),
    );
  };

  const buildCanvasChatOptimisticMetadata = (attachments) => {
    if (!Array.isArray(attachments) || attachments.length === 0) {
      return '';
    }
    return JSON.stringify({
      attachments,
    });
  };

  const buildOptimisticCanvasChatMessages = ({
    prompt,
    attachments,
    requestId,
    submittedAt,
  }) => [
    {
      id: `${requestId}-user`,
      role: 'user',
      prompt,
      created_time: submittedAt,
      client_request_id: requestId,
      metadata: buildCanvasChatOptimisticMetadata(attachments),
    },
    {
      id: `${requestId}-assistant`,
      role: 'assistant',
      prompt: '',
      status: 'generating',
      created_time: submittedAt,
      client_request_id: requestId,
      reasoning_content: '',
      error_message: '',
    },
  ];

  const updateCanvasSessionInState = (session) => {
    upsertCanvasSessionInCollections(session);
  };

  const buildCanvasChatSessionSettingsDraft = (session) => ({
    model: String(session?.current_model || ''),
    temperature:
      session?.chat_temperature === 0 || Number(session?.chat_temperature)
        ? String(session?.chat_temperature)
        : DEFAULT_CHAT_TEMPERATURE,
    contextCount:
      session?.chat_context_count === 0 || Number(session?.chat_context_count)
        ? String(session?.chat_context_count)
        : DEFAULT_CHAT_CONTEXT_COUNT,
    webSearchEnabled:
      typeof session?.web_search_enabled === 'boolean'
        ? session.web_search_enabled
        : DEFAULT_CHAT_WEB_SEARCH_ENABLED,
    systemPrompt: String(session?.system_prompt || ''),
    summaryEnabled:
      typeof session?.summary_enabled === 'boolean'
        ? session.summary_enabled
        : true,
    summaryTriggerMessages:
      session?.summary_trigger_messages === 0 ||
      Number(session?.summary_trigger_messages)
        ? String(session?.summary_trigger_messages)
        : DEFAULT_CHAT_SUMMARY_TRIGGER_MESSAGES,
    summaryRecentMessages:
      session?.summary_recent_messages === 0 ||
      Number(session?.summary_recent_messages)
        ? String(session?.summary_recent_messages)
        : DEFAULT_CHAT_SUMMARY_RECENT_MESSAGES,
  });

  const openCanvasChatSessionSettings = (session) => {
    const sessionIdentifier = getCanvasSessionIdentifier(session);
    if (session?.mode !== CANVAS_MODE_CHAT || !sessionIdentifier) {
      return;
    }
    setChatSessionSettingsSessionId(sessionIdentifier);
    setChatSessionSettingsDraft(buildCanvasChatSessionSettingsDraft(session));
    setChatSessionSettingsVisible(true);
  };

  const closeCanvasChatSessionSettings = () => {
    setChatSessionSettingsVisible(false);
    setChatSessionSettingsSaving(false);
    setChatSessionSettingsSessionId(null);
    setChatSessionSettingsDraft(null);
  };

  const handleCanvasChatSessionSettingsField = (key, value) => {
    setChatSessionSettingsDraft((prev) =>
      prev
        ? {
            ...prev,
            [key]: value,
          }
        : prev,
    );
  };

  const saveCanvasChatSessionSettings = async () => {
    if (!chatSessionSettingsSessionId || !chatSessionSettingsDraft) {
      return;
    }
    const session = findCanvasSessionByIdentifier(chatSessionSettingsSessionId);
    if (!session) {
      showError(t('会话不存在'));
      return;
    }
    const selectedDraftModelOption =
      getChatModelsWithPreservedCurrent(
        chatSessionSettingsDraft.model,
        t('当前会话模型，现不可用'),
      ).find(
        (item) => item?.request_model === chatSessionSettingsDraft.model,
      ) || null;
    const webSearchVisible = getCanvasChatWebSearchVisibility(
      selectedDraftModelOption,
    );

    const payload = {
      current_model: chatSessionSettingsDraft.model,
      chat_temperature: Number(chatSessionSettingsDraft.temperature),
      chat_context_count: Number(chatSessionSettingsDraft.contextCount),
      web_search_enabled: webSearchVisible
        ? !!chatSessionSettingsDraft.webSearchEnabled
        : false,
      system_prompt: chatSessionSettingsDraft.systemPrompt,
      summary_enabled: !!chatSessionSettingsDraft.summaryEnabled,
      summary_trigger_messages: Number(
        chatSessionSettingsDraft.summaryTriggerMessages,
      ),
      summary_recent_messages: Number(
        chatSessionSettingsDraft.summaryRecentMessages,
      ),
    };

    if (!payload.current_model) {
      showError(t('请选择模型'));
      return;
    }
    if (
      !Number.isFinite(payload.chat_temperature) ||
      !Number.isFinite(payload.chat_context_count) ||
      !Number.isFinite(payload.summary_trigger_messages) ||
      !Number.isFinite(payload.summary_recent_messages)
    ) {
      showError(t('会话配置无效'));
      return;
    }

    setChatSessionSettingsSaving(true);
    try {
      const res = await API.patch(buildCanvasSessionApiPath(session), payload);
      if (!res.data.success || !res.data.data) {
        showError(res.data.message || t('保存会话设置失败'));
        return;
      }
      updateCanvasSessionInState(res.data.data);
      const sessionIdentifier = getCanvasSessionIdentifier(session);
      if (
        generationModeRef.current === CANVAS_MODE_CHAT &&
        selectedCanvasSessionIdsRef.current?.[CANVAS_MODE_CHAT] ===
          sessionIdentifier
      ) {
        setChatModel(String(res.data.data.current_model || ''));
        setChatTemperature(
          String(res.data.data.chat_temperature ?? DEFAULT_CHAT_TEMPERATURE),
        );
        setChatContext(
          String(
            res.data.data.chat_context_count ?? DEFAULT_CHAT_CONTEXT_COUNT,
          ),
        );
        setChatWebSearchEnabled(!!res.data.data.web_search_enabled);
      }
      showSuccess(t('会话设置已更新'));
      closeCanvasChatSessionSettings();
    } catch (error) {
      showError(error.message || t('保存会话设置失败'));
    } finally {
      setChatSessionSettingsSaving(false);
    }
  };

  const toggleCanvasSessionContext = async (session) => {
    if (session?.mode !== CANVAS_MODE_CHAT) {
      return;
    }
    if (
      chatStreaming &&
      chatStreamingSessionIdRef.current &&
      chatStreamingSessionIdRef.current === getCanvasSessionIdentifier(session)
    ) {
      showError(t('请先停止当前对话生成'));
      return;
    }
    const isRestoring = Number(session.clear_context_message_id) > 0;
    try {
      const res = await API.patch(
        buildCanvasSessionApiPath(session),
        isRestoring
          ? { clear_context_message_id: 0 }
          : { clear_context_to_latest: true },
      );
      if (!res.data.success || !res.data.data) {
        showError(
          res.data.message ||
            (isRestoring ? t('恢复上下文失败') : t('清空上下文失败')),
        );
        return;
      }
      updateCanvasSessionInState(res.data.data);
      showSuccess(
        isRestoring ? t('已恢复完整上下文') : t('后续消息将从新话题开始'),
      );
    } catch (error) {
      showError(
        error.message ||
          (isRestoring ? t('恢复上下文失败') : t('清空上下文失败')),
      );
    }
  };

  const buildCanvasStreamRequestUrl = (path) => {
    const baseURL = String(API.defaults.baseURL || '').replace(/\/$/, '');
    return `${baseURL}${path}`;
  };

  const stopChatStream = ({ syncUI = true } = {}) => {
    const controller = chatStreamAbortRef.current;
    if (controller) {
      controller.abort();
      chatStreamAbortRef.current = null;
    }
    const streamingSessionId = chatStreamingSessionIdRef.current;
    const streamingMessageId = chatStreamingMessageIdRef.current;
    chatStreamingSessionIdRef.current = null;
    chatStreamingMessageIdRef.current = null;
    setChatStreaming(false);
    if (syncUI && streamingSessionId && streamingMessageId) {
      upsertCanvasMessagesForSession(streamingSessionId, [
        {
          id: streamingMessageId,
          status: 'stopped',
        },
      ]);
      bumpChatStreamRenderVersion();
    }
  };

  const parseCanvasSSEPayload = (raw) => {
    const normalized = String(raw || '').replace(/\r\n/g, '\n');
    const blocks = normalized.split('\n\n');
    const events = [];
    let remainder = '';

    blocks.forEach((block, index) => {
      const isLastBlock = index === blocks.length - 1;
      if (isLastBlock && normalized.endsWith('\n\n') === false) {
        remainder = block;
        return;
      }

      const lines = block.split('\n');
      let eventType = 'message';
      const dataLines = [];

      lines.forEach((line) => {
        if (line.startsWith('event:')) {
          eventType = line.slice(6).trim() || 'message';
          return;
        }
        if (line.startsWith('data:')) {
          dataLines.push(line.slice(5).trimStart());
        }
      });

      if (dataLines.length > 0) {
        events.push({
          event: eventType,
          data: dataLines.join('\n'),
        });
      }
    });

    return { events, remainder };
  };

  const renameCanvasSession = async (session) => {
    const currentTitle = session?.title || '';
    const nextTitle = window.prompt(t('重命名会话'), currentTitle);
    if (nextTitle === null) {
      return;
    }
    const trimmedTitle = nextTitle.trim();
    if (!trimmedTitle) {
      showError(t('标题不能为空'));
      return;
    }
    try {
      const res = await API.patch(buildCanvasSessionApiPath(session), {
        title: trimmedTitle,
      });
      if (!res.data.success) {
        showError(res.data.message || t('重命名失败'));
        return;
      }
      updateCanvasSessionInState(res.data.data);
    } catch (error) {
      showError(error.message || t('重命名失败'));
    }
  };

  const toggleCanvasSessionPin = async (session) => {
    try {
      const res = await API.patch(buildCanvasSessionApiPath(session), {
        pinned: !session.pinned,
      });
      if (!res.data.success) {
        showError(res.data.message || t('更新置顶失败'));
        return;
      }
      updateCanvasSessionInState(res.data.data);
    } catch (error) {
      showError(error.message || t('更新置顶失败'));
    }
  };

  const deleteCanvasSession = async (session) => {
    const sessionIdentifier = getCanvasSessionIdentifier(session);
    if (!sessionIdentifier) {
      return;
    }
    setDeletingCanvasSession(true);
    try {
      const res = await API.delete(buildCanvasSessionApiPath(session));
      if (!res.data.success) {
        showError(res.data.message || t('删除会话失败'));
        return;
      }
      showSuccess(t('删除成功'));
      removeCanvasSessionFromCollections(session);
      setSelectedCanvasSessionIds((prev) => {
        if (prev[session.mode] !== sessionIdentifier) {
          return prev;
        }
        return {
          ...prev,
          [session.mode]: null,
        };
      });
      if (isCurrentCanvasMessageSession(sessionIdentifier)) {
        canvasMessagesRequestSeqRef.current += 1;
        canvasMessagesSessionIdRef.current = null;
        setCanvasMessagesSessionId(null);
        setCanvasMessages([]);
        setCanvasMessagesHasMore(false);
        setCanvasMessagesNextCursor('');
        setCanvasMessagesError('');
        setCanvasMessagesLoading(false);
        setCanvasMessagesLoadingMore(false);
      }
      if (routeSessionIdNormalized === sessionIdentifier) {
        syncCanvasRouteToBlank({ replace: true });
      }
      if (session.mode === CANVAS_MODE_IMAGE) {
        loadTasks(true, { forceRefresh: true });
      }
      if (session.mode === CANVAS_MODE_VIDEO) {
        loadVideoTasks();
      }
      refreshRecentCanvasSessions({ silent: true });
    } catch (error) {
      showError(error.message || t('删除会话失败'));
    } finally {
      setDeletingCanvasSession(false);
    }
  };

  useEffect(() => {
    if (!assetLibraryVisible) {
      return;
    }
    loadCanvasAssets(canvasAssetPage || 1);
  }, [assetLibraryVisible]);

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

  const getModelDisplayName = (model) => getCanvasModelDisplayName(model);

  function normalizeCanvasChatModelRecord(item) {
    const requestModel = String(item?.request_model || '').trim();
    if (!requestModel) {
      return null;
    }
    return {
      request_model: requestModel,
      display_name: getCanvasModelDisplayName(item) || requestModel,
      model_series: String(item?.model_series || '').trim(),
      request_endpoint: String(item?.request_endpoint || '').trim(),
      description: String(item?.description || '').trim(),
      vendor_icon: String(item?.vendor_icon || '').trim(),
      chat_capabilities: normalizeCanvasChatCapabilities(
        item?.chat_capabilities,
      ),
    };
  }

  const selectImageModelFromCatalog = (model) => {
    if (!model?.request_model) {
      return;
    }
    setGenerationMode(CANVAS_MODE_IMAGE);
    if (model.model_series) {
      setSelectedSeries(model.model_series);
    }
    setSelectedModel(model.request_model);
    updateCurrentCanvasSessionModel(CANVAS_MODE_IMAGE, model.request_model);
  };

  const selectVideoModelFromCatalog = (model) => {
    if (!model?.request_model) {
      return;
    }
    setGenerationMode(CANVAS_MODE_VIDEO);
    if (model.model_series) {
      setVideoSelectedSeries(model.model_series);
    }
    setVideoSelectedModel(model.request_model);
    updateCurrentCanvasSessionModel(CANVAS_MODE_VIDEO, model.request_model);
  };

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
        setCanvasMessages((prevMessages) =>
          applyCanvasTaskUpdates(prevMessages, newItems, 'image'),
        );
        const hasTotal = Number.isFinite(res.data.data.total);
        const newTotal = hasTotal ? res.data.data.total : taskTotal;
        setTaskListError('');
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
        setSelectedTask((prev) => {
          if (prev && newItems.some((task) => task.id === prev.id)) {
            return {
              ...prev,
              ...newItems.find((task) => task.id === prev.id),
            };
          }
          return prev || newItems[0] || null;
        });
      } else if (!silent) {
        const message = res.data.message || t('加载任务列表失败');
        setTaskListError(message);
        showError(message);
      }
    } catch (error) {
      if (requestSeq === taskListRequestSeqRef.current && !silent) {
        const message = error.message || t('加载任务列表失败');
        setTaskListError(message);
        showError(message);
      }
    } finally {
      if (!silent && requestSeq === taskListRequestSeqRef.current) {
        setLoadingTasks(false);
      }
    }
  };

  const loadVideoTasks = async (silent = false, options = {}) => {
    const { forceRefresh = false } = options;
    const queryState = videoTaskListStateRef.current || {
      page: videoTaskPage,
      pageSize: videoTaskPageSize,
      statusFilter: videoTaskStatusFilter,
      modelFilter: videoTaskModelFilter,
      timeFilter: videoTaskTimeFilter,
    };
    const shouldAdvanceSeq = !silent || forceRefresh;
    const requestSeq = shouldAdvanceSeq
      ? videoTaskListRequestSeqRef.current + 1
      : videoTaskListRequestSeqRef.current;
    if (shouldAdvanceSeq) {
      videoTaskListRequestSeqRef.current = requestSeq;
    }
    if (!silent) {
      setVideoLoadingTasks(true);
    }
    try {
      const params = {
        p: queryState.page,
        page_size: queryState.pageSize,
      };
      const cursorHistory = videoTaskCursorHistoryRef.current;
      params.cursor = cursorHistory[queryState.page - 1] || '';
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
      const res = await API.get('/api/video-generation/tasks', {
        params,
        timeout: TASK_LIST_REQUEST_TIMEOUT_MS,
        skipErrorHandler: true,
      });
      if (requestSeq !== videoTaskListRequestSeqRef.current) {
        return;
      }
      if (res.data.success) {
        const newItems = res.data.data.items || [];
        setCanvasMessages((prevMessages) =>
          applyCanvasTaskUpdates(prevMessages, newItems, 'video'),
        );
        const latestCompletedTime = newItems.reduce(
          (latest, task) => {
            const completedAt = Number(task?.completed_time) || 0;
            return completedAt > latest ? completedAt : latest;
          },
          Number(videoTaskUpdatesCompletedSinceRef.current) || 0,
        );
        if (latestCompletedTime > videoTaskUpdatesCompletedSinceRef.current) {
          videoTaskUpdatesCompletedSinceRef.current = latestCompletedTime;
        }
        const hasTotal = Number.isFinite(res.data.data.total);
        setVideoTaskListError('');
        setVideoTasks((prev) => {
          if (prev.length === newItems.length) {
            const unchanged = newItems.every((newTask, index) =>
              areVideoTasksVisuallyEquivalent(prev[index], newTask),
            );
            if (unchanged) {
              return prev;
            }
          }
          return newItems;
        });
        if (hasTotal) {
          setVideoTaskTotal(res.data.data.total);
        }
        const nextCursor = res.data.data.next_cursor || '';
        const nextCursorHistory = videoTaskCursorHistoryRef.current.slice(
          0,
          queryState.page,
        );
        if (nextCursor) {
          nextCursorHistory[queryState.page] = nextCursor;
        }
        videoTaskCursorHistoryRef.current = nextCursorHistory;
        setVideoTaskNextCursor(nextCursor);
        setVideoTaskHasMore(res.data.data.has_more === true);
        setVideoSelectedTask((prev) => {
          if (prev && newItems.some((task) => task.id === prev.id)) {
            return {
              ...prev,
              ...newItems.find((task) => task.id === prev.id),
            };
          }
          return prev || newItems[0] || null;
        });
      } else if (!silent) {
        const message = res.data.message || t('加载视频任务列表失败');
        setVideoTaskListError(message);
        showError(message);
      }
    } catch (error) {
      if (requestSeq === videoTaskListRequestSeqRef.current && !silent) {
        const message = error.message || t('加载视频任务列表失败');
        setVideoTaskListError(message);
        showError(message);
      }
    } finally {
      if (!silent && requestSeq === videoTaskListRequestSeqRef.current) {
        setVideoLoadingTasks(false);
      }
    }
  };

  const mergeTaskUpdates = (updates) => {
    if (!Array.isArray(updates) || updates.length === 0) {
      return;
    }
    setTasks((prevTasks) =>
      mergeTaskCollections(prevTasks, updates, taskPageSize),
    );
    setCanvasMessages((prevMessages) =>
      applyCanvasTaskUpdates(prevMessages, updates, 'image'),
    );
  };

  const mergeVideoTaskUpdates = (updates) => {
    if (!Array.isArray(updates) || updates.length === 0) {
      return;
    }
    setVideoTasks((prevTasks) =>
      mergeTaskCollections(prevTasks, updates, videoTaskPageSize),
    );
    setVideoSelectedTask((prevTask) => {
      if (!prevTask) {
        return prevTask;
      }
      const updatedTask = updates.find((task) => task?.id === prevTask.id);
      if (!updatedTask) {
        return prevTask;
      }
      return {
        ...prevTask,
        ...updatedTask,
      };
    });
    setCanvasMessages((prevMessages) =>
      applyCanvasTaskUpdates(prevMessages, updates, 'video'),
    );
  };

  const loadVideoTaskUpdates = async () => {
    if (!isDefaultVideoTaskViewState(videoTaskListStateRef.current)) {
      return;
    }
    const completedSince = Math.max(
      1,
      Number(videoTaskUpdatesCompletedSinceRef.current) || 0,
    );

    try {
      const res = await API.get('/api/video-generation/tasks/updates', {
        params: {
          completed_since: completedSince,
          limit: Math.max(
            (videoTaskListStateRef.current?.pageSize || videoTaskPageSize) * 2,
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
      if (latestCompletedTime > videoTaskUpdatesCompletedSinceRef.current) {
        videoTaskUpdatesCompletedSinceRef.current = latestCompletedTime;
      }
      mergeVideoTaskUpdates(res.data.data?.items || []);
    } catch (error) {
      console.error('Failed to load video task updates:', error);
    }
  };

  const updateCanvasMessageTask = (updatedTask, mode) => {
    if (!updatedTask?.id) {
      return;
    }
    const taskType =
      mode === CANVAS_MODE_VIDEO ? 'video_generation' : 'image_generation';
    setCanvasMessages((prevMessages) =>
      updateCanvasMessagesByTask(
        prevMessages,
        taskType,
        updatedTask.id,
        (message) => {
          if (mode === CANVAS_MODE_VIDEO) {
            return {
              ...message,
              status: updatedTask.status,
              error_message:
                updatedTask.error_message ||
                updatedTask.fail_reason ||
                message.error_message,
              video_task: {
                ...(message.video_task || {}),
                ...updatedTask,
              },
            };
          }
          return {
            ...message,
            status: updatedTask.status,
            error_message:
              updatedTask.error_message ||
              updatedTask.fail_reason ||
              message.error_message,
            image_task: {
              ...(message.image_task || {}),
              ...updatedTask,
            },
          };
        },
        canvasMessageIndexesRef.current,
      ),
    );
  };

  const syncSelectedTaskFromCanvasMessages = (messages) => {
    const items = Array.isArray(messages) ? messages : [];
    for (let index = items.length - 1; index >= 0; index -= 1) {
      const message = items[index];
      if (!message?.task_id) {
        continue;
      }
      const taskType = getCanvasMessageTaskType(message);
      if (taskType === 'image_generation' && message.image_task) {
        setSelectedTask(message.image_task);
        return;
      }
      if (taskType === 'video_generation' && message.video_task) {
        setVideoSelectedTask(message.video_task);
        return;
      }
    }
  };

  const getCanvasMessageReferenceFiles = (message) => {
    if (!message) {
      return [];
    }
    const taskType = getCanvasMessageTaskType(message);
    const task = message.image_task || message.video_task || null;
    const params = parseCanvasTaskParams(task?.params || task?.request_params);
    const references = [];
    const seen = new Set();
    const addFiles = (values, prefix) => {
      collectTaskReferenceValues(values).forEach((url, index) => {
        if (seen.has(url)) {
          return;
        }
        seen.add(url);
        references.push({
          uid: `${prefix}-${message.id || task?.id || 'message'}-${index}`,
          name: `${prefix}-${index + 1}`,
          url,
        });
      });
    };

    addFiles(
      message.reference_images || message.reference_image,
      'message-reference',
    );
    addFiles(
      params.reference_images || params.reference_image,
      'task-reference',
    );
    addFiles(
      task?.reference_images || task?.reference_image,
      taskType === 'video_generation' ? 'video-reference' : 'task-reference',
    );
    return references;
  };

  const getCanvasMessageReferencePreviewSrc = (message) => {
    const firstReference = getCanvasMessageReferenceFiles(message).find(
      (file) => file?.url,
    );
    return firstReference?.url || '';
  };

  const getCanvasMessageMedia = (message) => {
    if (!message) {
      return null;
    }
    const taskType = getCanvasMessageTaskType(message);
    if (taskType === 'image_generation') {
      const task = message.image_task || {};
      return {
        kind: 'image',
        status: task.status || message.status,
        src: task.thumbnail_url || task.image_url || '',
        previewSrc: task.image_url || task.thumbnail_url || '',
        error:
          task.error_message || task.fail_reason || message.error_message || '',
      };
    }
    if (taskType === 'video_generation') {
      const task = message.video_task || {};
      return {
        kind: 'video',
        status: task.status || message.status,
        src: task.thumbnail_url || task.result_url || task.video_url || '',
        error:
          task.error_message || task.fail_reason || message.error_message || '',
      };
    }
    return null;
  };

  const getCanvasMessageExplicitAspectRatio = (message) => {
    const task = message?.image_task || message?.video_task || {};
    const params = parseCanvasTaskParams(task?.params || task?.request_params);
    const metadata = parseCanvasTaskParams(task?.image_metadata);
    const metadataDetails = parseCanvasTaskParams(metadata?.metadata);
    const candidates = [
      message?.canvas_aspect_ratio,
      params.aspect_ratio,
      params.aspectRatio,
      task?.aspect_ratio,
      task?.aspectRatio,
      task?.resolution,
      task?.image_size,
      task?.imageSize,
      params.resolution,
      params.image_size,
      params.imageSize,
    ];
    for (const candidate of candidates) {
      const normalized = normalizeAspectRatioText(candidate);
      if (normalized) {
        return normalized;
      }
    }
    const dimensionSources = [task, params, metadata, metadataDetails];
    let outputWidth = 0;
    let outputHeight = 0;
    for (const source of dimensionSources) {
      if (!source || typeof source !== 'object') {
        continue;
      }
      outputWidth =
        outputWidth ||
        Number(
          source.output_width ||
            source.outputWidth ||
            source.image_width ||
            source.imageWidth ||
            source.width ||
            0,
        );
      outputHeight =
        outputHeight ||
        Number(
          source.output_height ||
            source.outputHeight ||
            source.image_height ||
            source.imageHeight ||
            source.height ||
            0,
        );
      if (outputWidth > 0 && outputHeight > 0) {
        return buildAspectRatioTextFromDimensions(outputWidth, outputHeight);
      }
      const normalizedSize = normalizeAspectRatioText(
        source.output_size_text ||
          source.outputSizeText ||
          source.size_text ||
          source.sizeText ||
          source.output_size ||
          source.outputSize ||
          source.size ||
          source.dimensions,
      );
      if (normalizedSize) {
        return normalizedSize;
      }
    }
    return '';
  };

  const getCanvasMessageAspectRatio = (message) => {
    return getCanvasMessageExplicitAspectRatio(message) || '1 / 1';
  };

  const getCanvasMediaCardSize = (
    aspectRatio,
    { batchLayout = false, batchAspectRatio = '', mediaKind = '' } = {},
  ) => {
    const isImageCard = mediaKind === 'image';
    if (batchLayout) {
      return {
        width: '100%',
        maxWidth: '100%',
        maxHeight: 'none',
        aspectRatio: isImageCard
          ? '1 / 1'
          : batchAspectRatio || aspectRatio || '1 / 1',
      };
    }
    if (isImageCard) {
      const fixedCardSize = isMobile ? 280 : 360;
      return {
        width: '100%',
        maxWidth: `${fixedCardSize}px`,
        maxHeight: `${fixedCardSize}px`,
        aspectRatio: '1 / 1',
      };
    }
    const match = String(aspectRatio || '').match(
      /^(\d+(?:\.\d+)?)\s*\/\s*(\d+(?:\.\d+)?)$/,
    );
    const ratio = match ? Number(match[1]) / Number(match[2]) : 1;
    const safeRatio = Number.isFinite(ratio) && ratio > 0 ? ratio : 1;
    const maxWidth = isMobile ? 320 : 420;
    const maxHeight = isMobile ? 280 : 360;
    const width = Math.min(maxWidth, Math.round(maxHeight * safeRatio));
    return {
      width: `${width}px`,
      maxWidth: '100%',
      aspectRatio,
    };
  };

  const mergeCanvasMessageTaskDetail = (message, detail) => {
    if (!detail) {
      return message;
    }
    if (getCanvasMessageTaskType(message) === 'video_generation') {
      return {
        ...message,
        video_task: {
          ...(message.video_task || {}),
          ...detail,
        },
      };
    }
    return {
      ...message,
      image_task: {
        ...(message.image_task || {}),
        ...detail,
      },
    };
  };

  const loadCanvasMessageTaskDetail = async (message) => {
    const task = message?.image_task || message?.video_task || null;
    const taskId = task?.id;
    if (!taskId) {
      return task;
    }

    const taskType = getCanvasMessageTaskType(message);
    const cacheKey = `${taskType || 'task'}:${taskId}`;
    if (canvasMessageDetailCacheRef.current.has(cacheKey)) {
      return canvasMessageDetailCacheRef.current.get(cacheKey);
    }

    const requestSeq =
      (canvasMessageDetailRequestSeqRef.current.get(cacheKey) || 0) + 1;
    canvasMessageDetailRequestSeqRef.current.set(cacheKey, requestSeq);

    try {
      const endpoint =
        taskType === 'video_generation'
          ? `/api/canvas/video-generation/tasks/${taskId}`
          : `/api/image-generation/tasks/${taskId}`;
      const res = await API.get(endpoint);
      if (
        requestSeq !== canvasMessageDetailRequestSeqRef.current.get(cacheKey)
      ) {
        return task;
      }
      if (res.data.success && res.data.data) {
        canvasMessageDetailCacheRef.current.set(cacheKey, res.data.data);
        return res.data.data;
      }
    } catch (error) {
      console.error('Failed to load canvas message task detail:', error);
    }

    canvasMessageDetailCacheRef.current.set(cacheKey, task);
    return task;
  };

  const attachPendingReferencesToMessages = (messages, referenceFiles) => {
    if (!Array.isArray(messages) || messages.length === 0) {
      return messages || [];
    }
    const files = (referenceFiles || []).filter((file) => file?.url);
    if (files.length === 0) {
      return messages;
    }

    return messages.map((message) => {
      if (!message?.task_id || message.role === 'user') {
        return message;
      }
      return {
        ...message,
        reference_images: files.map((file) => ({
          uid: file.uid || file.name || file.url,
          name: file.name || t('参考图'),
          url: file.url,
        })),
      };
    });
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

  const removeImageTasksFromLocalState = (taskIds, options = {}) => {
    const idSet = buildComparableIdSet(taskIds);
    if (idSet.size === 0) {
      return;
    }
    const hasExplicitDecrementTotal = Object.prototype.hasOwnProperty.call(
      options,
      'decrementTotal',
    );
    const parsedDecrementTotal = Number(options.decrementTotal);
    const fallbackDecrementTotal = tasks.filter((task) =>
      idSet.has(normalizeComparableId(task.id)),
    ).length;
    const decrementTotal =
      hasExplicitDecrementTotal && Number.isFinite(parsedDecrementTotal)
        ? parsedDecrementTotal
        : fallbackDecrementTotal;
    const selectedTaskRemoved =
      selectedTask && idSet.has(normalizeComparableId(selectedTask.id));

    setTasks((prevTasks) =>
      prevTasks.filter((task) => !idSet.has(normalizeComparableId(task.id))),
    );
    setSelectedTaskIds((prev) => {
      const next = new Set(prev);
      idSet.forEach((taskId) => {
        next.delete(taskId);
        next.delete(Number(taskId));
      });
      return next;
    });
    setTaskTotal((prev) => Math.max(0, prev - decrementTotal));
    if (selectedTaskRemoved) {
      taskDetailRequestSeqRef.current += 1;
      setTaskModalVisible(false);
    }
    setSelectedTask((prev) => {
      if (!prev || !idSet.has(normalizeComparableId(prev.id))) {
        return prev;
      }
      return null;
    });
  };

  const removeVideoTasksFromLocalState = (taskIds, options = {}) => {
    const idSet = buildComparableIdSet(taskIds);
    if (idSet.size === 0) {
      return;
    }
    const hasExplicitDecrementTotal = Object.prototype.hasOwnProperty.call(
      options,
      'decrementTotal',
    );
    const parsedDecrementTotal = Number(options.decrementTotal);
    const fallbackDecrementTotal = videoTasks.filter((task) =>
      idSet.has(normalizeComparableId(task.id)),
    ).length;
    const decrementTotal =
      hasExplicitDecrementTotal && Number.isFinite(parsedDecrementTotal)
        ? parsedDecrementTotal
        : fallbackDecrementTotal;
    const selectedTaskRemoved =
      videoSelectedTask &&
      idSet.has(normalizeComparableId(videoSelectedTask.id));

    setVideoTasks((prevTasks) =>
      prevTasks.filter((task) => !idSet.has(normalizeComparableId(task.id))),
    );
    setVideoSelectedTaskIds((prev) => {
      const next = new Set(prev);
      idSet.forEach((taskId) => {
        next.delete(taskId);
        next.delete(Number(taskId));
      });
      return next;
    });
    setVideoTaskTotal((prev) => Math.max(0, prev - decrementTotal));
    if (selectedTaskRemoved) {
      setVideoTaskModalVisible(false);
    }
    setVideoSelectedTask((prev) => {
      if (!prev || !idSet.has(normalizeComparableId(prev.id))) {
        return prev;
      }
      return null;
    });
  };

  const handleTaskCardClick = async (task) => {
    if (!task?.id) return;

    taskDetailRequestSeqRef.current += 1;
    const requestSeq = taskDetailRequestSeqRef.current;

    setSelectedTask(task);
    setTaskModalVisible(false);

    try {
      const res = await API.get(`/api/image-generation/tasks/${task.id}`);
      if (requestSeq !== taskDetailRequestSeqRef.current) {
        return;
      }
      if (res.data.success) {
        updateTaskInList(res.data.data);
        setSelectedTask(res.data.data);
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
        setVideoSelectedTask(res.data.data);
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
    setVideoTaskHasMore(false);
    setVideoTaskNextCursor('');
    videoTaskCursorHistoryRef.current = [''];
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
      const successfulResultIndexes = results
        .map((result, index) =>
          result.status === 'fulfilled' && result.value.data?.success
            ? index
            : -1,
        )
        .filter((index) => index >= 0);
      const successCount = successfulResultIndexes.length;
      const failCount = results.length - successCount;

      if (successCount > 0) {
        showSuccess(t('成功删除 {{count}} 个任务', { count: successCount }));
        const selectedIds = Array.from(selectedTaskIds);
        const deletedTaskIds = new Set(
          successfulResultIndexes.map((index) => selectedIds[index]),
        );
        removeImageTasksFromLocalState(deletedTaskIds);
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
      const selectedIds = Array.from(videoSelectedTaskIds);
      const successIds = new Set(
        selectedIds.filter((taskId, index) => {
          const result = results[index];
          return result?.status === 'fulfilled' && result.value.data?.success;
        }),
      );
      if (successIds.size > 0) {
        showSuccess(t('成功删除 {{count}} 个任务', { count: successIds.size }));
        removeVideoTasksFromLocalState(successIds);
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
      };

      sseRef.current = eventSource;
    } catch (error) {
      console.error('Failed to connect SSE:', error);
      setSseConnected(false);
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
      if (generationMode === CANVAS_MODE_VIDEO) {
        if (isDefaultVideoTaskViewState(videoTaskListStateRef.current)) {
          loadVideoTaskUpdates();
          return;
        }
        loadVideoTasks(true);
        return;
      }
      if (generationMode !== CANVAS_MODE_IMAGE) {
        return;
      }
      if (isDefaultTaskViewState(taskListStateRef.current)) {
        loadTaskUpdates();
        return;
      }
      loadTasks(true); // 静默刷新，不触发 Spin 遮罩
    }, pollingIntervalRef.current * 1000);
  };

  // 停止轮询
  const stopPolling = () => {
    if (pollingTimerRef.current) {
      clearInterval(pollingTimerRef.current);
      pollingTimerRef.current = null;
    }
  };

  useEffect(() => {
    const shouldPoll =
      isPageVisible &&
      (generationMode === CANVAS_MODE_VIDEO
        ? hasActiveVideoTasks
        : generationMode === CANVAS_MODE_IMAGE
          ? !sseConnected && hasActiveTasks
          : false);
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
    updateCanvasMessageTask(updatedTask, CANVAS_MODE_IMAGE);
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
    updateCanvasMessageTask(updatedTask, CANVAS_MODE_VIDEO);
    setVideoTasks((prev) => {
      const index = prev.findIndex((task) => task.id === updatedTask.id);
      if (index === -1) {
        if (isDefaultVideoTaskViewState(videoTaskListStateRef.current)) {
          return mergeTaskCollections(prev, [updatedTask], videoTaskPageSize);
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
      isDefaultVideoTaskViewState(videoTaskListStateRef.current) &&
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
    if (chatModel) {
      try {
        localStorage.setItem(STORAGE_KEYS.CHAT_MODEL, chatModel);
      } catch (e) {
        console.error('Failed to save chatModel:', e);
      }
    }
  }, [chatModel]);

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEYS.CHAT_TEMPERATURE, chatTemperature);
    } catch (e) {
      console.error('Failed to save chatTemperature:', e);
    }
  }, [chatTemperature]);

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEYS.CHAT_CONTEXT, chatContext);
    } catch (e) {
      console.error('Failed to save chatContext:', e);
    }
  }, [chatContext]);

  useEffect(() => {
    try {
      localStorage.setItem(
        STORAGE_KEYS.CHAT_WEB_SEARCH,
        chatWebSearchEnabled ? 'true' : 'false',
      );
    } catch (e) {
      console.error('Failed to save chatWebSearchEnabled:', e);
    }
  }, [chatWebSearchEnabled]);

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
    if (!selectedModelData || !modelSupportsImageEditing(selectedModelData)) {
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

  const readUploadFileAsDataURL = (file) =>
    new Promise((resolve, reject) => {
      if (!file?.fileInstance) {
        resolve(String(file?.url || ''));
        return;
      }
      const reader = new FileReader();
      reader.onload = (event) => resolve(String(event?.target?.result || ''));
      reader.onerror = reject;
      reader.readAsDataURL(file.fileInstance);
    });

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

  const buildCanvasChatUploadFile = (file, kind) => {
    if (!file) {
      return null;
    }
    return {
      uid: `chat-${kind}-${Date.now()}`,
      name: file.name || '',
      size: file.size || 0,
      type: file.type || '',
      fileInstance: file,
    };
  };

  const inferCanvasChatFileMimeType = (file) => {
    const explicitType = String(file?.type || file?.fileInstance?.type || '')
      .trim()
      .toLowerCase();
    if (explicitType) {
      return explicitType;
    }
    const name = String(file?.name || '')
      .trim()
      .toLowerCase();
    if (name.endsWith('.pdf')) {
      return 'application/pdf';
    }
    if (name.endsWith('.txt')) {
      return 'text/plain';
    }
    return '';
  };

  const isCanvasChatImageUploadFile = (file) => {
    const explicitType = String(file?.type || file?.fileInstance?.type || '')
      .trim()
      .toLowerCase();
    if (explicitType.startsWith('image/')) {
      return true;
    }
    const name = String(file?.name || '')
      .trim()
      .toLowerCase();
    return CANVAS_CHAT_IMAGE_FILE_NAME_PATTERN.test(name);
  };

  const getCanvasChatUploadAccept = (uploadVisibility) => {
    if (uploadVisibility.showImageUpload && uploadVisibility.showFileUpload) {
      return `image/*,${CANVAS_CHAT_SUPPORTED_FILE_ACCEPT}`;
    }
    if (uploadVisibility.showImageUpload) {
      return 'image/*';
    }
    if (uploadVisibility.showFileUpload) {
      return CANVAS_CHAT_SUPPORTED_FILE_ACCEPT;
    }
    return '';
  };

  const extractCanvasChatTransferFiles = (transfer) => {
    if (!transfer) {
      return [];
    }
    const itemFiles = Array.from(transfer.items || [])
      .filter((item) => item?.kind === 'file')
      .map((item) => item.getAsFile())
      .filter(Boolean);
    if (itemFiles.length > 0) {
      return itemFiles;
    }
    return Array.from(transfer.files || []).filter(Boolean);
  };

  const hasCanvasChatTransferFiles = (transfer) => {
    if (!transfer) {
      return false;
    }
    if (extractCanvasChatTransferFiles(transfer).length > 0) {
      return true;
    }
    return Array.from(transfer.types || []).includes('Files');
  };

  const getPreferredCanvasChatTransferFile = (files) => {
    if (!Array.isArray(files) || files.length === 0) {
      return null;
    }
    return (
      files.find((file) => isCanvasChatImageUploadFile(file)) ||
      files.find((file) =>
        CANVAS_CHAT_SUPPORTED_FILE_MIME_TYPES.has(
          inferCanvasChatFileMimeType(file),
        ),
      ) ||
      files[0]
    );
  };

  const handleChatImageUpload = async ({ fileList }) => {
    const selectedFile = fileList[0] || null;
    if (!selectedFile) {
      setChatImageAttachment(null);
      return;
    }
    try {
      const dataUrl = await readUploadFileAsDataURL(selectedFile);
      setChatImageAttachment({
        kind: 'image',
        uid: selectedFile.uid || `chat-image-${Date.now()}`,
        name: selectedFile.name || t('图片'),
        mime_type:
          selectedFile.fileInstance?.type ||
          String(dataUrl).slice(5, String(dataUrl).indexOf(';')) ||
          'image/*',
        data: dataUrl,
        url: dataUrl,
      });
    } catch (error) {
      showError(error.message || t('读取图片失败'));
    }
  };

  const validateCanvasChatFileUpload = (file) => {
    const mimeType = inferCanvasChatFileMimeType(file);
    if (!CANVAS_CHAT_SUPPORTED_FILE_MIME_TYPES.has(mimeType)) {
      showError(t('当前仅支持上传 PDF 或 TXT 文件'));
      return false;
    }
    return true;
  };

  const triggerHiddenChatUploadInput = (inputRef) => {
    const input = inputRef?.current;
    if (!input) {
      return;
    }
    input.value = '';
    input.click();
  };

  const handleCanvasChatAttachmentSelect = (selectedFile) => {
    if (!selectedFile) {
      return false;
    }
    const uploadVisibility = getCanvasChatUploadVisibility(
      activeChatModelOption,
    );
    if (isCanvasChatImageUploadFile(selectedFile)) {
      if (!uploadVisibility.showImageUpload) {
        showError(t('当前模型不支持图片上传'));
        return false;
      }
      const uploadFile = buildCanvasChatUploadFile(selectedFile, 'image');
      if (!uploadFile || !validateImageSize(uploadFile)) {
        return false;
      }
      void handleChatImageUpload({ fileList: [uploadFile] });
      return true;
    }

    const uploadFile = buildCanvasChatUploadFile(selectedFile, 'file');
    if (!uploadFile) {
      return false;
    }
    if (
      !CANVAS_CHAT_SUPPORTED_FILE_MIME_TYPES.has(
        inferCanvasChatFileMimeType(uploadFile),
      )
    ) {
      showError(t('当前仅支持上传图片、PDF 或 TXT 文件'));
      return false;
    }
    if (!uploadVisibility.showFileUpload) {
      showError(t('当前模型不支持文件上传'));
      return false;
    }
    if (!validateCanvasChatFileUpload(uploadFile)) {
      return false;
    }
    void handleChatFileUpload({ fileList: [uploadFile] });
    return true;
  };

  const handleChatAttachmentInputChange = (event) => {
    const selectedFile = event.target.files?.[0] || null;
    event.target.value = '';
    handleCanvasChatAttachmentSelect(selectedFile);
  };

  const handleChatFileUpload = async ({ fileList }) => {
    const selectedFile = fileList[0] || null;
    if (!selectedFile) {
      setChatFileAttachment(null);
      return;
    }
    try {
      const dataUrl = await readUploadFileAsDataURL(selectedFile);
      setChatFileAttachment({
        kind: 'file',
        uid: selectedFile.uid || `chat-file-${Date.now()}`,
        name: selectedFile.name || t('文件'),
        mime_type: inferCanvasChatFileMimeType(selectedFile),
        data: dataUrl,
      });
    } catch (error) {
      showError(error.message || t('读取文件失败'));
    }
  };

  const handleRemoveChatImageAttachment = () => {
    setChatImageAttachment(null);
  };

  const handleRemoveChatFileAttachment = () => {
    setChatFileAttachment(null);
  };

  const resetChatComposerDragState = () => {
    chatComposerDragDepthRef.current = 0;
    setChatComposerDragActive(false);
  };

  const handleChatComposerDragEnter = (event) => {
    if (!hasCanvasChatTransferFiles(event.dataTransfer)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    chatComposerDragDepthRef.current += 1;
    setChatComposerDragActive(true);
  };

  const handleChatComposerDragOver = (event) => {
    if (!hasCanvasChatTransferFiles(event.dataTransfer)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'copy';
    }
    if (!chatComposerDragActive) {
      setChatComposerDragActive(true);
    }
  };

  const handleChatComposerDragLeave = (event) => {
    if (!hasCanvasChatTransferFiles(event.dataTransfer)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    chatComposerDragDepthRef.current = Math.max(
      0,
      chatComposerDragDepthRef.current - 1,
    );
    if (chatComposerDragDepthRef.current === 0) {
      setChatComposerDragActive(false);
    }
  };

  const handleChatComposerDrop = (event) => {
    const transferFiles = extractCanvasChatTransferFiles(event.dataTransfer);
    if (transferFiles.length === 0) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    resetChatComposerDragState();
    handleCanvasChatAttachmentSelect(
      getPreferredCanvasChatTransferFile(transferFiles),
    );
  };

  const handleChatComposerPaste = (event) => {
    const transferFiles = extractCanvasChatTransferFiles(event.clipboardData);
    const selectedFile = getPreferredCanvasChatTransferFile(transferFiles);
    if (!selectedFile) {
      return;
    }
    event.preventDefault();
    handleCanvasChatAttachmentSelect(selectedFile);
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
      let canvasReferenceFiles = [];

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
        canvasReferenceFiles = base64Images
          .map((url, index) => ({
            uid: referenceImages[index]?.uid || `reference-${index}`,
            name: referenceImages[index]?.name || `${t('参考图')}-${index + 1}`,
            url,
          }))
          .filter((file) => file.url);
      }
      if (
        modelSupportsMaskEditing(selectedModelData) &&
        maskImage?.fileInstance
      ) {
        params.mask = await new Promise((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = (e) => resolve(e.target.result);
          reader.onerror = reject;
          reader.readAsDataURL(maskImage.fileInstance);
        });
      }

      const canvasSession = await ensureCanvasSession(CANVAS_MODE_IMAGE);
      const clientRequestId = generateCanvasClientRequestId();
      // UI uses inspiration wording; backend task DTO still expects prompt.
      const taskPayload = {
        model_id: selectedModel,
        group: selectedGroup,
        prompt: inspiration.trim(),
        request_endpoint: selectedModelData.request_endpoint,
        params: JSON.stringify(params),
        client_request_id: clientRequestId,
      };
      const submittedAt = Math.floor(Date.now() / 1000);
      const canvasSessionIdentifier = getCanvasSessionIdentifier(canvasSession);
      replaceCanvasMessagesForSession(
        canvasSessionIdentifier,
        clientRequestId,
        [
          {
            id: `${clientRequestId}-user`,
            role: 'user',
            prompt: inspiration.trim(),
            created_time: submittedAt,
            client_request_id: clientRequestId,
            canvas_aspect_ratio: aspectRatio || '',
          },
          ...Array.from({ length: taskCount }, (_, index) => ({
            id:
              index === 0
                ? `${clientRequestId}-assistant`
                : `${clientRequestId}-assistant-${index + 1}`,
            role: 'assistant',
            prompt: inspiration.trim(),
            status: 'generating',
            task_type: 'image_generation',
            created_time: submittedAt,
            client_request_id: clientRequestId,
            canvas_aspect_ratio: aspectRatio || '',
            reference_images: canvasReferenceFiles,
            image_task: {
              id:
                index === 0
                  ? `pending-${clientRequestId}`
                  : `pending-${clientRequestId}-${index + 1}`,
              status: 'generating',
              prompt: inspiration.trim(),
              model_id: selectedModel,
              selected_group: selectedGroup,
              aspect_ratio: aspectRatio || '',
              thumbnail_url: '',
              image_url: '',
              error_message: '',
            },
          })),
        ],
        {
          canvasAspectRatio: aspectRatio || '',
        },
      );
      const results = await Promise.allSettled(
        Array.from({ length: taskCount }, () =>
          API.post(
            buildCanvasSessionApiPath(canvasSession, '/messages'),
            taskPayload,
          ),
        ),
      );

      const createdTasks = [];
      const createdMessages = [];
      let firstError = '';

      results.forEach((result) => {
        if (result.status === 'fulfilled' && result.value.data.success) {
          const messages = result.value.data.data || [];
          createdMessages.push(...messages);
          messages.forEach((message) => {
            if (message?.image_task) {
              createdTasks.push(message.image_task);
            }
          });
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
        replaceCanvasMessagesForSession(
          canvasSessionIdentifier,
          clientRequestId,
          attachPendingReferencesToMessages(
            createdMessages,
            canvasReferenceFiles,
          ),
          { canvasAspectRatio: aspectRatio || '' },
        );
        refreshRecentCanvasSessions({ silent: true });
        setInspiration('');
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
        setSelectedTask(createdTasks[0]);
        loadTasks(false, {
          disableDuplicate: true,
          forceRefresh: true,
        });
      }

      if (createdTasks.length === 0) {
        removeCanvasMessagesForRequest(
          canvasSessionIdentifier,
          clientRequestId,
        );
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
    let canvasSession = null;
    let clientRequestId = '';
    try {
      let base64Image = '';
      let canvasReferenceFiles = [];
      if (videoReferenceImage?.fileInstance) {
        base64Image = await new Promise((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = (e) => resolve(e.target.result);
          reader.onerror = reject;
          reader.readAsDataURL(videoReferenceImage.fileInstance);
        });
      } else if (videoReferenceImage?.url) {
        base64Image = videoReferenceImage.url;
      }
      if (base64Image) {
        canvasReferenceFiles = [
          {
            uid: videoReferenceImage.uid || 'video-reference-0',
            name: videoReferenceImage.name || t('首帧图'),
            url: base64Image,
          },
        ];
      }
      const params = {
        duration: videoDuration,
        resolution: videoResolution,
        aspect_ratio: videoAspectRatio,
      };
      if (base64Image) {
        params.reference_images = [base64Image];
      }
      canvasSession = await ensureCanvasSession(CANVAS_MODE_VIDEO);
      const canvasSessionIdentifier = getCanvasSessionIdentifier(canvasSession);
      clientRequestId = generateCanvasClientRequestId();
      const taskPayload = {
        model_id: videoSelectedModel,
        prompt: videoPrompt.trim(),
        request_endpoint: videoSelectedModelData.request_endpoint,
        params: JSON.stringify(params),
        client_request_id: clientRequestId,
      };
      replaceCanvasMessagesForSession(
        canvasSessionIdentifier,
        clientRequestId,
        [
          {
            id: `${clientRequestId}-user`,
            role: 'user',
            prompt: videoPrompt.trim(),
            created_time: Math.floor(Date.now() / 1000),
            client_request_id: clientRequestId,
            canvas_aspect_ratio: videoAspectRatio || '',
          },
          {
            id: `${clientRequestId}-assistant`,
            role: 'assistant',
            prompt: videoPrompt.trim(),
            status: 'queued',
            task_type: 'video_generation',
            created_time: Math.floor(Date.now() / 1000),
            client_request_id: clientRequestId,
            canvas_aspect_ratio: videoAspectRatio || '',
            reference_images: canvasReferenceFiles,
            video_task: {
              id: `pending-${clientRequestId}`,
              status: 'queued',
              prompt: videoPrompt.trim(),
              model_id: videoSelectedModel,
              aspect_ratio: videoAspectRatio || '',
              resolution: videoResolution || '',
              duration: videoDuration || 0,
              thumbnail_url: '',
              video_url: '',
              fail_reason: '',
            },
          },
        ],
        {
          canvasAspectRatio: videoAspectRatio || '',
        },
      );
      const res = await API.post(
        buildCanvasSessionApiPath(canvasSession, '/messages'),
        taskPayload,
      );
      if (!res.data.success) {
        removeCanvasMessagesForRequest(
          canvasSessionIdentifier,
          clientRequestId,
        );
        showError(res.data.message || t('创建视频任务失败'));
        return;
      }
      const createdMessages = res.data.data || [];
      const newTask =
        createdMessages.find((message) => message?.video_task)?.video_task ||
        null;
      showSuccess(t('视频任务已创建，正在生成中...'));
      replaceCanvasMessagesForSession(
        canvasSessionIdentifier,
        clientRequestId,
        attachPendingReferencesToMessages(
          createdMessages,
          canvasReferenceFiles,
        ),
        { canvasAspectRatio: videoAspectRatio || '' },
      );
      refreshRecentCanvasSessions({ silent: true });
      setVideoSelectedTask(newTask);
      if (
        newTask &&
        isDefaultVideoTaskViewState(videoTaskListStateRef.current)
      ) {
        setVideoTasks((prev) =>
          mergeTaskCollections(prev, [newTask], videoTaskPageSize),
        );
      } else {
        loadVideoTasks();
      }
      if (
        newTask &&
        isDefaultVideoTaskViewState(videoTaskListStateRef.current)
      ) {
        setVideoTaskTotal((prev) => prev + 1);
      }
    } catch (error) {
      const canvasSessionIdentifier = getCanvasSessionIdentifier(canvasSession);
      if (canvasSessionIdentifier && clientRequestId) {
        removeCanvasMessagesForRequest(
          canvasSessionIdentifier,
          clientRequestId,
        );
      }
      showError(
        error.response?.data?.message || error.message || t('创建视频任务失败'),
      );
    } finally {
      setVideoGenerating(false);
    }
  };

  const formatTimestamp = (timestamp) => {
    if (!timestamp) {
      return '-';
    }
    const date = new Date(timestamp * 1000);
    const pad = (value) => String(value).padStart(2, '0');
    return `${date.getFullYear()}/${pad(date.getMonth() + 1)}/${pad(
      date.getDate(),
    )} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
  };

  const summarizeText = (value, fallback = t('暂无提示词')) => {
    const text = String(value || '').trim();
    if (!text) {
      return fallback;
    }
    return text.length > 54 ? `${text.slice(0, 54)}...` : text;
  };

  const getTaskTitle = (task, fallback) =>
    summarizeText(task?.prompt || task?.title, fallback);

  const getAssetKey = (asset) => asset?.task_id || asset?.id;

  const getAssetImageUrl = (asset) => asset?.image_url || asset?.thumbnail_url;

  const getAssetComparableKey = (asset) =>
    normalizeComparableId(getAssetKey(asset));

  const currentCanvasAssetIds = canvasAssets
    .map((asset) => getAssetComparableKey(asset))
    .filter((assetId) => assetId !== '');
  const selectedCanvasAssets = canvasAssets.filter((asset) =>
    selectedCanvasAssetIds.has(getAssetComparableKey(asset)),
  );
  const selectedCanvasAssetCount = selectedCanvasAssets.length;
  const allCurrentCanvasAssetsSelected =
    currentCanvasAssetIds.length > 0 &&
    currentCanvasAssetIds.every((assetId) =>
      selectedCanvasAssetIds.has(assetId),
    );
  const partiallyCurrentCanvasAssetsSelected =
    selectedCanvasAssetCount > 0 && !allCurrentCanvasAssetsSelected;

  const readAssetStringValue = (sources, keys) => {
    for (const source of sources) {
      if (!source || typeof source !== 'object') {
        continue;
      }
      for (const key of keys) {
        const value = source[key];
        if (value === undefined || value === null) {
          continue;
        }
        const text = String(value).trim();
        if (text) {
          return text;
        }
      }
    }
    return '';
  };

  const readAssetPositiveNumber = (sources, keys) => {
    const value = readAssetStringValue(sources, keys);
    if (!value) {
      return 0;
    }
    const parsed = Number(value);
    return Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed) : 0;
  };

  const parseAssetDimensionText = (value) => {
    const match = String(value || '')
      .trim()
      .match(/^(\d+)\s*[xX*×]\s*(\d+)$/);
    if (!match) {
      return '';
    }
    const width = Number(match[1]);
    const height = Number(match[2]);
    if (!Number.isFinite(width) || !Number.isFinite(height)) {
      return '';
    }
    return width > 0 && height > 0 ? `${width}x${height}` : '';
  };

  const joinAssetMetaParts = (parts) => {
    const seen = new Set();
    return parts
      .map((part) => String(part || '').trim())
      .filter((part) => {
        if (!part || seen.has(part)) {
          return false;
        }
        seen.add(part);
        return true;
      })
      .join(' · ');
  };

  const getAssetMetadataSources = (asset) => {
    const params = parseCanvasTaskParams(asset?.params);
    const metadata = parseCanvasTaskParams(asset?.image_metadata);
    const metadataDetails = parseCanvasTaskParams(metadata.metadata);
    return { params, metadata, metadataDetails };
  };

  const getAssetDisplayName = (asset) =>
    asset?.display_name || asset?.model_name || '-';

  const getAssetTaskIdText = (asset) => {
    const taskId = asset?.task_id;
    const id = asset?.id;
    if (taskId && id && taskId !== id) {
      return `${taskId} / ${id}`;
    }
    return taskId || id || '-';
  };

  const getAssetSizeText = (asset) => {
    const { params, metadata, metadataDetails } =
      getAssetMetadataSources(asset);
    const metadataSources = [metadata, metadataDetails];
    const allSources = [params, metadata, metadataDetails];
    const width = readAssetPositiveNumber(metadataSources, [
      'width',
      'output_width',
      'image_width',
      'outputWidth',
      'imageWidth',
    ]);
    const height = readAssetPositiveNumber(metadataSources, [
      'height',
      'output_height',
      'image_height',
      'outputHeight',
      'imageHeight',
    ]);
    if (width > 0 && height > 0) {
      return `${width}x${height}`;
    }

    const dimensionText = parseAssetDimensionText(
      readAssetStringValue(metadataSources, [
        'size',
        'output_size',
        'dimensions',
        'outputSize',
      ]),
    );
    if (dimensionText) {
      return dimensionText;
    }

    return (
      joinAssetMetaParts([
        readAssetStringValue(allSources, ['aspect_ratio', 'aspectRatio']),
        readAssetStringValue(
          [params],
          ['resolution', 'image_size', 'imageSize'],
        ),
        readAssetStringValue(allSources, [
          'size',
          'output_size',
          'dimensions',
          'outputSize',
        ]),
      ]) || '-'
    );
  };

  const handleCanvasAssetSelect = (asset, checked) => {
    const assetId = getAssetComparableKey(asset);
    if (!assetId) {
      return;
    }
    setSelectedCanvasAssetIds((prev) => {
      const next = new Set(prev);
      if (checked) {
        next.add(assetId);
      } else {
        next.delete(assetId);
      }
      return next;
    });
  };

  const handleCanvasAssetSelectAll = (checked) => {
    setSelectedCanvasAssetIds(
      checked ? new Set(currentCanvasAssetIds) : new Set(),
    );
  };

  const removeDeletedCanvasAssetsFromState = (deletedAssets) => {
    const deletedList = [].concat(deletedAssets || []);
    const deletedIds = deletedList
      .map((asset) => getAssetComparableKey(asset))
      .filter((assetId) => assetId !== '');
    if (deletedIds.length === 0) {
      return;
    }
    const deletedIdSet = new Set(deletedIds);

    setCanvasAssets((prev) =>
      prev.filter((item) => !deletedIdSet.has(getAssetComparableKey(item))),
    );
    setCanvasAssetsTotal((prev) => Math.max(0, prev - deletedIdSet.size));
    setSelectedCanvasAssetIds((prev) => {
      const next = new Set(prev);
      deletedIdSet.forEach((assetId) => next.delete(assetId));
      return next;
    });
    setSelectedAssetPreview((prev) => {
      if (!prev || !deletedIdSet.has(getAssetComparableKey(prev))) {
        return prev;
      }
      return null;
    });
    removeImageTasksFromLocalState(deletedIds, {
      decrementTotal: deletedList.filter((asset) =>
        imageTaskMatchesCurrentTaskFilters({
          ...asset,
          id: getAssetKey(asset),
          status: 'success',
        }),
      ).length,
    });
  };

  const handleModeChange = (mode) => {
    if (!CANVAS_MODES.includes(mode)) {
      return;
    }
    setGenerationMode(mode);
    setMobileTaskbarVisible(false);
  };

  const selectCanvasSession = (session) => {
    const mode = CANVAS_MODES.includes(session?.mode)
      ? session.mode
      : CANVAS_MODE_IMAGE;
    const sessionIdentifier = getCanvasSessionIdentifier(session);
    if (!sessionIdentifier) {
      return;
    }
    if (
      generationMode === mode &&
      selectedCanvasSessionIds[mode] === sessionIdentifier &&
      routeSessionIdNormalized === sessionIdentifier
    ) {
      return;
    }
    setGenerationMode(mode);
    setSelectedCanvasSessionIds((prev) => ({
      ...prev,
      [mode]: sessionIdentifier,
    }));
    setMobileTaskbarVisible(false);
    syncCanvasRouteToSession(session, { replace: false });
  };

  useEffect(() => {
    if (
      generationMode !== CANVAS_MODE_IMAGE ||
      selectedCanvasSession?.mode !== CANVAS_MODE_IMAGE ||
      !selectedCanvasSession.current_model ||
      models.length === 0
    ) {
      return;
    }
    const model = models.find(
      (item) => item.request_model === selectedCanvasSession.current_model,
    );
    if (!model) {
      return;
    }
    if (model.model_series) {
      setSelectedSeries(model.model_series);
    }
    setSelectedModel(model.request_model);
  }, [
    generationMode,
    models,
    selectedCanvasSession?.id,
    selectedCanvasSession?.current_model,
    selectedCanvasSession?.mode,
  ]);

  useEffect(() => {
    if (
      generationMode !== CANVAS_MODE_VIDEO ||
      selectedCanvasSession?.mode !== CANVAS_MODE_VIDEO ||
      !selectedCanvasSession.current_model ||
      videoModels.length === 0
    ) {
      return;
    }
    const model = videoModels.find(
      (item) => item.request_model === selectedCanvasSession.current_model,
    );
    if (!model) {
      return;
    }
    if (model.model_series) {
      setVideoSelectedSeries(model.model_series);
    }
    setVideoSelectedModel(model.request_model);
  }, [
    generationMode,
    selectedCanvasSession?.id,
    selectedCanvasSession?.current_model,
    selectedCanvasSession?.mode,
    videoModels,
  ]);

  useEffect(() => {
    if (
      generationMode !== CANVAS_MODE_CHAT ||
      selectedCanvasSession?.mode !== CANVAS_MODE_CHAT
    ) {
      return;
    }
    if (selectedCanvasSession.current_model) {
      setChatModel(String(selectedCanvasSession.current_model));
    } else {
      setChatModel((current) => {
        const normalizedCurrent = String(current || '').trim();
        if (
          normalizedCurrent &&
          chatModels.some((item) => item?.request_model === normalizedCurrent)
        ) {
          return normalizedCurrent;
        }
        const firstUsable = chatModels.find((item) => item?.usable !== false);
        return String(firstUsable?.request_model || '');
      });
    }
    if (Number.isFinite(Number(selectedCanvasSession.chat_temperature))) {
      setChatTemperature(String(selectedCanvasSession.chat_temperature));
    }
    if (Number.isFinite(Number(selectedCanvasSession.chat_context_count))) {
      setChatContext(String(selectedCanvasSession.chat_context_count));
    }
    setChatWebSearchEnabled(!!selectedCanvasSession.web_search_enabled);
  }, [
    chatModels,
    generationMode,
    selectedCanvasSession?.chat_context_count,
    selectedCanvasSession?.chat_temperature,
    selectedCanvasSession?.web_search_enabled,
    selectedCanvasSession?.id,
    selectedCanvasSession?.current_model,
    selectedCanvasSession?.mode,
  ]);

  const handleNewBlankChat = () => {
    setGenerationMode(CANVAS_MODE_CHAT);
    setSelectedCanvasSessionIds((prev) => ({
      ...prev,
      [CANVAS_MODE_CHAT]: null,
    }));
    setChatPrompt('');
    setMobileTaskbarVisible(false);
    if (routeSessionIdNormalized) {
      syncCanvasRouteToBlank({ replace: false });
    }
  };

  const handleProjectEntryClick = () => {
    showError(t('暂未开放'));
  };

  const handleSendChatMessage = async (promptOverride = '') => {
    if (chatStreaming) {
      showChatStreamingNotice();
      return;
    }

    const prompt = String(promptOverride || chatPrompt).trim();
    if (!prompt) {
      showError(t('请输入消息'));
      return;
    }
    if (!chatModel) {
      showError(t('请选择模型'));
      return;
    }
    if (activeChatModelOption?.usable === false) {
      showError(
        activeChatModelOption.unavailable_reason || t('当前选择的模型不可用'),
      );
      return;
    }

    const chatUploadVisibility = getCanvasChatUploadVisibility(
      activeChatModelOption,
    );
    const webSearchVisible = getCanvasChatWebSearchVisibility(
      activeChatModelOption,
    );
    if (chatImageAttachment && !chatUploadVisibility.showImageUpload) {
      showError(t('当前模型不支持图片上传'));
      return;
    }
    if (chatFileAttachment && !chatUploadVisibility.showFileUpload) {
      showError(t('当前模型不支持文件上传'));
      return;
    }

    const attachments = (
      promptOverride ? [] : [chatImageAttachment, chatFileAttachment]
    )
      .filter(Boolean)
      .map((attachment) => ({
        kind:
          attachment.kind ||
          (attachment === chatImageAttachment ? 'image' : 'file'),
        name: attachment.name,
        mime_type: attachment.mime_type,
        data: attachment.data,
      }));

    const temperatureValue = Number(chatTemperature);
    const contextCountValue = Number(chatContext);
    const previousChatPrompt = promptOverride ? '' : chatPrompt;
    const previousChatImageAttachment = promptOverride
      ? null
      : chatImageAttachment;
    const previousChatFileAttachment = promptOverride
      ? null
      : chatFileAttachment;
    let activeSessionId = null;
    let clientRequestId = '';
    let hasServerSnapshot = false;
    if (!promptOverride) {
      setChatPrompt('');
      setChatImageAttachment(null);
      setChatFileAttachment(null);
    }

    try {
      const canvasSession = await ensureCanvasSession(CANVAS_MODE_CHAT);
      activeSessionId = getCanvasSessionIdentifier(canvasSession);
      clientRequestId = generateCanvasClientRequestId();
      ensureCanvasMessagesSessionForSend(activeSessionId);
      replaceCanvasMessagesForSession(
        activeSessionId,
        clientRequestId,
        buildOptimisticCanvasChatMessages({
          prompt,
          attachments,
          requestId: clientRequestId,
          submittedAt: Math.floor(Date.now() / 1000),
        }),
      );
      const requestURL = buildCanvasStreamRequestUrl(
        buildCanvasSessionApiPath(activeSessionId, '/messages'),
      );
      const controller = new AbortController();
      chatStreamAbortRef.current = controller;
      chatStreamingSessionIdRef.current = activeSessionId;
      chatStreamingMessageIdRef.current = `${clientRequestId}-assistant`;
      setChatStreaming(true);
      setCanvasMessagesError('');

      const response = await fetch(requestURL, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'text/event-stream',
          'New-API-User': String(getUserIdFromLocalStorage()),
          ...authHeader(),
        },
        body: JSON.stringify({
          prompt,
          model_id: chatModel,
          attachments,
          web_search_enabled: webSearchVisible
            ? !!chatWebSearchEnabled
            : undefined,
          stream: true,
          client_request_id: clientRequestId,
          temperature: Number.isFinite(temperatureValue)
            ? temperatureValue
            : undefined,
          context_count: Number.isFinite(contextCountValue)
            ? contextCountValue
            : undefined,
        }),
        credentials: 'include',
        signal: controller.signal,
      });

      if (!response.ok) {
        let message = t('发送失败');
        try {
          const contentType = response.headers.get('content-type') || '';
          if (contentType.includes('application/json')) {
            const data = await response.json();
            message = data?.message || message;
          } else {
            const text = await response.text();
            if (text) {
              message = text;
            }
          }
        } catch (e) {
          // ignore parse error
        }
        throw new Error(message);
      }
      if (!response.body) {
        throw new Error(t('聊天流不可用'));
      }

      refreshRecentCanvasSessions({ silent: true });

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';
      let streamErrorMessage = '';

      const applyStreamEvent = (event) => {
        if (!event?.data) {
          return;
        }
        let payload = null;
        try {
          payload = JSON.parse(event.data);
        } catch (error) {
          console.error('Failed to parse canvas SSE payload:', error);
          return;
        }

        if (payload?.session) {
          updateCanvasSessionInState(payload.session);
        }

        if (event.event === 'canvas.message.created') {
          const messages = Array.isArray(payload?.messages)
            ? payload.messages
            : [];
          const assistantMessage = messages.find(
            (message) => message?.role === 'assistant',
          );
          if (assistantMessage?.id) {
            chatStreamingMessageIdRef.current = assistantMessage.id;
          }
          if (messages.length > 0) {
            hasServerSnapshot = true;
            replaceCanvasMessagesForSession(
              activeSessionId,
              clientRequestId,
              messages,
            );
          }
          bumpChatStreamRenderVersion();
          return;
        }

        if (payload?.message) {
          if (payload.message?.id) {
            hasServerSnapshot = true;
          }
          upsertCanvasMessagesForSession(activeSessionId, [payload.message]);
          if (payload.message?.id) {
            chatStreamingMessageIdRef.current = payload.message.id;
          }
        }

        if (event.event === 'canvas.message.delta') {
          if (!payload?.message?.id && chatStreamingMessageIdRef.current) {
            appendCanvasChatDeltaForSession(
              activeSessionId,
              chatStreamingMessageIdRef.current,
              payload?.delta || '',
              payload?.reasoning_delta || '',
            );
          }
          bumpChatStreamRenderVersion();
          return;
        }

        if (event.event === 'canvas.message.completed') {
          bumpChatStreamRenderVersion();
          refreshRecentCanvasSessions({ silent: true });
          return;
        }

        if (event.event === 'canvas.message.error') {
          streamErrorMessage =
            payload?.error || payload?.message?.error_message || t('发送失败');
          bumpChatStreamRenderVersion();
        }
      };

      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          break;
        }
        buffer += decoder.decode(value, { stream: true });
        const parsed = parseCanvasSSEPayload(buffer);
        buffer = parsed.remainder;
        parsed.events.forEach(applyStreamEvent);
      }

      if (buffer) {
        const parsed = parseCanvasSSEPayload(`${buffer}\n\n`);
        parsed.events.forEach(applyStreamEvent);
      }

      if (streamErrorMessage) {
        showError(streamErrorMessage);
      }
    } catch (error) {
      if (activeSessionId && clientRequestId && !hasServerSnapshot) {
        removeCanvasMessagesForRequest(activeSessionId, clientRequestId);
      }
      if (!promptOverride && !hasServerSnapshot) {
        setChatPrompt(previousChatPrompt);
        setChatImageAttachment(previousChatImageAttachment);
        setChatFileAttachment(previousChatFileAttachment);
      }
      if (error?.name === 'AbortError') {
        return;
      }
      showError(error.message || t('发送失败'));
    } finally {
      chatStreamAbortRef.current = null;
      chatStreamingMessageIdRef.current = null;
      chatStreamingSessionIdRef.current = null;
      setChatStreaming(false);
      if (activeSessionId) {
        refreshRecentCanvasSessions({ silent: true });
        const normalizedActiveSessionId = String(activeSessionId || '').trim();
        if (
          generationModeRef.current === CANVAS_MODE_CHAT &&
          (String(
            selectedCanvasSessionIdsRef.current?.[CANVAS_MODE_CHAT] || '',
          ).trim() === normalizedActiveSessionId ||
            String(canvasMessagesSessionIdRef.current || '').trim() ===
              normalizedActiveSessionId)
        ) {
          window.setTimeout(() => {
            if (
              generationModeRef.current === CANVAS_MODE_CHAT &&
              (String(
                selectedCanvasSessionIdsRef.current?.[CANVAS_MODE_CHAT] || '',
              ).trim() === normalizedActiveSessionId ||
                String(canvasMessagesSessionIdRef.current || '').trim() ===
                  normalizedActiveSessionId)
            ) {
              loadCanvasMessages(normalizedActiveSessionId, { silent: true });
            }
          }, 200);
        }
      }
    }
  };

  const downloadAsset = (asset) => {
    const imageUrl = getAssetImageUrl(asset);
    if (!imageUrl) {
      return;
    }
    const link = document.createElement('a');
    link.href = imageUrl;
    link.download = `image-${getAssetKey(asset) || Date.now()}.png`;
    link.target = '_blank';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  const deleteAsset = async (asset) => {
    const assetKey = getAssetKey(asset);
    if (!assetKey) {
      return;
    }
    try {
      const res = await API.delete(`/api/image-generation/tasks/${assetKey}`);
      if (!res.data.success) {
        showError(res.data.message || t('删除失败'));
        return;
      }
      showSuccess(t('删除成功'));
      removeDeletedCanvasAssetsFromState([asset]);
    } catch (error) {
      showError(error.message || t('删除失败'));
    }
  };

  const deleteSelectedCanvasAssets = async () => {
    if (selectedCanvasAssets.length === 0) {
      showError(t('请先选择要删除的资产'));
      return;
    }
    setAssetBatchDeleting(true);
    try {
      const results = await Promise.allSettled(
        selectedCanvasAssets.map((asset) =>
          API.delete(`/api/image-generation/tasks/${getAssetKey(asset)}`),
        ),
      );
      const successfulAssets = [];
      results.forEach((result, index) => {
        if (result.status === 'fulfilled' && result.value?.data?.success) {
          successfulAssets.push(selectedCanvasAssets[index]);
        }
      });
      const successCount = successfulAssets.length;
      const failCount = results.length - successCount;

      if (successCount > 0) {
        removeDeletedCanvasAssetsFromState(successfulAssets);
        showSuccess(t('删除成功 {{count}} 个任务', { count: successCount }));
      }
      if (failCount > 0) {
        showError(t('删除失败 {{count}} 个任务', { count: failCount }));
      }
    } catch (error) {
      showError(error.message || t('批量删除失败'));
    } finally {
      setAssetBatchDeleting(false);
    }
  };

  const addAssetToChat = () => {
    showError(t('聊天模式暂不支持插入资产'));
  };

  const addAssetToImageReferences = (asset) => {
    const imageUrl = getAssetImageUrl(asset);
    if (!imageUrl) {
      showError(t('该资产没有可用图片'));
      return;
    }
    if (!selectedModelSupportsEditing) {
      showError(t('当前模型不支持图像编辑'));
      return;
    }
    const limit = getReferenceImageLimit(selectedModelData);
    if (limit > 0 && referenceImages.length >= limit) {
      showError(
        t('当前模型最多只能上传 {{count}} 张参考图', {
          count: limit,
        }),
      );
      return;
    }
    const remoteReference = buildRemoteReferenceFile(imageUrl);
    if (!remoteReference) {
      return;
    }
    setReferenceImages((prev) => [
      ...prev,
      {
        ...remoteReference,
        uid: `asset-${getAssetKey(asset) || Date.now()}`,
        name: asset.prompt || t('图片资产'),
      },
    ]);
    showSuccess(t('已作为参考图插入图片任务'));
  };

  const addAssetToVideoReference = (asset) => {
    const imageUrl = getAssetImageUrl(asset);
    if (!imageUrl) {
      showError(t('该资产没有可用图片'));
      return;
    }
    if (!videoSelectedModelSupportsImageToVideo) {
      showError(t('当前模型不支持图生视频'));
      return;
    }
    const remoteReference = buildRemoteReferenceFile(imageUrl);
    if (!remoteReference) {
      return;
    }
    setVideoReferenceImage({
      ...remoteReference,
      uid: `video-asset-${getAssetKey(asset) || Date.now()}`,
      name: asset.prompt || t('首帧图'),
    });
    showSuccess(t('已作为首帧图插入视频任务'));
  };

  const insertAssetIntoCurrentMode = (asset) => {
    if (generationMode === CANVAS_MODE_CHAT) {
      addAssetToChat(asset);
      return;
    }
    if (generationMode === CANVAS_MODE_VIDEO) {
      addAssetToVideoReference(asset);
      return;
    }
    addAssetToImageReferences(asset);
  };

  const styles = {
    container: {
      display: 'flex',
      height: 'calc(100dvh - 64px)',
      marginTop: 64,
      overflow: 'hidden',
      background: 'var(--canvas-page-bg)',
    },
    leftPanel: {
      width: isMobile ? '100%' : 232,
      minWidth: isMobile ? 0 : 232,
      display: 'flex',
      flexDirection: 'column',
      borderRight: '1px solid var(--canvas-border)',
      background: 'var(--canvas-sidebar-bg)',
      overflow: 'hidden',
      position: 'relative',
      transition: isMobile
        ? undefined
        : 'width 0.24s ease, min-width 0.24s ease, opacity 0.2s ease, border-color 0.24s ease',
    },
    leftPanelCollapsed: {
      width: isMobile ? '100%' : 52,
      minWidth: isMobile ? 0 : 52,
      opacity: 1,
      pointerEvents: 'auto',
    },
    sidebarHeader: {
      minHeight: 42,
      flexShrink: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 6,
      padding: '12px 12px 6px',
    },
    sidebarHeaderActionButton: {
      height: 32,
      borderRadius: 10,
      padding: '0 10px',
    },
    sidebarIconButton: {
      width: 32,
      height: 32,
      minWidth: 32,
      borderRadius: 10,
    },
    sidebarNav: {
      display: 'flex',
      flexDirection: 'column',
      gap: 4,
      padding: '0 12px 10px',
    },
    sidebarNavItem: {
      width: '100%',
      minHeight: 40,
      border: 'none',
      borderRadius: 10,
      background: 'transparent',
      color: 'var(--canvas-text-secondary)',
      display: 'flex',
      alignItems: 'center',
      gap: 10,
      padding: '0 10px',
      cursor: 'pointer',
      textAlign: 'left',
      fontSize: 14,
      fontWeight: 500,
      transition:
        'background 0.16s ease, color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease',
    },
    sidebarNavItemPrimary: {
      minHeight: 42,
      margin: '2px 0 4px',
      background: 'var(--canvas-sidebar-action-bg)',
      color: 'var(--canvas-text-primary)',
      fontWeight: 650,
      boxShadow: 'var(--canvas-sidebar-action-shadow)',
    },
    sidebarNavItemHover: {
      background: 'var(--canvas-sidebar-row-hover-bg)',
      color: 'var(--canvas-text-primary)',
    },
    sidebarNavItemActive: {
      background: 'var(--canvas-sidebar-row-active-bg)',
      color: 'var(--canvas-text-primary)',
      boxShadow: 'var(--canvas-sidebar-row-active-shadow)',
    },
    sidebarNavItemMuted: {
      color: 'var(--canvas-text-muted)',
    },
    sidebarNavIcon: {
      width: 26,
      minWidth: 26,
      height: 26,
      borderRadius: 8,
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: 'var(--canvas-text-muted)',
      background: 'var(--canvas-sidebar-icon-bg)',
      flexShrink: 0,
    },
    sidebarNavIconActive: {
      color: 'var(--canvas-text-primary)',
      background: 'var(--canvas-sidebar-icon-active-bg)',
    },
    sidebarNavIconPrimary: {
      color: 'var(--canvas-primary)',
      background: 'var(--canvas-primary-soft-bg)',
    },
    sidebarNavLabel: {
      flex: 1,
      minWidth: 0,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    sidebarNavSuffix: {
      marginLeft: 'auto',
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: 'var(--canvas-text-muted)',
      flexShrink: 0,
      transition: 'transform 0.16s ease',
    },
    taskList: {
      flex: 1,
      minHeight: 0,
      overflowY: 'auto',
      display: 'flex',
      flexDirection: 'column',
      gap: 4,
      padding: '0 12px 10px',
    },
    scrollPanel: {
      minHeight: 0,
      overflowY: 'auto',
      overscrollBehavior: 'contain',
      WebkitOverflowScrolling: 'touch',
    },
    taskListItem: {
      width: '100%',
      minHeight: 38,
      borderRadius: 12,
      border: 'none',
      background: 'transparent',
      padding: '7px 10px',
      display: 'flex',
      alignItems: 'center',
      gap: 10,
      textAlign: 'left',
      cursor: 'pointer',
      opacity: 0.82,
      transition:
        'background 0.16s, color 0.16s, opacity 0.16s, box-shadow 0.16s',
    },
    taskListItemHover: {
      background: 'var(--canvas-sidebar-row-hover-bg)',
      opacity: 0.96,
    },
    taskListItemActive: {
      background: 'var(--canvas-sidebar-row-active-bg)',
      opacity: 1,
      boxShadow: 'var(--canvas-sidebar-row-active-shadow)',
    },
    taskListText: {
      minWidth: 0,
      flex: 1,
    },
    taskListTitle: {
      fontSize: 14,
      fontWeight: 600,
      color: 'var(--canvas-text-primary)',
      lineHeight: 1.35,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    sessionListItem: {
      paddingRight: 8,
    },
    sessionTypeIcon: {
      width: 26,
      minWidth: 26,
      height: 26,
      borderRadius: 8,
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: 'var(--canvas-text-muted)',
      background: 'var(--canvas-sidebar-icon-bg)',
      flexShrink: 0,
    },
    sessionTypeIconActive: {
      color: 'var(--canvas-text-primary)',
      background: 'var(--canvas-sidebar-icon-active-bg)',
    },
    sessionListTitle: {
      fontSize: 14,
      fontWeight: 500,
      color: 'var(--canvas-text-primary)',
      lineHeight: 1.35,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    sessionMenuButton: {
      width: 28,
      height: 28,
      minWidth: 28,
      borderRadius: 8,
      flexShrink: 0,
    },
    sidebarRail: {
      width: '100%',
      minHeight: 0,
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: 8,
      padding: '10px 7px',
    },
    sidebarRailSection: {
      width: '100%',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: 8,
    },
    sidebarRailDivider: {
      width: 28,
      height: 1,
      background: 'var(--canvas-border)',
      margin: '2px 0',
      flexShrink: 0,
    },
    sidebarRailButton: {
      width: 38,
      height: 38,
      minWidth: 38,
      borderRadius: 12,
    },
    sidebarRailButtonActive: {
      background: 'var(--canvas-sidebar-row-active-bg)',
      color: 'var(--canvas-text-primary)',
      boxShadow: 'var(--canvas-sidebar-row-active-shadow)',
    },
    recentListFooter: {
      display: 'flex',
      justifyContent: 'center',
      padding: '6px 0 2px',
    },
    rightPanel: {
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      background: 'var(--canvas-page-bg)',
      overflow: 'hidden',
      position: 'relative',
    },
    rightContent: {
      flex: 1,
      minHeight: 0,
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
    stage: {
      flex: 1,
      minHeight: 0,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'flex-end',
      padding: isMobile ? '20px 12px 28px' : '56px 24px 76px',
      overflowY: 'auto',
    },
    stageInner: {
      width: '100%',
      maxWidth: 980,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'stretch',
      gap: isMobile ? 12 : 16,
    },
    stageTitle: {
      color: 'var(--canvas-text-primary)',
      textAlign: 'center',
      fontSize: isMobile ? 18 : 24,
      fontWeight: 700,
      letterSpacing: 0,
      lineHeight: 1.2,
    },
    stageSubtitle: {
      color: 'var(--canvas-text-secondary)',
      textAlign: 'center',
      fontSize: isMobile ? 12 : 13,
      lineHeight: 1.5,
      maxWidth: 720,
      margin: '0 auto',
    },
    stagePanelWrap: {
      width: '100%',
      display: 'flex',
      justifyContent: 'center',
    },
    stagePanel: {
      width: '100%',
      maxWidth: 900,
      borderRadius: 12,
      border: '1px solid var(--canvas-border)',
      background: 'var(--canvas-card-bg)',
      boxShadow: 'var(--canvas-shadow-lg)',
      padding: isMobile ? '14px' : '18px',
    },
    promptArea: {
      borderRadius: 'inherit',
      border: 'none',
      background: 'transparent',
      boxShadow: 'none',
      minHeight: isMobile ? 110 : 118,
      padding: isMobile ? '8px 10px 9px' : '9px 12px 10px',
      display: 'flex',
      flexDirection: 'column',
      gap: 0,
      overflow: 'hidden',
    },
    promptInputShell: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'stretch',
      gap: 8,
      minWidth: 0,
      minHeight: isMobile ? 88 : 96,
      borderRadius: 16,
      background: 'var(--canvas-surface-subtle)',
      boxShadow: 'none',
      padding: isMobile ? '8px 9px' : 9,
    },
    promptInputShellDragActive: {
      background: 'rgba(231, 236, 255, 0.9)',
      boxShadow: 'inset 0 0 0 1px var(--canvas-primary-soft-border)',
    },
    promptInlineAssets: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      flexWrap: 'wrap',
      minHeight: 0,
    },
    promptLeadingSlot: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      flexWrap: 'wrap',
      minWidth: 0,
    },
    promptInput: {
      flex: 1,
      minWidth: 0,
      minHeight: isMobile ? 46 : 48,
      maxHeight: 220,
      alignSelf: 'stretch',
      border: 'none',
      background: 'transparent',
      resize: 'none',
      boxShadow: 'none',
      color: 'var(--canvas-text-primary)',
      caretColor: 'var(--canvas-primary)',
      fontSize: isMobile ? 14 : 16,
      lineHeight: isMobile ? 1.55 : 1.6,
      padding: 0,
      overflowY: 'auto',
    },
    promptControls: {
      display: 'flex',
      justifyContent: 'space-between',
      gap: isMobile ? 8 : 12,
      alignItems: 'center',
      flexWrap: isMobile ? 'nowrap' : 'wrap',
      minHeight: 40,
      padding: isMobile ? '6px 0 0' : '8px 0 0',
      borderTop: '1px solid rgba(229, 231, 235, 0.72)',
    },
    promptControlsLeft: {
      display: 'flex',
      alignItems: 'center',
      gap: 7,
      flexWrap: isMobile ? 'nowrap' : 'wrap',
      minWidth: 0,
      flex: 1,
      overflowX: isMobile ? 'auto' : 'visible',
      overflowY: 'hidden',
      paddingBottom: isMobile ? 2 : 0,
    },
    promptControlsRight: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'flex-end',
      gap: 7,
      minWidth: isMobile ? 40 : 0,
      flexShrink: 0,
      marginLeft: 'auto',
    },
    promptAssetBar: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      flexWrap: 'wrap',
      minHeight: 36,
    },
    uploadIconBtn: {
      '--canvas-control-border': 'var(--canvas-border)',
      '--canvas-control-bg': 'var(--canvas-toolbar-bg)',
      '--canvas-control-text': 'var(--canvas-text-secondary)',
      '--canvas-control-shadow': 'none',
      width: 36,
      height: 36,
      minWidth: 36,
      borderRadius: 12,
      border: '1px solid var(--canvas-control-border)',
      background: 'var(--canvas-control-bg)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      cursor: 'pointer',
      color: 'var(--canvas-control-text)',
      boxShadow: 'var(--canvas-control-shadow)',
      transition:
        'transform 0.18s ease, opacity 0.18s ease, border-color 0.18s ease, background 0.18s ease, color 0.18s ease, box-shadow 0.18s ease',
    },
    uploadIconBtnActive: {
      '--canvas-control-border': 'var(--canvas-primary-soft-border)',
      '--canvas-control-bg': 'var(--canvas-primary-soft-bg)',
      '--canvas-control-text': 'var(--canvas-primary)',
      '--canvas-control-shadow': '0 8px 18px rgba(109, 93, 246, 0.08)',
    },
    pillButton: {
      '--canvas-pill-border': 'rgba(226, 232, 240, 0.96)',
      '--canvas-pill-bg': 'rgba(248, 250, 252, 0.96)',
      '--canvas-pill-text': 'var(--canvas-text-secondary)',
      '--canvas-pill-shadow': 'none',
      height: 36,
      minHeight: 36,
      borderRadius: 12,
      border: '1px solid var(--canvas-pill-border)',
      background: 'var(--canvas-pill-bg)',
      color: 'var(--canvas-pill-text)',
      padding: '0 12px',
      display: 'inline-flex',
      alignItems: 'center',
      gap: 7,
      fontSize: 13,
      fontWeight: 500,
      lineHeight: 1,
      boxShadow: 'var(--canvas-pill-shadow)',
      transition:
        'transform 0.18s ease, border-color 0.18s ease, background 0.18s ease, color 0.18s ease, box-shadow 0.18s ease',
    },
    pillButtonIconOnly: {
      width: 36,
      minWidth: 36,
      padding: 0,
      justifyContent: 'center',
      gap: 0,
    },
    pillButtonSelected: {
      '--canvas-pill-border': 'rgba(209, 213, 219, 0.96)',
      '--canvas-pill-bg': 'rgba(255, 255, 255, 0.96)',
      '--canvas-pill-text': 'var(--canvas-text-primary)',
    },
    pillButtonOpen: {
      '--canvas-pill-border': 'var(--canvas-primary-soft-border)',
      '--canvas-pill-bg': 'var(--canvas-primary-soft-bg)',
      '--canvas-pill-text': 'var(--canvas-primary)',
      '--canvas-pill-shadow': '0 8px 18px rgba(109, 93, 246, 0.08)',
    },
    pillButtonMuted: {
      '--canvas-pill-text': 'var(--canvas-text-secondary)',
    },
    pillButtonIcon: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      flex: '0 0 auto',
      color: 'currentColor',
    },
    pillButtonLabel: {
      maxWidth: isMobile ? 120 : 170,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    darkMenu: {
      minWidth: 240,
      maxWidth: 340,
      border: '1px solid var(--canvas-border)',
      borderRadius: 10,
      background: 'var(--canvas-card-bg)',
      boxShadow: 'var(--canvas-shadow-overlay)',
      padding: 6,
    },
    darkMenuItem: {
      color: 'var(--canvas-text-primary)',
      borderRadius: 8,
      margin: 0,
      padding: '8px 10px',
      whiteSpace: 'nowrap',
    },
    darkMenuItemContent: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 8,
      minWidth: 0,
    },
    dropdownOptionMetaWrap: {
      minWidth: 0,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'flex-start',
      gap: 2,
    },
    dropdownOptionMeta: {
      fontSize: 12,
      color: 'var(--canvas-text-muted)',
      lineHeight: 1.4,
    },
    darkMenuItemActive: {
      background: 'var(--canvas-primary-soft-bg)',
      color: 'var(--canvas-primary)',
    },
    imageParamPanel: {
      width: isMobile ? 'calc(100vw - 32px)' : 380,
      maxWidth: 'calc(100vw - 32px)',
      border: '1px solid var(--canvas-border)',
      borderRadius: 10,
      background: 'var(--canvas-card-bg)',
      boxShadow: 'var(--canvas-shadow-overlay)',
      padding: isMobile ? 12 : 14,
    },
    imageParamSection: {
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
    },
    imageParamSectionTitle: {
      fontSize: 13,
      fontWeight: 650,
      color: 'var(--canvas-text-primary)',
      lineHeight: 1.3,
    },
    imageParamOptionRow: {
      display: 'flex',
      gap: 7,
      flexWrap: 'wrap',
      minWidth: 0,
    },
    imageParamOption: {
      minHeight: 32,
      borderRadius: 8,
      border: '1px solid var(--canvas-border)',
      background: 'var(--canvas-toolbar-bg)',
      color: 'var(--canvas-text-secondary)',
      padding: '0 11px',
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      fontSize: 13,
      lineHeight: 1.2,
      cursor: 'pointer',
      maxWidth: '100%',
      whiteSpace: 'nowrap',
    },
    imageParamOptionActive: {
      borderColor: 'var(--canvas-primary-soft-border)',
      background: 'var(--canvas-primary-soft-bg)',
      color: 'var(--canvas-primary)',
      fontWeight: 650,
    },
    imageParamDivider: {
      height: 1,
      background: 'var(--canvas-border)',
      margin: '12px 0',
    },
    imageParamSizeRow: {
      display: 'grid',
      gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)',
      gap: 8,
    },
    imageParamSizeField: {
      minHeight: 34,
      borderRadius: 8,
      border: '1px solid var(--canvas-border)',
      background: 'var(--canvas-toolbar-bg)',
      color: 'var(--canvas-text-primary)',
      padding: '0 10px',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 8,
      minWidth: 0,
    },
    imageParamSizeLabel: {
      flex: '0 0 auto',
      color: 'var(--canvas-text-muted)',
      fontSize: 12,
    },
    imageParamSizeValue: {
      minWidth: 0,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      fontSize: 13,
      fontWeight: 650,
    },
    modelMenuOption: {
      display: 'flex',
      alignItems: 'flex-start',
      gap: 8,
      minWidth: 0,
    },
    modelMenuIcon: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      flex: '0 0 auto',
      marginTop: 1,
      color: 'currentColor',
    },
    modelMenuItem: {
      display: 'flex',
      flexDirection: 'column',
      gap: 2,
      minWidth: 0,
    },
    modelMenuTitle: {
      fontSize: 13,
      lineHeight: 1.35,
      color: 'inherit',
      whiteSpace: 'normal',
      wordBreak: 'break-word',
    },
    modelMenuMeta: {
      fontSize: 11,
      lineHeight: 1.4,
      color: 'var(--canvas-text-muted)',
      wordBreak: 'break-all',
    },
    selectModelOption: {
      display: 'flex',
      alignItems: 'center',
      gap: 10,
      width: '100%',
    },
    selectModelOptionText: {
      minWidth: 0,
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      gap: 2,
    },
    selectModelOptionTitle: {
      fontSize: 14,
      lineHeight: 1.4,
      color: 'var(--canvas-text-primary)',
    },
    selectModelOptionMeta: {
      fontSize: 12,
      lineHeight: 1.4,
      color: 'var(--canvas-text-muted)',
      wordBreak: 'break-all',
    },
    compactField: {
      minWidth: 112,
    },
    label: {
      display: 'block',
      fontSize: 13,
      fontWeight: 500,
      color: 'var(--canvas-text-primary)',
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
      color: 'var(--canvas-text-primary)',
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
      color: 'var(--canvas-text-primary)',
    },
    modelList: {
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
    },
    modelCard: {
      width: '100%',
      borderRadius: 8,
      border: '1px solid var(--canvas-border)',
      background: 'var(--canvas-card-bg)',
      padding: '10px 12px',
      textAlign: 'left',
      cursor: 'pointer',
      transition: 'border-color 0.2s, background 0.2s',
    },
    modelCardActive: {
      borderColor: 'var(--canvas-primary)',
      background: 'var(--canvas-primary-soft-bg)',
    },
    modelCardTitle: {
      display: 'block',
      marginBottom: 5,
      color: 'var(--canvas-text-primary)',
      fontSize: 15,
      fontWeight: 650,
      lineHeight: 1.35,
    },
    modelCardMeta: {
      display: 'block',
      color: 'var(--canvas-text-muted)',
      fontSize: 12,
      lineHeight: 1.4,
      wordBreak: 'break-all',
    },
    generateIconBtn: {
      width: 36,
      height: 36,
      minWidth: 36,
      borderRadius: 8,
      border: '1px solid var(--canvas-border)',
      background: 'var(--canvas-toolbar-bg)',
      color: 'var(--canvas-text-muted)',
      cursor: 'pointer',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      transition:
        'opacity 0.2s, background 0.2s, border-color 0.2s, color 0.2s',
    },
    generateIconBtnEmbedded: {
      '--canvas-submit-bg': 'var(--canvas-toolbar-bg)',
      '--canvas-submit-border': 'var(--canvas-border)',
      '--canvas-submit-color': 'var(--canvas-text-muted)',
      '--canvas-submit-shadow': 'none',
      width: 40,
      height: 40,
      minWidth: 40,
      borderRadius: 14,
      border: '1px solid var(--canvas-submit-border)',
      background: 'var(--canvas-submit-bg)',
      color: 'var(--canvas-submit-color)',
      cursor: 'pointer',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexShrink: 0,
      boxShadow: 'var(--canvas-submit-shadow)',
      transition:
        'transform 0.18s ease, box-shadow 0.18s ease, opacity 0.18s ease, background 0.18s ease, border-color 0.18s ease, color 0.18s ease',
    },
    generateStopBtnEmbedded: {
      width: 40,
      height: 40,
      minWidth: 40,
      borderRadius: 14,
      border: '1px solid var(--canvas-status-neutral-border)',
      background: 'var(--canvas-toolbar-bg)',
      color: 'var(--canvas-text-secondary)',
      cursor: 'pointer',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexShrink: 0,
      boxShadow: 'none',
      transition:
        'transform 0.18s ease, box-shadow 0.18s ease, opacity 0.18s ease, background 0.18s ease, border-color 0.18s ease, color 0.18s ease',
    },
    generateStopIcon: {
      width: 12,
      height: 12,
      borderRadius: 2,
      background: 'currentColor',
      display: 'block',
    },
    addImageBtn: {
      width: 36,
      height: 36,
      borderRadius: 8,
      border: '1px dashed var(--canvas-border)',
      background: 'var(--canvas-toolbar-bg)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      cursor: 'pointer',
      color: 'var(--canvas-text-muted)',
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
      background: 'var(--canvas-empty-accent-bg)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    paramItem: {
      width: isMobile ? 88 : 104,
      flexShrink: 0,
    },
    referenceImageThumb: {
      width: 36,
      height: 36,
      borderRadius: 8,
      objectFit: 'cover',
      border: '1px solid var(--canvas-border)',
    },
    referenceImageContainer: {
      position: 'relative',
      display: 'inline-block',
    },
    chatFileChip: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 6,
      maxWidth: 220,
      minHeight: 36,
      borderRadius: 10,
      border: '1px solid var(--canvas-border)',
      background: 'var(--canvas-toolbar-bg)',
      padding: '0 8px 0 10px',
    },
    chatFileChipLink: {
      minWidth: 0,
      flex: 1,
      display: 'inline-flex',
      alignItems: 'center',
      gap: 6,
      color: 'var(--canvas-text-secondary)',
      textDecoration: 'none',
    },
    chatFileChipText: {
      minWidth: 0,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      fontSize: 12,
      lineHeight: 1.4,
    },
    chatFileChipRemove: {
      width: 20,
      height: 20,
      minWidth: 20,
      borderRadius: '50%',
      border: 'none',
      background: 'transparent',
      color: 'var(--canvas-text-muted)',
      cursor: 'pointer',
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: 0,
    },
    removeImageBtn: {
      position: 'absolute',
      top: -6,
      right: -6,
      width: 18,
      height: 18,
      borderRadius: '50%',
      background: 'var(--canvas-error)',
      border: 'none',
      cursor: 'pointer',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: 'var(--canvas-user-message-text)',
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
    composerParameterRow: {
      display: 'flex',
      alignItems: 'center',
      gap: 7,
      flexWrap: isMobile ? 'nowrap' : 'wrap',
      minWidth: 0,
      flexShrink: 0,
    },
    composerActionGroup: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      flexShrink: 0,
    },
    filterLabel: {
      fontSize: 13,
      color: 'var(--canvas-text-muted)',
    },
    tasksGrid: {
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
      gap: isMobile ? 10 : 12,
      padding: 0,
      width: '100%',
      alignContent: 'start',
      flexShrink: 0,
    },
    workspaceTopbar: {
      height: isMobile ? 44 : 0,
      flexShrink: 0,
      borderBottom: 'none',
      background: 'var(--canvas-toolbar-bg)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 12,
      padding: isMobile ? '0 10px' : 0,
    },
    workspaceTopbarLeft: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      minWidth: 0,
    },
    workspaceBody: {
      flex: 1,
      minHeight: 0,
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
      background: 'var(--canvas-page-bg)',
    },
    workspaceScrollPanel: {
      flex: 1,
      minHeight: 0,
      overflowY: 'auto',
      display: 'flex',
      flexDirection: 'column',
      background: 'var(--canvas-page-bg)',
      overscrollBehavior: 'contain',
      WebkitOverflowScrolling: 'touch',
    },
    mainViewport: {
      width: '100%',
      padding: isMobile ? '10px 10px 6px' : '16px 24px 6px',
    },
    composerDock: {
      marginTop: 'auto',
      position: 'sticky',
      bottom: 0,
      zIndex: 2,
      flexShrink: 0,
      borderTop: 'none',
      background:
        'linear-gradient(180deg, rgba(245, 247, 251, 0) 0%, rgba(245, 247, 251, 0.26) 42%, rgba(245, 247, 251, 0.88) 100%)',
      padding: isMobile ? '6px 12px 10px' : '4px 24px 10px',
    },
    chatAutoFollowDock: {
      width: '100%',
      maxWidth: isMobile ? '100%' : 872,
      margin: '0 auto 10px',
      display: 'flex',
      justifyContent: 'flex-end',
      padding: isMobile ? '0 2px' : '0 4px',
    },
    chatAutoFollowButton: {
      height: 32,
      borderRadius: 999,
      border: '1px solid var(--canvas-primary-soft-border)',
      background: 'rgba(255, 255, 255, 0.94)',
      color: 'var(--canvas-primary)',
      padding: '0 12px',
      display: 'inline-flex',
      alignItems: 'center',
      gap: 6,
      fontSize: 13,
      fontWeight: 600,
      boxShadow: 'var(--canvas-shadow-sm)',
      cursor: 'pointer',
      transition:
        'transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease, background 0.18s ease, color 0.18s ease',
    },
    composerShell: {
      width: '100%',
      maxWidth: isMobile ? '100%' : 860,
      margin: '0 auto',
      border: '1px solid var(--canvas-composer-shell-border)',
      borderRadius: 'var(--canvas-input-radius)',
      background: 'var(--canvas-composer-shell-bg)',
      boxShadow: 'var(--canvas-composer-shell-shadow)',
      padding: 0,
    },
    chatStream: {
      minHeight: '100%',
      display: 'flex',
      flexDirection: 'column',
      gap: isMobile ? 10 : 12,
      maxWidth: 1040,
      margin: '0 auto',
      padding: isMobile ? '10px 10px 14px' : '14px 16px 18px',
      width: '100%',
      background: 'transparent',
      border: 'none',
      borderRadius: 0,
      boxShadow: 'none',
    },
    chatContextNotice: {
      width: '100%',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 12,
      border: '1px solid var(--canvas-status-info-border)',
      background: 'var(--canvas-status-info-bg)',
      borderRadius: 14,
      padding: isMobile ? '10px 12px' : '12px 14px',
      boxShadow: 'var(--canvas-shadow-sm)',
    },
    chatContextNoticeText: {
      flex: 1,
      minWidth: 0,
      color: 'var(--canvas-status-info-text)',
      lineHeight: 1.5,
    },
    canvasStreamEmpty: {
      maxWidth: 1000,
      margin: '0 auto',
      width: '100%',
      padding: isMobile ? '6px 0 10px' : '8px 0 12px',
    },
    canvasMessageList: {
      display: 'flex',
      flexDirection: 'column',
      gap: isMobile ? 8 : 10,
      width: '100%',
    },
    canvasMessageHistoryHint: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 8,
      width: '100%',
      color: 'var(--canvas-text-muted)',
      fontSize: 12,
      lineHeight: 1.5,
      padding: '2px 0 4px',
    },
    canvasChatMessageRow: {
      width: '100%',
      display: 'flex',
      flexDirection: 'column',
      gap: isMobile ? 6 : 7,
    },
    canvasChatMessageRowUser: {
      alignItems: 'flex-end',
    },
    canvasChatMessageRowAssistant: {
      alignItems: 'flex-start',
    },
    canvasChatUserBubble: {
      width: 'fit-content',
      maxWidth: isMobile ? '84%' : '70%',
      borderRadius: '16px 16px 4px 16px',
      border: '1px solid var(--canvas-user-message-border)',
      background: 'var(--canvas-user-message-bg)',
      color: 'var(--canvas-user-message-text)',
      padding: isMobile ? '12px 14px' : '13px 16px',
      boxShadow: '0 4px 14px rgba(109, 93, 246, 0.08)',
    },
    canvasChatUserBubbleSelected: {
      boxShadow: 'var(--canvas-selected-shadow)',
    },
    canvasChatUserPrompt: {
      whiteSpace: 'pre-wrap',
      color: 'inherit',
      lineHeight: 1.66,
      wordBreak: 'break-word',
    },
    canvasChatAttachmentList: {
      display: 'flex',
      flexWrap: 'wrap',
      gap: 8,
      marginTop: 10,
    },
    canvasChatAssistantContent: {
      width: '100%',
      maxWidth: isMobile ? '100%' : '82%',
      color: 'var(--canvas-text-primary)',
      lineHeight: 1.74,
      padding: 0,
      display: 'flex',
      flexDirection: 'column',
      gap: isMobile ? 8 : 10,
      borderRadius: 0,
      border: 'none',
      background: 'transparent',
      boxShadow: 'none',
    },
    canvasChatAssistantContentSelected: {
      background: 'transparent',
      borderColor: 'transparent',
      boxShadow: 'none',
    },
    canvasChatReasoningWrap: {
      width: '100%',
      display: 'flex',
      flexDirection: 'column',
      gap: 4,
      borderRadius: 0,
      border: 'none',
      background: 'transparent',
      overflow: 'visible',
      boxShadow: 'none',
    },
    canvasChatReasoningToggle: {
      width: 'fit-content',
      maxWidth: '100%',
      border: 'none',
      background: 'transparent',
      padding: 0,
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'flex-start',
      gap: 8,
      cursor: 'pointer',
      color: 'var(--canvas-thought-title)',
      textAlign: 'left',
    },
    canvasChatReasoningToggleLead: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 8,
      minWidth: 0,
      flex: '0 1 auto',
    },
    canvasChatReasoningIcon: {
      width: 16,
      height: 16,
      borderRadius: 0,
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: 'var(--canvas-thought-title)',
      background: 'transparent',
      border: 'none',
      flexShrink: 0,
    },
    canvasChatReasoningToggleText: {
      fontSize: 13,
      fontWeight: 500,
      color: 'inherit',
      lineHeight: 1.6,
      whiteSpace: 'normal',
    },
    canvasChatReasoningToggleArrow: {
      color: 'var(--canvas-text-muted)',
      flexShrink: 0,
    },
    canvasChatReasoningPanel: {
      borderTop: 'none',
      padding: 0,
      background: 'transparent',
    },
    canvasChatReasoningMarkdown: {
      width: '100%',
      color: 'var(--canvas-thought-content)',
      fontSize: 13,
      lineHeight: 1.78,
    },
    canvasChatMarkdown: {
      width: '100%',
      color: 'var(--canvas-text-primary)',
    },
    canvasChatTextStatus: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 8,
      width: 'fit-content',
      maxWidth: '100%',
      minHeight: 0,
      padding: 0,
      border: 'none',
      background: 'transparent',
      boxShadow: 'none',
      color: 'var(--canvas-text-secondary)',
      fontSize: 13,
      lineHeight: 1.6,
    },
    canvasChatTextStatusNeutral: {
      color: 'var(--canvas-text-muted)',
    },
    canvasChatTextStatusError: {
      color: 'var(--canvas-status-error-text)',
    },
    canvasChatStatusDetail: {
      maxWidth: '100%',
      whiteSpace: 'pre-wrap',
      wordBreak: 'break-word',
      overflowWrap: 'anywhere',
    },
    canvasChatInlineStatus: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 8,
      width: 'fit-content',
      maxWidth: '100%',
      minHeight: 36,
      padding: '8px 12px',
      borderRadius: 12,
      border: '1px solid var(--canvas-status-info-border)',
      background: 'var(--canvas-status-info-bg)',
      boxShadow: 'var(--canvas-shadow-sm)',
      color: 'var(--canvas-status-info-text)',
      lineHeight: 1.55,
    },
    canvasChatInlineStatusNeutral: {
      borderColor: 'var(--canvas-status-neutral-border)',
      background: 'var(--canvas-status-neutral-bg)',
      color: 'var(--canvas-status-neutral-text)',
    },
    canvasChatInlineStatusError: {
      borderColor: 'var(--canvas-status-error-border)',
      background: 'var(--canvas-status-error-bg)',
      color: 'var(--canvas-status-error-text)',
    },
    canvasChatErrorText: {
      maxWidth: '100%',
      whiteSpace: 'pre-wrap',
      wordBreak: 'break-word',
      overflowWrap: 'anywhere',
      lineHeight: 1.55,
    },
    canvasChatMetaRow: {
      width: '100%',
      display: 'flex',
      alignItems: 'flex-start',
      justifyContent: 'space-between',
      gap: 12,
      minHeight: 28,
    },
    canvasChatMetaRowUser: {
      justifyContent: 'flex-end',
    },
    canvasChatStatusSlot: {
      flex: 1,
      minWidth: 0,
      display: 'flex',
      alignItems: 'center',
      minHeight: 28,
    },
    canvasChatStatusSlotEmpty: {
      visibility: 'hidden',
    },
    canvasChatActions: {
      display: 'flex',
      alignItems: 'center',
      gap: 6,
      opacity: isMobile ? 0.82 : 0.22,
      transform: 'translateY(0)',
      transition: 'opacity 0.16s ease, transform 0.16s ease',
      pointerEvents: 'auto',
    },
    canvasChatActionsVisible: {
      opacity: 1,
      transform: 'translateY(0)',
      pointerEvents: 'auto',
    },
    canvasChatActionButton: {
      '--canvas-action-button-bg': 'rgba(248, 250, 252, 0.94)',
      '--canvas-action-button-border': 'rgba(226, 232, 240, 0.96)',
      '--canvas-action-button-text': 'var(--canvas-text-secondary)',
      '--canvas-action-button-shadow': '0 1px 2px rgba(15, 23, 42, 0.04)',
      width: 30,
      minWidth: 30,
      height: 30,
      minHeight: 30,
      borderRadius: 11,
      border: '1px solid var(--canvas-action-button-border)',
      background: 'var(--canvas-action-button-bg)',
      color: 'var(--canvas-action-button-text)',
      padding: 0,
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      cursor: 'pointer',
      boxShadow: 'var(--canvas-action-button-shadow)',
      transition:
        'background 0.16s ease, color 0.16s ease, opacity 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease',
    },
    canvasChatActionButtonDisabled: {
      opacity: 0.5,
      cursor: 'not-allowed',
    },
    canvasMessageRow: {
      width: 'fit-content',
      maxWidth: isMobile ? '92%' : '70%',
      borderRadius: isMobile ? '18px' : '20px',
      padding: isMobile ? '12px 14px' : '14px 16px',
      border: '1px solid var(--canvas-border)',
      background: 'var(--canvas-card-bg)',
      color: 'var(--canvas-text-primary)',
      lineHeight: 1.62,
      wordBreak: 'break-word',
      boxShadow: 'var(--canvas-card-shadow)',
    },
    canvasMessageRowUser: {
      alignSelf: 'flex-end',
      borderRadius: isMobile ? '20px 20px 12px 20px' : '22px 22px 12px 22px',
      background: 'var(--canvas-user-message-bg)',
      borderColor: 'var(--canvas-user-message-border)',
    },
    canvasMessageRowAssistant: {
      alignSelf: 'flex-start',
      background: 'var(--canvas-card-bg)',
    },
    canvasGenerationRow: {
      width: '100%',
      display: 'flex',
      flexDirection: 'column',
      gap: 10,
      alignItems: 'flex-start',
    },
    canvasGenerationRowUser: {
      alignItems: 'flex-end',
    },
    canvasMessageRowSelected: {
      borderColor: 'var(--canvas-primary-soft-border)',
      background: 'var(--canvas-message-selected-bg)',
      boxShadow: 'var(--canvas-selected-shadow)',
    },
    canvasGenerationRowSelected: {
      filter: 'drop-shadow(0 8px 18px rgba(15, 23, 42, 0.1))',
    },
    canvasMessageHead: {
      display: 'flex',
      justifyContent: 'flex-start',
      gap: 12,
      marginBottom: 6,
    },
    canvasMessageBody: {
      display: 'flex',
      flexDirection: 'column',
      gap: 10,
      width: '100%',
    },
    canvasMessagePrompt: {
      maxWidth: isMobile ? '92%' : 680,
      whiteSpace: 'pre-wrap',
      color: 'var(--canvas-text-primary)',
      lineHeight: 1.66,
    },
    canvasMessageRefs: {
      display: 'flex',
      flexWrap: 'wrap',
      gap: 8,
    },
    canvasReferenceThumbWrap: {
      width: 140,
    },
    canvasReferenceThumb: {
      display: 'block',
      width: '100%',
      height: 96,
      objectFit: 'cover',
      borderRadius: 8,
      border: '1px solid var(--canvas-border)',
      background: 'var(--canvas-toolbar-bg)',
    },
    canvasGenerationCardWrap: {
      display: 'flex',
      flexDirection: 'column',
      gap: 10,
      alignItems: 'flex-start',
      width: '100%',
    },
    canvasGenerationBatchRow: {
      width: '100%',
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
      alignItems: 'stretch',
    },
    canvasGenerationBatchGrid: {
      width: '100%',
      maxWidth: isMobile ? '100%' : 980,
      display: 'grid',
      gridTemplateColumns: isMobile
        ? 'repeat(auto-fit, minmax(min(320px, 100%), 1fr))'
        : 'repeat(auto-fit, minmax(220px, 1fr))',
      gap: isMobile ? 8 : 6,
      alignItems: 'start',
      alignSelf: 'flex-start',
    },
    canvasGenerationBatchCard: {
      minWidth: 0,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'stretch',
    },
    canvasMediaCard: {
      width: '100%',
      minWidth: 0,
      maxWidth: isMobile ? 320 : 420,
      maxHeight: isMobile ? 280 : 360,
      position: 'relative',
      overflow: 'hidden',
      borderRadius: 8,
      border: '1px solid var(--canvas-border)',
      background: 'var(--canvas-card-bg)',
      boxShadow: 'var(--canvas-card-shadow)',
    },
    canvasMediaCardClickable: {
      cursor: 'zoom-in',
    },
    canvasMediaFrame: {
      position: 'absolute',
      inset: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      overflow: 'hidden',
    },
    canvasMediaContent: {
      width: '100%',
      height: '100%',
      objectFit: 'cover',
      objectPosition: 'center',
      display: 'block',
      background: 'var(--canvas-toolbar-bg)',
    },
    canvasMediaVideoContent: {
      width: '100%',
      height: '100%',
      objectFit: 'contain',
      display: 'block',
      background: 'var(--canvas-toolbar-bg)',
    },
    canvasMediaStatusBody: {
      width: '100%',
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 8,
      color: 'var(--canvas-text-muted)',
      padding: 16,
    },
    canvasMediaStatusBodyOverlay: {
      width: '100%',
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 8,
      color: 'var(--canvas-media-panel-text)',
      padding: 16,
    },
    canvasMediaStatusOverlay: {
      position: 'absolute',
      inset: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      background: 'var(--canvas-media-overlay)',
      pointerEvents: 'none',
    },
    canvasMediaStatusOverlayError: {
      background: 'var(--canvas-media-overlay-error)',
    },
    canvasMediaStatusOverlayInteractive: {
      pointerEvents: 'auto',
    },
    canvasMediaStatusOverlayText: {
      color: 'var(--canvas-media-panel-text)',
      maxWidth: '100%',
      whiteSpace: 'pre-wrap',
      wordBreak: 'break-word',
      overflowWrap: 'anywhere',
      textAlign: 'center',
      lineHeight: 1.5,
      textShadow: '0 1px 2px rgba(0, 0, 0, 0.28)',
      userSelect: 'text',
    },
    canvasErrorText: {
      maxWidth: '100%',
      whiteSpace: 'pre-wrap',
      wordBreak: 'break-word',
      overflowWrap: 'anywhere',
      textAlign: 'center',
      lineHeight: 1.5,
      userSelect: 'text',
    },
    canvasErrorBlock: {
      width: '100%',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: 8,
    },
    canvasErrorCopyButton: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 4,
      padding: '4px 10px',
      borderRadius: 999,
      border: '1px solid var(--canvas-danger-chip-border)',
      background: 'var(--canvas-danger-chip-bg)',
      color: 'var(--canvas-danger-chip-text)',
      cursor: 'pointer',
      fontSize: 12,
      lineHeight: 1,
      boxShadow: 'var(--canvas-shadow-md)',
    },
    canvasErrorCopyButtonOverlay: {
      border: '1px solid var(--canvas-media-control-border)',
      background: 'var(--canvas-media-control-bg)',
      color: 'var(--canvas-media-control-text)',
    },
    canvasMessageResult: {
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
      alignItems: 'flex-start',
    },
    canvasMessageResultFrame: {
      maxWidth: '100%',
    },
    messageResultImage: {
      display: 'block',
      width: '100%',
      height: '100%',
      objectFit: 'cover',
      objectPosition: 'center',
      background: 'var(--canvas-toolbar-bg)',
    },
    messageResultVideo: {
      display: 'block',
      width: '100%',
      maxWidth: isMobile ? '100%' : 560,
      maxHeight: isMobile ? 420 : 620,
      borderRadius: 8,
      background: 'var(--canvas-preview-surface)',
    },
    messagePending: {
      minWidth: 180,
      minHeight: 96,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 8,
      padding: '12px 14px',
      borderRadius: 14,
      border: '1px solid var(--canvas-status-neutral-border)',
      background: 'var(--canvas-status-neutral-bg)',
      boxShadow: 'var(--canvas-shadow-sm)',
      color: 'var(--canvas-status-neutral-text)',
    },
    assetGrid: {
      display: 'grid',
      gridTemplateColumns: isMobile
        ? 'repeat(2, minmax(0, 1fr))'
        : 'repeat(3, minmax(0, 1fr))',
      gap: 12,
    },
    assetCard: {
      position: 'relative',
      border: '1px solid var(--canvas-border)',
      borderRadius: 10,
      overflow: 'hidden',
      background: 'var(--canvas-card-bg)',
      display: 'flex',
      flexDirection: 'column',
      cursor: 'pointer',
      minWidth: 0,
      transition:
        'border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease',
    },
    assetSelect: {
      position: 'absolute',
      top: 6,
      left: 6,
      zIndex: 1,
      padding: 3,
      borderRadius: 6,
      background: 'var(--canvas-media-control-bg)',
      lineHeight: 1,
    },
    assetThumb: {
      width: '100%',
      aspectRatio: '1 / 1',
      objectFit: 'cover',
      background: 'var(--canvas-toolbar-bg)',
      display: 'block',
    },
    assetActions: {
      display: 'flex',
      flexWrap: 'wrap',
      gap: 6,
      padding: 8,
      borderTop: '1px solid var(--canvas-border)',
    },
    assetCardMeta: {
      padding: '8px',
      display: 'flex',
      flexDirection: 'column',
      gap: 2,
      minWidth: 0,
    },
    assetPreviewOverlay: {
      position: 'fixed',
      inset: 0,
      zIndex: 1200,
      padding: isMobile ? 12 : 24,
      background: 'var(--canvas-overlay-bg)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    assetPreviewPanel: {
      width: 'min(100%, 980px)',
      maxHeight: 'calc(100vh - 48px)',
      display: 'grid',
      gridTemplateColumns: isMobile
        ? '1fr'
        : 'minmax(0, 1.35fr) minmax(280px, 0.65fr)',
      gap: 0,
      overflow: 'hidden',
      borderRadius: 10,
      border: '1px solid var(--canvas-border)',
      background: 'var(--canvas-card-bg)',
      boxShadow: 'var(--canvas-shadow-lg)',
    },
    assetPreviewImagePane: {
      minHeight: isMobile ? 220 : 520,
      maxHeight: isMobile ? '48vh' : 'calc(100vh - 48px)',
      background: 'var(--canvas-toolbar-bg)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      overflow: 'hidden',
    },
    assetPreviewImage: {
      width: '100%',
      height: '100%',
      maxHeight: isMobile ? '48vh' : 'calc(100vh - 48px)',
      objectFit: 'contain',
      display: 'block',
    },
    canvasImagePreviewPanel: {
      position: 'relative',
      width: 'min(100%, 980px)',
      maxHeight: 'calc(100vh - 48px)',
      overflow: 'hidden',
      borderRadius: 10,
      border: '1px solid var(--canvas-border)',
      background: 'var(--canvas-card-bg)',
      boxShadow: 'var(--canvas-shadow-lg)',
    },
    canvasImagePreviewImage: {
      width: '100%',
      height: '100%',
      maxHeight: 'calc(100vh - 48px)',
      objectFit: 'contain',
      display: 'block',
      background: 'var(--canvas-toolbar-bg)',
    },
    canvasImagePreviewActions: {
      position: 'absolute',
      top: isMobile ? 10 : 12,
      right: isMobile ? 10 : 12,
      zIndex: 2,
      display: 'flex',
      gap: 8,
    },
    canvasImagePreviewActionBtn: {
      width: isMobile ? 36 : 32,
      height: isMobile ? 36 : 32,
      borderRadius: 999,
      border:
        '1px solid var(--canvas-media-control-border, rgba(255, 255, 255, 0.16))',
      background: 'var(--canvas-media-control-bg, rgba(15, 23, 42, 0.58))',
      color: 'var(--canvas-media-control-text, #fff)',
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      cursor: 'pointer',
      boxShadow: '0 8px 22px rgba(15, 23, 42, 0.18)',
      backdropFilter: 'blur(8px)',
      transition: 'transform 0.15s ease, background 0.15s ease',
    },
    assetPreviewInfo: {
      padding: isMobile ? 12 : 16,
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
      overflowY: 'auto',
      maxHeight: isMobile ? '44vh' : 'calc(100vh - 48px)',
      minWidth: 0,
    },
    assetPreviewMetaItem: {
      display: 'flex',
      flexDirection: 'column',
      gap: 3,
      padding: 0,
      borderRadius: 0,
      background: 'transparent',
      minWidth: 0,
    },
    drawerFooterPager: {
      marginTop: 12,
      display: 'flex',
      justifyContent: 'space-between',
      alignItems: 'center',
      gap: 8,
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
    !!selectedModel && !!selectedModelData && imageUiState.canGenerate;

  const canGenerateVideo =
    !!videoSelectedModel &&
    !!videoSelectedModelData &&
    !!videoPrompt.trim() &&
    !!videoSelectedModelData?.duration_options?.length &&
    ((!!videoReferenceImage && videoSelectedModelSupportsImageToVideo) ||
      (!videoReferenceImage && videoSelectedModelSupportsTextToVideo));

  const renderReferenceThumb = (file, onRemove) => (
    <div
      key={file.uid || file.name || file.url}
      style={styles.referenceImageContainer}
    >
      <img
        src={
          file.url ||
          (file.fileInstance && URL.createObjectURL(file.fileInstance))
        }
        alt=''
        style={styles.referenceImageThumb}
      />
      <button
        type='button'
        aria-label={t('移除图片')}
        style={styles.removeImageBtn}
        onClick={onRemove}
      >
        <IconDelete size='extra-small' />
      </button>
    </div>
  );

  const buildCanvasPreviewFilename = (taskId) => {
    const normalizedTaskId = String(taskId || '').trim();
    if (normalizedTaskId) {
      return `image-${normalizedTaskId}.png`;
    }
    return `canvas-image-${Date.now()}.png`;
  };

  const buildCanvasImagePreviewState = ({
    src = '',
    downloadSrc = '',
    taskId = '',
    messageId = '',
  } = {}) => {
    const normalizedSrc = String(src || '').trim();
    const normalizedDownloadSrc = String(downloadSrc || '').trim();
    const normalizedTaskId = String(taskId || '').trim();
    const normalizedMessageId = String(messageId || '').trim();

    if (!normalizedSrc && !normalizedDownloadSrc) {
      return null;
    }

    return {
      src: normalizedSrc || normalizedDownloadSrc,
      downloadSrc: normalizedDownloadSrc || normalizedSrc,
      filename: buildCanvasPreviewFilename(normalizedTaskId),
      taskId: normalizedTaskId,
      messageId: normalizedMessageId,
    };
  };

  const handleDownloadCanvasPreviewImage = (preview) => {
    const href = String(preview?.downloadSrc || preview?.src || '').trim();
    if (!href) {
      return;
    }

    const link = document.createElement('a');
    link.href = href;
    link.download = preview?.filename || buildCanvasPreviewFilename('');
    link.target = '_blank';
    link.rel = 'noopener noreferrer';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  const renderChatFileAttachmentChip = (file, onRemove = null) => (
    <div key={file.uid || file.name} style={styles.chatFileChip}>
      <a
        href={file.data || file.url || '#'}
        target='_blank'
        rel='noreferrer'
        style={styles.chatFileChipLink}
      >
        <IconArchive size='small' />
        <span style={styles.chatFileChipText}>{file.name}</span>
      </a>
      {onRemove ? (
        <button
          type='button'
          aria-label={t('移除文件')}
          style={styles.chatFileChipRemove}
          onClick={onRemove}
        >
          <IconDelete size='extra-small' />
        </button>
      ) : null}
    </div>
  );

  const renderCanvasChatAttachment = (attachment) => {
    if (!attachment) {
      return null;
    }
    if (attachment.kind === 'image') {
      return renderCanvasReferenceThumb(
        {
          uid: `${attachment.kind}-${attachment.name}`,
          url: attachment.data,
        },
        attachment.name || t('图片附件'),
      );
    }
    return renderChatFileAttachmentChip({
      uid: `${attachment.kind}-${attachment.name}`,
      name: attachment.name || t('文件附件'),
      data: attachment.data,
    });
  };

  const renderPillDropdown = ({
    key,
    label,
    icon = null,
    value,
    displayValue,
    options,
    onChange,
    disabled = false,
    emptyText = t('暂无可用选项'),
    extraContent = null,
  }) => {
    const dropdownKey = key;
    const iconOnly = isMobile && !!icon;
    const controlKind =
      dropdownKey === 'composer-mode'
        ? 'mode-switch'
        : dropdownKey.includes('model')
          ? 'model-selector'
          : 'parameter';
    const buttonText = label
      ? `${label} ${displayValue || t('请选择')}`
      : displayValue || t('请选择');
    const closeDropdown = () => {
      setActiveDropdownKey((current) =>
        current === dropdownKey ? '' : current,
      );
    };
    const menu = (
      <Dropdown.Menu style={styles.darkMenu}>
        {options.length > 0 ? (
          options.map((option) => {
            const selected = option.value === value;
            const optionDisabled = !!option.disabled;
            return (
              <Dropdown.Item
                key={option.value}
                style={{
                  ...styles.darkMenuItem,
                  ...(selected ? styles.darkMenuItemActive : null),
                  opacity: optionDisabled ? 0.5 : 1,
                  cursor: optionDisabled ? 'not-allowed' : 'pointer',
                }}
                onClick={() => {
                  if (optionDisabled) {
                    return;
                  }
                  onChange(option.value);
                  closeDropdown();
                }}
              >
                <span
                  style={{
                    ...styles.darkMenuItemContent,
                    alignItems: option.meta ? 'flex-start' : 'center',
                  }}
                >
                  {option.icon ? (
                    <span style={styles.pillButtonIcon}>{option.icon}</span>
                  ) : null}
                  {option.meta ? (
                    <span style={styles.dropdownOptionMetaWrap}>
                      <span>{option.label}</span>
                      <span style={styles.dropdownOptionMeta}>
                        {option.meta}
                      </span>
                    </span>
                  ) : (
                    <span>{option.label}</span>
                  )}
                </span>
              </Dropdown.Item>
            );
          })
        ) : (
          <div
            style={{
              ...styles.darkMenuItem,
              color: 'var(--canvas-text-muted)',
            }}
          >
            {emptyText}
          </div>
        )}
        {extraContent ? (
          <div
            style={{
              marginTop: 6,
              borderTop: '1px solid var(--canvas-border)',
            }}
          >
            {extraContent}
          </div>
        ) : null}
      </Dropdown.Menu>
    );

    return (
      <Dropdown
        key={dropdownKey}
        trigger='click'
        position='bottomLeft'
        render={menu}
        visible={!disabled && activeDropdownKey === dropdownKey}
        onVisibleChange={(visible) => {
          setActiveDropdownKey((current) =>
            visible ? dropdownKey : current === dropdownKey ? '' : current,
          );
        }}
      >
        <button
          className='canvas-composer-pill'
          type='button'
          aria-label={buttonText}
          title={buttonText}
          aria-haspopup='menu'
          aria-expanded={!disabled && activeDropdownKey === dropdownKey}
          data-composer-control-kind={controlKind}
          data-composer-control-active={
            value !== '' && value !== null && value !== undefined
              ? 'true'
              : 'false'
          }
          style={{
            ...styles.pillButton,
            ...(iconOnly ? styles.pillButtonIconOnly : null),
            ...(value !== '' && value !== null && value !== undefined
              ? styles.pillButtonSelected
              : styles.pillButtonMuted),
            ...(!disabled && activeDropdownKey === dropdownKey
              ? styles.pillButtonOpen
              : null),
            opacity: disabled ? 0.55 : 1,
            cursor: disabled ? 'not-allowed' : 'pointer',
          }}
          disabled={disabled}
        >
          {icon ? <span style={styles.pillButtonIcon}>{icon}</span> : null}
          {!iconOnly ? (
            <span style={styles.pillButtonLabel}>{buttonText}</span>
          ) : null}
          {!iconOnly ? <IconChevronDown size='small' /> : null}
        </button>
      </Dropdown>
    );
  };

  const getAspectRatioDisplay = (value) =>
    String(value || '')
      .trim()
      .toLowerCase() === 'auto'
      ? t('智能')
      : String(value || '');

  const getAspectRatioSummaryDisplay = (value) =>
    String(value || '')
      .trim()
      .toLowerCase() === 'auto'
      ? t('智能比例')
      : String(value || '');

  const getResolutionDisplay = (value) => {
    const text = String(value || '').trim();
    const normalized = text.toUpperCase();
    if (normalized === '2K') {
      return t('高清 2K');
    }
    if (normalized === '4K') {
      return t('超清 4K');
    }
    return text;
  };

  const renderImageParamOption = ({ value, label, selected, onClick }) => (
    <button
      key={value}
      type='button'
      style={{
        ...styles.imageParamOption,
        ...(selected ? styles.imageParamOptionActive : null),
      }}
      onClick={(event) => {
        event.stopPropagation();
        onClick(value);
      }}
    >
      {label}
    </button>
  );

  const renderChatSettingsDropdown = () => {
    const dropdownKey = 'chat-settings';
    const iconOnly = isMobile;
    const showWebSearchToggle = getCanvasChatWebSearchVisibility(
      activeChatModelOption,
    );
    const panel = (
      <div
        style={styles.imageParamPanel}
        onClick={(event) => event.stopPropagation()}
      >
        <div style={styles.imageParamSection}>
          <div style={styles.imageParamSectionTitle}>{t('温度')}</div>
          <div style={styles.imageParamOptionRow}>
            {['0', '0.2', '0.7', '1', '1.5'].map((item) =>
              renderImageParamOption({
                value: item,
                label: item,
                selected: item === chatTemperature,
                onClick: (value) => {
                  const nextValue = String(value);
                  setChatTemperature(nextValue);
                  const parsed = Number(nextValue);
                  if (Number.isFinite(parsed)) {
                    updateCurrentCanvasChatSessionConfig(
                      { chat_temperature: parsed },
                      t('更新会话温度失败'),
                    );
                  }
                },
              }),
            )}
          </div>
        </div>
        <div style={styles.imageParamDivider} />
        <div style={styles.imageParamSection}>
          <div style={styles.imageParamSectionTitle}>{t('上下文')}</div>
          <div style={styles.imageParamOptionRow}>
            {['0', '4', '8', '16', '32'].map((item) =>
              renderImageParamOption({
                value: item,
                label: item,
                selected: item === chatContext,
                onClick: (value) => {
                  const nextValue = String(value);
                  setChatContext(nextValue);
                  const parsed = Number(nextValue);
                  if (Number.isFinite(parsed)) {
                    updateCurrentCanvasChatSessionConfig(
                      { chat_context_count: parsed },
                      t('更新会话上下文失败'),
                    );
                  }
                },
              }),
            )}
          </div>
        </div>
        {showWebSearchToggle ? (
          <>
            <div style={styles.imageParamDivider} />
            <div style={styles.imageParamSection}>
              <div style={styles.imageParamSectionTitle}>{t('联网搜索')}</div>
              <Checkbox
                checked={chatWebSearchEnabled}
                onChange={(event) => {
                  const nextValue = !!event.target.checked;
                  setChatWebSearchEnabled(nextValue);
                  updateCurrentCanvasChatSessionConfig(
                    { web_search_enabled: nextValue },
                    t('更新联网搜索设置失败'),
                  );
                }}
              >
                {t('当前会话启用联网搜索')}
              </Checkbox>
            </div>
          </>
        ) : null}
      </div>
    );

    return (
      <Dropdown
        key={dropdownKey}
        trigger='click'
        position='bottomLeft'
        render={panel}
        visible={activeDropdownKey === dropdownKey}
        onVisibleChange={(visible) => {
          setActiveDropdownKey((current) =>
            visible ? dropdownKey : current === dropdownKey ? '' : current,
          );
        }}
      >
        <button
          className='canvas-composer-pill'
          type='button'
          aria-label={t('聊天设置')}
          title={t('聊天设置')}
          aria-haspopup='dialog'
          aria-expanded={activeDropdownKey === dropdownKey}
          data-composer-control-kind='parameter'
          data-composer-control-active={
            activeDropdownKey === dropdownKey ? 'true' : 'false'
          }
          style={{
            ...styles.pillButton,
            ...(iconOnly ? styles.pillButtonIconOnly : null),
            ...(activeDropdownKey === dropdownKey
              ? styles.pillButtonOpen
              : styles.pillButtonSelected),
          }}
        >
          <IconSetting size='small' />
          {!iconOnly ? <span>{t('设置')}</span> : null}
          {!iconOnly ? <IconChevronDown size='small' /> : null}
        </button>
      </Dropdown>
    );
  };

  const renderGenerationParametersDropdown = ({
    key,
    ariaLabel,
    aspectRatios,
    resolutions,
    aspectRatioValue,
    resolutionValue,
    onAspectRatioChange,
    onResolutionChange,
    showAspectRatioSelector,
    showResolutionSelector,
    quantityValue = 0,
    quantityOptions = [],
    onQuantityChange = null,
  }) => {
    const hasAspectRatios = showAspectRatioSelector && aspectRatios.length > 0;
    const hasResolutions = showResolutionSelector && resolutions.length > 0;
    const iconOnly = isMobile;
    const hasQuantitySelector =
      Array.isArray(quantityOptions) &&
      quantityOptions.length > 0 &&
      typeof onQuantityChange === 'function';
    if (!hasAspectRatios && !hasResolutions && !hasQuantitySelector) {
      return null;
    }

    const dropdownKey = key;
    const dimensionPreview = getImageDimensionPreview(
      hasResolutions ? resolutionValue : '',
      hasAspectRatios ? aspectRatioValue : '',
    );
    const displayParts = [];
    if (hasAspectRatios && aspectRatioValue) {
      displayParts.push(getAspectRatioSummaryDisplay(aspectRatioValue));
    }
    if (hasResolutions && resolutionValue) {
      displayParts.push(getResolutionDisplay(resolutionValue));
    }
    if (hasQuantitySelector && quantityValue > 0) {
      displayParts.push(`${t('数量')} ${quantityValue}`);
    }
    const displayValue =
      displayParts.length > 0 ? displayParts.join(' | ') : t('请选择');
    const buttonLabel =
      displayValue && displayValue !== t('请选择')
        ? `${ariaLabel} ${displayValue}`
        : ariaLabel;
    const panel = (
      <div
        style={styles.imageParamPanel}
        onClick={(event) => event.stopPropagation()}
      >
        {hasAspectRatios ? (
          <div style={styles.imageParamSection}>
            <div style={styles.imageParamSectionTitle}>{t('选择比例')}</div>
            <div style={styles.imageParamOptionRow}>
              {aspectRatios.map((ratio) =>
                renderImageParamOption({
                  value: ratio,
                  label: getAspectRatioDisplay(ratio),
                  selected: ratio === aspectRatioValue,
                  onClick: onAspectRatioChange,
                }),
              )}
            </div>
          </div>
        ) : null}
        {hasAspectRatios && hasResolutions ? (
          <div style={styles.imageParamDivider} />
        ) : null}
        {hasResolutions ? (
          <div style={styles.imageParamSection}>
            <div style={styles.imageParamSectionTitle}>{t('选择分辨率')}</div>
            <div style={styles.imageParamOptionRow}>
              {resolutions.map((item) =>
                renderImageParamOption({
                  value: item,
                  label: getResolutionDisplay(item),
                  selected: item === resolutionValue,
                  onClick: onResolutionChange,
                }),
              )}
            </div>
          </div>
        ) : null}
        {(hasAspectRatios || hasResolutions) && hasQuantitySelector ? (
          <div style={styles.imageParamDivider} />
        ) : null}
        {hasQuantitySelector ? (
          <div style={styles.imageParamSection}>
            <div style={styles.imageParamSectionTitle}>{t('生成数量')}</div>
            <div style={styles.imageParamOptionRow}>
              {quantityOptions.map((item) =>
                renderImageParamOption({
                  value: item,
                  label: String(item),
                  selected: item === quantityValue,
                  onClick: onQuantityChange,
                }),
              )}
            </div>
          </div>
        ) : null}
        <div style={styles.imageParamDivider} />
        <div style={styles.imageParamSection}>
          <div style={styles.imageParamSectionTitle}>{t('尺寸')}</div>
          <div style={styles.imageParamSizeRow}>
            <div style={styles.imageParamSizeField}>
              <span style={styles.imageParamSizeLabel}>W</span>
              <span style={styles.imageParamSizeValue}>
                {dimensionPreview.width}
              </span>
            </div>
            <div style={styles.imageParamSizeField}>
              <span style={styles.imageParamSizeLabel}>H</span>
              <span style={styles.imageParamSizeValue}>
                {dimensionPreview.height}
              </span>
            </div>
          </div>
        </div>
      </div>
    );

    return (
      <Dropdown
        key={dropdownKey}
        trigger='click'
        position='bottomLeft'
        render={panel}
        visible={activeDropdownKey === dropdownKey}
        onVisibleChange={(visible) => {
          setActiveDropdownKey((current) =>
            visible ? dropdownKey : current === dropdownKey ? '' : current,
          );
        }}
      >
        <button
          className='canvas-composer-pill'
          type='button'
          aria-label={buttonLabel}
          title={buttonLabel}
          aria-haspopup='dialog'
          aria-expanded={activeDropdownKey === dropdownKey}
          data-composer-control-kind='parameter'
          data-composer-control-active={
            displayParts.length > 0 ? 'true' : 'false'
          }
          style={{
            ...styles.pillButton,
            ...(iconOnly ? styles.pillButtonIconOnly : null),
            ...(displayParts.length > 0
              ? styles.pillButtonSelected
              : styles.pillButtonMuted),
            ...(activeDropdownKey === dropdownKey
              ? styles.pillButtonOpen
              : null),
            maxWidth: isMobile ? '100%' : 260,
          }}
        >
          <IconRealSizeStroked size='small' />
          {!iconOnly ? (
            <span style={styles.pillButtonLabel}>{displayValue}</span>
          ) : null}
          {!iconOnly ? <IconChevronDown size='small' /> : null}
        </button>
      </Dropdown>
    );
  };

  const renderImageParametersDropdown = () =>
    renderGenerationParametersDropdown({
      key: 'image-parameters',
      ariaLabel: t('图片参数'),
      aspectRatios: availableAspectRatios,
      resolutions: availableResolutions,
      aspectRatioValue: aspectRatio,
      resolutionValue: resolution,
      onAspectRatioChange: setAspectRatio,
      onResolutionChange: setResolution,
      showAspectRatioSelector: showImageAspectRatioSelector,
      showResolutionSelector: showImageResolutionSelector,
      quantityValue: quantity,
      quantityOptions: Array.from(
        { length: DEFAULT_MAX_BATCH_TASKS },
        (_, index) => index + 1,
      ),
      onQuantityChange: (value) => setQuantity(normalizeTaskCount(value)),
    });

  const renderVideoParametersDropdown = () =>
    renderGenerationParametersDropdown({
      key: 'video-parameters',
      ariaLabel: t('视频参数'),
      aspectRatios: videoAvailableAspectRatios,
      resolutions: videoAvailableResolutions,
      aspectRatioValue: videoAspectRatio,
      resolutionValue: videoResolution,
      onAspectRatioChange: setVideoAspectRatio,
      onResolutionChange: setVideoResolution,
      showAspectRatioSelector: showVideoAspectRatioSelector,
      showResolutionSelector: showVideoResolutionSelector,
    });

  const renderModelDropdown = (isVideoMode, activeModelLabel) => {
    const dropdownKey = isVideoMode ? 'video-model' : 'image-model';
    const modelOptions = isVideoMode ? enabledVideoModels : enabledImageModels;
    const selectedValue = isVideoMode ? videoSelectedModel : selectedModel;
    const selectedModelItem = modelOptions.find(
      (item) => item.request_model === selectedValue,
    );
    const iconOnly = isMobile;
    const buttonLabel = isVideoMode
      ? `${t('选择视频模型')} ${activeModelLabel}`
      : `${t('选择图片模型')} ${activeModelLabel}`;
    const handleSelect = (requestModel) => {
      const model = modelOptions.find(
        (item) => item.request_model === requestModel,
      );
      if (!model) {
        return;
      }
      if (isVideoMode) {
        selectVideoModelFromCatalog(model);
        return;
      }
      selectImageModelFromCatalog(model);
    };
    const menu = (
      <Dropdown.Menu
        style={{ ...styles.darkMenu, minWidth: isMobile ? 280 : 320 }}
      >
        {modelOptions.length > 0 ? (
          modelOptions.map((model) => {
            const selected = model.request_model === selectedValue;
            return (
              <Dropdown.Item
                key={model.request_model}
                style={{
                  ...styles.darkMenuItem,
                  ...(selected ? styles.darkMenuItemActive : null),
                }}
                onClick={() => {
                  handleSelect(model.request_model);
                  setActiveDropdownKey((current) =>
                    current === dropdownKey ? '' : current,
                  );
                }}
              >
                <div style={styles.modelMenuOption}>
                  <span style={styles.modelMenuIcon}>
                    <CanvasModelSeriesIcon series={model.model_series} />
                  </span>
                  <div style={styles.modelMenuItem}>
                    <span style={styles.modelMenuTitle}>
                      {getModelDisplayName(model)}
                    </span>
                  </div>
                </div>
              </Dropdown.Item>
            );
          })
        ) : (
          <div
            style={{
              ...styles.darkMenuItem,
              color: 'var(--canvas-text-muted)',
            }}
          >
            {isVideoMode
              ? t('当前没有可用视频模型')
              : t('当前分组下没有可用图片模型')}
          </div>
        )}
      </Dropdown.Menu>
    );

    return (
      <Dropdown
        key={dropdownKey}
        trigger='click'
        position='bottomLeft'
        render={menu}
        visible={modelOptions.length > 0 && activeDropdownKey === dropdownKey}
        onVisibleChange={(visible) => {
          setActiveDropdownKey((current) =>
            visible ? dropdownKey : current === dropdownKey ? '' : current,
          );
        }}
      >
        <button
          className='canvas-composer-pill'
          type='button'
          aria-label={buttonLabel}
          title={buttonLabel}
          aria-haspopup='menu'
          aria-expanded={
            modelOptions.length > 0 && activeDropdownKey === dropdownKey
          }
          data-canvas-model-selector={
            isVideoMode ? CANVAS_MODE_VIDEO : CANVAS_MODE_IMAGE
          }
          data-composer-control-kind='model-selector'
          data-composer-control-active={selectedModelItem ? 'true' : 'false'}
          style={{
            ...styles.pillButton,
            ...(iconOnly ? styles.pillButtonIconOnly : null),
            ...(selectedModelItem
              ? styles.pillButtonSelected
              : styles.pillButtonMuted),
            ...(modelOptions.length > 0 && activeDropdownKey === dropdownKey
              ? styles.pillButtonOpen
              : null),
          }}
          disabled={modelOptions.length === 0}
        >
          <CanvasModelSeriesIcon
            series={selectedModelItem?.model_series}
            size='small'
          />
          {!iconOnly ? (
            <span
              style={{
                ...styles.pillButtonLabel,
                maxWidth: isMobile ? 140 : 260,
              }}
            >
              {`${t('模型')} ${activeModelLabel}`}
            </span>
          ) : null}
          {!iconOnly ? <IconChevronDown size='small' /> : null}
        </button>
      </Dropdown>
    );
  };

  const renderSidebarEmpty = (title, description, action = null) => (
    <div style={{ padding: '28px 10px' }}>
      <Empty title={title} description={description} />
      {action ? (
        <div
          style={{ display: 'flex', justifyContent: 'center', marginTop: 12 }}
        >
          {action}
        </div>
      ) : null}
    </div>
  );

  const renderCanvasBlankSessionDescription = () => {
    const activeModel =
      generationMode === CANVAS_MODE_CHAT
        ? activeChatModelOption
        : generationMode === CANVAS_MODE_VIDEO
          ? videoSelectedModelData
          : selectedModelData;
    const description = String(activeModel?.description || '').trim();
    const vendorIcon = String(activeModel?.vendor_icon || '').trim();
    const modelSeries = String(activeModel?.model_series || '').trim();
    const useCatalogDescription = description && vendorIcon;
    const text = useCatalogDescription ? description : t('准备好开始了吗？');
    let iconNode = null;
    if (useCatalogDescription) {
      iconNode = (
        <span
          aria-hidden='true'
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            lineHeight: 1,
            flex: '0 0 auto',
          }}
        >
          {getLobeHubIcon(vendorIcon, 18)}
        </span>
      );
    } else if (modelSeries) {
      iconNode = <CanvasModelSeriesIcon series={modelSeries} size='small' />;
    }
    if (!iconNode) {
      return text;
    }
    return (
      <span
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 8,
          flexWrap: 'wrap',
        }}
      >
        {iconNode}
        <span>{text}</span>
      </span>
    );
  };

  const renderSidebarNavItem = ({
    key,
    label,
    icon,
    active = false,
    muted = false,
    primary = false,
    onClick,
    suffix = null,
    ariaExpanded,
  }) => {
    const hovered = !isMobile && hoveredSidebarNavKey === key;
    return (
      <button
        key={key}
        type='button'
        aria-expanded={ariaExpanded}
        className='canvas-sidebar-nav-item'
        data-canvas-sidebar-active={active ? 'true' : undefined}
        data-canvas-sidebar-primary={primary ? 'true' : undefined}
        style={{
          ...styles.sidebarNavItem,
          ...(primary ? styles.sidebarNavItemPrimary : null),
          ...(hovered && !active ? styles.sidebarNavItemHover : null),
          ...(active ? styles.sidebarNavItemActive : null),
          ...(muted ? styles.sidebarNavItemMuted : null),
        }}
        onClick={onClick}
        onMouseEnter={() => {
          if (!isMobile) {
            setHoveredSidebarNavKey(key);
          }
        }}
        onMouseLeave={() => {
          if (!isMobile) {
            setHoveredSidebarNavKey((current) =>
              current === key ? '' : current,
            );
          }
        }}
      >
        <span
          style={{
            ...styles.sidebarNavIcon,
            ...(primary ? styles.sidebarNavIconPrimary : null),
            ...(active ? styles.sidebarNavIconActive : null),
          }}
        >
          {icon}
        </span>
        <span style={styles.sidebarNavLabel}>{label}</span>
        {suffix ? <span style={styles.sidebarNavSuffix}>{suffix}</span> : null}
      </button>
    );
  };

  const renderSidebarRailButton = ({
    key,
    label,
    icon,
    active = false,
    onClick,
  }) => (
    <Tooltip key={key} content={label} position='right'>
      <Button
        type='tertiary'
        aria-label={label}
        className='canvas-sidebar-rail-button'
        data-canvas-sidebar-active={active ? 'true' : undefined}
        icon={icon}
        style={{
          ...styles.sidebarRailButton,
          ...(active ? styles.sidebarRailButtonActive : null),
        }}
        onClick={onClick}
      />
    </Tooltip>
  );

  const renderCollapsedSidebarRail = () => (
    <div style={styles.sidebarRail} data-canvas-sidebar-rail='true'>
      <div style={styles.sidebarRailSection}>
        {renderSidebarRailButton({
          key: 'show-sidebar',
          label: t('显示侧边栏'),
          icon: <IconSidebar />,
          onClick: () => setDesktopSidebarCollapsed(false),
        })}
        {renderSidebarRailButton({
          key: 'new-chat',
          label: t('新聊天'),
          icon: <IconPlus />,
          active:
            generationMode === CANVAS_MODE_CHAT && !selectedCanvasSessionId,
          onClick: handleNewBlankChat,
        })}
      </div>
      <div style={styles.sidebarRailDivider} />
      <div style={styles.sidebarRailSection}>
        {renderSidebarRailButton({
          key: 'asset-library',
          label: t('资产库'),
          icon: <IconArchive />,
          onClick: () => {
            setAssetLibraryVisible(true);
            setMobileTaskbarVisible(false);
          },
        })}
        {renderSidebarRailButton({
          key: 'projects',
          label: t('项目'),
          icon: <IconExternalOpen />,
          onClick: handleProjectEntryClick,
        })}
      </div>
    </div>
  );

  const renderCanvasSessionMenu = (session) => (
    <Dropdown.Menu style={styles.darkMenu}>
      <Dropdown.Item
        style={styles.darkMenuItem}
        onClick={(event) => {
          event?.domEvent?.stopPropagation?.();
          renameCanvasSession(session);
        }}
      >
        {t('重命名')}
      </Dropdown.Item>
      <Dropdown.Item
        style={styles.darkMenuItem}
        onClick={(event) => {
          event?.domEvent?.stopPropagation?.();
          toggleCanvasSessionPin(session);
        }}
      >
        {session.pinned ? t('取消置顶') : t('置顶')}
      </Dropdown.Item>
      {session.mode === CANVAS_MODE_CHAT ? (
        <Dropdown.Item
          style={styles.darkMenuItem}
          onClick={(event) => {
            event?.domEvent?.stopPropagation?.();
            openCanvasChatSessionSettings(session);
          }}
        >
          {t('会话设置')}
        </Dropdown.Item>
      ) : null}
      {session.mode === CANVAS_MODE_CHAT ? (
        <Dropdown.Item
          style={styles.darkMenuItem}
          onClick={(event) => {
            event?.domEvent?.stopPropagation?.();
            toggleCanvasSessionContext(session);
          }}
        >
          {Number(session.clear_context_message_id) > 0
            ? t('恢复上下文')
            : t('清空上下文')}
        </Dropdown.Item>
      ) : null}
      <Dropdown.Item
        style={{
          ...styles.darkMenuItem,
          color: 'var(--canvas-danger-chip-text)',
        }}
        onClick={(event) => {
          event?.domEvent?.stopPropagation?.();
          if (window.confirm(t('确认删除该会话？'))) {
            deleteCanvasSession(session);
          }
        }}
      >
        {t('删除')}
      </Dropdown.Item>
    </Dropdown.Menu>
  );

  const renderCanvasSessionIcon = (mode) => {
    if (mode === CANVAS_MODE_CHAT) {
      return <IconCommentStroked size='small' />;
    }
    if (mode === CANVAS_MODE_VIDEO) {
      return <IconVideo size='small' />;
    }
    return <IconImage size='small' />;
  };

  const toggleRecentCanvasSessionsExpanded = () => {
    setRecentCanvasSessions((prev) => ({
      ...prev,
      expanded: !prev.expanded,
    }));
  };

  const handleRecentCanvasSessionsScroll = (event) => {
    if (
      recentCanvasSessions.initialLoading ||
      recentCanvasSessions.loadingMore ||
      !recentCanvasSessions.hasMore
    ) {
      return;
    }
    const container = event.currentTarget;
    const remainingDistance =
      container.scrollHeight - container.scrollTop - container.clientHeight;
    if (remainingDistance <= 48) {
      loadRecentCanvasSessions();
    }
  };

  const handleCanvasWorkspaceScroll = (event) => {
    const container = event.currentTarget;
    syncCanvasAutoFollowState(container);
    const selectedSessionIdentifier = getCanvasSessionIdentifier(
      selectedCanvasSession,
    );
    if (
      generationMode !== CANVAS_MODE_CHAT ||
      !selectedSessionIdentifier ||
      canvasMessagesLoadingMore ||
      !canvasMessagesHasMore
    ) {
      return;
    }
    if (container.scrollTop <= CANVAS_HISTORY_AUTOLOAD_TOP_THRESHOLD) {
      void loadOlderCanvasMessages(selectedSessionIdentifier);
    }
  };

  const renderCanvasSessionList = () => (
    <Spin
      spinning={recentCanvasSessions.initialLoading || deletingCanvasSession}
    >
      <div
        style={{ ...styles.taskList, ...styles.scrollPanel }}
        className='canvas-scroll-panel canvas-sidebar-session-list'
        onScroll={handleRecentCanvasSessionsScroll}
      >
        {recentCanvasSessions.error &&
        recentCanvasSessions.items.length === 0 ? (
          renderSidebarEmpty(
            t('会话加载失败'),
            recentCanvasSessions.error,
            <Button
              size='small'
              icon={<IconRefresh />}
              onClick={() => loadRecentCanvasSessions({ reset: true })}
            >
              {t('重试')}
            </Button>,
          )
        ) : recentCanvasSessions.items.length === 0 ? (
          <div style={{ minHeight: 28 }} />
        ) : (
          <>
            {recentCanvasSessions.items.map((session) => {
              const sessionMode = CANVAS_MODES.includes(session.mode)
                ? session.mode
                : CANVAS_MODE_IMAGE;
              const sessionIdentifier = getCanvasSessionIdentifier(session);
              const active =
                generationMode === sessionMode &&
                sessionIdentifier === selectedCanvasSessionIds[sessionMode];
              const hovered =
                !isMobile && hoveredCanvasSessionId === sessionIdentifier;
              return (
                <div
                  key={sessionIdentifier || session.id}
                  role='button'
                  tabIndex={0}
                  className='canvas-sidebar-session-item'
                  data-canvas-sidebar-active={active ? 'true' : undefined}
                  style={{
                    ...styles.taskListItem,
                    ...styles.sessionListItem,
                    ...(hovered && !active ? styles.taskListItemHover : null),
                    ...(active ? styles.taskListItemActive : null),
                  }}
                  onClick={() => selectCanvasSession(session)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault();
                      selectCanvasSession(session);
                    }
                  }}
                  onMouseEnter={() => {
                    if (!isMobile) {
                      setHoveredCanvasSessionId(sessionIdentifier);
                    }
                  }}
                  onMouseLeave={() => {
                    if (!isMobile) {
                      setHoveredCanvasSessionId((current) =>
                        current === sessionIdentifier ? null : current,
                      );
                    }
                  }}
                >
                  <span
                    style={{
                      ...styles.sessionTypeIcon,
                      ...(active ? styles.sessionTypeIconActive : null),
                    }}
                  >
                    {renderCanvasSessionIcon(sessionMode)}
                  </span>
                  <div style={styles.taskListText}>
                    <div style={styles.sessionListTitle}>
                      {summarizeText(session.title, t('新会话'))}
                    </div>
                  </div>
                  <Dropdown
                    trigger='click'
                    position='bottomRight'
                    render={renderCanvasSessionMenu(session)}
                  >
                    <Button
                      size='small'
                      type='tertiary'
                      aria-label={t('会话菜单')}
                      className='canvas-sidebar-session-menu'
                      data-canvas-session-menu-active={
                        active ? 'true' : undefined
                      }
                      icon={<IconMore />}
                      style={styles.sessionMenuButton}
                      onClick={(event) => event.stopPropagation()}
                    />
                  </Dropdown>
                </div>
              );
            })}
            {recentCanvasSessions.loadingMore ? (
              <div style={styles.recentListFooter}>
                <Spin size='small' spinning />
              </div>
            ) : null}
            {recentCanvasSessions.error ? (
              <div style={styles.recentListFooter}>
                <Button
                  size='small'
                  type='tertiary'
                  icon={<IconRefresh />}
                  onClick={() => refreshRecentCanvasSessions({ silent: false })}
                >
                  {t('重试')}
                </Button>
              </div>
            ) : null}
          </>
        )}
      </div>
    </Spin>
  );

  const renderTaskSidebar = ({ collapsed = false } = {}) => (
    <div
      style={{
        ...styles.leftPanel,
        ...(collapsed ? styles.leftPanelCollapsed : null),
      }}
      data-canvas-task-sidebar={generationMode}
      data-canvas-sidebar-collapsed={collapsed ? 'true' : undefined}
    >
      {collapsed ? (
        renderCollapsedSidebarRail()
      ) : (
        <>
          <div style={styles.sidebarHeader}>
            <Button
              type='tertiary'
              icon={<IconArchive />}
              style={styles.sidebarHeaderActionButton}
              onClick={() => {
                setAssetLibraryVisible(true);
                setMobileTaskbarVisible(false);
              }}
            >
              {t('资产库')}
            </Button>
            {!isMobile ? (
              <Tooltip content={t('隐藏侧边栏')} position='bottom'>
                <Button
                  type='tertiary'
                  aria-label={t('隐藏侧边栏')}
                  data-canvas-sidebar-collapse='true'
                  icon={<IconChevronLeft />}
                  style={styles.sidebarIconButton}
                  onClick={() => setDesktopSidebarCollapsed(true)}
                />
              </Tooltip>
            ) : null}
          </div>
          <div style={styles.sidebarNav}>
            {renderSidebarNavItem({
              key: 'projects',
              label: t('项目'),
              icon: <IconExternalOpen />,
              muted: true,
              onClick: handleProjectEntryClick,
            })}
            {renderSidebarNavItem({
              key: 'new-chat',
              label: t('新聊天'),
              icon: <IconPlus />,
              active:
                generationMode === CANVAS_MODE_CHAT && !selectedCanvasSessionId,
              primary: true,
              onClick: handleNewBlankChat,
            })}
            {renderSidebarNavItem({
              key: 'recent-sessions',
              label: t('最近'),
              icon: <IconClock />,
              onClick: toggleRecentCanvasSessionsExpanded,
              ariaExpanded: recentCanvasSessions.expanded,
              suffix: (
                <IconChevronDown
                  size='small'
                  style={{
                    transform: recentCanvasSessions.expanded
                      ? 'rotate(0deg)'
                      : 'rotate(-90deg)',
                  }}
                />
              ),
            })}
          </div>

          {recentCanvasSessions.expanded ? renderCanvasSessionList() : null}
        </>
      )}
    </div>
  );

  const renderWeakDetails = (title, children) => (
    <details
      style={{
        borderTop: '1px solid var(--canvas-border)',
        paddingTop: 10,
        marginTop: 2,
      }}
    >
      <summary
        style={{
          cursor: 'pointer',
          color: 'var(--canvas-text-muted)',
          fontSize: 12,
          lineHeight: 1.5,
        }}
      >
        {title}
      </summary>
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          gap: 8,
          marginTop: 10,
        }}
      >
        {children}
      </div>
    </details>
  );

  const renderRecentTaskGrid = () => {
    if (generationMode === CANVAS_MODE_VIDEO) {
      if (videoTasks.length === 0) {
        return null;
      }
      return (
        <div
          style={{
            ...styles.tasksGrid,
            gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
            padding: 0,
          }}
        >
          {videoTasks.map((task) => (
            <VideoGenerationTaskCard
              key={task.id}
              task={task}
              selected={videoSelectedTaskIds.has(task.id)}
              onSelectChange={handleVideoTaskSelect}
              onClick={() => handleVideoTaskCardClick(task)}
            />
          ))}
        </div>
      );
    }
    if (tasks.length === 0) {
      return null;
    }
    return (
      <div
        style={{
          ...styles.tasksGrid,
          gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))',
          padding: 0,
        }}
      >
        {tasks.map((task) => (
          <ImageGenerationTaskCard
            key={task.id}
            task={task}
            selected={selectedTaskIds.has(task.id)}
            onSelectChange={handleTaskSelect}
            onClick={() => handleTaskCardClick(task)}
          />
        ))}
      </div>
    );
  };

  const renderChatWorkspace = () => (
    <div style={styles.chatStream} className='canvas-chat-stream'>
      {renderCanvasMessageStream()}
    </div>
  );

  const renderCanvasReferenceThumb = (file, fallbackLabel) => (
    <div
      key={file.uid}
      data-canvas-reference-thumb='true'
      style={styles.canvasReferenceThumbWrap}
    >
      <img src={file.url} alt='' style={styles.canvasReferenceThumb} />
      <Text
        type='tertiary'
        size='small'
        style={{ display: 'block', marginTop: 4 }}
      >
        {fallbackLabel}
      </Text>
    </div>
  );

  const openCanvasImagePreview = async (message, media) => {
    const initialSrc = media?.previewSrc || media?.src || '';
    const task = message?.image_task || null;
    const taskId = task?.id;
    const messageId = message?.id;
    const initialPreview = buildCanvasImagePreviewState({
      src: initialSrc,
      downloadSrc: task?.image_url || media?.src || initialSrc,
      taskId,
      messageId,
    });
    if (initialPreview) {
      setSelectedCanvasImagePreview(initialPreview);
    }

    if (!task?.id || task?.image_url) {
      return;
    }

    const previewRequestSeq = canvasImagePreviewRequestSeqRef.current + 1;
    canvasImagePreviewRequestSeqRef.current = previewRequestSeq;
    const detail = await loadCanvasMessageTaskDetail(message);
    const detailSrc = detail?.image_url || detail?.thumbnail_url || '';
    if (!detailSrc || detailSrc === initialSrc) {
      setSelectedCanvasImagePreview((prev) => {
        if (
          !prev ||
          prev.messageId !== String(messageId || '') ||
          prev.taskId !== String(taskId || '') ||
          previewRequestSeq !== canvasImagePreviewRequestSeqRef.current
        ) {
          return prev;
        }
        const nextDownloadSrc =
          detail?.image_url || prev.downloadSrc || prev.src;
        if (nextDownloadSrc === prev.downloadSrc) {
          return prev;
        }
        return {
          ...prev,
          downloadSrc: nextDownloadSrc,
        };
      });
      return;
    }

    setCanvasMessages((prev) =>
      updateCanvasMessageById(
        prev,
        message.id,
        (item) => mergeCanvasMessageTaskDetail(item, detail),
        canvasMessageIndexesRef.current,
      ),
    );
    setSelectedCanvasImagePreview((prev) => {
      if (
        !prev ||
        prev.messageId !== String(messageId || '') ||
        prev.taskId !== String(taskId || '') ||
        previewRequestSeq !== canvasImagePreviewRequestSeqRef.current
      ) {
        return prev;
      }
      const nextPreview = buildCanvasImagePreviewState({
        src: detailSrc,
        downloadSrc: detail?.image_url || detailSrc,
        taskId,
        messageId,
      });
      return nextPreview || prev;
    });
  };

  const getCanvasBatchAspectRatio = (batch) => {
    const userRatio = getCanvasMessageExplicitAspectRatio(batch?.userMessage);
    if (userRatio) {
      return userRatio;
    }
    for (const message of batch?.assistantMessages || []) {
      const ratio = getCanvasMessageExplicitAspectRatio(message);
      if (ratio) {
        return ratio;
      }
    }
    return '1 / 1';
  };

  const getCanvasChatMessageDisplay = (message) =>
    extractCanvasChatReasoning(message);

  const getCanvasChatMessageText = (message) =>
    getCanvasChatMessageDisplay(message).content;

  const toggleCanvasReasoningExpansion = (messageId) => {
    if (!messageId) {
      return;
    }
    setCanvasChatReasoningUiStateByMessageId((prev) => {
      const currentState =
        prev[messageId] || createCanvasChatReasoningUiState();
      return {
        ...prev,
        [messageId]: {
          ...currentState,
          isExpanded: !currentState.isExpanded,
        },
      };
    });
  };

  const getCanvasChatRetryPrompt = (message) => {
    return canvasChatRetryPromptByMessageId[String(message?.id || '')] || '';
  };

  const getCanvasChatCopyTooltipText = (message) =>
    canvasChatCopiedMessageId === message?.id
      ? t('已复制')
      : message?.role === 'user'
        ? t('复制问题')
        : t('复制回答');

  const getCanvasChatReasoningToggleText = (uiState) => {
    if (uiState?.isReasoningStreaming) {
      return t('思考过程');
    }
    const durationMs = getCanvasChatReasoningDurationMs(uiState);
    if (durationMs <= 0) {
      return t('思考过程');
    }
    if (durationMs < 2000) {
      return t('思考了几秒');
    }
    return t('思考了 {{count}} 秒', {
      count: Math.max(1, Math.round(durationMs / 1000)),
    });
  };

  const resetCanvasChatCopiedTooltip = () => {
    if (canvasChatCopiedMessageTimerRef.current) {
      window.clearTimeout(canvasChatCopiedMessageTimerRef.current);
      canvasChatCopiedMessageTimerRef.current = null;
    }
    setActiveCanvasChatActionTooltipKey('');
    setCanvasChatCopiedMessageId(null);
  };

  const handleCopyCanvasChatMessage = async (event, message) => {
    event?.stopPropagation?.();
    const text = getCanvasChatMessageText(message);
    if (!text) {
      return;
    }
    const ok = await copy(text);
    if (ok) {
      resetCanvasChatCopiedTooltip();
      setCanvasChatCopiedMessageId(message?.id ?? null);
      setActiveCanvasChatActionTooltipKey(`copy-${message?.id ?? ''}`);
      canvasChatCopiedMessageTimerRef.current = window.setTimeout(() => {
        setCanvasChatCopiedMessageId((current) =>
          current === (message?.id ?? null) ? null : current,
        );
        setActiveCanvasChatActionTooltipKey((current) =>
          current === `copy-${message?.id ?? ''}` ? '' : current,
        );
        canvasChatCopiedMessageTimerRef.current = null;
      }, 1200);
      showSuccess(t('已复制到剪贴板'));
    } else {
      showError(t('复制失败'));
    }
  };

  const handleRetryCanvasChatMessage = async (event, message) => {
    event?.stopPropagation?.();
    if (chatStreaming) {
      return;
    }
    const prompt = getCanvasChatRetryPrompt(message);
    if (!prompt) {
      showError(t('未找到可重发的提问'));
      return;
    }
    await handleSendChatMessage(prompt);
  };

  const handleReuseCanvasChatPrompt = (event, message) => {
    event?.stopPropagation?.();
    const prompt = String(message?.prompt || '').trim();
    if (!prompt) {
      showInfo(t('该问题暂无可编辑内容'));
      return;
    }
    setChatPrompt(prompt);
    scrollCanvasViewportToBottom();
    focusCanvasChatComposerInput();
  };

  const handleResumeCanvasAutoFollow = () => {
    scrollCanvasViewportToBottom({
      behavior: 'smooth',
      enableAutoFollow: true,
    });
  };

  const renderCanvasChatActionButton = ({
    actionKey,
    tooltip,
    ariaLabel,
    message,
    onClick,
    icon,
    disabled = false,
  }) => {
    const tooltipKey = `${actionKey}-${message?.id ?? ''}`;
    const isCopyAction = actionKey === 'copy';
    const tooltipVisible =
      activeCanvasChatActionTooltipKey === tooltipKey ||
      (isCopyAction && canvasChatCopiedMessageId === message?.id);
    return (
      <Tooltip
        content={tooltip}
        position='top'
        trigger='custom'
        visible={tooltipVisible}
      >
        <button
          type='button'
          aria-label={ariaLabel}
          title={ariaLabel}
          className='canvas-chat-action-button'
          data-canvas-chat-action-kind={actionKey}
          data-canvas-chat-action-active={
            isCopyAction && canvasChatCopiedMessageId === message?.id
              ? 'true'
              : 'false'
          }
          style={{
            ...styles.canvasChatActionButton,
            ...(disabled ? styles.canvasChatActionButtonDisabled : null),
          }}
          onMouseEnter={() => {
            if (!isCopyAction) {
              resetCanvasChatCopiedTooltip();
            }
            setActiveCanvasChatActionTooltipKey(tooltipKey);
          }}
          onFocus={() => {
            if (!isCopyAction) {
              resetCanvasChatCopiedTooltip();
            }
            setActiveCanvasChatActionTooltipKey(tooltipKey);
          }}
          onMouseLeave={() => {
            setActiveCanvasChatActionTooltipKey((current) =>
              current === tooltipKey ? '' : current,
            );
          }}
          onBlur={() => {
            setActiveCanvasChatActionTooltipKey((current) =>
              current === tooltipKey ? '' : current,
            );
          }}
          onClick={(event) => onClick(event, message)}
          disabled={disabled}
        >
          {icon}
        </button>
      </Tooltip>
    );
  };

  const renderCanvasChatStatus = (statusInfo) => {
    if (!statusInfo) {
      return null;
    }
    const toneClassName =
      statusInfo.tone === 'error'
        ? 'canvas-chat-text-status canvas-chat-text-status--error'
        : statusInfo.tone === 'neutral'
          ? 'canvas-chat-text-status canvas-chat-text-status--neutral'
          : 'canvas-chat-text-status';
    return (
      <div
        style={{
          ...styles.canvasChatTextStatus,
          ...(statusInfo.tone === 'error'
            ? styles.canvasChatTextStatusError
            : null),
          ...(statusInfo.tone === 'neutral'
            ? styles.canvasChatTextStatusNeutral
            : null),
          ...(statusInfo.tone === 'error' ? styles.canvasChatErrorText : null),
        }}
        className={toneClassName}
        role='status'
        aria-live='polite'
      >
        {statusInfo.pending ? <Spin size='small' /> : null}
        <span>{statusInfo.label}</span>
        {statusInfo.detail ? (
          <span
            style={styles.canvasChatStatusDetail}
          >{`· ${statusInfo.detail}`}</span>
        ) : null}
      </div>
    );
  };

  const renderCanvasChatContextDivider = ({
    key,
    hasFollowingMessages = true,
  } = {}) => (
    <div
      key={key}
      style={styles.chatContextNotice}
      className='canvas-chat-context-divider'
      data-canvas-chat-context-divider='true'
    >
      <div style={styles.chatContextNoticeText}>
        <Text size='small' style={{ color: 'inherit' }}>
          {hasFollowingMessages
            ? t('以下消息从新话题继续，以上内容仅作展示，不再参与后续上下文')
            : t('新的话题将从这里开始，以上内容仅作展示，不再参与后续上下文')}
        </Text>
      </div>
      <Button
        size='small'
        type='tertiary'
        onClick={() => toggleCanvasSessionContext(selectedCanvasSession)}
      >
        {t('恢复上下文')}
      </Button>
    </div>
  );

  const handleCopyCanvasError = async (event, errorText) => {
    event?.stopPropagation?.();
    const text = String(errorText || '').trim();
    if (!text) {
      return;
    }
    const ok = await copy(text);
    if (ok) {
      showSuccess(t('已复制到剪贴板'));
    } else {
      showError(t('复制失败'));
    }
  };

  const renderCanvasErrorBlock = (
    errorText,
    { overlay = false, size = 'small' } = {},
  ) => {
    const text = String(errorText || '').trim() || t('生成失败');
    return (
      <div style={styles.canvasErrorBlock}>
        <Text
          type={overlay ? undefined : 'danger'}
          size={size}
          style={
            overlay
              ? styles.canvasMediaStatusOverlayText
              : styles.canvasErrorText
          }
        >
          {text}
        </Text>
        <button
          type='button'
          aria-label={t('复制')}
          title={t('复制')}
          style={{
            ...styles.canvasErrorCopyButton,
            ...(overlay ? styles.canvasErrorCopyButtonOverlay : null),
          }}
          onClick={(event) => handleCopyCanvasError(event, text)}
        >
          <IconCopy size='small' />
          <span>{t('复制')}</span>
        </button>
      </div>
    );
  };

  const renderCanvasMediaCard = (
    message,
    { batchLayout = false, batchAspectRatio = '' } = {},
  ) => {
    const media = getCanvasMessageMedia(message);
    const taskType = getCanvasMessageTaskType(message);
    const isVideo = taskType === 'video_generation';
    const aspectRatio = getCanvasMessageAspectRatio(message);
    const task = message?.image_task || message?.video_task || null;
    const status = media?.status || task?.status || message?.status || '';
    const isDone =
      (isVideo && (status === 'completed' || status === 'success')) ||
      (!isVideo && status === 'success');
    const isFailed = status === 'failed';
    const normalizedStatus = String(status || '').toLowerCase();
    const isQueued = ['queued', 'pending', 'submitted'].includes(
      normalizedStatus,
    );
    const referencePreviewSrc = getCanvasMessageReferencePreviewSrc(message);
    const showReferencePreview = Boolean(referencePreviewSrc) && !isDone;
    const canPreviewImage = isDone && media?.kind === 'image' && media.src;
    const errorText = media?.error || message.error_message || t('生成失败');
    const statusLabel = isFailed
      ? errorText
      : isQueued
        ? t('排队中')
        : isVideo
          ? t('正在生成视频')
          : t('正在生成图片');

    let content = (
      <div style={styles.canvasMediaStatusBody}>
        <Spin size='small' />
        <Text type='tertiary' size='small'>
          {isVideo ? t('正在生成视频') : t('正在生成图片')}
        </Text>
      </div>
    );

    if (showReferencePreview) {
      content = (
        <>
          <img
            data-canvas-message-reference-preview='true'
            src={referencePreviewSrc}
            alt=''
            style={styles.canvasMediaContent}
          />
          <div
            style={{
              ...styles.canvasMediaStatusOverlay,
              ...(isFailed ? styles.canvasMediaStatusOverlayInteractive : null),
              ...(isFailed ? styles.canvasMediaStatusOverlayError : null),
            }}
          >
            <div style={styles.canvasMediaStatusBodyOverlay}>
              {isQueued || isFailed ? (
                <IconClock size='small' />
              ) : (
                <Spin size='small' />
              )}
              {isFailed ? (
                renderCanvasErrorBlock(errorText, {
                  overlay: true,
                  size: 'small',
                })
              ) : (
                <Text size='small' style={styles.canvasMediaStatusOverlayText}>
                  {statusLabel}
                </Text>
              )}
            </div>
          </div>
        </>
      );
    } else if (isFailed) {
      content = (
        <div style={styles.canvasMediaStatusBody}>
          <IconClock size='small' />
          {renderCanvasErrorBlock(errorText, { size: 'small' })}
        </div>
      );
    } else if (isDone && media?.kind === 'image' && media.src) {
      content = (
        <img
          data-canvas-message-result='image'
          src={media.src}
          alt=''
          style={styles.canvasMediaContent}
        />
      );
    } else if (isDone && media?.kind === 'video') {
      content = (
        <PlayableVideo
          task={message.video_task || null}
          poster={media.src}
          style={styles.canvasMediaVideoContent}
          statusStyle={styles.canvasMediaStatusBody}
          statusTextStyle={styles.canvasErrorText}
          onClick={() => setVideoSelectedTask(message.video_task || null)}
          videoProps={{ 'data-canvas-message-result': 'video' }}
        />
      );
    }

    return (
      <div
        style={{
          ...styles.canvasMediaCard,
          ...(canPreviewImage ? styles.canvasMediaCardClickable : null),
          ...getCanvasMediaCardSize(aspectRatio, {
            batchLayout,
            batchAspectRatio,
            mediaKind: media?.kind || '',
          }),
        }}
        onClick={
          canPreviewImage
            ? (event) => {
                event.stopPropagation();
                openCanvasImagePreview(message, media);
              }
            : undefined
        }
      >
        <div style={styles.canvasMediaFrame}>{content}</div>
      </div>
    );
  };

  const renderCanvasGenerationCard = (
    message,
    { batchLayout = false, batchAspectRatio = '' } = {},
  ) => {
    return renderCanvasMediaCard(message, { batchLayout, batchAspectRatio });
  };

  const renderCanvasGenerationBatch = (batch) => {
    const userMessage = batch.userMessage || {};
    const batchAspectRatio = getCanvasBatchAspectRatio(batch);
    const showMessageReferences = generationMode !== CANVAS_MODE_IMAGE;
    const referenceMap = new Map();
    [userMessage, ...(batch.assistantMessages || [])].forEach((message) => {
      getCanvasMessageReferenceFiles(message).forEach((file) => {
        const key = file.url || file.uid || file.name;
        if (key && !referenceMap.has(key)) {
          referenceMap.set(key, file);
        }
      });
    });
    const references = Array.from(referenceMap.values());
    return (
      <div
        key={batch.id}
        data-canvas-message-batch='image_generation'
        style={styles.canvasGenerationBatchRow}
      >
        <div
          data-canvas-message-row='true'
          style={{
            ...styles.canvasMessageRow,
            ...styles.canvasMessageRowUser,
          }}
        >
          <div style={styles.canvasMessageBody}>
            <div style={styles.canvasMessagePrompt}>
              {userMessage.prompt || t('已添加素材引用')}
            </div>
            {showMessageReferences && references.length > 0 ? (
              <div style={styles.canvasMessageRefs}>
                {references.map((file) =>
                  renderCanvasReferenceThumb(file, t('参考图')),
                )}
              </div>
            ) : null}
          </div>
        </div>
        <div style={styles.canvasGenerationBatchGrid}>
          {(batch.assistantMessages || []).map((message) => (
            <div
              key={message.id || message.task_id || message.image_task?.id}
              data-canvas-generation-batch-card='true'
              style={styles.canvasGenerationBatchCard}
              onClick={async () => {
                setSelectedCanvasMessageId(message.id);
                const detail = await loadCanvasMessageTaskDetail(message);
                if (!detail) {
                  return;
                }
                setCanvasMessages((prev) =>
                  updateCanvasMessageById(
                    prev,
                    message.id,
                    (item) => mergeCanvasMessageTaskDetail(item, detail),
                    canvasMessageIndexesRef.current,
                  ),
                );
              }}
            >
              {renderCanvasGenerationCard(message, {
                batchLayout: true,
                batchAspectRatio,
              })}
            </div>
          ))}
        </div>
      </div>
    );
  };

  const renderCanvasMessageResult = (message) => {
    const media = getCanvasMessageMedia(message);
    if (!media) {
      return null;
    }
    if (media.kind === 'image') {
      if (media.status === 'success' && media.src) {
        const aspectRatio = getCanvasMessageAspectRatio(message);
        return (
          <div style={styles.canvasMessageResultFrame}>
            <div
              style={{
                ...styles.canvasMediaCard,
                ...styles.canvasMediaCardClickable,
                ...getCanvasMediaCardSize(aspectRatio, {
                  mediaKind: 'image',
                }),
              }}
              onClick={(event) => {
                event.stopPropagation();
                openCanvasImagePreview(message, media);
              }}
            >
              <div style={styles.canvasMediaFrame}>
                <img
                  data-canvas-message-result='image'
                  src={media.src}
                  alt=''
                  style={styles.messageResultImage}
                />
              </div>
            </div>
          </div>
        );
      }
      if (media.status === 'failed') {
        return (
          <div
            style={{
              ...styles.canvasChatInlineStatus,
              ...styles.canvasChatInlineStatusError,
              ...styles.canvasChatErrorText,
            }}
            className='canvas-chat-inline-status canvas-chat-inline-status--error'
          >
            <span>{media.error || t('生成失败')}</span>
          </div>
        );
      }
      return (
        <div
          style={styles.messagePending}
          className='canvas-message-pending canvas-chat-inline-status--neutral'
        >
          {media.status === 'generating' ? (
            <Spin size='small' />
          ) : (
            <IconClock />
          )}
          <span>
            {media.status === 'generating' ? t('生成中') : t('排队中')}
          </span>
        </div>
      );
    }
    if (media.kind === 'video') {
      if (media.status === 'completed') {
        return (
          <div style={styles.canvasMessageResultFrame}>
            <PlayableVideo
              task={message.video_task || null}
              poster={media.src}
              style={styles.messageResultVideo}
              statusTextStyle={styles.canvasErrorText}
              onClick={() => setVideoSelectedTask(message.video_task || null)}
              videoProps={{ 'data-canvas-message-result': 'video' }}
            />
          </div>
        );
      }
      if (media.status === 'failed') {
        return (
          <div
            style={{
              ...styles.canvasChatInlineStatus,
              ...styles.canvasChatInlineStatusError,
              ...styles.canvasChatErrorText,
            }}
            className='canvas-chat-inline-status canvas-chat-inline-status--error'
          >
            <span>{media.error || t('生成失败')}</span>
          </div>
        );
      }
      return (
        <div
          style={styles.messagePending}
          className='canvas-message-pending canvas-chat-inline-status--neutral'
        >
          {media.status === 'in_progress' ? (
            <Spin size='small' />
          ) : (
            <IconPlayCircle />
          )}
          <span>
            {media.status === 'in_progress' ? t('生成中') : t('排队中')}
          </span>
        </div>
      );
    }
    return null;
  };

  const renderCanvasMessage = (message) => {
    if (message?.render_type === CANVAS_RENDERABLE_IMAGE_BATCH) {
      return renderCanvasGenerationBatch(message);
    }

    const isUser = message.role === 'user';
    const references = getCanvasMessageReferenceFiles(message);
    const showMessageReferences = generationMode !== CANVAS_MODE_IMAGE;
    const isSelected = selectedCanvasMessageId === message.id;
    const isChatMode = generationMode === CANVAS_MODE_CHAT;
    const chatAttachments = isChatMode
      ? extractCanvasChatAttachments(message)
      : [];
    const handleMessageClick = async () => {
      setSelectedCanvasMessageId(message.id);
      if (isUser || references.length > 0 || isChatMode) {
        return;
      }
      const detail = await loadCanvasMessageTaskDetail(message);
      if (!detail) {
        return;
      }
      setCanvasMessages((prev) =>
        updateCanvasMessageById(
          prev,
          message.id,
          (item) => mergeCanvasMessageTaskDetail(item, detail),
          canvasMessageIndexesRef.current,
        ),
      );
    };

    if (isChatMode) {
      const chatStatus = String(message?.status || '')
        .trim()
        .toLowerCase();
      const assistantDisplay = getCanvasChatMessageDisplay(message);
      const assistantText = assistantDisplay.content;
      const assistantReasoning = assistantDisplay.reasoningContent;
      const hasAssistantReasoning = assistantDisplay.hasReasoning;
      const isLiveReasoningMessage =
        chatStreaming &&
        String(chatStreamingSessionIdRef.current || '') ===
          String(canvasMessagesSessionId || '') &&
        String(chatStreamingMessageIdRef.current || '') === String(message.id);
      const reasoningUiState =
        canvasChatReasoningUiStateByMessageId[String(message.id)] ||
        (hasAssistantReasoning
          ? syncCanvasChatReasoningUiState({
              message,
              previousState: null,
              hasReasoning: true,
              isLiveMessage: isLiveReasoningMessage,
              now: Date.now(),
            })
          : null);
      const reasoningExpanded = !!reasoningUiState?.isExpanded;
      const reasoningTriggerText = hasAssistantReasoning
        ? getCanvasChatReasoningToggleText(reasoningUiState)
        : '';
      const hideAssistantTextWhileReasoning = shouldHideCanvasChatAssistantText(
        {
          hasReasoning: hasAssistantReasoning,
          reasoningUiState,
        },
      );
      const displayAssistantText = hideAssistantTextWhileReasoning
        ? ''
        : assistantText;
      const retryPrompt = isUser ? '' : getCanvasChatRetryPrompt(message);
      const actionVisible =
        hoveredCanvasChatMessageId === message.id || isSelected;
      const assistantStatusInfo = !isUser
        ? chatStatus === 'failed'
          ? {
              tone: 'error',
              label: t('失败'),
              detail: String(message.error_message || '').trim(),
              pending: false,
            }
          : chatStatus === 'stopped'
            ? {
                tone: 'neutral',
                label: t('已停止'),
                detail: '',
                pending: false,
              }
            : chatStatus === 'generating'
              ? {
                  tone: 'default',
                  label:
                    hasAssistantReasoning &&
                    reasoningUiState?.isReasoningStreaming &&
                    !displayAssistantText
                      ? t('思考中')
                      : t('回答中'),
                  detail: '',
                  pending: true,
                }
              : null
        : null;
      return (
        <div
          key={message.id}
          data-canvas-message-row='true'
          className={
            isUser
              ? 'canvas-chat-message-row canvas-chat-message-row--user'
              : 'canvas-chat-message-row canvas-chat-message-row--assistant'
          }
          style={{
            ...styles.canvasChatMessageRow,
            ...(isUser
              ? styles.canvasChatMessageRowUser
              : styles.canvasChatMessageRowAssistant),
          }}
          onClick={handleMessageClick}
          onMouseEnter={() => {
            if (!isMobile) {
              setHoveredCanvasChatMessageId(message.id);
            }
          }}
          onMouseLeave={() => {
            if (!isMobile) {
              setHoveredCanvasChatMessageId((current) =>
                current === message.id ? null : current,
              );
            }
          }}
        >
          {isUser ? (
            <>
              <div
                className='canvas-chat-user-bubble'
                style={{
                  ...styles.canvasChatUserBubble,
                  ...(isSelected ? styles.canvasChatUserBubbleSelected : null),
                }}
              >
                <div style={styles.canvasChatUserPrompt}>
                  {message.prompt || t('请输入消息')}
                </div>
                {chatAttachments.length > 0 ? (
                  <div style={styles.canvasChatAttachmentList}>
                    {chatAttachments.map((attachment) =>
                      renderCanvasChatAttachment(attachment),
                    )}
                  </div>
                ) : null}
                {showMessageReferences && references.length > 0 ? (
                  <div style={styles.canvasMessageRefs}>
                    {references.map((file) =>
                      renderCanvasReferenceThumb(file, t('参考图')),
                    )}
                  </div>
                ) : null}
              </div>
              <div
                style={{
                  ...styles.canvasChatMetaRow,
                  ...styles.canvasChatMetaRowUser,
                }}
                className='canvas-chat-message-meta-row canvas-chat-message-meta-row--user'
              >
                <div
                  style={{
                    ...styles.canvasChatActions,
                    ...(actionVisible ? styles.canvasChatActionsVisible : null),
                  }}
                  className='canvas-chat-actions'
                >
                  {renderCanvasChatActionButton({
                    actionKey: 'copy',
                    tooltip: getCanvasChatCopyTooltipText(message),
                    ariaLabel: t('复制问题'),
                    message,
                    onClick: handleCopyCanvasChatMessage,
                    icon: <IconCopy size='small' />,
                  })}
                  {renderCanvasChatActionButton({
                    actionKey: 'reuse',
                    tooltip: t('放回输入框编辑'),
                    ariaLabel: t('放回输入框编辑后发送'),
                    message,
                    onClick: handleReuseCanvasChatPrompt,
                    icon: <IconEdit size='small' />,
                  })}
                </div>
              </div>
            </>
          ) : (
            <div
              className='canvas-chat-assistant-card'
              style={{
                ...styles.canvasChatAssistantContent,
                ...(isSelected
                  ? styles.canvasChatAssistantContentSelected
                  : null),
              }}
            >
              {hasAssistantReasoning ? (
                <div
                  style={styles.canvasChatReasoningWrap}
                  className='canvas-chat-reasoning-card'
                >
                  <button
                    type='button'
                    aria-label={reasoningTriggerText}
                    style={styles.canvasChatReasoningToggle}
                    onClick={(event) => {
                      event.stopPropagation();
                      toggleCanvasReasoningExpansion(message.id);
                    }}
                  >
                    <span style={styles.canvasChatReasoningToggleLead}>
                      <span style={styles.canvasChatReasoningIcon}>
                        <Brain size={14} strokeWidth={2} />
                      </span>
                      <span style={styles.canvasChatReasoningToggleText}>
                        {reasoningTriggerText}
                      </span>
                    </span>
                    <IconChevronDown
                      style={{
                        ...styles.canvasChatReasoningToggleArrow,
                        transform: reasoningExpanded
                          ? 'rotate(0deg)'
                          : 'rotate(-90deg)',
                        transition: 'transform 0.16s ease',
                      }}
                    />
                  </button>
                  {reasoningExpanded ? (
                    <div style={styles.canvasChatReasoningPanel}>
                      <MarkdownRenderer
                        content={assistantReasoning}
                        className='canvas-chat-markdown canvas-chat-markdown--reasoning'
                        style={styles.canvasChatReasoningMarkdown}
                      />
                    </div>
                  ) : null}
                </div>
              ) : null}
              {displayAssistantText ? (
                <MarkdownRenderer
                  content={displayAssistantText}
                  className='canvas-chat-markdown canvas-chat-markdown--assistant'
                  style={styles.canvasChatMarkdown}
                />
              ) : null}
              {showMessageReferences && references.length > 0 ? (
                <div style={styles.canvasMessageRefs}>
                  {references.map((file) =>
                    renderCanvasReferenceThumb(file, t('参考图')),
                  )}
                </div>
              ) : null}
              <div
                style={styles.canvasChatMetaRow}
                className='canvas-chat-message-meta-row canvas-chat-message-meta-row--assistant'
              >
                <div
                  style={{
                    ...styles.canvasChatStatusSlot,
                    ...(assistantStatusInfo
                      ? null
                      : styles.canvasChatStatusSlotEmpty),
                  }}
                >
                  {renderCanvasChatStatus(assistantStatusInfo)}
                </div>
                <div
                  style={{
                    ...styles.canvasChatActions,
                    ...(actionVisible ? styles.canvasChatActionsVisible : null),
                    marginLeft: 'auto',
                  }}
                  className='canvas-chat-actions'
                >
                  {displayAssistantText
                    ? renderCanvasChatActionButton({
                        actionKey: 'copy',
                        tooltip: getCanvasChatCopyTooltipText(message),
                        ariaLabel: t('复制回答'),
                        message,
                        onClick: handleCopyCanvasChatMessage,
                        icon: <IconCopy size='small' />,
                      })
                    : null}
                  {retryPrompt
                    ? renderCanvasChatActionButton({
                        actionKey: 'retry',
                        tooltip: t('重发'),
                        ariaLabel: t('重发'),
                        message,
                        onClick: handleRetryCanvasChatMessage,
                        icon: <IconRefresh size='small' />,
                        disabled: chatStreaming,
                      })
                    : null}
                </div>
              </div>
            </div>
          )}
        </div>
      );
    }

    if (isUser) {
      return (
        <div
          key={message.id}
          data-canvas-message-row='true'
          className='canvas-message-card canvas-message-card--user'
          style={{
            ...styles.canvasMessageRow,
            ...styles.canvasMessageRowUser,
            ...(isSelected ? styles.canvasMessageRowSelected : null),
          }}
        >
          <div style={styles.canvasMessageBody}>
            <div style={styles.canvasMessagePrompt}>
              {message.prompt || t('已添加素材引用')}
            </div>
            {showMessageReferences && references.length > 0 ? (
              <div style={styles.canvasMessageRefs}>
                {references.map((file) =>
                  renderCanvasReferenceThumb(file, t('参考图')),
                )}
              </div>
            ) : null}
          </div>
        </div>
      );
    }

    return (
      <div
        key={message.id}
        data-canvas-message-row='true'
        style={{
          ...styles.canvasGenerationRow,
          ...(isSelected ? styles.canvasGenerationRowSelected : null),
        }}
        onClick={handleMessageClick}
      >
        <div style={styles.canvasGenerationCardWrap}>
          {renderCanvasGenerationCard(message)}
          {showMessageReferences && references.length > 0 ? (
            <div style={styles.canvasMessageRefs}>
              {references.map((file) =>
                renderCanvasReferenceThumb(file, t('参考图')),
              )}
            </div>
          ) : null}
        </div>
      </div>
    );
  };

  const renderCanvasMessageStream = () => {
    if (!selectedCanvasSession) {
      return (
        <div style={styles.canvasStreamEmpty}>
          {renderSidebarEmpty(
            t('空白会话'),
            renderCanvasBlankSessionDescription(),
          )}
        </div>
      );
    }
    if (canvasMessagesLoading) {
      return (
        <div style={styles.canvasStreamEmpty}>
          <div
            style={{ padding: 24, display: 'flex', justifyContent: 'center' }}
          >
            <Spin />
          </div>
        </div>
      );
    }
    if (canvasMessagesError) {
      return (
        <div style={styles.canvasStreamEmpty}>
          {renderSidebarEmpty(
            t('消息加载失败'),
            canvasMessagesError,
            <Button
              size='small'
              onClick={() =>
                loadCanvasMessages(
                  getCanvasSessionIdentifier(selectedCanvasSession),
                )
              }
            >
              {t('重试')}
            </Button>,
          )}
        </div>
      );
    }
    if (displayedCanvasMessages.length === 0) {
      return (
        <div style={styles.canvasStreamEmpty}>
          {renderSidebarEmpty(
            t('空白会话'),
            renderCanvasBlankSessionDescription(),
          )}
        </div>
      );
    }
    return (
      <div style={styles.canvasMessageList} className='canvas-message-list'>
        {generationMode === CANVAS_MODE_CHAT &&
        (canvasMessagesHasMore || canvasMessagesLoadingMore) ? (
          <div
            style={styles.canvasMessageHistoryHint}
            className='canvas-chat-history-hint'
          >
            {canvasMessagesLoadingMore ? <Spin size='small' /> : null}
            <span>
              {canvasMessagesLoadingMore
                ? t('加载更早消息中')
                : t('上滚加载更早消息')}
            </span>
          </div>
        ) : null}
        {renderableCanvasMessages.map((message, index) => (
          <React.Fragment key={message.id}>
            {generationMode === CANVAS_MODE_CHAT &&
            chatContextDividerIndex === index
              ? renderCanvasChatContextDivider({
                  key: `chat-context-divider-${message.id}`,
                  hasFollowingMessages: true,
                })
              : null}
            {renderCanvasMessage(message)}
          </React.Fragment>
        ))}
        {generationMode === CANVAS_MODE_CHAT &&
        chatContextDividerIndex === renderableCanvasMessages.length
          ? renderCanvasChatContextDivider({
              key: 'chat-context-divider-tail',
              hasFollowingMessages: false,
            })
          : null}
      </div>
    );
  };

  const renderComposerModeSwitch = () => {
    const options = [
      {
        value: CANVAS_MODE_CHAT,
        label: t('对话'),
        icon: <IconCommentStroked size='small' />,
      },
      {
        value: CANVAS_MODE_IMAGE,
        label: t('图片'),
        icon: <IconImage size='small' />,
      },
      {
        value: CANVAS_MODE_VIDEO,
        label: t('视频'),
        icon: <IconVideo size='small' />,
      },
    ];
    const activeOption =
      options.find((option) => option.value === generationMode) || options[0];
    return renderPillDropdown({
      key: 'composer-mode',
      label: '',
      icon: activeOption.icon,
      value: generationMode,
      displayValue: activeOption.label,
      onChange: handleModeChange,
      options,
    });
  };

  const renderComposer = () => {
    const isChatMode = generationMode === CANVAS_MODE_CHAT;
    const isVideoMode = generationMode === CANVAS_MODE_VIDEO;
    const isImageMode = generationMode === CANVAS_MODE_IMAGE;
    const activeModel = isVideoMode
      ? videoSelectedModelData
      : selectedModelData;
    const activeChatModelLabel = activeChatModelOption
      ? getChatModelDisplayText(activeChatModelOption)
      : chatModel || t('请选择模型');
    const activeModelLabel = isChatMode
      ? activeChatModelLabel
      : activeModel
        ? getModelDisplayName(activeModel)
        : t('请选择模型');
    const activePrompt = isChatMode
      ? chatPrompt
      : isVideoMode
        ? videoPrompt
        : inspiration;
    const chatUploadVisibility = getCanvasChatUploadVisibility(
      activeChatModelOption,
    );
    const showChatImageUpload =
      isChatMode && chatUploadVisibility.showImageUpload;
    const showChatFileUpload =
      isChatMode && chatUploadVisibility.showFileUpload;
    const chatUploadAccept = getCanvasChatUploadAccept(chatUploadVisibility);
    const hasChatInlineAttachments =
      isChatMode && (!!chatImageAttachment || !!chatFileAttachment);
    const promptHasContent = activePrompt.trim().length > 0;
    const submitLoading = isChatMode
      ? false
      : isVideoMode
        ? videoGenerating
        : isImageMode
          ? generating
          : false;
    const submitDisabled = isChatMode
      ? !promptHasContent
      : isVideoMode
        ? videoGenerating || !canGenerateVideo
        : generating || !canGenerate;
    const isChatSubmitButtonStopState = isChatMode && chatStreaming;
    const submitButtonDisabled = isChatSubmitButtonStopState
      ? false
      : submitDisabled;
    const submitButtonLabel = isChatSubmitButtonStopState
      ? t('停止回复')
      : isChatMode
        ? t('发送消息')
        : isVideoMode
          ? t('生成视频')
          : t('生成图片');
    const submitEmphasis = promptHasContent ? 'primary' : 'idle';
    const placeholder = isChatMode
      ? t('输入消息...')
      : isVideoMode
        ? t('描述镜头、动作、风格、时长...')
        : t('输入提示词，生成图片或继续当前图片任务...');

    const handleComposerSubmit = () => {
      if (submitDisabled) {
        return;
      }
      if (isChatMode) {
        handleSendChatMessage();
        return;
      }
      if (isVideoMode) {
        handleGenerateVideo();
        return;
      }
      handleGenerate();
    };
    const handleComposerActionButtonClick = () => {
      if (isChatSubmitButtonStopState) {
        stopChatStream();
        return;
      }
      handleComposerSubmit();
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
    const renderUploadIconButton = ({
      disabled = false,
      title = t('上传图片'),
      icon = <IconPlus size='small' />,
    } = {}) => (
      <div
        className='canvas-composer-upload-btn'
        aria-label={title}
        aria-disabled={disabled}
        title={title}
        data-composer-control-kind='upload'
        style={{
          ...styles.uploadIconBtn,
          opacity: disabled ? 0.5 : 1,
          cursor: disabled ? 'not-allowed' : 'pointer',
        }}
      >
        {icon}
      </div>
    );
    const renderChatUploadButton = () => {
      if (!showChatImageUpload && !showChatFileUpload) {
        return null;
      }
      const buttonLabel = t('上传');

      return (
        <>
          <input
            ref={chatAttachmentUploadInputRef}
            type='file'
            accept={chatUploadAccept}
            style={{ display: 'none' }}
            onChange={handleChatAttachmentInputChange}
          />
          <button
            className='canvas-composer-upload-btn'
            type='button'
            aria-label={buttonLabel}
            title={buttonLabel}
            data-composer-control-kind='upload'
            style={{
              ...styles.uploadIconBtn,
              cursor: 'pointer',
            }}
            onClick={() =>
              triggerHiddenChatUploadInput(chatAttachmentUploadInputRef)
            }
          >
            <IconPlus size='small' />
          </button>
        </>
      );
    };
    const chatDropdownModels = getChatModelsWithPreservedCurrent(
      chatModel,
      String(chatModel || '').trim() ===
        String(selectedCanvasSession?.current_model || '').trim()
        ? t('当前会话模型，现不可用')
        : t('当前选择的模型，现不可用'),
    );
    const chatComposerParameters = [
      renderPillDropdown({
        key: 'chat-model',
        label: t('模型'),
        icon: (
          <CanvasModelSeriesIcon
            series={activeChatModelOption?.model_series}
            size='small'
          />
        ),
        value: chatModel,
        displayValue: activeChatModelLabel,
        onChange: (value) => {
          setChatModel(value);
          updateCurrentCanvasSessionModel(CANVAS_MODE_CHAT, value);
        },
        options: chatDropdownModels.map((model) => ({
          value: model.request_model,
          label: buildChatModelOptionLabel(model),
          icon: (
            <CanvasModelSeriesIcon series={model.model_series} size='small' />
          ),
          disabled: model.usable === false,
        })),
        disabled: chatDropdownModels.length === 0,
      }),
      renderChatSettingsDropdown(),
    ];
    const imageComposerParameters = [
      renderModelDropdown(false, activeModelLabel),
      renderImageParametersDropdown(),
      selectedModelSupportsMaskEditing && referenceImages.length > 0 && (
        <button
          key='image-advanced'
          className='canvas-composer-pill'
          type='button'
          aria-label={t('高级')}
          title={t('高级')}
          data-composer-control-kind='parameter'
          data-composer-control-active={
            composerAdvancedVisible ? 'true' : 'false'
          }
          style={{
            ...styles.pillButton,
            ...(isMobile ? styles.pillButtonIconOnly : null),
            ...(composerAdvancedVisible
              ? styles.pillButtonOpen
              : styles.pillButtonSelected),
            cursor: 'pointer',
          }}
          onClick={() => setComposerAdvancedVisible((current) => !current)}
        >
          <IconSetting size='small' />
          {!isMobile ? <span>{t('高级')}</span> : null}
        </button>
      ),
    ].filter(Boolean);
    const videoComposerParameters = [
      renderModelDropdown(true, activeModelLabel),
      renderVideoParametersDropdown(),
      renderPillDropdown({
        key: 'video-duration',
        label: t('时长'),
        icon: <IconClock size='small' />,
        value: videoDuration,
        displayValue: videoDuration ? `${videoDuration}s` : '',
        onChange: setVideoDuration,
        options: (videoSelectedModelData?.duration_options || []).map(
          (item) => ({
            value: item,
            label: `${item}s`,
          }),
        ),
        disabled: !videoSelectedModelData?.duration_options?.length,
      }),
    ].filter(Boolean);
    const activeParameters = isChatMode
      ? chatComposerParameters
      : isVideoMode
        ? videoComposerParameters
        : imageComposerParameters;
    const showPromptUploadEntry =
      showChatImageUpload ||
      showChatFileUpload ||
      (isVideoMode && videoSelectedModelSupportsImageToVideo) ||
      (isImageMode && selectedModelSupportsEditing);
    const hasInlineReferenceThumbs =
      hasChatInlineAttachments ||
      (isImageMode && referenceImages.length > 0) ||
      (isVideoMode &&
        videoSelectedModelSupportsImageToVideo &&
        !!videoReferenceImage);
    const promptUploadControl = !showPromptUploadEntry ? null : isChatMode ? (
      renderChatUploadButton()
    ) : isImageMode && selectedModelSupportsEditing ? (
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
        {renderUploadIconButton({
          disabled: referenceImageLimitReached,
          title: referenceImageLimitReached
            ? t('已达到当前模型参考图上限')
            : t('上传图片'),
        })}
      </Upload>
    ) : isVideoMode && videoSelectedModelSupportsImageToVideo ? (
      <Upload
        action=''
        accept='image/*'
        multiple={false}
        fileList={videoReferenceImage ? [videoReferenceImage] : []}
        onChange={handleVideoReferenceUpload}
        showUploadList={false}
        beforeUpload={validateImageSize}
      >
        {renderUploadIconButton({ title: t('上传图片') })}
      </Upload>
    ) : null;

    return (
      <div style={styles.composerDock} className='canvas-composer-dock'>
        {isChatMode &&
        displayedCanvasMessages.length > 0 &&
        !canvasAutoFollowEnabled ? (
          <div style={styles.chatAutoFollowDock}>
            <button
              type='button'
              className='canvas-chat-follow-resume'
              style={styles.chatAutoFollowButton}
              onClick={handleResumeCanvasAutoFollow}
            >
              <IconChevronDown size='small' />
              <span>
                {chatStreaming ? t('回到底部并继续跟随') : t('回到底部')}
              </span>
            </button>
          </div>
        ) : null}
        <div
          style={styles.composerShell}
          className='canvas-composer-shell'
          data-canvas-composer={generationMode}
          data-canvas-composer-shell='true'
        >
          <div style={styles.promptArea} className='canvas-composer-card'>
            <div
              style={{
                ...styles.promptInputShell,
                ...(isChatMode && chatComposerDragActive
                  ? styles.promptInputShellDragActive
                  : null),
              }}
              className='canvas-composer-input-shell'
              data-canvas-chat-drag-active={
                isChatMode && chatComposerDragActive ? 'true' : 'false'
              }
              onDragEnter={isChatMode ? handleChatComposerDragEnter : undefined}
              onDragOver={isChatMode ? handleChatComposerDragOver : undefined}
              onDragLeave={isChatMode ? handleChatComposerDragLeave : undefined}
              onDrop={isChatMode ? handleChatComposerDrop : undefined}
            >
              {hasInlineReferenceThumbs ? (
                <div
                  style={styles.promptLeadingSlot}
                  className='canvas-composer-leading-slot'
                >
                  <div
                    style={styles.promptInlineAssets}
                    className='canvas-composer-inline-assets'
                  >
                    {isChatMode && chatImageAttachment
                      ? renderReferenceThumb(chatImageAttachment, () =>
                          handleRemoveChatImageAttachment(),
                        )
                      : null}
                    {isChatMode && chatFileAttachment
                      ? renderChatFileAttachmentChip(
                          chatFileAttachment,
                          handleRemoveChatFileAttachment,
                        )
                      : null}
                    {isImageMode
                      ? referenceImages.map((file) =>
                          renderReferenceThumb(file, () =>
                            handleImageRemove(file),
                          ),
                        )
                      : null}
                    {isVideoMode &&
                    videoSelectedModelSupportsImageToVideo &&
                    videoReferenceImage
                      ? renderReferenceThumb(
                          videoReferenceImage,
                          handleVideoReferenceRemove,
                        )
                      : null}
                  </div>
                </div>
              ) : null}
              <TextArea
                aria-label={placeholder}
                className='canvas-composer-input'
                data-canvas-prompt-input={generationMode}
                placeholder={placeholder}
                value={activePrompt}
                onChange={
                  isChatMode
                    ? setChatPrompt
                    : isVideoMode
                      ? setVideoPrompt
                      : setInspiration
                }
                onKeyDown={handleComposerKeyDown}
                onCompositionStart={() => {
                  composerComposingRef.current = true;
                }}
                onCompositionEnd={() => {
                  composerComposingRef.current = false;
                }}
                onPaste={isChatMode ? handleChatComposerPaste : undefined}
                maxLength={5000}
                borderless
                autosize={{ minRows: 2, maxRows: 8 }}
                style={styles.promptInput}
              />
              <div
                style={styles.promptControls}
                className='canvas-composer-controls'
              >
                <div
                  style={styles.promptControlsLeft}
                  className='canvas-composer-controls-left'
                >
                  {promptUploadControl}
                  {renderComposerModeSwitch()}
                  <div
                    style={styles.composerParameterRow}
                    className='canvas-composer-parameter-row'
                  >
                    {activeParameters}
                  </div>
                </div>
                <div
                  style={styles.promptControlsRight}
                  className='canvas-composer-controls-right'
                >
                  <div style={styles.composerActionGroup}>
                    <button
                      className='canvas-composer-submit'
                      aria-label={submitButtonLabel}
                      title={submitButtonLabel}
                      style={
                        isChatSubmitButtonStopState
                          ? styles.generateStopBtnEmbedded
                          : {
                              ...styles.generateIconBtnEmbedded,
                              '--canvas-submit-bg': promptHasContent
                                ? 'var(--canvas-primary)'
                                : 'var(--canvas-toolbar-bg)',
                              '--canvas-submit-border': promptHasContent
                                ? 'var(--canvas-primary)'
                                : 'var(--canvas-border)',
                              '--canvas-submit-color': promptHasContent
                                ? 'var(--text-inverse)'
                                : 'var(--canvas-text-muted)',
                              '--canvas-submit-shadow': promptHasContent
                                ? '0 10px 22px rgba(109, 93, 246, 0.16)'
                                : 'none',
                            }
                      }
                      data-submit-emphasis={submitEmphasis}
                      data-submit-state={
                        isChatSubmitButtonStopState ? 'stop' : 'send'
                      }
                      onClick={handleComposerActionButtonClick}
                      disabled={submitButtonDisabled}
                      type='button'
                    >
                      {isChatSubmitButtonStopState ? (
                        <span style={styles.generateStopIcon} />
                      ) : submitLoading ? (
                        <Spin size='small' />
                      ) : (
                        <IconSend size='small' />
                      )}
                    </button>
                  </div>
                </div>
              </div>
            </div>

            {isImageMode &&
            selectedModelSupportsMaskEditing &&
            referenceImages.length > 0 &&
            composerAdvancedVisible ? (
              <div
                style={{
                  borderTop: '1px solid var(--canvas-border)',
                  paddingTop: 10,
                  marginTop: 8,
                }}
              >
                <Text
                  size='small'
                  style={{
                    display: 'block',
                    marginBottom: 8,
                    color: 'var(--canvas-text-muted)',
                  }}
                >
                  {t('遮罩会与第一张参考图一起作为标准编辑请求提交')}
                </Text>
                <div
                  style={{
                    display: 'flex',
                    gap: 8,
                    flexWrap: 'wrap',
                    alignItems: 'center',
                  }}
                >
                  {maskImage
                    ? renderReferenceThumb(maskImage, handleMaskRemove)
                    : null}
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
                        cursor:
                          referenceImages.length === 0
                            ? 'not-allowed'
                            : 'pointer',
                      }}
                    >
                      <IconSetting size='large' />
                    </div>
                  </Upload>
                </div>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    );
  };

  const renderAssetCard = (asset) => {
    const imageUrl = getAssetImageUrl(asset);
    const assetId = getAssetComparableKey(asset);
    const isSelected = selectedCanvasAssetIds.has(assetId);
    const stopCardActionPropagation = (event) => {
      event.stopPropagation();
    };
    return (
      <div
        key={getAssetKey(asset)}
        style={styles.assetCard}
        data-canvas-asset-card='true'
        onClick={() => setSelectedAssetPreview(asset)}
      >
        <div style={styles.assetSelect} onClick={stopCardActionPropagation}>
          <Checkbox
            checked={isSelected}
            onChange={(event) =>
              handleCanvasAssetSelect(asset, event.target.checked)
            }
            aria-label={t('选择资产')}
          />
        </div>
        {imageUrl ? (
          <img src={imageUrl} alt='' style={styles.assetThumb} />
        ) : (
          <div
            style={{
              ...styles.assetThumb,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <IconImage size='large' />
          </div>
        )}
        <div style={styles.assetCardMeta}>
          <Text size='small' strong ellipsis={{ rows: 1 }}>
            {getAssetDisplayName(asset)}
          </Text>
        </div>
        <div style={styles.assetActions}>
          {generationMode !== CANVAS_MODE_CHAT ? (
            <Button
              size='small'
              type='primary'
              onClick={(event) => {
                event.stopPropagation();
                insertAssetIntoCurrentMode(asset);
              }}
            >
              {generationMode === CANVAS_MODE_VIDEO ? t('首帧') : t('参考图')}
            </Button>
          ) : null}
          <Button
            size='small'
            aria-label={t('下载图片')}
            icon={<IconDownload />}
            onClick={(event) => {
              event.stopPropagation();
              downloadAsset(asset);
            }}
          />
          <span onClick={stopCardActionPropagation}>
            <Popconfirm
              title={t('确认删除该资产？')}
              content={t('删除后无法恢复，请确认是否继续')}
              okType='danger'
              onConfirm={() => deleteAsset(asset)}
            >
              <Button
                size='small'
                type='danger'
                theme='borderless'
                aria-label={t('删除资产')}
                icon={<IconDelete />}
              />
            </Popconfirm>
          </span>
        </div>
      </div>
    );
  };

  const renderAssetPreviewMeta = (label, value) => (
    <div style={styles.assetPreviewMetaItem}>
      <Text type='tertiary' size='small'>
        {label}
      </Text>
      <Text size='small' style={{ wordBreak: 'break-word' }}>
        {value || '-'}
      </Text>
    </div>
  );

  const renderAssetPreviewOverlay = () => {
    if (!selectedAssetPreview) {
      return null;
    }
    const imageUrl = getAssetImageUrl(selectedAssetPreview);
    return (
      <div
        style={styles.assetPreviewOverlay}
        onClick={() => setSelectedAssetPreview(null)}
        data-canvas-asset-preview='true'
      >
        <div
          style={styles.assetPreviewPanel}
          onClick={(event) => event.stopPropagation()}
        >
          <div style={styles.assetPreviewImagePane}>
            {imageUrl ? (
              <img src={imageUrl} alt='' style={styles.assetPreviewImage} />
            ) : (
              <IconImage size='extra-large' />
            )}
          </div>
          <div style={styles.assetPreviewInfo}>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                gap: 12,
              }}
            >
              <div style={{ minWidth: 0 }}>
                <Text strong ellipsis={{ rows: 2 }}>
                  {getAssetDisplayName(selectedAssetPreview)}
                </Text>
                <Text
                  type='tertiary'
                  size='small'
                  ellipsis={{ rows: 1 }}
                  style={{ display: 'block', marginTop: 4 }}
                >
                  {selectedAssetPreview.model_id || '-'}
                </Text>
              </div>
              <Button
                size='small'
                type='tertiary'
                onClick={() => setSelectedAssetPreview(null)}
              >
                {t('关闭')}
              </Button>
            </div>
            {renderAssetPreviewMeta(
              t('名称'),
              getAssetDisplayName(selectedAssetPreview),
            )}
            {renderAssetPreviewMeta(
              'model_id',
              selectedAssetPreview.model_id || '-',
            )}
            {renderAssetPreviewMeta(
              t('尺寸/比例'),
              getAssetSizeText(selectedAssetPreview),
            )}
            {renderAssetPreviewMeta(
              t('任务 ID'),
              getAssetTaskIdText(selectedAssetPreview),
            )}
            {renderWeakDetails(
              t('更多信息'),
              <>
                {renderAssetPreviewMeta(
                  t('创建时间'),
                  formatTimestamp(selectedAssetPreview.created_time),
                )}
                {renderAssetPreviewMeta(
                  t('完成时间'),
                  formatTimestamp(selectedAssetPreview.completed_time),
                )}
                {renderAssetPreviewMeta(
                  t('提示词'),
                  selectedAssetPreview.prompt || '-',
                )}
              </>,
            )}
          </div>
        </div>
      </div>
    );
  };

  const renderCanvasImagePreviewOverlay = () => {
    if (!selectedCanvasImagePreview?.src) {
      return null;
    }
    const canDownload = !!(
      selectedCanvasImagePreview.downloadSrc || selectedCanvasImagePreview.src
    );
    return (
      <div
        style={styles.assetPreviewOverlay}
        onClick={() => setSelectedCanvasImagePreview(null)}
        data-canvas-image-preview='true'
      >
        <div
          style={styles.canvasImagePreviewPanel}
          onClick={(event) => event.stopPropagation()}
        >
          {canDownload ? (
            <div style={styles.canvasImagePreviewActions}>
              <button
                type='button'
                title={t('下载图片')}
                aria-label={t('下载图片')}
                style={styles.canvasImagePreviewActionBtn}
                onClick={(event) => {
                  event.stopPropagation();
                  handleDownloadCanvasPreviewImage(selectedCanvasImagePreview);
                }}
              >
                <IconDownload size='small' />
              </button>
            </div>
          ) : null}
          <img
            src={selectedCanvasImagePreview.src}
            alt=''
            style={styles.canvasImagePreviewImage}
          />
        </div>
      </div>
    );
  };

  const renderAssetDrawer = () => (
    <SideSheet
      visible={assetLibraryVisible}
      title={t('资产库')}
      width={isMobile ? '100%' : 560}
      onCancel={() => setAssetLibraryVisible(false)}
      bodyStyle={{
        padding: isMobile ? 12 : 16,
        background: 'var(--canvas-sidebar-bg)',
      }}
      data-canvas-asset-drawer='true'
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              gap: 8,
              flexWrap: 'wrap',
            }}
          >
            <Text type='tertiary' size='small'>
              {t('资产总数')}：{canvasAssetsTotal}
            </Text>
            <Button
              size='small'
              icon={<IconRefresh />}
              onClick={() => loadCanvasAssets(canvasAssetPage || 1)}
            >
              {t('刷新')}
            </Button>
          </div>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 8,
              flexWrap: 'wrap',
            }}
          >
            <Checkbox
              checked={allCurrentCanvasAssetsSelected}
              indeterminate={partiallyCurrentCanvasAssetsSelected}
              disabled={canvasAssets.length === 0 || canvasAssetsLoading}
              onChange={(event) =>
                handleCanvasAssetSelectAll(event.target.checked)
              }
            >
              {allCurrentCanvasAssetsSelected
                ? t('取消选择当前页')
                : t('选择当前页')}
            </Checkbox>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                flexWrap: 'wrap',
              }}
            >
              <Tag color={selectedCanvasAssetCount > 0 ? 'blue' : 'grey'}>
                {t('已选 {{count}} 项', { count: selectedCanvasAssetCount })}
              </Tag>
              <Button
                size='small'
                type='tertiary'
                disabled={selectedCanvasAssetCount === 0 || assetBatchDeleting}
                onClick={() => setSelectedCanvasAssetIds(new Set())}
              >
                {t('取消选择')}
              </Button>
              <Popconfirm
                title={t('确认删除选中资产？')}
                content={t('删除后无法恢复，请确认是否继续')}
                okType='danger'
                onConfirm={deleteSelectedCanvasAssets}
              >
                <Button
                  size='small'
                  type='danger'
                  icon={<IconDelete />}
                  loading={assetBatchDeleting}
                  disabled={selectedCanvasAssetCount === 0}
                >
                  {t('删除选中')}
                </Button>
              </Popconfirm>
            </div>
          </div>
        </div>
        <Spin spinning={canvasAssetsLoading}>
          {canvasAssetsError ? (
            renderSidebarEmpty(
              t('加载资产失败'),
              canvasAssetsError,
              <Button size='small' onClick={() => loadCanvasAssets(1)}>
                {t('重试')}
              </Button>,
            )
          ) : canvasAssets.length === 0 ? (
            renderSidebarEmpty(
              t('暂无图片资产'),
              t('完成图片生成后会出现在这里'),
            )
          ) : (
            <div style={styles.assetGrid}>
              {canvasAssets.map(renderAssetCard)}
            </div>
          )}
        </Spin>
        <div style={styles.drawerFooterPager}>
          <Button
            size='small'
            disabled={canvasAssetPage <= 1 || canvasAssetsLoading}
            onClick={() => loadCanvasAssets(Math.max(1, canvasAssetPage - 1))}
          >
            {t('上一页')}
          </Button>
          <Text type='tertiary' size='small'>
            {t('第 {{page}} 页', { page: canvasAssetPage })}
          </Text>
          <Button
            size='small'
            disabled={
              canvasAssetsLoading ||
              canvasAssetPage * DEFAULT_ASSET_PAGE_SIZE >= canvasAssetsTotal
            }
            onClick={() => loadCanvasAssets(canvasAssetPage + 1)}
          >
            {t('下一页')}
          </Button>
        </div>
      </div>
    </SideSheet>
  );

  const renderCanvasChatSessionSettingsSheet = () => {
    const session = findCanvasSessionByIdentifier(chatSessionSettingsSessionId);
    const draft = chatSessionSettingsDraft;
    if (!session || !draft) {
      return null;
    }

    const availableChatModels = getChatModelsWithPreservedCurrent(
      draft.model,
      t('当前会话模型，现不可用'),
    );
    const availableChatModelOptions = availableChatModels.map((item) => ({
      value: item.request_model,
      label: getModelDisplayName(item),
      disabled: item.usable === false,
      model: item,
    }));
    const draftModelOption =
      availableChatModels.find((item) => item?.request_model === draft.model) ||
      null;
    const showWebSearchToggle =
      getCanvasChatWebSearchVisibility(draftModelOption);
    const renderChatModelOption = (renderProps) => {
      const {
        disabled,
        selected,
        className,
        style,
        onMouseEnter,
        onClick,
        label,
        value,
      } = renderProps;
      const modelItem = availableChatModels.find(
        (item) => item?.request_model === value,
      );
      const unavailableReason =
        modelItem?.usable === false
          ? String(modelItem?.unavailable_reason || t('当前不可用'))
          : '';
      return (
        <div
          style={style}
          className={className}
          onClick={() => !disabled && onClick?.()}
          onMouseEnter={() => onMouseEnter?.()}
        >
          <div style={styles.selectModelOption}>
            <CanvasModelSeriesIcon series={modelItem?.model_series} />
            <div style={styles.selectModelOptionText}>
              <span style={styles.selectModelOptionTitle}>
                {label || getChatModelDisplayText(modelItem) || value}
              </span>
              {unavailableReason ? (
                <span style={styles.selectModelOptionMeta}>
                  {unavailableReason}
                </span>
              ) : null}
            </div>
            {selected ? <Text type='success'>{t('已选')}</Text> : null}
          </div>
        </div>
      );
    };
    const renderChatModelSelectedItem = (optionNode) => {
      const modelItem = availableChatModels.find(
        (item) => item?.request_model === optionNode?.value,
      );
      const unavailableReason =
        modelItem?.usable === false
          ? String(modelItem?.unavailable_reason || t('当前不可用'))
          : '';
      return (
        <div style={styles.selectModelOption}>
          <CanvasModelSeriesIcon series={modelItem?.model_series} />
          <div style={styles.selectModelOptionText}>
            <span style={styles.selectModelOptionTitle}>
              {optionNode?.label ||
                getChatModelDisplayText(modelItem) ||
                optionNode?.value ||
                t('请选择模型')}
            </span>
            {unavailableReason ? (
              <span style={styles.selectModelOptionMeta}>
                {unavailableReason}
              </span>
            ) : null}
          </div>
        </div>
      );
    };

    return (
      <SideSheet
        visible={chatSessionSettingsVisible}
        title={t('会话设置')}
        width={isMobile ? '100%' : 520}
        onCancel={closeCanvasChatSessionSettings}
        bodyStyle={{
          padding: isMobile ? 12 : 16,
          background: 'var(--canvas-sidebar-bg)',
        }}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <Text strong>{t('模型')}</Text>
            <Select
              value={draft.model}
              optionList={availableChatModelOptions}
              onChange={(value) => {
                const nextModel = String(value || '');
                const nextModelOption =
                  availableChatModels.find(
                    (item) => item?.request_model === nextModel,
                  ) || null;
                handleCanvasChatSessionSettingsField('model', nextModel);
                if (!getCanvasChatWebSearchVisibility(nextModelOption)) {
                  handleCanvasChatSessionSettingsField(
                    'webSearchEnabled',
                    false,
                  );
                }
              }}
              placeholder={t('请选择模型')}
              renderOptionItem={renderChatModelOption}
              renderSelectedItem={renderChatModelSelectedItem}
            />
          </div>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: isMobile ? '1fr' : '1fr 1fr',
              gap: 12,
            }}
          >
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              <Text strong>{t('温度')}</Text>
              <Select
                value={draft.temperature}
                optionList={['0', '0.2', '0.7', '1', '1.5'].map((item) => ({
                  label: item,
                  value: item,
                }))}
                onChange={(value) =>
                  handleCanvasChatSessionSettingsField(
                    'temperature',
                    String(value || '0'),
                  )
                }
              />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              <Text strong>{t('上下文')}</Text>
              <Select
                value={draft.contextCount}
                optionList={['0', '4', '8', '16', '32'].map((item) => ({
                  label: item,
                  value: item,
                }))}
                onChange={(value) =>
                  handleCanvasChatSessionSettingsField(
                    'contextCount',
                    String(value || '0'),
                  )
                }
              />
            </div>
          </div>
          {showWebSearchToggle ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              <Checkbox
                checked={draft.webSearchEnabled}
                onChange={(event) =>
                  handleCanvasChatSessionSettingsField(
                    'webSearchEnabled',
                    !!event.target.checked,
                  )
                }
              >
                {t('启用联网搜索')}
              </Checkbox>
            </div>
          ) : null}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <Text strong>{t('系统提示词')}</Text>
            <TextArea
              value={draft.systemPrompt}
              onChange={(value) =>
                handleCanvasChatSessionSettingsField('systemPrompt', value)
              }
              placeholder={t('设置该会话的系统提示词，可为空')}
              autosize={{ minRows: 4, maxRows: 10 }}
              maxLength={4000}
            />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <Checkbox
              checked={draft.summaryEnabled}
              onChange={(event) =>
                handleCanvasChatSessionSettingsField(
                  'summaryEnabled',
                  !!event.target.checked,
                )
              }
            >
              {t('启用摘要记忆')}
            </Checkbox>
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: isMobile ? '1fr' : '1fr 1fr',
                gap: 12,
                opacity: draft.summaryEnabled ? 1 : 0.6,
              }}
            >
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                <Text strong>{t('摘要触发阈值')}</Text>
                <Select
                  value={draft.summaryTriggerMessages}
                  disabled={!draft.summaryEnabled}
                  optionList={CHAT_SUMMARY_STRATEGY_OPTIONS.map((item) => ({
                    label: item,
                    value: item,
                  }))}
                  onChange={(value) =>
                    handleCanvasChatSessionSettingsField(
                      'summaryTriggerMessages',
                      String(value || '0'),
                    )
                  }
                />
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                <Text strong>{t('摘要保留消息')}</Text>
                <Select
                  value={draft.summaryRecentMessages}
                  disabled={!draft.summaryEnabled}
                  optionList={CHAT_SUMMARY_STRATEGY_OPTIONS.map((item) => ({
                    label: item,
                    value: item,
                  }))}
                  onChange={(value) =>
                    handleCanvasChatSessionSettingsField(
                      'summaryRecentMessages',
                      String(value || '0'),
                    )
                  }
                />
              </div>
            </div>
          </div>
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
            <Button type='tertiary' onClick={closeCanvasChatSessionSettings}>
              {t('取消')}
            </Button>
            <Button
              type='primary'
              loading={chatSessionSettingsSaving}
              onClick={saveCanvasChatSessionSettings}
            >
              {t('保存')}
            </Button>
          </div>
        </div>
      </SideSheet>
    );
  };

  const renderWorkspace = () => (
    <div style={styles.rightPanel}>
      {isMobile ? (
        <div style={styles.workspaceTopbar}>
          <div style={styles.workspaceTopbarLeft}>
            <Button
              type='tertiary'
              aria-label={t('打开任务栏')}
              data-canvas-mobile-taskbar-trigger='true'
              icon={<IconMenu />}
              onClick={() => setMobileTaskbarVisible(true)}
            />
          </div>
        </div>
      ) : null}
      <div style={styles.workspaceBody} className='canvas-workspace-body'>
        <div
          ref={canvasMessageViewportRef}
          style={styles.workspaceScrollPanel}
          className='canvas-workspace-scroll-panel canvas-scroll-panel'
          onScroll={handleCanvasWorkspaceScroll}
        >
          <div style={styles.mainViewport}>
            {generationMode === CANVAS_MODE_CHAT
              ? renderChatWorkspace()
              : renderCanvasMessageStream()}
          </div>
          {renderComposer()}
        </div>
      </div>
    </div>
  );

  return (
    <div
      style={styles.container}
      className='canvas-theme-scope'
      data-canvas-theme-scope='true'
    >
      {!isMobile
        ? renderTaskSidebar({ collapsed: desktopSidebarCollapsed })
        : null}
      {renderWorkspace()}
      {isMobile ? (
        <SideSheet
          visible={mobileTaskbarVisible}
          title={t('导航')}
          width='100%'
          onCancel={() => setMobileTaskbarVisible(false)}
          bodyStyle={{ padding: 0, background: 'var(--canvas-sidebar-bg)' }}
        >
          {renderTaskSidebar()}
        </SideSheet>
      ) : null}
      {renderAssetDrawer()}
      {renderCanvasChatSessionSettingsSheet()}
      {renderAssetPreviewOverlay()}
      {renderCanvasImagePreviewOverlay()}
      <ImageGenerationTaskModal
        visible={taskModalVisible}
        task={selectedTask}
        onClose={() => setTaskModalVisible(false)}
        onRetrySuccess={(task) => {
          updateTaskInList(task);
          setSelectedTask(task);
        }}
        onDeleted={(taskId) => {
          removeImageTasksFromLocalState([taskId]);
        }}
      />
      <VideoGenerationTaskModal
        visible={videoTaskModalVisible}
        task={videoSelectedTask}
        onClose={() => setVideoTaskModalVisible(false)}
        onRetrySuccess={(task) => {
          updateVideoTaskInList(task);
          setVideoSelectedTask(task);
        }}
        onDeleted={(taskId) => {
          removeVideoTasksFromLocalState([taskId]);
        }}
      />
    </div>
  );
};

export default ImageGeneration;
