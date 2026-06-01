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
import { getLobeHubIcon } from './render';

const MODEL_SERIES_DEFINITIONS = [
  {
    key: 'openai',
    aliases: ['openai', 'dalle'],
    displayName: 'OpenAI / DALL-E',
    shortLabel: 'OA',
    background:
      'linear-gradient(135deg, rgba(16, 163, 127, 0.96), rgba(5, 122, 85, 0.96))',
    color: '#f8fafc',
  },
  {
    key: 'anthropic',
    aliases: ['anthropic', 'claude'],
    displayName: 'Anthropic / Claude',
    shortLabel: 'CL',
    background:
      'linear-gradient(135deg, rgba(146, 99, 82, 0.96), rgba(107, 68, 54, 0.96))',
    color: '#f8fafc',
  },
  {
    key: 'google',
    aliases: ['google', 'gemini'],
    displayName: 'Google / Gemini',
    shortLabel: 'GM',
    background:
      'linear-gradient(135deg, rgba(59, 130, 246, 0.96), rgba(34, 197, 94, 0.92))',
    color: '#f8fafc',
  },
  {
    key: 'zhipu',
    aliases: ['zhipu', 'glm', 'chatglm'],
    displayName: 'Zhipu / GLM',
    shortLabel: 'GLM',
    background:
      'linear-gradient(135deg, rgba(14, 165, 233, 0.96), rgba(8, 145, 178, 0.96))',
    color: '#f8fafc',
  },
  {
    key: 'baidu',
    aliases: ['baidu', 'wenxin', 'ernie'],
    displayName: 'Baidu / ERNIE',
    shortLabel: 'WX',
    background:
      'linear-gradient(135deg, rgba(37, 99, 235, 0.96), rgba(29, 78, 216, 0.96))',
    color: '#f8fafc',
  },
  {
    key: 'alibaba',
    aliases: ['alibaba', 'tongyi', 'qwen'],
    displayName: 'Alibaba / Qwen',
    shortLabel: 'QW',
    background:
      'linear-gradient(135deg, rgba(249, 115, 22, 0.96), rgba(234, 88, 12, 0.96))',
    color: '#fff7ed',
  },
  {
    key: 'tencent',
    aliases: ['tencent', 'hunyuan'],
    displayName: 'Tencent / Hunyuan',
    shortLabel: 'HY',
    background:
      'linear-gradient(135deg, rgba(6, 182, 212, 0.96), rgba(14, 116, 144, 0.96))',
    color: '#ecfeff',
  },
  {
    key: 'xunfei',
    aliases: ['xunfei', 'spark'],
    displayName: 'iFlytek / Spark',
    shortLabel: 'SP',
    background:
      'linear-gradient(135deg, rgba(251, 146, 60, 0.96), rgba(245, 158, 11, 0.96))',
    color: '#fff7ed',
  },
  {
    key: 'deepseek',
    aliases: ['deepseek'],
    displayName: 'DeepSeek',
    shortLabel: 'DS',
    background:
      'linear-gradient(135deg, rgba(99, 102, 241, 0.96), rgba(67, 56, 202, 0.96))',
    color: '#eef2ff',
  },
  {
    key: 'minimax',
    aliases: ['minimax'],
    displayName: 'MiniMax',
    shortLabel: 'MM',
    background:
      'linear-gradient(135deg, rgba(236, 72, 153, 0.96), rgba(190, 24, 93, 0.96))',
    color: '#fdf2f8',
  },
  {
    key: 'moonshot',
    aliases: ['moonshot'],
    displayName: 'Moonshot',
    shortLabel: 'MS',
    background:
      'linear-gradient(135deg, rgba(168, 85, 247, 0.96), rgba(124, 58, 237, 0.96))',
    color: '#faf5ff',
  },
  {
    key: 'doubao',
    aliases: ['doubao'],
    displayName: 'Doubao',
    shortLabel: 'DB',
    background:
      'linear-gradient(135deg, rgba(239, 68, 68, 0.96), rgba(220, 38, 38, 0.96))',
    color: '#fef2f2',
  },
  {
    key: 'cohere',
    aliases: ['cohere'],
    displayName: 'Cohere',
    shortLabel: 'CO',
    background:
      'linear-gradient(135deg, rgba(251, 113, 133, 0.96), rgba(225, 29, 72, 0.96))',
    color: '#fff1f2',
  },
  {
    key: 'mistral',
    aliases: ['mistral'],
    displayName: 'Mistral',
    shortLabel: 'MT',
    background:
      'linear-gradient(135deg, rgba(251, 146, 60, 0.96), rgba(194, 65, 12, 0.96))',
    color: '#fff7ed',
  },
  {
    key: 'yi',
    aliases: ['yi'],
    displayName: 'Yi',
    shortLabel: 'YI',
    background:
      'linear-gradient(135deg, rgba(234, 179, 8, 0.96), rgba(202, 138, 4, 0.96))',
    color: '#fefce8',
  },
];

const MODEL_SERIES_BY_KEY = MODEL_SERIES_DEFINITIONS.reduce((map, definition) => {
  map[definition.key] = definition;
  return map;
}, {});

const MODEL_SERIES_LOBEHUB_ICON_MAP = {
  openai: 'OpenAI',
  anthropic: 'Claude.Color',
  google: 'Gemini.Color',
  zhipu: 'Zhipu.Color',
  baidu: 'Wenxin.Color',
  alibaba: 'Qwen.Color',
  tencent: 'Hunyuan.Color',
  xunfei: 'Spark.Color',
  deepseek: 'DeepSeek.Color',
  minimax: 'Minimax.Color',
  moonshot: 'Moonshot',
  doubao: 'Doubao.Color',
  cohere: 'Cohere.Color',
  mistral: 'Mistral.Color',
  yi: 'Yi.Color',
};

const normalizeSeriesToken = (value) =>
  String(value || '')
    .trim()
    .toLowerCase()
    .replace(/[|,]+/g, '/');

const MODEL_SERIES_ALIAS_MAP = MODEL_SERIES_DEFINITIONS.reduce((map, definition) => {
  definition.aliases.forEach((alias) => {
    map[normalizeSeriesToken(alias)] = definition.key;
  });
  return map;
}, {});

const tokenizeSeriesValue = (value) => {
  const raw = normalizeSeriesToken(value);
  if (!raw) {
    return [];
  }
  const segments = raw
    .split(/[\/\s]+/)
    .map((segment) => segment.trim())
    .filter(Boolean);
  return Array.from(new Set([raw, ...segments]));
};

const humanizeSeriesValue = (value) =>
  String(value || '')
    .trim()
    .split(/[\/_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' / ');

const buildFallbackShortLabel = (value) => {
  const segments = String(value || '')
    .trim()
    .split(/[\/_-]+/)
    .filter(Boolean);
  if (segments.length >= 2) {
    return `${segments[0][0]}${segments[1][0]}`.toUpperCase();
  }
  if (segments.length === 1 && segments[0]) {
    return segments[0].slice(0, 2).toUpperCase();
  }
  return '?';
};

export const normalizeModelSeriesAlias = (value) => {
  const candidates = tokenizeSeriesValue(value);
  for (const candidate of candidates) {
    const matched = MODEL_SERIES_ALIAS_MAP[candidate];
    if (matched) {
      return matched;
    }
  }
  return '';
};

export const canonicalizeModelSeriesValue = (value) => {
  const normalized = normalizeModelSeriesAlias(value);
  if (normalized) {
    return normalized;
  }
  return String(value || '').trim();
};

export const getModelSeriesMeta = (value) => {
  const rawValue = String(value || '').trim();
  const normalized = normalizeModelSeriesAlias(rawValue);
  const definition = normalized ? MODEL_SERIES_BY_KEY[normalized] : null;
  const displayName = definition?.displayName || humanizeSeriesValue(rawValue);
  return {
    rawValue,
    key: normalized || rawValue,
    normalizedKey: normalized,
    displayName: displayName || 'Unknown',
    shortLabel: definition?.shortLabel || buildFallbackShortLabel(rawValue),
    background:
      definition?.background ||
      'linear-gradient(135deg, rgba(71, 85, 105, 0.94), rgba(51, 65, 85, 0.94))',
    color: definition?.color || '#f8fafc',
    isKnown: !!definition,
  };
};

export const getModelSeriesLobeHubIconName = (value) => {
  const normalized = normalizeModelSeriesAlias(value);
  if (!normalized) {
    return '';
  }
  return MODEL_SERIES_LOBEHUB_ICON_MAP[normalized] || '';
};

export const formatModelSeriesLabel = (value, fallback = '-') => {
  const meta = getModelSeriesMeta(value);
  return meta.rawValue || meta.isKnown ? meta.displayName : fallback;
};

export const getModelSeriesOptionList = () =>
  MODEL_SERIES_DEFINITIONS.map((definition) => ({
    value: definition.key,
    label: definition.displayName,
  }));

const MODEL_SERIES_ICON_SIZES = {
  small: { size: 18, graphicSize: 18, fontSize: 9, borderRadius: 6 },
  medium: { size: 22, graphicSize: 20, fontSize: 10, borderRadius: 7 },
  large: { size: 26, graphicSize: 24, fontSize: 11, borderRadius: 8 },
};

const getModelSeriesIconSizeConfig = (size) =>
  MODEL_SERIES_ICON_SIZES[size] || MODEL_SERIES_ICON_SIZES.medium;

export const ModelSeriesIcon = ({
  series,
  size = 'medium',
  style = undefined,
  title = undefined,
}) => {
  const meta = getModelSeriesMeta(series);
  const { size: iconSize, fontSize, borderRadius } =
    getModelSeriesIconSizeConfig(size);

  return (
    <span
      aria-hidden='true'
      title={title || meta.displayName}
      style={{
        width: iconSize,
        height: iconSize,
        minWidth: iconSize,
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        borderRadius,
        background: meta.background,
        color: meta.color,
        fontSize,
        fontWeight: 700,
        letterSpacing: 0.2,
        lineHeight: 1,
        boxShadow: 'inset 0 0 0 1px rgba(255, 255, 255, 0.12)',
        textTransform: 'uppercase',
        ...style,
      }}
    >
      {meta.shortLabel}
    </span>
  );
};

export const CanvasModelSeriesIcon = ({
  series,
  size = 'medium',
  style = undefined,
  title = undefined,
}) => {
  const meta = getModelSeriesMeta(series);
  const iconName = getModelSeriesLobeHubIconName(series);
  if (!iconName) {
    return (
      <ModelSeriesIcon
        series={series}
        size={size}
        style={style}
        title={title || meta.displayName}
      />
    );
  }

  const { size: slotSize, graphicSize } = getModelSeriesIconSizeConfig(size);
  return (
    <span
      aria-hidden='true'
      title={title || meta.displayName}
      style={{
        width: slotSize,
        height: slotSize,
        minWidth: slotSize,
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        flex: '0 0 auto',
        lineHeight: 1,
        ...style,
      }}
    >
      {getLobeHubIcon(iconName, graphicSize || slotSize)}
    </span>
  );
};
