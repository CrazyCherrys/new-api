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

import { toBoolean } from '../../../helpers';

export const WORKER_SETTING_DEFAULTS = Object.freeze({
  'worker_setting.max_workers': 4,
  'worker_setting.user_custom_key_enabled': false,
  'worker_setting.user_custom_base_url_allowed': false,
  'worker_setting.storage_type': 'local',
  'worker_setting.local_storage_path': '',
  'worker_setting.s3_endpoint': '',
  'worker_setting.s3_bucket': '',
  'worker_setting.s3_region': '',
  'worker_setting.s3_access_key': '',
  'worker_setting.s3_secret_key': '',
  'worker_setting.s3_path_prefix': '',
  'worker_setting.s3_url_mode': 'direct',
  'worker_setting.s3_public_base_url': '',
  'worker_setting.result_storage_type': 'local',
  'worker_setting.result_local_storage_path': '',
  'worker_setting.result_s3_endpoint': '',
  'worker_setting.result_s3_bucket': '',
  'worker_setting.result_s3_region': '',
  'worker_setting.result_s3_access_key': '',
  'worker_setting.result_s3_secret_key': '',
  'worker_setting.result_s3_path_prefix': '',
  'worker_setting.result_s3_url_mode': '',
  'worker_setting.result_s3_public_base_url': '',
  'worker_setting.reference_storage_type': 'local',
  'worker_setting.reference_local_storage_path': '',
  'worker_setting.reference_s3_endpoint': '',
  'worker_setting.reference_s3_bucket': '',
  'worker_setting.reference_s3_region': '',
  'worker_setting.reference_s3_access_key': '',
  'worker_setting.reference_s3_secret_key': '',
  'worker_setting.reference_s3_path_prefix': '',
  'worker_setting.reference_s3_url_mode': '',
  'worker_setting.reference_s3_public_base_url': '',
  'worker_setting.image_timeout': 120,
  'worker_setting.video_timeout': 600,
  'worker_setting.retry_delay': 5,
  'worker_setting.max_retries': 3,
  'worker_setting.polling_interval': 5,
  'worker_setting.inspiration_page_cache_ttl': 300,
  'worker_setting.inspiration_first_page_cache_ttl': 900,
  'worker_setting.auto_cleanup_enabled': false,
  'worker_setting.retention_days': 30,
  'worker_setting.reference_auto_cleanup_enabled': false,
  'worker_setting.reference_retention_days': 7,
  'worker_setting.max_image_size': 10,
});

export function getWorkerSettingEffectiveDisplayInputs(rawOptions = {}) {
  const nextInputs = normalizeWorkerSettingInputs(rawOptions);

  const resultStorageType =
    nextInputs['worker_setting.result_storage_type'] ||
    nextInputs['worker_setting.storage_type'] ||
    'local';
  const referenceStorageType =
    nextInputs['worker_setting.reference_storage_type'] ||
    nextInputs['worker_setting.result_storage_type'] ||
    nextInputs['worker_setting.storage_type'] ||
    'local';
  const resultLocalStoragePath =
    nextInputs['worker_setting.result_local_storage_path'] ||
    nextInputs['worker_setting.local_storage_path'] ||
    '';
  const resultS3Endpoint =
    nextInputs['worker_setting.result_s3_endpoint'] ||
    nextInputs['worker_setting.s3_endpoint'] ||
    '';
  const resultS3Bucket =
    nextInputs['worker_setting.result_s3_bucket'] ||
    nextInputs['worker_setting.s3_bucket'] ||
    '';
  const resultS3Region =
    nextInputs['worker_setting.result_s3_region'] ||
    nextInputs['worker_setting.s3_region'] ||
    '';
  const resultS3PathPrefix =
    nextInputs['worker_setting.result_s3_path_prefix'] ||
    nextInputs['worker_setting.s3_path_prefix'] ||
    '';
  const resultS3URLMode =
    nextInputs['worker_setting.result_s3_url_mode'] ||
    nextInputs['worker_setting.s3_url_mode'] ||
    'direct';
  const resultS3PublicBaseURL =
    nextInputs['worker_setting.result_s3_public_base_url'] ||
    nextInputs['worker_setting.s3_public_base_url'] ||
    '';
  const referenceLocalStoragePath =
    nextInputs['worker_setting.reference_local_storage_path'] ||
    nextInputs['worker_setting.result_local_storage_path'] ||
    nextInputs['worker_setting.local_storage_path'] ||
    '';
  const referenceS3Endpoint =
    nextInputs['worker_setting.reference_s3_endpoint'] ||
    nextInputs['worker_setting.result_s3_endpoint'] ||
    nextInputs['worker_setting.s3_endpoint'] ||
    '';
  const referenceS3Bucket =
    nextInputs['worker_setting.reference_s3_bucket'] ||
    nextInputs['worker_setting.result_s3_bucket'] ||
    nextInputs['worker_setting.s3_bucket'] ||
    '';
  const referenceS3Region =
    nextInputs['worker_setting.reference_s3_region'] ||
    nextInputs['worker_setting.result_s3_region'] ||
    nextInputs['worker_setting.s3_region'] ||
    '';
  const referenceS3PathPrefix =
    nextInputs['worker_setting.reference_s3_path_prefix'] ||
    nextInputs['worker_setting.result_s3_path_prefix'] ||
    nextInputs['worker_setting.s3_path_prefix'] ||
    '';
  const referenceS3URLMode =
    nextInputs['worker_setting.reference_s3_url_mode'] ||
    nextInputs['worker_setting.result_s3_url_mode'] ||
    nextInputs['worker_setting.s3_url_mode'] ||
    'direct';
  const referenceS3PublicBaseURL =
    nextInputs['worker_setting.reference_s3_public_base_url'] ||
    nextInputs['worker_setting.result_s3_public_base_url'] ||
    nextInputs['worker_setting.s3_public_base_url'] ||
    '';

  return {
    resultStorageType,
    referenceStorageType,
    resultLocalStoragePath,
    resultS3Endpoint,
    resultS3Bucket,
    resultS3Region,
    resultS3PathPrefix,
    resultS3URLMode,
    resultS3PublicBaseURL,
    referenceLocalStoragePath,
    referenceS3Endpoint,
    referenceS3Bucket,
    referenceS3Region,
    referenceS3PathPrefix,
    referenceS3URLMode,
    referenceS3PublicBaseURL,
  };
}

function normalizeWorkerSettingValue(key, value) {
  const defaultValue = WORKER_SETTING_DEFAULTS[key];
  if (typeof defaultValue === 'boolean') {
    return toBoolean(value);
  }
  if (typeof defaultValue === 'number') {
    const parsedValue = parseInt(value, 10);
    return Number.isNaN(parsedValue) ? defaultValue : parsedValue;
  }
  return value;
}

export function normalizeWorkerSettingInputs(rawOptions = {}) {
  const nextInputs = { ...WORKER_SETTING_DEFAULTS };
  Object.keys(WORKER_SETTING_DEFAULTS).forEach((key) => {
    if (!Object.prototype.hasOwnProperty.call(rawOptions, key)) {
      return;
    }
    nextInputs[key] = normalizeWorkerSettingValue(key, rawOptions[key]);
  });

  const hasExplicitResultStorageType =
    Object.prototype.hasOwnProperty.call(
      rawOptions,
      'worker_setting.result_storage_type',
    ) &&
    String(rawOptions['worker_setting.result_storage_type'] || '').trim() !== '';
  const hasExplicitReferenceStorageType =
    Object.prototype.hasOwnProperty.call(
      rawOptions,
      'worker_setting.reference_storage_type',
    ) &&
    String(rawOptions['worker_setting.reference_storage_type'] || '').trim() !==
      '';

  if (!hasExplicitResultStorageType) {
    nextInputs['worker_setting.result_storage_type'] =
      nextInputs['worker_setting.storage_type'] || 'local';
  }
  if (!hasExplicitReferenceStorageType) {
    nextInputs['worker_setting.reference_storage_type'] =
      nextInputs['worker_setting.result_storage_type'] ||
      nextInputs['worker_setting.storage_type'] ||
      'local';
  }

  return nextInputs;
}

export function buildWorkerSettingInputsFromOptionList(optionItems = []) {
  const rawOptions = {};
  optionItems.forEach((item) => {
    if (!item?.key) {
      return;
    }
    rawOptions[item.key] = item.value;
  });
  return normalizeWorkerSettingInputs(rawOptions);
}
