import { describe, expect, test } from 'bun:test';
import {
  getCanvasImageSelectorVisibility,
  getCanvasImageUiState,
} from './canvasRules';

describe('canvas image model rules', () => {
  test('image-generation-only models use prompt-only flow', () => {
    const state = getCanvasImageUiState({
      model: {
        request_endpoint: 'openai',
        image_capabilities: ['image_generation'],
      },
      prompt: 'draw a skyline',
      referenceImages: [],
      selectedGroupHasAvailableToken: true,
    });

    expect(state.supportsGeneration).toBe(true);
    expect(state.supportsEditing).toBe(false);
    expect(state.supportsMaskEditing).toBe(false);
    expect(state.requiresReferenceImage).toBe(false);
    expect(state.canGenerate).toBe(true);
  });

  test('image-editing-only models require a reference image', () => {
    const model = {
      request_endpoint: 'openai-response',
      image_capabilities: ['image_editing'],
    };

    expect(
      getCanvasImageUiState({
        model,
        prompt: 'edit this',
        referenceImages: [],
        selectedGroupHasAvailableToken: true,
      }).canGenerate,
    ).toBe(false);

    const state = getCanvasImageUiState({
      model,
      prompt: 'edit this',
      referenceImages: [{ uid: 'ref-1' }],
      selectedGroupHasAvailableToken: true,
    });
    expect(state.supportsGeneration).toBe(false);
    expect(state.supportsEditing).toBe(true);
    expect(state.supportsMaskEditing).toBe(true);
    expect(state.requiresReferenceImage).toBe(true);
    expect(state.canGenerate).toBe(true);
  });

  test('mask editing is limited to OpenAI image endpoints', () => {
    const state = getCanvasImageUiState({
      model: {
        request_endpoint: 'gemini',
        image_capabilities: ['image_editing'],
      },
      prompt: 'edit this',
      referenceImages: [{ uid: 'ref-1' }],
      selectedGroupHasAvailableToken: true,
    });

    expect(state.supportsEditing).toBe(true);
    expect(state.supportsMaskEditing).toBe(false);
    expect(state.canGenerate).toBe(true);
  });

  test('resolution and aspect controls appear only when configured', () => {
    expect(
      getCanvasImageSelectorVisibility({
        model: {
          request_endpoint: 'openai',
        },
        aspectRatios: '["1:1","16:9"]',
        resolutions: '["1K","2K"]',
      }),
    ).toEqual({
      showImageAspectRatioSelector: true,
      showImageResolutionSelector: false,
    });

    expect(
      getCanvasImageSelectorVisibility({
        model: {
          request_endpoint: 'gemini',
        },
        aspectRatios: [],
        resolutions: ['1K', '2K'],
      }),
    ).toEqual({
      showImageAspectRatioSelector: false,
      showImageResolutionSelector: true,
    });
  });

  test('configured selectors must have selected values before generation', () => {
    const base = {
      model: {
        request_endpoint: 'openai',
        image_capabilities: ['image_generation'],
      },
      prompt: 'draw a skyline',
      selectedGroupHasAvailableToken: true,
      showImageAspectRatioSelector: true,
      showImageResolutionSelector: true,
    };

    expect(getCanvasImageUiState(base).canGenerate).toBe(false);
    expect(
      getCanvasImageUiState({
        ...base,
        aspectRatio: '16:9',
        resolution: '2K',
      }).canGenerate,
    ).toBe(true);
  });
});
