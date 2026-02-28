package service

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
)

// ModelRPMService 模型 RPM 限流服务
type ModelRPMService struct {
	// 可以在这里添加配置字段，如数据库连接等
}

// NewModelRPMService 创建模型 RPM 限流服务实例
func NewModelRPMService() *ModelRPMService {
	return &ModelRPMService{}
}

// ResolveLimit 解析用户模型的 RPM 限制
// 参数：
//   - userID: 用户ID
//   - modelID: 模型ID（模型名称）
// 返回：
//   - rpm: RPM 限制值（0 表示不限制）
//   - error: 错误信息
func (s *ModelRPMService) ResolveLimit(userID int, modelID string) (int, error) {
	// 当前实现：从全局配置读取 RPM 限制
	// 未来可以扩展为从数据库读取用户级别或模型级别的配置

	// 检查是否启用限流
	if !setting.ModelRequestRateLimitEnabled {
		return 0, nil // 0 表示不限制
	}

	// 使用成功请求数作为 RPM 限制
	// 这里可以根据实际需求调整，例如：
	// 1. 从数据库查询用户特定的 RPM 配置
	// 2. 根据模型类型设置不同的 RPM 限制
	// 3. 根据用户组设置不同的 RPM 限制
	rpm := setting.ModelRequestRateLimitSuccessCount

	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("ResolveLimit: userID=%d, modelID=%s, rpm=%d", userID, modelID, rpm))
	}

	return rpm, nil
}

// Acquire 尝试获取 RPM 限流槽位
// 参数：
//   - userID: 用户ID
//   - modelID: 模型ID（模型名称）
// 返回：
//   - acquired: 是否成功获取槽位
//   - retryAfter: 如果未获取到槽位，需要等待的时间
//   - error: 错误信息
func (s *ModelRPMService) Acquire(userID int, modelID string) (bool, time.Duration, error) {
	// 1. 解析 RPM 限制
	rpm, err := s.ResolveLimit(userID, modelID)
	if err != nil {
		return false, 0, fmt.Errorf("failed to resolve RPM limit: %w", err)
	}

	// 如果 RPM 为 0，表示不限制
	if rpm == 0 {
		return true, 0, nil
	}

	// 2. 构建限流 key
	// 格式：rpm:user:{userID}:model:{modelID}
	key := fmt.Sprintf("rpm:user:%d:model:%s", userID, modelID)

	// 3. 获取时间窗口（默认 1 分钟）
	window := time.Duration(setting.ModelRequestRateLimitDurationMinutes) * time.Minute

	// 4. 尝试获取槽位
	acquired, retryAfter, err := common.AcquireRPMSlot(key, rpm, window)
	if err != nil {
		return false, 0, fmt.Errorf("failed to acquire RPM slot: %w", err)
	}

	if common.DebugEnabled {
		if acquired {
			common.SysLog(fmt.Sprintf("RPM slot acquired: userID=%d, modelID=%s, rpm=%d", userID, modelID, rpm))
		} else {
			common.SysLog(fmt.Sprintf("RPM limit exceeded: userID=%d, modelID=%s, rpm=%d, retryAfter=%v", userID, modelID, rpm, retryAfter))
		}
	}

	return acquired, retryAfter, nil
}

// AcquireWithGroup 尝试获取 RPM 限流槽位（支持用户组配置）
// 参数：
//   - userID: 用户ID
//   - modelID: 模型ID（模型名称）
//   - group: 用户组
// 返回：
//   - acquired: 是否成功获取槽位
//   - retryAfter: 如果未获取到槽位，需要等待的时间
//   - error: 错误信息
func (s *ModelRPMService) AcquireWithGroup(userID int, modelID string, group string) (bool, time.Duration, error) {
	// 检查是否启用限流
	if !setting.ModelRequestRateLimitEnabled {
		return true, 0, nil
	}

	// 1. 尝试获取分组的限流配置
	rpm := setting.ModelRequestRateLimitSuccessCount
	if group != "" {
		_, groupSuccessCount, found := setting.GetGroupRateLimit(group)
		if found {
			rpm = groupSuccessCount
		}
	}

	// 如果 RPM 为 0，表示不限制
	if rpm == 0 {
		return true, 0, nil
	}

	// 2. 构建限流 key
	key := fmt.Sprintf("rpm:user:%d:model:%s", userID, modelID)
	if group != "" {
		key = fmt.Sprintf("rpm:group:%s:user:%d:model:%s", group, userID, modelID)
	}

	// 3. 获取时间窗口
	window := time.Duration(setting.ModelRequestRateLimitDurationMinutes) * time.Minute

	// 4. 尝试获取槽位
	acquired, retryAfter, err := common.AcquireRPMSlot(key, rpm, window)
	if err != nil {
		return false, 0, fmt.Errorf("failed to acquire RPM slot: %w", err)
	}

	if common.DebugEnabled {
		if acquired {
			common.SysLog(fmt.Sprintf("RPM slot acquired: userID=%d, modelID=%s, group=%s, rpm=%d", userID, modelID, group, rpm))
		} else {
			common.SysLog(fmt.Sprintf("RPM limit exceeded: userID=%d, modelID=%s, group=%s, rpm=%d, retryAfter=%v", userID, modelID, group, rpm, retryAfter))
		}
	}

	return acquired, retryAfter, nil
}
