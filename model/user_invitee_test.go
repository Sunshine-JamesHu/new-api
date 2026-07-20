package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserInviteesScopesIncludesDeletedAndPaginates(t *testing.T) {
	truncateTables(t)

	inviter := User{
		Username: "invite-list-owner",
		AffCode:  "invite-list-owner-code",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, DB.Create(&inviter).Error)
	otherInviter := User{
		Username: "invite-list-other-owner",
		AffCode:  "invite-list-other-owner-code",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, DB.Create(&otherInviter).Error)

	invitees := []*User{
		{
			Username:    "invite-list-enabled",
			DisplayName: "Enabled Invitee",
			AffCode:     "invite-list-enabled-code",
			Status:      common.UserStatusEnabled,
			UsedQuota:   100,
			InviterId:   inviter.Id,
			CreatedAt:   1000,
		},
		{
			Username:    "invite-list-disabled",
			DisplayName: "Disabled Invitee",
			AffCode:     "invite-list-disabled-code",
			Status:      common.UserStatusDisabled,
			UsedQuota:   200,
			InviterId:   inviter.Id,
			CreatedAt:   2000,
		},
		{
			Username:    "invite-list-deleted",
			DisplayName: "Deleted Invitee",
			AffCode:     "invite-list-deleted-code",
			Status:      common.UserStatusEnabled,
			UsedQuota:   300,
			InviterId:   inviter.Id,
			CreatedAt:   3000,
		},
		{
			Username:    "invite-list-unrelated",
			DisplayName: "Unrelated Invitee",
			AffCode:     "invite-list-unrelated-code",
			Status:      common.UserStatusEnabled,
			UsedQuota:   400,
			InviterId:   otherInviter.Id,
			CreatedAt:   4000,
		},
	}
	require.NoError(t, DB.Create(invitees).Error)
	require.NoError(t, DB.Delete(invitees[2]).Error)

	pageOne, total, err := GetUserInvitees(inviter.Id, &common.PageInfo{Page: 1, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, pageOne, 2)
	assert.Equal(t, invitees[2].Id, pageOne[0].Id)
	assert.Equal(t, "Deleted Invitee", pageOne[0].DisplayName)
	assert.Equal(t, int64(3000), pageOne[0].CreatedAt)
	assert.Equal(t, 300, pageOne[0].UsedQuota)
	assert.Equal(t, common.UserStatusEnabled, pageOne[0].Status)
	assert.True(t, pageOne[0].Deleted)
	assert.Equal(t, invitees[1].Id, pageOne[1].Id)
	assert.False(t, pageOne[1].Deleted)

	pageTwo, total, err := GetUserInvitees(inviter.Id, &common.PageInfo{Page: 2, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, pageTwo, 1)
	assert.Equal(t, invitees[0].Id, pageTwo[0].Id)
	assert.Equal(t, "Enabled Invitee", pageTwo[0].DisplayName)
}
