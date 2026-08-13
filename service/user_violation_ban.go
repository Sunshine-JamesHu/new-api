package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
)

const userViolationBanRedisKeyPrefix = "user:violation-ban:"

var userViolationBanRedisWarning sync.Once

// HandleUserViolationBan records one matching upstream response and disables
// the user when the configured sliding-window threshold is reached.
func HandleUserViolationBan(ctx context.Context, userID int, apiErr *types.NewAPIError) {
	if userID <= 0 || apiErr == nil {
		return
	}

	config := setting.GetUserViolationBanConfig()
	if !config.Matches(apiErr.ResponseStatusCode(), apiErr.ResponseBody()) {
		return
	}
	if !common.RedisEnabled || common.RDB == nil {
		userViolationBanRedisWarning.Do(func() {
			logger.LogWarn(ctx, "user violation ban skipped because Redis is unavailable")
		})
		return
	}

	key := fmt.Sprintf("%s%d", userViolationBanRedisKeyPrefix, userID)
	expiration := time.Duration(config.WindowHours) * time.Hour
	count, err := common.RedisIncrWithSlidingExpiration(key, expiration)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("user violation ban counter failed for user %d: %v", userID, err))
		return
	}
	if count < config.Threshold {
		return
	}

	user, err := model.GetUserById(userID, false)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("user violation ban lookup failed for user %d: %v", userID, err))
		return
	}
	if user.Status != common.UserStatusEnabled || user.Role >= common.RoleAdminUser {
		return
	}

	user.Status = common.UserStatusDisabled
	if err := user.Update(false); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("user violation ban update failed for user %d: %v", userID, err))
		return
	}
	if err := model.InvalidateUserTokensCache(userID); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("user violation ban token cache invalidation failed for user %d: %v", userID, err))
	}
	logger.LogWarn(ctx, fmt.Sprintf("user %d was automatically disabled after %d violation responses", userID, count))
}
