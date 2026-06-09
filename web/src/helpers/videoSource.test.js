import { describe, expect, test } from 'bun:test';
import { getVideoSourceCandidates, isProtectedVideoProxyURL } from './videoSource';

describe('video source helpers', () => {
  test('detects ordinary and Canvas protected proxy URLs', () => {
    expect(isProtectedVideoProxyURL('/v1/videos/task_plain/content')).toBe(true);
    expect(
      isProtectedVideoProxyURL('/api/canvas/videos/task_canvas/content'),
    ).toBe(true);
    expect(
      isProtectedVideoProxyURL(
        'https://gateway.example.com/api/canvas/videos/task_canvas/content',
      ),
    ).toBe(true);
  });

  test('keeps direct media URLs as unauthenticated candidates', () => {
    const candidates = getVideoSourceCandidates({
      result_url: 'https://cdn.example.com/video.mp4',
      video_url: '/api/canvas/videos/task_canvas/content',
    });

    expect(candidates).toHaveLength(2);
    expect(candidates[0]).toMatchObject({
      resolved: 'https://cdn.example.com/video.mp4',
      requiresAuthBlob: false,
    });
    expect(candidates[1].requiresAuthBlob).toBe(true);
  });

  test('marks ordinary and Canvas proxy candidates for authenticated blob loading', () => {
    const candidates = getVideoSourceCandidates({
      result_url: '/v1/videos/task_plain/content',
      video_url: '/api/canvas/videos/task_canvas/content',
    });

    expect(candidates).toHaveLength(2);
    expect(candidates.every((candidate) => candidate.requiresAuthBlob)).toBe(
      true,
    );
  });
});
