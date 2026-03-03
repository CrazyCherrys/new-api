# Phase 2: 前端核心功能实施计划

## 实施概述

将 DreamStudio 的图像生成 UI 迁移到 new-api，使用 React 18 + Semi Design 重新实现。

## 技术栈对比

| 项目 | DreamStudio | new-api |
|------|-------------|---------|
| 框架 | Vue 3 | React 18 |
| UI 库 | Tailwind CSS | Semi Design |
| 状态管理 | Pinia | React Hooks |
| 路由 | Vue Router | React Router |
| 构建工具 | Vite | Vite |

## 并行实施任务

### Task 1: Generator 组件（图像生成表单）

**负责人：** frontend-builder-1
**文件：** `web/src/pages/ImageGeneration/Generator.jsx`

**功能需求：**
1. 模型选择器（Semi Design Select）
2. Prompt 输入框（Semi Design TextArea，支持多行）
3. 参数配置：
   - 分辨率选择（Select）
   - 生成数量（InputNumber，1-10）
4. 生成按钮（Button，loading 状态）
5. 实时状态提示（Toast）

**API 集成：**
- `POST /api/v1/image-tasks/generate` - 创建任务
- `GET /api/user/models` - 获取可用模型列表

**参考实现：**
- DreamStudio: `/root/WorkSpaces/DreamStudio/frontend/src/views/HomeView.vue:157-400`
- 使用 Semi Design 组件替代 Tailwind 样式

**约束：**
- [HC-8] 必须使用 Semi Design 组件
- [HC-1] 遵循 new-api 现有代码风格
- [SC-1] 响应式设计，支持移动端

**输出文件：**
- `web/src/pages/ImageGeneration/Generator.jsx` (约 300 行)
- `web/src/pages/ImageGeneration/generator.module.css` (可选)

---

### Task 2: History 页面（历史记录列表）

**负责人：** frontend-builder-2
**文件：** `web/src/pages/ImageGeneration/History.jsx`

**功能需求：**
1. 任务列表展示（Semi Design Table 或 List）
2. 状态筛选（Select：全部/进行中/成功/失败）
3. 分页控件（Pagination）
4. 图片预览（Image Grid）
5. 详情弹窗（Modal）
6. 删除功能（Popconfirm + API 调用）

**API 集成：**
- `GET /api/v1/image-tasks/history` - 获取历史记录（分页）
- `GET /api/v1/image-tasks/history/:id` - 获取任务详情
- `DELETE /api/v1/image-tasks/history/:id` - 删除任务

**参考实现：**
- DreamStudio: `/root/WorkSpaces/DreamStudio/frontend/src/views/HomeView.vue:600-900`
- 使用 Semi Design Table/List 组件

**约束：**
- [HC-8] 必须使用 Semi Design 组件
- [HC-1] 遵循 new-api 现有代码风格
- [SC-2] 支持实时刷新（轮询或 WebSocket）

**输出文件：**
- `web/src/pages/ImageGeneration/History.jsx` (约 400 行)
- `web/src/pages/ImageGeneration/ImageDetailModal.jsx` (约 200 行)

---

### Task 3: 主页面集成 + 路由配置

**负责人：** frontend-builder-3
**文件：** `web/src/pages/ImageGeneration/index.jsx`

**功能需求：**
1. 主页面布局（Layout）
   - 左侧：Generator 组件
   - 右侧：History 组件
   - 响应式布局（移动端垂直堆叠）
2. 路由配置
   - 添加 `/image-generation` 路由
   - 权限控制（需要登录）
3. SiderBar 集成
   - 添加"AI 图像生成"菜单项
   - 图标：`IconImage` 或 `IconPicture`
   - 位置：在"聊天"和"设置"之间

**API 集成：**
- 无（仅布局和路由）

**参考实现：**
- DreamStudio: `/root/WorkSpaces/DreamStudio/frontend/src/views/HomeView.vue:108-156`
- new-api 现有页面布局：`web/src/pages/Channel/index.js`

**约束：**
- [HC-8] 必须使用 Semi Design Layout 组件
- [HC-1] 遵循 new-api 现有路由模式
- [DEP-5] 依赖 Task 1 和 Task 2 的组件

**输出文件：**
- `web/src/pages/ImageGeneration/index.jsx` (约 150 行)
- 修改 `web/src/App.js` - 添加路由
- 修改 `web/src/components/SiderBar.js` - 添加菜单项

---

## 实施顺序

**并行执行：** Task 1 和 Task 2 可以并行开发（无依赖）
**串行执行：** Task 3 依赖 Task 1 和 Task 2 完成后集成

## API 端点映射

| DreamStudio API | new-api API | 说明 |
|-----------------|-------------|------|
| `POST /images/generate` | `POST /api/v1/image-tasks/generate` | 创建任务 |
| `GET /images/history` | `GET /api/v1/image-tasks/history` | 历史记录 |
| `GET /images/history/:id` | `GET /api/v1/image-tasks/history/:id` | 任务详情 |
| `DELETE /images/history/:id` | `DELETE /api/v1/image-tasks/history/:id` | 删除任务 |

## 数据结构映射

### ImageTask (后端模型)
```go
type ImageTask struct {
    ID             int64           `json:"id"`
    CreatedAt      int64           `json:"created_at"`
    UpdatedAt      int64           `json:"updated_at"`
    UserID         int             `json:"user_id"`
    ModelID        string          `json:"model_id"`
    Prompt         string          `json:"prompt"`
    Resolution     string          `json:"resolution"`
    Count          int             `json:"count"`
    Status         ImageTaskStatus `json:"status"` // pending/running/succeeded/failed
    ErrorMessage   string          `json:"error_message"`
    ImageURLs      []string        `json:"image_urls"`
    Attempts       int             `json:"attempts"`
}
```

### 前端 TypeScript 接口（参考）
```typescript
interface ImageTask {
  id: number
  created_at: number
  updated_at: number
  user_id: number
  model_id: string
  prompt: string
  resolution?: string
  count: number
  status: 'pending' | 'running' | 'succeeded' | 'failed'
  error_message?: string
  image_urls: string[]
  attempts: number
}
```

## 成功标准

- [ ] Generator 组件可以成功创建图像生成任务
- [ ] History 页面可以展示任务列表和状态
- [ ] 详情弹窗可以查看生成的图片
- [ ] 删除功能正常工作
- [ ] SiderBar 菜单项正确显示和跳转
- [ ] 响应式布局在移动端正常显示
- [ ] 所有组件使用 Semi Design 样式
- [ ] 代码风格与 new-api 现有代码一致

## 注意事项

1. **i18n 支持：** 所有文本使用 `t('key')` 国际化
2. **错误处理：** 使用 Semi Design Toast 显示错误信息
3. **Loading 状态：** 所有异步操作显示 loading 状态
4. **权限控制：** 检查用户登录状态
5. **代码注释：** 使用中文注释（与 new-api 现有代码一致）
