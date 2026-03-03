# Team Research: DreamStudio AI 图像生成功能集成

## 增强后的需求

**项目目标：** 将 DreamStudio 的完整 AI 图像生成功能集成到 new-api 项目中

**核心功能范围：**
1. **异步任务系统**：用户提交生图请求后立即返回任务ID，后台 Worker 自动处理
2. **智能重试机制**：失败任务自动重试，支持指数退避和可重试错误识别
3. **灵活存储系统**：支持本地文件系统和 S3 兼容对象存储
4. **模型限流（RPM）**：单模型请求频率限制，防止滥用
5. **多平台支持**：OpenAI、Gemini、自定义端点

**技术约束：**
- 数据库：必须同时兼容 SQLite、MySQL ≥5.7.8、PostgreSQL ≥9.6
- JSON 处理：必须使用 `common/json.go` 包装函数
- 前端技术栈：React 18 + Semi Design + Bun（需从 Vue 转换）
- 后端架构：复用 new-api 的 relay 系统、Setting 系统、Channel 系统
- 数据模型：创建独立的 ImageTask 模型（不扩展现有 Task）

**实现策略：**
- **后端**：参考 DreamStudio 的服务层设计，适配 new-api 的架构模式
- **前端**：核心生图界面参考 DreamStudio 的 UI/UX，用 React + Semi Design 重写；管理界面使用 new-api 现有风格
- **管理功能**：完整的配置界面（存储、生成参数、模型管理、RPM限制）

---

## 约束集

### 硬约束

#### 后端约束

- **[HC-1]** 数据库兼容性：所有数据库操作必须同时兼容 SQLite/MySQL/PostgreSQL
  - 来源：new-api CLAUDE.md Rule 2
  - 影响：不能使用 PostgreSQL 特有的 JSONB 操作符、MySQL 的 GROUP_CONCAT
  - 解决：使用 GORM 抽象层，避免原生 SQL

- **[HC-2]** JSON 处理：必须使用 `common/json.go` 包装函数
  - 来源：new-api CLAUDE.md Rule 1
  - 影响：所有 JSON marshal/unmarshal 操作
  - 函数：`common.Marshal()`, `common.Unmarshal()`, `common.UnmarshalJsonStr()`

- **[HC-3]** 数据模型设计：创建独立的 ImageTask 模型，不扩展现有 Task
  - 来源：用户需求确认
  - 原因：Task 模型已用于 Midjourney/Suno，职责分离
  - 表名：`image_generation_tasks`

- **[HC-4]** 架构集成：必须复用 new-api 的现有系统
  - relay 系统：用于 API 转发和渠道选择
  - Setting 系统：用于配置管理
  - Channel 系统：用于渠道管理和负载均衡
  - 来源：new-api 架构约束

- **[HC-5]** Worker 并发安全：任务获取必须使用原子操作
  - 来源：DreamStudio 实现模式
  - 方案：`FOR UPDATE SKIP LOCKED` 或 CAS（Compare-And-Swap）
  - 参考：new-api 的 `model/task_cas_test.go`

- **[HC-6]** 数据库字段引用：保留字段需使用变量
  - 来源：new-api CLAUDE.md Rule 2
  - 示例：使用 `commonGroupCol`, `commonKeyCol` 而非直接引用 `group`, `key`

#### 前端约束

- **[HC-7]** 技术栈转换：Vue 3 Composition API → React 18 Hooks
  - 来源：Gemini 前端探索
  - 挑战：响应式系统差异、生命周期映射
  - 映射：`ref/reactive` → `useState`, `computed` → `useMemo`, `watch` → `useEffect`

- **[HC-8]** UI 组件库：必须使用 Semi Design
  - 来源：new-api 前端架构
  - 影响：DreamStudio 的自定义 Tailwind 组件需重写
  - 参考：`web/src/pages/Midjourney/index.jsx`

- **[HC-9]** 包管理器：使用 Bun
  - 来源：new-api CLAUDE.md Rule 3
  - 命令：`bun install`, `bun run dev`, `bun run build`

- **[HC-10]** 路由集成：需添加到 new-api 的路由系统
  - 来源：Gemini 前端探索
  - 文件：`web/src/components/layout/SiderBar.jsx`
  - 路径：`/image` (生图主界面), `/image/history` (历史记录)

### 软约束

- **[SC-1]** 代码风格：遵循 new-api 的现有代码风格
  - 来源：工程最佳实践
  - 参考：现有 controller/service/model 层实现

- **[SC-2]** 错误处理：使用 new-api 的错误处理模式
  - 来源：new-api 架构约束
  - 参考：`types/error.go`, `service/relay_error_handler.go`

- **[SC-3]** 日志记录：使用 new-api 的日志系统
  - 来源：new-api 架构约束
  - 参考：`logger/` 包

- **[SC-4]** 国际化：前端使用 react-i18next，后端使用 go-i18n
  - 来源：new-api CLAUDE.md
  - 前端：`web/src/i18n/locales/{lang}.json`
  - 后端：`i18n/` 目录

- **[SC-5]** 组件拆分：DreamStudio 的 HomeView.vue (1500+ 行) 需拆分
  - 来源：Gemini 前端探索
  - 建议：按功能拆分为多个 React 组件

### 依赖关系

- **[DEP-1]** ImageTaskService → ImageGenerationService
  - Worker 调用生成服务执行实际生图

- **[DEP-2]** ImageGenerationService → StorageService
  - 生成后的图片需存储

- **[DEP-3]** ImageGenerationService → ChannelSelector
  - 需从渠道系统选择可用账号

- **[DEP-4]** ImageTaskService → SettingService
  - 读取配置（超时、重试次数等）

- **[DEP-5]** 前端生图界面 → 后端 API 端点
  - `/api/v1/images/generate` (创建任务)
  - `/api/v1/images/history` (查询历史)
  - `/api/v1/images/history/:id` (任务详情)

- **[DEP-6]** 管理界面 → Setting 系统
  - 存储配置、生成参数、模型管理

### 风险

- **[RISK-1]** 数据库迁移复杂性
  - 描述：需创建新表并确保跨 DB 兼容
  - 缓解：使用 GORM AutoMigrate，编写兼容性测试
  - 参考：`model/main.go` 的迁移模式

- **[RISK-2]** Worker 启动时机
  - 描述：Worker 需在应用启动时自动启动
  - 缓解：在 `main.go` 中初始化 ImageTaskService 并调用 Start()
  - 参考：DreamStudio 的 `ProvideImageTaskWorker`

- **[RISK-3]** 前端组件复杂度
  - 描述：HomeView.vue 逻辑过于复杂（1500+ 行）
  - 缓解：拆分为多个 React 组件（Generator, History, Settings）
  - 来源：Gemini 前端探索

- **[RISK-4]** 存储系统集成
  - 描述：需实现本地存储和 S3 存储切换
  - 缓解：参考 DreamStudio 的 StorageService 设计
  - 配置：通过 Setting 系统管理

- **[RISK-5]** RPM 限流实现
  - 描述：需实现模型级别的请求频率限制
  - 缓解：使用 Redis 滑动窗口计数器
  - 参考：DreamStudio 的 ModelRPMService

- **[RISK-6]** 现有图像 API 冲突
  - 描述：new-api 已有 `/v1/images/generations` 端点用于 relay
  - 缓解：使用不同的路由前缀（如 `/api/v1/image-tasks/`）
  - 来源：Codex 后端探索

---

## 成功判据

### 后端成功判据

- **[OK-1]** 用户可通过 API 提交异步生图请求并立即获得任务ID
  - 验证：POST `/api/v1/image-tasks/generate` 返回 `task_id`

- **[OK-2]** 后台 Worker 自动处理任务，支持失败重试
  - 验证：任务状态从 `pending` → `running` → `succeeded`/`failed`
  - 验证：失败任务自动重试，最多 3 次

- **[OK-3]** 生成的图片可存储到本地或 S3
  - 验证：图片 URL 可访问
  - 验证：S3 配置生效

- **[OK-4]** 支持模型级别的 RPM 限制
  - 验证：超过限制时返回 429 错误
  - 验证：等待后可继续请求

- **[OK-5]** 数据库操作在 SQLite/MySQL/PostgreSQL 上均正常
  - 验证：运行迁移脚本无错误
  - 验证：CRUD 操作在三种数据库上均成功

### 前端成功判据

- **[OK-6]** 用户能在 new-api 侧边栏看到 "AI 绘画" 入口
  - 验证：侧边栏显示图标和文字

- **[OK-7]** 生图界面支持 Prompt 输入、参数调整、参考图上传
  - 验证：界面元素完整且功能正常

- **[OK-8]** 历史记录支持自动轮询更新任务状态
  - 验证：任务状态实时更新
  - 验证：完成后停止轮询

- **[OK-9]** 管理后台可配置生成超时、S3 存储、模型可见性
  - 验证：配置项保存成功
  - 验证：配置生效

- **[OK-10]** 前端界面与 new-api 整体风格协调
  - 验证：使用 Semi Design 组件
  - 验证：布局和交互一致

---

## 开放问题（已解决）

### Q1: 是否需要完全保留 DreamStudio 的 'Studio' 视觉风格？
**回答：** 混合方案 - 核心生图界面参考 DreamStudio 的 UI/UX 设计，管理界面使用 new-api 现有风格
**约束：** [HC-8] UI 组件库必须使用 Semi Design

### Q2: AI 视频生成功能是否同步迁移？
**回答：** 暂不迁移 - 本次仅实现图像生成功能
**约束：** 功能范围限定为图像生成

### Q3: 存储配置是否需要与 new-api 现有的存储系统合并？
**回答：** 作为 AI 图像专用配置 - 通过 Setting 系统管理，独立于其他存储配置
**约束：** [DEP-6] 管理界面 → Setting 系统

### Q4: 路由冲突如何处理？
**回答：** 使用不同的路由前缀 - new-api 的 `/v1/images/generations` 用于 relay，新功能使用 `/api/v1/image-tasks/`
**约束：** [RISK-6] 现有图像 API 冲突

### Q5: Worker 如何启动？
**回答：** 在 `main.go` 中初始化并启动 - 参考 DreamStudio 的 `ProvideImageTaskWorker` 模式
**约束：** [RISK-2] Worker 启动时机

---

## 技术实现要点

### 后端实现要点

#### 1. 数据模型设计

```go
// model/image_task.go
type ImageTask struct {
    ID             int64           `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
    CreatedAt      int64           `json:"created_at" gorm:"index"`
    UpdatedAt      int64           `json:"updated_at"`
    UserID         int             `json:"user_id" gorm:"index"`
    ModelID        string          `json:"model_id" gorm:"type:varchar(200)"`
    Prompt         string          `json:"prompt" gorm:"type:text"`
    Resolution     string          `json:"resolution" gorm:"type:varchar(32)"`
    AspectRatio    string          `json:"aspect_ratio" gorm:"type:varchar(32)"`
    ReferenceImage string          `json:"reference_image" gorm:"type:text"`
    Count          int             `json:"count" gorm:"default:1"`
    Status         string          `json:"status" gorm:"type:varchar(20);index"`
    ErrorMessage   string          `json:"error_message" gorm:"type:text"`
    ImageURLs      json.RawMessage `json:"image_urls" gorm:"type:json"` // 跨DB兼容
    Attempts       int             `json:"attempts" gorm:"default:0"`
    LastError      string          `json:"last_error" gorm:"type:text"`
    NextAttemptAt  *int64          `json:"next_attempt_at" gorm:"index"`
    CompletedAt    *int64          `json:"completed_at"`
}
```

**关键点：**
- 使用 `json.RawMessage` 存储 JSON 数据（跨 DB 兼容）
- 时间戳使用 `int64` Unix 时间（避免时区问题）
- 索引优化：`user_id`, `status`, `next_attempt_at`

#### 2. Worker 原子操作

```go
// model/image_task.go
func ClaimNextPending(db *gorm.DB) (*ImageTask, error) {
    var task ImageTask
    now := time.Now().Unix()

    // 使用 FOR UPDATE SKIP LOCKED 确保并发安全
    err := db.Transaction(func(tx *gorm.DB) error {
        return tx.Where("status = ?", "pending").
            Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
            Order("created_at ASC").
            Limit(1).
            Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
            First(&task).Error
    })

    if err != nil {
        return nil, err
    }

    // 更新状态为 running
    task.Status = "running"
    task.Attempts++
    task.UpdatedAt = time.Now().Unix()
    db.Save(&task)

    return &task, nil
}
```

#### 3. Setting 系统集成

```go
// setting/system_setting/image_generation.go
type ImageGenerationSetting struct {
    StorageS3Enabled         bool   `json:"storage_s3_enabled"`
    StorageS3Endpoint        string `json:"storage_s3_endpoint"`
    StorageS3Bucket          string `json:"storage_s3_bucket"`
    StorageS3AccessKey       string `json:"storage_s3_access_key"`
    StorageS3SecretKey       string `json:"storage_s3_secret_key"`
    ImageTimeoutSeconds      int    `json:"image_timeout_seconds"`
    ImageMaxRetryAttempts    int    `json:"image_max_retry_attempts"`
    ImageRetryIntervalSeconds int   `json:"image_retry_interval_seconds"`
}
```

### 前端实现要点

#### 1. 组件拆分策略

```
web/src/pages/ImageGeneration/
├── index.jsx                 # 主入口
├── components/
│   ├── Generator.jsx         # 生图界面
│   ├── PromptInput.jsx       # Prompt 输入
│   ├── ParameterPanel.jsx    # 参数面板
│   ├── ReferenceUpload.jsx   # 参考图上传
│   ├── History.jsx           # 历史记录
│   ├── TaskCard.jsx          # 任务卡片
│   └── ImagePreview.jsx      # 图片预览
├── hooks/
│   ├── useImageGeneration.js # 生图逻辑
│   ├── useTaskPolling.js     # 任务轮询
│   └── useImageHistory.js    # 历史记录
└── api/
    └── imageApi.js           # API 调用
```

#### 2. Vue → React 转换映射

| Vue 3 Composition API | React 18 Hooks | 说明 |
|----------------------|----------------|------|
| `ref(value)` | `useState(value)` | 响应式状态 |
| `reactive(obj)` | `useState(obj)` | 对象状态 |
| `computed(() => ...)` | `useMemo(() => ..., [deps])` | 计算属性 |
| `watch(source, cb)` | `useEffect(() => { cb() }, [deps])` | 副作用 |
| `onMounted(cb)` | `useEffect(() => { cb() }, [])` | 挂载时执行 |
| `onUnmounted(cb)` | `useEffect(() => () => { cb() }, [])` | 卸载时执行 |

#### 3. API 调用封装

```javascript
// web/src/api/imageApi.js
import { API } from '../helpers/api';

export const imageAPI = {
  // 创建生图任务
  async createTask(payload) {
    const { data } = await API.post('/api/v1/image-tasks/generate', payload);
    return data;
  },

  // 查询历史记录
  async listHistory(params) {
    const { data } = await API.get('/api/v1/image-tasks/history', { params });
    return data;
  },

  // 获取任务详情
  async getTask(taskId) {
    const { data } = await API.get(`/api/v1/image-tasks/history/${taskId}`);
    return data;
  },

  // 删除任务
  async deleteTask(taskId) {
    await API.delete(`/api/v1/image-tasks/history/${taskId}`);
  }
};
```

---

## 实施建议

### 阶段 1: 后端核心功能（优先级：高）

1. **数据模型和迁移**
   - 创建 `model/image_task.go`
   - 编写数据库迁移脚本
   - 测试跨 DB 兼容性

2. **服务层实现**
   - `service/image_task_service.go` (Worker 和任务管理)
   - `service/image_generation_service.go` (实际生图逻辑)
   - `service/image_storage_service.go` (存储管理)

3. **Controller 和路由**
   - `controller/image_task.go`
   - `router/image-router.go`

4. **Setting 系统集成**
   - `setting/system_setting/image_generation.go`

### 阶段 2: 前端核心功能（优先级：高）

1. **生图主界面**
   - 创建 `web/src/pages/ImageGeneration/`
   - 实现 Generator 组件
   - 集成 API 调用

2. **历史记录页面**
   - 实现 History 组件
   - 任务状态轮询
   - 图片预览

3. **路由和导航**
   - 更新 `SiderBar.jsx`
   - 添加路由配置

### 阶段 3: 管理功能（优先级：中）

1. **管理界面**
   - 存储配置页面
   - 生成参数配置
   - 模型管理

2. **RPM 限流**
   - 实现 ModelRPMService
   - Redis 集成

### 阶段 4: 优化和测试（优先级：中）

1. **性能优化**
   - Worker 并发优化
   - 数据库查询优化
   - 前端渲染优化

2. **测试覆盖**
   - 单元测试
   - 集成测试
   - E2E 测试

---

## 参考文档

### DreamStudio 关键文件

**后端：**
- `backend/internal/service/image_task_service.go` - Worker 和任务管理
- `backend/internal/service/image_generation_service.go` - 生图逻辑
- `backend/internal/service/storage_service.go` - 存储管理
- `backend/internal/repository/image_task_repo.go` - 数据访问层
- `backend/migrations/047_image_generation_tasks.sql` - 数据库表结构

**前端：**
- `frontend/src/views/HomeView.vue` - 主界面（1500+ 行，需拆分）
- `frontend/src/api/images.ts` - API 调用
- `frontend/src/views/admin/SettingsView.vue` - 管理配置

### new-api 关键文件

**后端：**
- `model/task.go` - 现有 Task 模型（参考）
- `relay/image_handler.go` - 现有图像 relay 处理
- `setting/system_setting/` - Setting 系统
- `controller/midjourney.go` - Midjourney 实现（参考）

**前端：**
- `web/src/pages/Midjourney/index.jsx` - Midjourney 页面（参考）
- `web/src/components/layout/SiderBar.jsx` - 侧边栏
- `web/src/helpers/api.js` - API 封装

---

## 上下文使用情况

**当前上下文：** ~68K tokens
**建议：** 运行 `/clear` 后执行 `/ccg:team-plan dreamstudio-image-generation` 开始规划阶段
