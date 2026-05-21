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
  'worker_setting.result_storage_type': '',
  'worker_setting.result_local_storage_path': '',
  'worker_setting.result_s3_endpoint': '',
  'worker_setting.result_s3_bucket': '',
  'worker_setting.result_s3_region': '',
  'worker_setting.result_s3_access_key': '',
  'worker_setting.result_s3_secret_key': '',
  'worker_setting.result_s3_path_prefix': '',
  'worker_setting.result_s3_url_mode': '',
  'worker_setting.result_s3_public_base_url': '',
  'worker_setting.reference_storage_type': '',
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
  'worker_setting.inspiration_page_cache_ttl': 60,
  'worker_setting.inspiration_first_page_cache_ttl': 300,
  'worker_setting.auto_cleanup_enabled': false,
  'worker_setting.retention_days': 30,
  'worker_setting.reference_auto_cleanup_enabled': false,
  'worker_setting.reference_retention_days': 7,
  'worker_setting.max_image_size': 10,
});

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

  const baseStorageType = nextInputs['worker_setting.storage_type'];
  const baseLocalPath = nextInputs['worker_setting.local_storage_path'];
  const baseS3Endpoint = nextInputs['worker_setting.s3_endpoint'];
  const baseS3Bucket = nextInputs['worker_setting.s3_bucket'];
  const baseS3Region = nextInputs['worker_setting.s3_region'];
  const baseS3AccessKey = nextInputs['worker_setting.s3_access_key'];
  const baseS3SecretKey = nextInputs['worker_setting.s3_secret_key'];
  const baseS3PathPrefix = nextInputs['worker_setting.s3_path_prefix'];
  const baseS3URLMode = nextInputs['worker_setting.s3_url_mode'];
  const baseS3PublicBaseURL = nextInputs['worker_setting.s3_public_base_url'];

  if (!nextInputs['worker_setting.result_storage_type']) {
    nextInputs['worker_setting.result_storage_type'] = baseStorageType;
  }
  if (!nextInputs['worker_setting.result_local_storage_path']) {
    nextInputs['worker_setting.result_local_storage_path'] = baseLocalPath;
  }
  if (!nextInputs['worker_setting.result_s3_endpoint']) {
    nextInputs['worker_setting.result_s3_endpoint'] = baseS3Endpoint;
  }
  if (!nextInputs['worker_setting.result_s3_bucket']) {
    nextInputs['worker_setting.result_s3_bucket'] = baseS3Bucket;
  }
  if (!nextInputs['worker_setting.result_s3_region']) {
    nextInputs['worker_setting.result_s3_region'] = baseS3Region;
  }
  if (!nextInputs['worker_setting.result_s3_access_key']) {
    nextInputs['worker_setting.result_s3_access_key'] = baseS3AccessKey;
  }
  if (!nextInputs['worker_setting.result_s3_secret_key']) {
    nextInputs['worker_setting.result_s3_secret_key'] = baseS3SecretKey;
  }
  if (!nextInputs['worker_setting.result_s3_path_prefix']) {
    nextInputs['worker_setting.result_s3_path_prefix'] = baseS3PathPrefix;
  }
  if (!nextInputs['worker_setting.result_s3_url_mode']) {
    nextInputs['worker_setting.result_s3_url_mode'] = baseS3URLMode;
  }
  if (!nextInputs['worker_setting.result_s3_public_base_url']) {
    nextInputs['worker_setting.result_s3_public_base_url'] = baseS3PublicBaseURL;
  }

  if (!nextInputs['worker_setting.reference_storage_type']) {
    nextInputs['worker_setting.reference_storage_type'] =
      nextInputs['worker_setting.result_storage_type'];
  }
  if (!nextInputs['worker_setting.reference_local_storage_path']) {
    nextInputs['worker_setting.reference_local_storage_path'] =
      nextInputs['worker_setting.result_local_storage_path'];
  }
  if (!nextInputs['worker_setting.reference_s3_endpoint']) {
    nextInputs['worker_setting.reference_s3_endpoint'] =
      nextInputs['worker_setting.result_s3_endpoint'];
  }
  if (!nextInputs['worker_setting.reference_s3_bucket']) {
    nextInputs['worker_setting.reference_s3_bucket'] =
      nextInputs['worker_setting.result_s3_bucket'];
  }
  if (!nextInputs['worker_setting.reference_s3_region']) {
    nextInputs['worker_setting.reference_s3_region'] =
      nextInputs['worker_setting.result_s3_region'];
  }
  if (!nextInputs['worker_setting.reference_s3_access_key']) {
    nextInputs['worker_setting.reference_s3_access_key'] =
      nextInputs['worker_setting.result_s3_access_key'];
  }
  if (!nextInputs['worker_setting.reference_s3_secret_key']) {
    nextInputs['worker_setting.reference_s3_secret_key'] =
      nextInputs['worker_setting.result_s3_secret_key'];
  }
  if (!nextInputs['worker_setting.reference_s3_path_prefix']) {
    nextInputs['worker_setting.reference_s3_path_prefix'] =
      nextInputs['worker_setting.result_s3_path_prefix'];
  }
  if (!nextInputs['worker_setting.reference_s3_url_mode']) {
    nextInputs['worker_setting.reference_s3_url_mode'] =
      nextInputs['worker_setting.result_s3_url_mode'];
  }
  if (!nextInputs['worker_setting.reference_s3_public_base_url']) {
    nextInputs['worker_setting.reference_s3_public_base_url'] =
      nextInputs['worker_setting.result_s3_public_base_url'];
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
