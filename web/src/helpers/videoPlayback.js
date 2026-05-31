import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { API } from './api';
import { authHeader } from './auth';
import { getUserIdFromLocalStorage } from './utils';
import { getVideoSourceCandidates } from './videoSource';

const buildVideoRequestHeaders = () => {
  const headers = {
    ...authHeader(),
  };
  const userId = getUserIdFromLocalStorage();
  if (userId > 0) {
    headers['New-API-User'] = String(userId);
  }
  return headers;
};

const toPlaybackError = (error, kind = 'load') => {
  const status = error?.response?.status || 0;
  if (status === 401 || status === 403) {
    return { kind: 'auth', status };
  }
  if (status === 404) {
    return { kind: 'not_found', status };
  }
  return { kind, status };
};

const initialPlaybackState = {
  src: '',
  openUrl: '',
  loading: false,
  error: null,
};

export const usePlayableVideo = (task) => {
  const candidates = useMemo(
    () => getVideoSourceCandidates(task),
    [task?.result_url, task?.video_url],
  );
  const candidatesKey = useMemo(
    () => candidates.map((candidate) => candidate.resolved).join('|'),
    [candidates],
  );
  const [candidateIndex, setCandidateIndex] = useState(0);
  const [playbackState, setPlaybackState] = useState(initialPlaybackState);
  const objectUrlRef = useRef('');
  const previousCandidatesKeyRef = useRef(candidatesKey);

  const revokeObjectURL = useCallback(() => {
    if (!objectUrlRef.current) {
      return;
    }
    URL.revokeObjectURL(objectUrlRef.current);
    objectUrlRef.current = '';
  }, []);

  useEffect(() => () => revokeObjectURL(), [revokeObjectURL]);

  useEffect(() => {
    if (previousCandidatesKeyRef.current === candidatesKey) {
      return;
    }
    previousCandidatesKeyRef.current = candidatesKey;
    revokeObjectURL();
    setCandidateIndex(0);
    setPlaybackState(initialPlaybackState);
  }, [candidatesKey, revokeObjectURL]);

  const currentCandidate = candidates[candidateIndex] || null;

  useEffect(() => {
    if (!currentCandidate) {
      revokeObjectURL();
      setPlaybackState(initialPlaybackState);
      return undefined;
    }

    let cancelled = false;
    const abortController = new AbortController();

    const prepareSource = async () => {
      revokeObjectURL();
      setPlaybackState({
        src: '',
        openUrl: '',
        loading: currentCandidate.requiresAuthBlob,
        error: null,
      });

      try {
        if (!currentCandidate.requiresAuthBlob) {
          if (!cancelled) {
            setPlaybackState({
              src: currentCandidate.resolved,
              openUrl: currentCandidate.resolved,
              loading: false,
              error: null,
            });
          }
          return;
        }

        const response = await API.get(currentCandidate.resolved, {
          responseType: 'blob',
          withCredentials: true,
          headers: buildVideoRequestHeaders(),
          skipErrorHandler: true,
          signal: abortController.signal,
        });

        if (cancelled) {
          return;
        }

        const objectUrl = URL.createObjectURL(response.data);
        objectUrlRef.current = objectUrl;
        setPlaybackState({
          src: objectUrl,
          openUrl: objectUrl,
          loading: false,
          error: null,
        });
      } catch (error) {
        if (
          cancelled ||
          error?.name === 'AbortError' ||
          error?.name === 'CanceledError'
        ) {
          return;
        }

        revokeObjectURL();
        if (candidateIndex + 1 < candidates.length) {
          setCandidateIndex((index) => index + 1);
          return;
        }

        setPlaybackState({
          src: '',
          openUrl: '',
          loading: false,
          error: toPlaybackError(error),
        });
      }
    };

    prepareSource();

    return () => {
      cancelled = true;
      abortController.abort();
    };
  }, [candidateIndex, candidates.length, currentCandidate, revokeObjectURL]);

  const handlePlaybackError = useCallback(() => {
    revokeObjectURL();
    if (candidateIndex + 1 < candidates.length) {
      setCandidateIndex((index) => index + 1);
      return;
    }
    setPlaybackState({
      src: '',
      openUrl: '',
      loading: false,
      error: toPlaybackError(null, 'playback'),
    });
  }, [candidateIndex, candidates.length, revokeObjectURL]);

  const openInNewTab = useCallback(() => {
    if (!playbackState.openUrl) {
      return false;
    }
    window.open(playbackState.openUrl, '_blank', 'noopener');
    return true;
  }, [playbackState.openUrl]);

  return {
    ...playbackState,
    hasSource: candidates.length > 0,
    onPlaybackError: handlePlaybackError,
    openInNewTab,
  };
};
