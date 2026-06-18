import test from 'node:test';
import assert from 'node:assert/strict';
import {
  getWorkerSettingEffectiveDisplayInputs,
  getWorkerStorageDisplayConfig,
  WORKER_SETTING_DEFAULTS,
  WORKER_SETTING_LOCAL_PATHS,
} from './workerSettingSchema.js';

test('worker setting local display uses readonly fixed directories', () => {
  const display = getWorkerStorageDisplayConfig({
    'worker_setting.result_storage_type': 'local',
    'worker_setting.reference_storage_type': 'local',
  });

  assert.equal(display.result.localPathReadonly, true);
  assert.equal(display.reference.localPathReadonly, true);
  assert.equal(display.result.localPath, WORKER_SETTING_LOCAL_PATHS.result);
  assert.equal(
    display.reference.localPath,
    WORKER_SETTING_LOCAL_PATHS.reference,
  );
});

test('worker setting effective display prefers synthetic effective local paths', () => {
  const display = getWorkerSettingEffectiveDisplayInputs({
    'worker_setting.result_storage_type': 'local',
    'worker_setting.reference_storage_type': 'local',
    'worker_setting.effective_result_local_storage_path': '/mnt/results',
    'worker_setting.effective_reference_local_storage_path': '/mnt/references',
  });

  assert.equal(display.resultLocalStoragePath, '/mnt/results');
  assert.equal(display.referenceLocalStoragePath, '/mnt/references');
});

test('worker setting cleanup defaults use hour-based retention values', () => {
  assert.equal(WORKER_SETTING_DEFAULTS['worker_setting.retention_days'], 720);
  assert.equal(
    WORKER_SETTING_DEFAULTS['worker_setting.reference_retention_days'],
    168,
  );
});
