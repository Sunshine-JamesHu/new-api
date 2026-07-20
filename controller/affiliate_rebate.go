package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetUserAffiliateRebates(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)

	rebates, total, err := model.GetUserAffiliateRebates(userId, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pendingQuota, err := model.SumUserPendingAffiliateRebateQuota(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetItems(rebates)
	pageInfo.SetTotal(int(total))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":         pageInfo.Items,
			"total":         pageInfo.Total,
			"pending_quota": pendingQuota,
		},
	})
}

func GetUserInvitees(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)

	invitees, total, err := model.GetUserInvitees(userId, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetItems(invitees)
	pageInfo.SetTotal(int(total))
	common.ApiSuccess(c, pageInfo)
}
