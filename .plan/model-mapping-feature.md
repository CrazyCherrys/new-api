# 模型映射设置功能实施计划

## 功能概述

在管理员页面的渠道管理下方增加一个"模型映射"页面，用于配置模型ID映射关系，包括：
- 请求模型ID（用户请求时使用的模型标识）
- 实际调用模型ID（转发到上游时使用的模型标识）
- 模型显示名称（用户界面显示的友好名称）
- 模型系列（用于区分不同厂商，如 OpenAI、Claude、Gemini 等）
- 模型类型（区分对话、绘画、视频等不同类型）

## 数据库设计

### 新增表：model_mapping

```sql
CREATE TABLE model_mapping (
    id INTEGER PRIMARY KEY AUTO_INCREMENT,
    request_model VARCHAR(128) NOT NULL COMMENT '请求模型ID（用户请求时使用）',
    actual_model VARCHAR(128) NOT NULL COMMENT '实际调用模型ID（转发到上游）',
    display_name VARCHAR(255) NOT NULL COMMENT '模型显示名称',
    model_series VARCHAR(64) DEFAULT '' COMMENT '模型系列（厂商）',
    model_type INT DEFAULT 1 COMMENT '模型类型：1=对话 2=绘画 3=视频 4=音频',
    description TEXT COMMENT '模型描述',
    status INT DEFAULT 1 COMMENT '状态：1=启用 0=禁用',
    priority INT DEFAULT 0 COMMENT '优先级（数字越大优先级越高）',
    created_time BIGINT NOT NULL,
    updated_time BIGINT NOT NULL,
    UNIQUE KEY uk_request_model (request_model),
    INDEX idx_model_series (model_series),
    INDEX idx_model_type (model_type),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='模型映射配置表';
```

**字段说明：**
- `request_model`: 用户请求时使用的模型ID，唯一索引
- `actual_model`: 实际转发到上游的模型ID
- `display_name`: 在用户界面显示的友好名称
- `model_series`: 模型系列/厂商标识（如 openai、anthropic、google 等）
- `model_type`: 模型类型枚举值
- `priority`: 当有多个映射规则时的优先级

## 后端实现

### 1. 数据模型 (model/model_mapping.go)

```go
package model

import (
    "github.com/QuantumNous/new-api/common"
    "gorm.io/gorm"
)

// ModelMapping 模型映射配置
type ModelMapping struct {
    Id           int    `json:"id"`
    RequestModel string `json:"request_model" gorm:"size:128;not null;uniqueIndex:uk_request_model"`
    ActualModel  string `json:"actual_model" gorm:"size:128;not null"`
    DisplayName  string `json:"display_name" gorm:"size:255;not null"`
    ModelSeries  string `json:"model_series" gorm:"size:64;default:'';index"`
    ModelType    int    `json:"model_type" gorm:"default:1;index"` // 1=对话 2=绘画 3=视频 4=音频
    Description  string `json:"description" gorm:"type:text"`
    Status       int    `json:"status" gorm:"default:1;index"`
    Priority     int    `json:"priority" gorm:"default:0"`
    CreatedTime  int64  `json:"created_time" gorm:"bigint"`
    UpdatedTime  int64  `json:"updated_time" gorm:"bigint"`
}

func (mm *ModelMapping) Insert() error {
    now := common.GetTimestamp()
    mm.CreatedTime = now
    mm.UpdatedTime = now
    return DB.Create(mm).Error
}

func (mm *ModelMapping) Update() error {
    mm.UpdatedTime = common.GetTimestamp()
    return DB.Model(mm).Updates(mm).Error
}

func GetModelMapping(id int) (*ModelMapping, error) {
    var mm ModelMapping
    err := DB.First(&mm, id).Error
    return &mm, err
}

func GetAllModelMappings(startIdx int, num int) ([]*ModelMapping, error) {
    var mappings []*ModelMapping
    err := DB.Order("priority DESC, id DESC").Limit(num).Offset(startIdx).Find(&mappings).Error
    return mappings, err
}

func SearchModelMappings(keyword string, modelType int, startIdx int, num int) ([]*ModelMapping, int64, error) {
    var mappings []*ModelMapping
    var total int64

    query := DB.Model(&ModelMapping{})

    if keyword != "" {
        query = query.Where("request_model LIKE ? OR display_name LIKE ? OR model_series LIKE ?",
            "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
    }

    if modelType > 0 {
        query = query.Where("model_type = ?", modelType)
    }

    err := query.Count(&total).Error
    if err != nil {
        return nil, 0, err
    }

    err = query.Order("priority DESC, id DESC").Limit(num).Offset(startIdx).Find(&mappings).Error
    return mappings, total, err
}

func DeleteModelMapping(id int) error {
    return DB.Delete(&ModelMapping{}, id).Error
}

func GetModelMappingByRequestModel(requestModel string) (*ModelMapping, error) {
    var mm ModelMapping
    err := DB.Where("request_model = ? AND status = 1", requestModel).First(&mm).Error
    if err == gorm.ErrRecordNotFound {
        return nil, nil
    }
    return &mm, err
}
```

### 2. 控制器 (controller/model_mapping.go)

```go
package controller

import (
    "strconv"

    "github.com/QuantumNous/new-api/common"
    "github.com/QuantumNous/new-api/model"
    "github.com/gin-gonic/gin"
)

// GetAllModelMappings 获取模型映射列表（分页）
func GetAllModelMappings(c *gin.Context) {
    pageInfo := common.GetPageQuery(c)
    mappings, err := model.GetAllModelMappings(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
    if err != nil {
        common.ApiError(c, err)
        return
    }

    var total int64
    model.DB.Model(&model.ModelMapping{}).Count(&total)

    pageInfo.SetTotal(int(total))
    pageInfo.SetItems(mappings)
    common.ApiSuccess(c, pageInfo)
}

// SearchModelMappings 搜索模型映射
func SearchModelMappings(c *gin.Context) {
    keyword := c.Query("keyword")
    modelTypeStr := c.Query("model_type")
    modelType := 0
    if modelTypeStr != "" {
        modelType, _ = strconv.Atoi(modelTypeStr)
    }

    pageInfo := common.GetPageQuery(c)
    mappings, total, err := model.SearchModelMappings(keyword, modelType, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
    if err != nil {
        common.ApiError(c, err)
        return
    }

    pageInfo.SetTotal(int(total))
    pageInfo.SetItems(mappings)
    common.ApiSuccess(c, pageInfo)
}

// GetModelMapping 获取单个模型映射
func GetModelMapping(c *gin.Context) {
    idStr := c.Param("id")
    id, err := strconv.Atoi(idStr)
    if err != nil {
        common.ApiError(c, err)
        return
    }

    mapping, err := model.GetModelMapping(id)
    if err != nil {
        common.ApiError(c, err)
        return
    }

    common.ApiSuccess(c, mapping)
}

// CreateModelMapping 创建模型映射
func CreateModelMapping(c *gin.Context) {
    var mm model.ModelMapping
    if err := c.ShouldBindJSON(&mm); err != nil {
        common.ApiError(c, err)
        return
    }

    if mm.RequestModel == "" {
        common.ApiErrorMsg(c, "请求模型ID不能为空")
        return
    }

    if mm.ActualModel == "" {
        common.ApiErrorMsg(c, "实际调用模型ID不能为空")
        return
    }

    if mm.DisplayName == "" {
        common.ApiErrorMsg(c, "模型显示名称不能为空")
        return
    }

    // 检查是否已存在
    existing, _ := model.GetModelMappingByRequestModel(mm.RequestModel)
    if existing != nil {
        common.ApiErrorMsg(c, "该请求模型ID已存在")
        return
    }

    if err := mm.Insert(); err != nil {
        common.ApiError(c, err)
        return
    }

    common.ApiSuccess(c, mm)
}

// UpdateModelMapping 更新模型映射
func UpdateModelMapping(c *gin.Context) {
    var mm model.ModelMapping
    if err := c.ShouldBindJSON(&mm); err != nil {
        common.ApiError(c, err)
        return
    }

    if mm.Id == 0 {
        common.ApiErrorMsg(c, "ID不能为空")
        return
    }

    if mm.RequestModel == "" {
        common.ApiErrorMsg(c, "请求模型ID不能为空")
        return
    }

    if mm.ActualModel == "" {
        common.ApiErrorMsg(c, "实际调用模型ID不能为空")
        return
    }

    if mm.DisplayName == "" {
        common.ApiErrorMsg(c, "模型显示名称不能为空")
        return
    }

    if err := mm.Update(); err != nil {
        common.ApiError(c, err)
        return
    }

    common.ApiSuccess(c, mm)
}

// DeleteModelMapping 删除模型映射
func DeleteModelMapping(c *gin.Context) {
    idStr := c.Param("id")
    id, err := strconv.Atoi(idStr)
    if err != nil {
        common.ApiError(c, err)
        return
    }

    if err := model.DeleteModelMapping(id); err != nil {
        common.ApiError(c, err)
        return
    }

    common.ApiSuccess(c, nil)
}
```

### 3. 路由配置 (router/api-router.go)

在 `SetApiRouter` 函数中添加新的路由组：

```go
// Model Mapping (模型映射管理)
modelMappingRoute := apiRouter.Group("/model-mapping")
modelMappingRoute.Use(middleware.AdminAuth())
{
    modelMappingRoute.GET("/", controller.GetAllModelMappings)
    modelMappingRoute.GET("/search", controller.SearchModelMappings)
    modelMappingRoute.GET("/:id", controller.GetModelMapping)
    modelMappingRoute.POST("/", controller.CreateModelMapping)
    modelMappingRoute.PUT("/", controller.UpdateModelMapping)
    modelMappingRoute.DELETE("/:id", controller.DeleteModelMapping)
}
```

### 4. 数据库迁移

在 `model/main.go` 的 `InitDB` 函数中添加自动迁移：

```go
err = db.AutoMigrate(&ModelMapping{})
if err != nil {
    common.FatalLog("model mapping table migration failed: " + err.Error())
}
```

## 前端实现

### 1. 页面组件 (web/src/pages/ModelMapping/index.jsx)

```jsx
import React from 'react';
import ModelMappingTable from '../../components/table/model-mapping';

const ModelMapping = () => {
  return (
    <div className='mt-[60px] px-2'>
      <ModelMappingTable />
    </div>
  );
};

export default ModelMapping;
```

### 2. 表格组件 (web/src/components/table/model-mapping/index.jsx)

参考 `web/src/components/table/channels/index.jsx` 的结构实现。

### 3. 表格主体 (web/src/components/table/model-mapping/ModelMappingTable.jsx)

参考 `web/src/components/table/channels/ChannelsTable.jsx` 实现。

### 4. 编辑模态框 (web/src/components/table/model-mapping/modals/EditModelMappingModal.jsx)

包含以下字段：
- 请求模型ID（文本输入）
- 实际调用模型ID（文本输入）
- 模型显示名称（文本输入）
- 模型系列（下拉选择：OpenAI、Claude、Gemini、Azure、AWS、国产模型等）
- 模型类型（下拉选择：对话、绘画、视频、音频）
- 描述（文本域）
- 状态（开关）
- 优先级（数字输入）

### 5. 列定义 (web/src/components/table/model-mapping/ModelMappingColumnDefs.jsx)

定义表格列：
- ID
- 请求模型ID
- 实际调用模型ID
- 显示名称
- 模型系列（带标签颜色）
- 模型类型（带标签颜色）
- 状态（启用/禁用）
- 优先级
- 创建时间
- 操作（编辑、删除）

### 6. 数据钩子 (web/src/hooks/model-mapping/useModelMappingData.js)

管理状态和API调用。

### 7. 路由配置

在 `web/src/App.jsx` 中添加路由：

```jsx
<Route
  path='/console/model-mapping'
  element={
    <AdminRoute>
      <ModelMapping />
    </AdminRoute>
  }
/>
```

### 8. 侧边栏菜单

在 `web/src/components/layout/SiderBar.jsx` 的 `adminItems` 中添加：

```jsx
{
  text: t('模型映射'),
  itemKey: 'model-mapping',
  to: '/console/model-mapping',
  className: isAdmin() ? '' : 'tableHiddle',
}
```

### 9. 国际化

在 `web/src/i18n/locales/zh.json` 和其他语言文件中添加翻译键：

```json
{
  "模型映射": "模型映射",
  "请求模型ID": "请求模型ID",
  "实际调用模型ID": "实际调用模型ID",
  "模型显示名称": "模型显示名称",
  "模型系列": "模型系列",
  "模型类型": "模型类型",
  "对话": "对话",
  "绘画": "绘画",
  "视频": "视频",
  "音频": "音频"
}
```

## 实施步骤

### Phase 1: 后端基础 (30分钟)
1. 创建数据模型 `model/model_mapping.go`
2. 创建控制器 `controller/model_mapping.go`
3. 添加路由配置
4. 添加数据库迁移

### Phase 2: 前端基础 (45分钟)
1. 创建页面组件
2. 创建表格组件
3. 创建编辑模态框
4. 实现列定义
5. 创建数据钩子

### Phase 3: 集成与测试 (30分钟)
1. 添加路由配置
2. 更新侧边栏菜单
3. 添加国际化翻译
4. 功能测试
5. 跨数据库兼容性测试

### Phase 4: 中继集成（可选，后续扩展）
如需在请求转发时应用模型映射，需要修改 `relay/` 相关代码，在转发前查询映射表并替换模型ID。

## 技术要点

1. **数据库兼容性**：遵循 Rule 2，确保 SQLite、MySQL、PostgreSQL 三种数据库兼容
2. **JSON处理**：遵循 Rule 1，使用 `common.Marshal/Unmarshal`
3. **前端包管理**：遵循 Rule 3，使用 `bun` 命令
4. **代码风格**：参考现有的 Channel 和 Model 管理模块
5. **权限控制**：仅管理员可访问（使用 `middleware.AdminAuth()`）

## 预期效果

完成后，管理员可以：
1. 在"模型映射"页面查看所有映射配置
2. 添加新的模型映射规则
3. 编辑现有映射规则
4. 删除不需要的映射
5. 按模型系列、类型筛选
6. 搜索特定模型

用户侧效果：
- 在模型列表中看到配置的显示名称
- 请求时使用友好的模型ID
- 系统自动转换为实际的上游模型ID
