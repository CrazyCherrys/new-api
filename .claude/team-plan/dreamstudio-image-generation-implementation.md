# Team Plan: DreamStudio AI 图像生成功能集成

## 概述

将 DreamStudio 的 AI 图像生成功能集成到 new-api 项目中，包括异步任务系统、智能重试机制、灵活存储系统、模型限流和多平台支持。

## Gemini 分析摘要

**UI/UX 方案：**
- 采用"左配置-中创作-右/底预览"的响应式布局
- 左侧参数栏：模型选择、尺寸、采样步数、提示词相关性、种子值
- 中央主工作区：Prompt 输入框、参考图上传区、负向提示词
- 历史与列表：在主界面下方或右侧提供"最近生成"的小缩略图列表

**组件拆分建议：**
- `web/src/pages/DreamStudio/index.jsx` - 容器组件（状态管理）
- `web/src/pages/DreamStudio/DreamStudioPanel.jsx` - 布局组件
- `web/src/pages/DreamStudio/ParameterSidebar.jsx` - 参数控件
- `web/src/pages/DreamStudio/PromptEditor.jsx` - 提示词输入
- `web/src/pages/DreamStudio/ImageUploader.jsx` - 参考图处理
- `web/src/pages/DreamStudio/TaskGallery.jsx` - 历史任务展示
- `web/src/pages/DreamStudio/TaskCard.jsx` - 单个任务卡片

**轮询逻辑：**
- 前 30 秒每 3s 轮询，之后每 10s 轮询，总超时 5 分钟
- 使用 `usePolling` 自定义 Hook 管理 `setInterval`

## Codex 分析摘要

**现有实现状态：**
- ✅ 数据模型已创建：`model/image_task.go` (ImageTask 模型)
- ✅ 服务层已实现：`service/image_task.go` (ImageTaskService)
- ✅ 存储服务已实现：`service/image_storage.go` (ImageStorageService)
- ✅ 生成服务已实现：`service/image_generation.go` (ImageGenerationService)
- ✅ 控制器已创建：`controller/image_task.go` (ImageTaskController)
- ✅ 路由已配置：`router/image-router.go`
- ✅ 配置系统已集成：`setting/system_setting/image_generation.go`
- ✅ Worker 已在 main.go 中启动

**架构方案：**
- 使用 GORM AutoMigrate 确保跨数据库兼容
- Worker 使用 `FOR UPDATE SKIP LOCKED` 确保并发安全
- 配置通过 `config.GlobalConfig` 系统管理
- 图片存储支持本地文件系统和 S3

**关键发现：**
- 后端核心功能已基本实现，需要补充 relay 层的图像生成适配器
- 前端完全缺失，需要从零创建
- 需要添加管理界面用于配置管理

## 技术方案

### 后端方案（基于 Codex 分析）

**已完成部分：**
1. 数据模型层 (model/image_task.go) - 完整实现
2. 服务层 (service/image_task.go, image_generation.go, image_storage.go) - 完整实现
3. 控制器层 (controller/image_task.go) - 完整实现
4. 路由层 (router/image-router.go) - 完整实现
5. 配置系统 (setting/system_setting/image_generation.go) - 完整实现

**待补充部分：**
1. Relay 层图像生成适配器 (relay/image_programmatic.go) - 需完善多平台支持
2. 管理界面后端 API - 需添加配置管理端点

### 前端方案（基于 Gemini 分析）

**完全缺失，需要创建：**
1. 生图主界面 - 完整的 React 组件树
2. 历史记录页面 - 任务列表和轮询逻辑
3. 管理配置界面 - 存储、生成参数、模型管理
4. 路由和导航集成 - SiderBar 和 App.jsx 修改
5. API 调用封装 - helpers/api.js 扩展
6. 国际化支持 - i18n 文件添加

## 子任务列表

### Task 1: 补充 Relay 层图像生成适配器
- **类型**: 后端
- **文件范围**:
  - `relay/image_programmatic.go`
- **依赖**: 无
- **实施步骤**:
  1. 检查 `relay/image_programmatic.go` 的现有实现
  2. 确保支持 OpenAI、Gemini、自定义端点的图像生成 API
  3. 实现错误处理和重试逻辑
  4. 添加响应解析和 URL 提取
  5. 确保使用 `common.Marshal/Unmarshal` 处理 JSON
- **验收标准**:
  - 可以成功调用 OpenAI `/v1/images/generations` API
  - 可以成功调用 Gemini 图像生成 API
  - 错误处理完整，返回清晰的错误信息

### Task 2: 添加管理配置 API 端点
- **类型**: 后端
- **文件范围**:
  - `controller/image_task.go` (扩展)
  - `router/image-router.go` (扩展)
- **依赖**: 无
- **实施步骤**:
  1. 在 `controller/image_task.go` 添加 `GetImageGenerationConfig` 函数
  2. 在 `controller/image_task.go` 添加 `UpdateImageGenerationConfig` 函数
  3. 在 `router/image-router.go` 添加路由：
     - `GET /api/v1/image-tasks/config` - 获取配置
     - `PUT /api/v1/image-tasks/config` - 更新配置
  4. 实现权限检查（仅管理员可访问）
  5. 配置更新后调用 `config.GlobalConfig.LoadFromDB` 重新加载
- **验收标准**:
  - 管理员可以通过 API 获取当前配置
  - 管理员可以通过 API 更新配置
  - 配置更新后立即生效

### Task 3: 创建前端生图主界面 - 容器和布局
- **类型**: 前端
- **文件范围**:
  - `web/src/pages/DreamStudio/index.jsx`
  - `web/src/pages/DreamStudio/DreamStudioPanel.jsx`
- **依赖**: 无
- **实施步骤**:
  1. 创建 `web/src/pages/DreamStudio/` 目录
  2. 创建 `index.jsx` 作为容器组件：
     - 使用 `useReducer` 管理表单状态（15+ 参数）
     - 定义 actions: SET_PROMPT, SET_MODEL, SET_RESOLUTION 等
     - 初始化默认参数值
  3. 创建 `DreamStudioPanel.jsx` 布局组件：
     - 使用 Semi Design 的 `Layout` 组件
     - 左侧：参数面板（可折叠）
     - 中央：Prompt 编辑器和上传区
     - 下方：历史任务画廊
  4. 添加响应式设计（移动端适配）
- **验收标准**:
  - 页面布局正确显示
  - 状态管理正常工作
  - 响应式布局在不同屏幕尺寸下正常

### Task 4: 创建前端生图主界面 - 参数控件
- **类型**: 前端
- **文件范围**:
  - `web/src/pages/DreamStudio/ParameterSidebar.jsx`
- **依赖**: Task 3
- **实施步骤**:
  1. 创建 `ParameterSidebar.jsx` 组件
  2. 使用 Semi Design 组件实现参数控件：
     - `Select` - 模型选择
     - `Slider` - 采样步数 (1-50)
     - `Slider` - 提示词相关性 (1-20)
     - `InputNumber` - 种子值
     - `Select` - 分辨率选择
     - `Select` - 宽高比选择
     - `InputNumber` - 生成数量 (1-4)
  3. 每个控件变化时调用父组件的 dispatch 更新状态
  4. 添加参数说明 Tooltip
- **验收标准**:
  - 所有参数控件正常工作
  - 参数变化时状态正确更新
  - Tooltip 显示清晰的参数说明

### Task 5: 创建前端生图主界面 - Prompt 编辑器
- **类型**: 前端
- **文件范围**:
  - `web/src/pages/DreamStudio/PromptEditor.jsx`
- **依赖**: Task 3
- **实施步骤**:
  1. 创建 `PromptEditor.jsx` 组件
  2. 使用 Semi Design 的 `TextArea` 实现 Prompt 输入：
     - 支持 Auto-size
     - 显示字符计数
     - 最大长度限制 (2000 字符)
  3. 添加负向提示词输入（默认折叠）
  4. 添加"生成"按钮：
     - 调用 API 创建任务
     - 显示成本预估
     - 禁用状态处理（生成中）
  5. 添加快速提示词模板（可选）
- **验收标准**:
  - Prompt 输入框正常工作
  - 负向提示词可折叠/展开
  - 生成按钮点击后正确调用 API

### Task 6: 创建前端生图主界面 - 图片上传
- **类型**: 前端
- **文件范围**:
  - `web/src/pages/DreamStudio/ImageUploader.jsx`
- **依赖**: Task 3
- **实施步骤**:
  1. 创建 `ImageUploader.jsx` 组件
  2. 使用 Semi Design 的 `Upload` 组件：
     - 支持拖拽上传
     - 图片预览
     - 文件大小限制 (5MB)
     - 文件类型限制 (jpg, png, webp)
  3. 实现图片转 Base64 或上传到临时存储
  4. 添加图片编辑功能（裁剪、缩放）- 可选
  5. 上传成功后更新父组件状态
- **验收标准**:
  - 图片上传功能正常
  - 图片预览正确显示
  - Base64 转换或上传成功

### Task 7: 创建前端历史记录页面 - 任务画廊
- **类型**: 前端
- **文件范围**:
  - `web/src/pages/DreamStudio/TaskGallery.jsx`
  - `web/src/pages/DreamStudio/TaskCard.jsx`
- **依赖**: Task 3
- **实施步骤**:
  1. 创建 `TaskGallery.jsx` 组件：
     - 使用 Grid 布局显示任务卡片
     - 实现分页加载
     - 添加筛选器（状态、日期）
  2. 创建 `TaskCard.jsx` 组件：
     - 显示任务状态（pending, running, succeeded, failed）
     - 显示生成参数摘要
     - 显示图片缩略图（成功时）
     - 显示错误信息（失败时）
     - 添加操作按钮（查看详情、删除、重新生成）
  3. 实现任务状态轮询逻辑：
     - 创建 `usePolling` Hook
     - 前 30 秒每 3s 轮询，之后每 10s
     - 任务完成后停止轮询
  4. 添加图片预览功能（点击放大）
- **验收标准**:
  - 任务列表正确显示
  - 任务状态实时更新
  - 轮询逻辑正常工作
  - 图片预览功能正常

### Task 8: 创建前端 API 调用封装
- **类型**: 前端
- **文件范围**:
  - `web/src/helpers/imageApi.js`
- **依赖**: 无
- **实施步骤**:
  1. 创建 `web/src/helpers/imageApi.js` 文件
  2. 基于 `web/src/helpers/api.js` 的 API 封装模式实现：
     - `createImageTask(payload)` - POST /api/v1/image-tasks/generate
     - `getImageTask(taskId)` - GET /api/v1/image-tasks/history/:id
     - `listImageTasks(params)` - GET /api/v1/image-tasks/history
     - `deleteImageTask(taskId)` - DELETE /api/v1/image-tasks/history/:id
     - `getImageConfig()` - GET /api/v1/image-tasks/config
     - `updateImageConfig(config)` - PUT /api/v1/image-tasks/config
  3. 添加错误处理和重试逻辑
  4. 添加请求去重（防止重复提交）
- **验收标准**:
  - 所有 API 函数正常工作
  - 错误处理完整
  - 请求去重生效

### Task 9: 创建前端管理配置界面
- **类型**: 前端
- **文件范围**:
  - `web/src/pages/DreamStudio/AdminSettings.jsx`
- **依赖**: Task 8
- **实施步骤**:
  1. 创建 `AdminSettings.jsx` 组件
  2. 实现配置表单：
     - 存储配置：S3 开关、Endpoint、Bucket、AccessKey、SecretKey
     - 生成参数：超时时间、最大重试次数、重试间隔
     - 模型管理：可见模型列表、默认模型
     - RPM 限制：单模型请求频率限制
  3. 使用 Semi Design 的 `Form` 组件
  4. 添加配置验证（S3 连接测试）
  5. 保存配置后调用 API 更新
- **验收标准**:
  - 配置表单正确显示
  - 配置保存成功
  - S3 连接测试功能正常

### Task 10: 集成路由和导航
- **类型**: 前端
- **文件范围**:
  - `web/src/App.jsx`
  - `web/src/components/layout/SiderBar.jsx`
- **依赖**: Task 3, Task 7, Task 9
- **实施步骤**:
  1. 修改 `web/src/App.jsx` 添加路由：
     - `/console/dreamstudio` - 生图主界面
     - `/console/dreamstudio/admin` - 管理配置界面
  2. 修改 `web/src/components/layout/SiderBar.jsx` 添加菜单项：
     - 在 `workspaceItems` 中添加 "AI 绘画" 入口
     - 图标使用 `IconImage` 或自定义图标
     - 添加 localStorage 控制显示/隐藏
  3. 在 `helpers/render.jsx` 中注册图标
  4. 添加权限检查（管理界面仅管理员可访问）
- **验收标准**:
  - 侧边栏显示 "AI 绘画" 入口
  - 点击后正确跳转到生图界面
  - 管理界面仅管理员可访问

### Task 11: 添加国际化支持
- **类型**: 前端
- **文件范围**:
  - `web/src/i18n/locales/zh.json`
  - `web/src/i18n/locales/en.json`
- **依赖**: 无
- **实施步骤**:
  1. 在 `web/src/i18n/locales/zh.json` 添加中文翻译：
     - "AI 绘画"、"提示词"、"采样步数"、"生成中..."、"生成成功"、"生成失败" 等
  2. 在 `web/src/i18n/locales/en.json` 添加英文翻译：
     - "AI Image Generation", "Prompt", "Sampling Steps", "Generating...", "Success", "Failed" 等
  3. 在所有组件中使用 `useTranslation()` Hook
  4. 确保所有文本都已国际化
- **验收标准**:
  - 中文界面显示正确
  - 英文界面显示正确
  - 语言切换正常工作

## 文件冲突检查

✅ 无冲突 - 所有子任务的文件范围已严格隔离：
- Task 1: `relay/image_programmatic.go` (独立)
- Task 2: `controller/image_task.go` (扩展), `router/image-router.go` (扩展)
- Task 3: `web/src/pages/DreamStudio/index.jsx`, `DreamStudioPanel.jsx` (新建)
- Task 4: `web/src/pages/DreamStudio/ParameterSidebar.jsx` (新建)
- Task 5: `web/src/pages/DreamStudio/PromptEditor.jsx` (新建)
- Task 6: `web/src/pages/DreamStudio/ImageUploader.jsx` (新建)
- Task 7: `web/src/pages/DreamStudio/TaskGallery.jsx`, `TaskCard.jsx` (新建)
- Task 8: `web/src/helpers/imageApi.js` (新建)
- Task 9: `web/src/pages/DreamStudio/AdminSettings.jsx` (新建)
- Task 10: `web/src/App.jsx` (扩展), `web/src/components/layout/SiderBar.jsx` (扩展)
- Task 11: `web/src/i18n/locales/*.json` (扩展)

**潜在冲突：**
- Task 2 和 Task 10 都需要修改 `router/image-router.go`，但 Task 2 添加配置端点，Task 10 不涉及后端路由，无冲突
- Task 10 修改 `App.jsx` 和 `SiderBar.jsx`，但这两个文件不同，无冲突

## 并行分组

### Layer 1 (并行执行 - 后端基础)
- Task 1: 补充 Relay 层图像生成适配器
- Task 2: 添加管理配置 API 端点
- Task 8: 创建前端 API 调用封装
- Task 11: 添加国际化支持

### Layer 2 (依赖 Layer 1 - 前端核心组件)
- Task 3: 创建前端生图主界面 - 容器和布局

### Layer 3 (依赖 Task 3 - 前端子组件)
- Task 4: 创建前端生图主界面 - 参数控件
- Task 5: 创建前端生图主界面 - Prompt 编辑器
- Task 6: 创建前端生图主界面 - 图片上传
- Task 7: 创建前端历史记录页面 - 任务画廊

### Layer 4 (依赖 Task 8 - 管理界面)
- Task 9: 创建前端管理配置界面

### Layer 5 (依赖 Task 3, 7, 9 - 路由集成)
- Task 10: 集成路由和导航

## 实施建议

1. **Layer 1 优先级最高**：后端 API 和前端基础设施必须先完成
2. **Layer 2-3 是核心功能**：生图主界面是用户主要交互界面
3. **Layer 4-5 是增强功能**：管理界面和路由集成可以后续完善
4. **测试策略**：
   - 每个 Layer 完成后进行集成测试
   - Layer 3 完成后进行端到端测试
   - Layer 5 完成后进行完整功能测试

## 预估工作量

- **Layer 1**: 4-6 小时（4 个任务并行）
- **Layer 2**: 2-3 小时（1 个任务）
- **Layer 3**: 8-10 小时（4 个任务并行）
- **Layer 4**: 3-4 小时（1 个任务）
- **Layer 5**: 2-3 小时（1 个任务）

**总计**: 19-26 小时（使用 Agent Teams 并行执行）

## 后续优化建议

1. **性能优化**：
   - 图片懒加载
   - 任务列表虚拟滚动
   - API 请求缓存

2. **功能增强**：
   - 批量生成
   - 任务队列管理
   - 生成历史导出

3. **用户体验**：
   - 快速提示词模板
   - 参数预设保存
   - 生成进度实时显示
