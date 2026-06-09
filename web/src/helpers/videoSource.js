const ABSOLUTE_URL_RE = /^[a-z][a-z0-9+.-]*:/i;
const VIDEO_PROXY_PATH_RE =
  /^(?:\/v1\/videos|\/api\/canvas\/videos)\/[^/]+\/content$/i;

const normalizeUrl = (value) => String(value || '').trim();

export const getVideoMediaBaseURL = () => {
  const configured = normalizeUrl(import.meta.env.VITE_REACT_APP_SERVER_URL);
  if (configured) {
    return configured;
  }
  if (typeof window !== 'undefined' && window.location?.origin) {
    return window.location.origin;
  }
  return '';
};

export const resolveVideoSourceURL = (value) => {
  const raw = normalizeUrl(value);
  if (!raw) {
    return '';
  }
  if (
    raw.startsWith('blob:') ||
    raw.startsWith('data:') ||
    ABSOLUTE_URL_RE.test(raw)
  ) {
    return raw;
  }

  const baseURL = getVideoMediaBaseURL();
  if (!baseURL) {
    return raw;
  }

  try {
    return new URL(raw, baseURL).toString();
  } catch (error) {
    console.error('Failed to resolve video source URL:', error);
    return raw;
  }
};

const getResolvedPathname = (value) => {
  const resolved = resolveVideoSourceURL(value);
  if (!resolved) {
    return '';
  }
  try {
    return new URL(resolved).pathname;
  } catch (error) {
    return resolved.startsWith('/') ? resolved : '';
  }
};

export const isProtectedVideoProxyURL = (value) =>
  VIDEO_PROXY_PATH_RE.test(getResolvedPathname(value));

export const getVideoSourceCandidates = (task) => {
  const seen = new Set();

  return [task?.result_url, task?.video_url]
    .map(normalizeUrl)
    .filter(Boolean)
    .map((raw) => {
      const resolved = resolveVideoSourceURL(raw);
      return {
        raw,
        resolved,
        requiresAuthBlob: isProtectedVideoProxyURL(resolved),
      };
    })
    .filter(({ resolved }) => {
      if (!resolved || seen.has(resolved)) {
        return false;
      }
      seen.add(resolved);
      return true;
    });
};
