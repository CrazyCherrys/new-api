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

export const imageResolutionOptions = [
  '1024x1024',
  '1152x896',
  '896x1152',
  '1216x832',
  '832x1216',
  '1344x768',
  '768x1344',
  '1536x640',
  '640x1536',
];

export const aspectRatioOptions = [
  '1:1',
  '16:9',
  '9:16',
  '4:3',
  '3:4',
  '21:9',
  '9:21',
];

export const videoDurationOptions = ['2', '4', '6', '8', '10'];

export const defaultImageGenerationInputs = {
  // Storage
  storage_type: 'local',
  storage_local_path: './data/images',
  storage_s3_endpoint: '',
  storage_s3_bucket: '',
  storage_s3_access_key: '',
  storage_s3_secret_key: '',
  storage_s3_region: 'us-east-1',
  storage_s3_path_prefix: 'generated-images',
  // Generation
  image_timeout_seconds: 300,
  image_max_retry_attempts: 3,
  image_retry_interval_seconds: 10,
  image_worker_count: 2,
  image_stale_after_minutes: 10,
  // Global fallback model config
  enabled_models: [],
  default_model: '',
  default_resolution: '1024x1024',
  default_aspect_ratio: '1:1',
  max_image_count: 10,
  rpm_limit: 60,
  // Per-model config
  model_settings: {},
  // UI field
  s3_enabled: false,
};

export const modelConfigKeys = [
  'enabled_models',
  'default_model',
  'default_resolution',
  'default_aspect_ratio',
  'max_image_count',
  'rpm_limit',
  'model_settings',
];

export const baseConfigKeys = [
  'storage_type',
  'storage_local_path',
  'storage_s3_endpoint',
  'storage_s3_bucket',
  'storage_s3_access_key',
  'storage_s3_secret_key',
  'storage_s3_region',
  'storage_s3_path_prefix',
  'image_timeout_seconds',
  'image_max_retry_attempts',
  'image_retry_interval_seconds',
  'image_worker_count',
  'image_stale_after_minutes',
  'rpm_limit',
  's3_enabled',
];

const toArray = (value) => (Array.isArray(value) ? value : []);

const uniqStrings = (items) => {
  const seen = new Set();
  const result = [];
  toArray(items).forEach((item) => {
    if (typeof item !== 'string') {
      return;
    }
    const value = item.trim();
    if (!value || seen.has(value)) {
      return;
    }
    seen.add(value);
    result.push(value);
  });
  return result;
};

const ensurePositiveInt = (value, fallback) => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback;
  }
  return Math.floor(parsed);
};

export const createDefaultModelSetting = (
  modelId,
  rawSetting = {},
  globalConfig = defaultImageGenerationInputs,
) => {
  const modelID = (modelId || '').trim();
  const fallbackResolution =
    globalConfig.default_resolution || defaultImageGenerationInputs.default_resolution;
  const fallbackAspectRatio =
    globalConfig.default_aspect_ratio || defaultImageGenerationInputs.default_aspect_ratio;
  const fallbackMaxImageCount = ensurePositiveInt(
    globalConfig.max_image_count,
    defaultImageGenerationInputs.max_image_count,
  );
  const fallbackRpmLimit = ensurePositiveInt(
    globalConfig.rpm_limit,
    defaultImageGenerationInputs.rpm_limit,
  );

  const resolutions = uniqStrings(rawSetting.resolutions);
  const aspectRatios = uniqStrings(rawSetting.aspect_ratios);
  const durations = uniqStrings(rawSetting.durations);

  const defaultResolution =
    (rawSetting.default_resolution || '').trim() ||
    resolutions[0] ||
    fallbackResolution;
  const defaultAspectRatio =
    (rawSetting.default_aspect_ratio || '').trim() ||
    aspectRatios[0] ||
    fallbackAspectRatio;

  return {
    display_name: (rawSetting.display_name || '').trim() || modelID,
    request_model_id: (rawSetting.request_model_id || '').trim() || modelID,
    request_endpoint:
      (rawSetting.request_endpoint || '').trim() || 'openai',
    model_type: (rawSetting.model_type || '').trim() || 'image',
    default_resolution: defaultResolution,
    default_aspect_ratio: defaultAspectRatio,
    resolutions: resolutions.length > 0 ? resolutions : [defaultResolution],
    aspect_ratios: aspectRatios.length > 0 ? aspectRatios : [defaultAspectRatio],
    durations,
    max_image_count: ensurePositiveInt(
      rawSetting.max_image_count,
      fallbackMaxImageCount,
    ),
    rpm_limit: ensurePositiveInt(rawSetting.rpm_limit, fallbackRpmLimit),
    rpm_enabled: Boolean(rawSetting.rpm_enabled),
  };
};

export const normalizeModelSettings = (
  modelSettings = {},
  enabledModels = [],
  globalConfig = defaultImageGenerationInputs,
) => {
  const normalized = {};

  if (modelSettings && typeof modelSettings === 'object') {
    Object.entries(modelSettings).forEach(([modelId, rawSetting]) => {
      const id = (modelId || '').trim();
      if (!id || !rawSetting || typeof rawSetting !== 'object') {
        return;
      }
      normalized[id] = createDefaultModelSetting(id, rawSetting, globalConfig);
    });
  }

  uniqStrings(enabledModels).forEach((modelId) => {
    if (normalized[modelId]) {
      return;
    }
    normalized[modelId] = createDefaultModelSetting(modelId, {}, globalConfig);
  });

  const defaultModel = (globalConfig.default_model || '').trim();
  if (defaultModel && !normalized[defaultModel]) {
    normalized[defaultModel] = createDefaultModelSetting(
      defaultModel,
      {},
      globalConfig,
    );
  }

  return normalized;
};

export const deriveLegacyFieldsFromModelSettings = (
  inputs = defaultImageGenerationInputs,
) => {
  const modelSettings = normalizeModelSettings(
    inputs.model_settings,
    inputs.enabled_models,
    inputs,
  );
  const enabledModels = Object.keys(modelSettings);
  const defaultModel =
    enabledModels.includes(inputs.default_model)
      ? inputs.default_model
      : enabledModels[0] || '';
  const defaultModelSetting = defaultModel ? modelSettings[defaultModel] : null;

  return {
    ...inputs,
    model_settings: modelSettings,
    enabled_models: enabledModels,
    default_model: defaultModel,
    default_resolution:
      (inputs.default_resolution || '').trim() ||
      defaultModelSetting?.default_resolution ||
      defaultImageGenerationInputs.default_resolution,
    default_aspect_ratio:
      (inputs.default_aspect_ratio || '').trim() ||
      defaultModelSetting?.default_aspect_ratio ||
      defaultImageGenerationInputs.default_aspect_ratio,
    max_image_count: ensurePositiveInt(
      inputs.max_image_count,
      defaultModelSetting?.max_image_count || defaultImageGenerationInputs.max_image_count,
    ),
    rpm_limit: ensurePositiveInt(
      inputs.rpm_limit,
      defaultModelSetting?.rpm_limit || defaultImageGenerationInputs.rpm_limit,
    ),
  };
};

export function transformToFrontend(backendData = {}) {
  const merged = { ...defaultImageGenerationInputs, ...backendData };
  const normalized = deriveLegacyFieldsFromModelSettings(merged);
  return {
    ...normalized,
    s3_enabled: normalized.storage_type === 's3',
  };
}

export function transformToBackend(frontendData = {}) {
  const withLegacy = deriveLegacyFieldsFromModelSettings(frontendData);
  const { s3_enabled, ...rest } = withLegacy;
  return {
    ...rest,
    storage_type: s3_enabled ? 's3' : 'local',
  };
}

export function pickFields(source = {}, keys = []) {
  const target = {};
  keys.forEach((key) => {
    target[key] = source[key];
  });
  return target;
}
