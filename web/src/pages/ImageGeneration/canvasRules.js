const IMAGE_CAPABILITY_GENERATION = 'image_generation';
const IMAGE_CAPABILITY_EDITING = 'image_editing';
const OPENAI_MASK_EDIT_ENDPOINTS = new Set(['openai', 'openai-response']);

const normalizeRequestEndpoint = (endpoint) =>
  String(endpoint || '')
    .trim()
    .toLowerCase();

const parseJsonArray = (raw) => {
  if (Array.isArray(raw)) {
    return raw;
  }
  if (typeof raw !== 'string' || raw.trim() === '') {
    return [];
  }
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch (error) {
    return [];
  }
};

const normalizeImageCapabilities = (raw) =>
  parseJsonArray(raw)
    .map((item) => String(item || '').trim().toLowerCase())
    .filter(Boolean)
    .filter((value, index, array) => array.indexOf(value) === index);

export const getReferenceImageLimit = (model) => {
  const limit = Number(model?.reference_image_limit) || 0;
  return limit > 0 ? Math.floor(limit) : 0;
};

export const modelSupportsCapability = (model, capability) =>
  !!model &&
  normalizeImageCapabilities(model.image_capabilities).includes(capability);

export const modelSupportsImageGeneration = (model) =>
  modelSupportsCapability(model, IMAGE_CAPABILITY_GENERATION);

export const modelSupportsImageEditing = (model) =>
  modelSupportsCapability(model, IMAGE_CAPABILITY_EDITING);

export const modelSupportsMaskEditing = (model) =>
  modelSupportsImageEditing(model) &&
  OPENAI_MASK_EDIT_ENDPOINTS.has(normalizeRequestEndpoint(model?.request_endpoint));

export const getCanvasImageSelectorVisibility = ({
  aspectRatios,
  resolutions,
} = {}) => ({
  showImageAspectRatioSelector: parseJsonArray(aspectRatios).length > 0,
  showImageResolutionSelector: parseJsonArray(resolutions).length > 0,
});

export const getCanvasImageUiState = ({
  model,
  prompt,
  referenceImages = [],
  selectedGroupHasAvailableToken = false,
  showImageAspectRatioSelector = false,
  showImageResolutionSelector = false,
  aspectRatio = '',
  resolution = '',
} = {}) => {
  const supportsGeneration = modelSupportsImageGeneration(model);
  const supportsEditing = modelSupportsImageEditing(model);
  const supportsMaskEditing = modelSupportsMaskEditing(model);
  const requiresReferenceImage = supportsEditing && !supportsGeneration;
  const canGenerate =
    !!model &&
    !!String(prompt || '').trim() &&
    (!requiresReferenceImage || referenceImages.length > 0) &&
    (!showImageAspectRatioSelector || !!String(aspectRatio || '').trim()) &&
    (!showImageResolutionSelector || !!String(resolution || '').trim()) &&
    !!selectedGroupHasAvailableToken;

  return {
    supportsGeneration,
    supportsEditing,
    supportsMaskEditing,
    requiresReferenceImage,
    canGenerate,
  };
};
