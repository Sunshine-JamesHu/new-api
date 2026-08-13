package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserViolationBanTest(t *testing.T, role int) (*model.User, *miniredis.Miniredis) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled, previousRDB := common.RedisEnabled, common.RDB
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.Token{}))
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled, common.RDB = true, client
	t.Cleanup(func() {
		_ = client.Close()
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled, common.RDB = previousRedisEnabled, previousRDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	user := &model.User{
		Username:    "violation-ban-user",
		Password:    "password",
		Role:        role,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
	}
	require.NoError(t, db.Create(user).Error)
	now := time.Now().Unix()
	require.NoError(t, db.Create(&model.UserSession{
		SID:             "violation-ban-session",
		UserID:          user.Id,
		Version:         1,
		UserAuthVersion: 1,
		Status:          model.UserSessionStatusActive,
		RefreshHash:     "refresh-hash",
		LoginMethod:     "password",
		LastActiveAt:    now,
		ExpiresAt:       now + 3600,
	}).Error)
	return user, server
}

func configureUserViolationBan(t *testing.T, threshold int64) {
	t.Helper()
	previous := setting.GetUserViolationBanConfig()
	previousRules := setting.UserViolationBanRulesToJSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserViolationBanRulesByJSONString(previousRules))
		require.NoError(t, setting.UpdateUserViolationBanThreshold(strconv.FormatInt(previous.Threshold, 10)))
		require.NoError(t, setting.UpdateUserViolationBanWindowHours(strconv.Itoa(previous.WindowHours)))
		require.NoError(t, setting.UpdateUserViolationBanEnabled(strconv.FormatBool(previous.Enabled)))
	})
	require.NoError(t, setting.UpdateUserViolationBanRulesByJSONString(`[{"status_code":502,"keyword":"provider blocked"}]`))
	require.NoError(t, setting.UpdateUserViolationBanThreshold(strconv.FormatInt(threshold, 10)))
	require.NoError(t, setting.UpdateUserViolationBanWindowHours("2"))
	require.NoError(t, setting.UpdateUserViolationBanEnabled("true"))
}

func matchingViolationBanError() *types.NewAPIError {
	apiErr := types.NewErrorWithStatusCode(
		errors.New("upstream response failed"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)
	apiErr.SetResponseBody(`{"error":"Provider Blocked this request"}`)
	return apiErr
}

func TestHandleUserViolationBanDisablesUserAtThreshold(t *testing.T) {
	user, server := setupUserViolationBanTest(t, common.RoleCommonUser)
	configureUserViolationBan(t, 2)

	HandleUserViolationBan(context.Background(), user.Id, matchingViolationBanError())
	var afterFirst model.User
	require.NoError(t, model.DB.First(&afterFirst, user.Id).Error)
	assert.Equal(t, common.UserStatusEnabled, afterFirst.Status)
	count, err := server.Get(fmt.Sprintf("%s%d", userViolationBanRedisKeyPrefix, user.Id))
	require.NoError(t, err)
	assert.Equal(t, "1", count)

	HandleUserViolationBan(context.Background(), user.Id, matchingViolationBanError())
	var disabled model.User
	require.NoError(t, model.DB.First(&disabled, user.Id).Error)
	assert.Equal(t, common.UserStatusDisabled, disabled.Status)
	assert.EqualValues(t, 2, disabled.AuthVersion)

	var session model.UserSession
	require.NoError(t, model.DB.First(&session, "sid = ?", "violation-ban-session").Error)
	assert.Equal(t, model.UserSessionStatusRevoked, session.Status)
}

func TestHandleUserViolationBanSkipsAdministrator(t *testing.T) {
	user, _ := setupUserViolationBanTest(t, common.RoleAdminUser)
	configureUserViolationBan(t, 1)

	HandleUserViolationBan(context.Background(), user.Id, matchingViolationBanError())

	var updated model.User
	require.NoError(t, model.DB.First(&updated, user.Id).Error)
	assert.Equal(t, common.UserStatusEnabled, updated.Status)
	assert.EqualValues(t, 1, updated.AuthVersion)
}

func TestHandleUserViolationBanUsesOriginalUpstreamStatusCode(t *testing.T) {
	user, _ := setupUserViolationBanTest(t, common.RoleCommonUser)
	configureUserViolationBan(t, 1)

	apiErr := matchingViolationBanError()
	apiErr.StatusCode = http.StatusInternalServerError
	apiErr.SetResponseStatusCode(http.StatusBadGateway)
	HandleUserViolationBan(context.Background(), user.Id, apiErr)

	var updated model.User
	require.NoError(t, model.DB.First(&updated, user.Id).Error)
	assert.Equal(t, common.UserStatusDisabled, updated.Status)
}
