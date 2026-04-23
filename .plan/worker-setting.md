# Worker 设置配置页面持久化 — 实施计划

## 目标
在系统设置中新增 "Worker 设置" Tab 页，放在绘图设置右边。所有配置通过现有的 `Option` KV 存储机制持久化到数据库，使用分层配置模式（`worker_setting.*`）。

## 设计原则
- **模块化隔离**：所有新代码放在独立目录/文件中，避免修改上游核心文件（仅需在注册点做最小插入）
- **复用现有模式**：完全复制 `performance_setting` 的分层配置模式（struct + `config.GlobalConfig.Register` + `init()`），前端复制 `PerformanceSetting` 组件模式
- **合并友好**：对上游文件的修改仅限于 import 行和注册/插入行，最大限度降低合并冲突概率

## 配置项定义

| 配置 Key（DB 存储） | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `worker_setting.max_workers` | int | `4` | 最大并发 Worker 数量 |
| `worker_setting.storage_type` | string | `"local"` | 存储类型：`local` / `s3` |
| `worker_setting.local_storage_path` | string | `""` | 本地存储路径（空=系统临时目录） |
| `worker_setting.s3_endpoint` | string | `""` | S3 端点地址 |
| `worker_setting.s3_bucket` | string | `""` | S3 桶名 |
| `worker_setting.s3_region` | string | `""` | S3 区域 |
| `worker_setting.s3_access_key` | string | `""` | S3 Access Key |
| `worker_setting.s3_secret_key` | string | `""` | S3 Secret Key |
| `worker_setting.s3_path_prefix` | string | `""` | S3 对象路径前缀 |
| `worker_setting.image_timeout` | int | `120` | 图片任务超时时间（秒） |
| `worker_setting.video_timeout` | int | `600` | 视频任务超时时间（秒） |
| `worker_setting.retry_delay` | int | `5` | 图片生成失败后重试间隔（秒） |
| `worker_setting.max_retries` | int | `3` | 图片生成最大重试次数 |

---

## 实施步骤

### Step 1：后端 — 新建 `setting/worker_setting/config.go`

**新建文件**：`setting/worker_setting/config.go`

- 定义 `WorkerSetting` struct，字段对应上表，使用 `json` tag
- 定义包级变量 `workerSetting` 带默认值
- `init()` 中调用 `config.GlobalConfig.Register("worker_setting", &workerSetting)` 注册
- 提供 `GetWorkerSetting() *WorkerSetting` 访问函数

**参照**：`setting/performance_setting/config.go` 的完整模式

### Step 2：后端 — 在 `model/option.go` 中注册 import

**修改文件**：`model/option.go`

- 在 import 块新增 `_ "github.com/QuantumNous/new-api/setting/worker_setting"` 匿名导入
- 这确保 `init()` 被执行，配置注册到 `GlobalConfig`

> 由于使用分层配置模式 (`worker_setting.*`)，`handleConfigUpdate()` 会自动处理更新，无需在 `updateOptionMap` 的 switch 中添加任何 case。InitOptionMap 中 `ExportAllConfigs()` 也会自动导出所有注册配置。

**改动量**：仅 1 行 import

### Step 3：前端 — 新建 Worker 设置表单组件

**新建文件**：`web/src/pages/Setting/Worker/SettingsWorker.jsx`

- 使用 Semi Design Form 组件
- 分 4 个 Section：
  1. **Worker 并发设置**：`max_workers` (InputNumber, min=1, max=64)
  2. **存储设置**：`storage_type` (Select: local/s3)，根据选择条件展示：
     - local → `local_storage_path` (Input)
     - s3 → `s3_endpoint`, `s3_bucket`, `s3_region`, `s3_access_key`, `s3_secret_key`, `s3_path_prefix`
  3. **超时设置**：`image_timeout`, `video_timeout` (InputNumber, 单位秒)
  4. **重试设置**：`retry_delay`, `max_retries` (InputNumber)
- 提交逻辑：`compareObjects` + `API.put('/api/option/')` 批量更新，完全复用现有模式
- 使用 `useTranslation()` 的 `t()` 函数包裹所有中文文本

**参照**：`web/src/pages/Setting/Performance/SettingsPerformance.jsx` 的模式

### Step 4：前端 — 新建 Worker 设置容器组件

**新建文件**：`web/src/components/settings/WorkerSetting.jsx`

- 容器组件负责加载 options 数据、传递给子组件
- 完全复制 `PerformanceSetting.jsx` 的模式：
  - 定义 inputs state（key 为 `worker_setting.*`）
  - `getOptions()` 从 `/api/option/` 获取
  - `onRefresh()` 刷新
  - 渲染 `<Card><SettingsWorker options={inputs} refresh={onRefresh} /></Card>`

### Step 5：前端 — 在设置页面注册 Tab

**修改文件**：`web/src/pages/Setting/index.jsx`

- import `WorkerSetting` 组件
- import lucide-react 的 `Cpu` 图标（语义：Worker = 处理单元）
- 在 `drawing` Tab 后面（即 `panes.push` drawing 之后）插入：
  ```jsx
  panes.push({
    tab: (<span style={{...}}><Cpu size={18} />{t('Worker 设置')}</span>),
    content: <WorkerSetting />,
    itemKey: 'worker',
  });
  ```

**改动量**：约 10 行

### Step 6：前端 — i18n 翻译

**修改文件**：`web/src/i18n/locales/en.json`

- 添加所有新中文 key 的英文翻译（约 20 条）

**注**：中文 key 在 `t()` 中直接使用中文即作为 zh fallback，无需修改 `zh.json`

---

## 文件变更清单

| 操作 | 文件路径 | 说明 |
|---|---|---|
| **新建** | `setting/worker_setting/config.go` | 后端配置定义 + 注册 |
| **修改** (1行) | `model/option.go` | 匿名 import 触发 init |
| **新建** | `web/src/pages/Setting/Worker/SettingsWorker.jsx` | 表单子组件 |
| **新建** | `web/src/components/settings/WorkerSetting.jsx` | 容器组件 |
| **修改** (~10行) | `web/src/pages/Setting/index.jsx` | 注册 Tab |
| **修改** (~20行) | `web/src/i18n/locales/en.json` | 英文翻译 |

## 合并风险评估

- `model/option.go`：仅新增 1 行 import，合并冲突概率极低
- `web/src/pages/Setting/index.jsx`：仅新增 import + 1 个 panes.push，中等冲突概率但易解决
- `en.json`：新增 key 行，按字母排序插入，低冲突概率
- 其他均为**新建文件**，零冲突
