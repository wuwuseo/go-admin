package ws

import (
	"github.com/gofiber/fiber/v3"
	"github.com/wuwuseo/cmf/jwt"
	"go.uber.org/zap"

	"go-admin/internal/global"
	"go-admin/internal/utils"
)

// IssueTicket 颁发实时通知 ticket（管理端与用户端共用，身份来自 JWT）
// @Summary 颁发实时通知 ticket
// @Description 一次性 ticket，2 分钟内有效，用于建立 WebSocket 连接
// @Tags 站内消息
// @Produce json
// @Success 200 {object} utils.Response{data=map[string]any}
// @Router /api/v1/admin/auth/realtime/ticket [post]
// @Router /api/v1/auth/realtime/ticket [post]
func IssueTicket(c fiber.Ctx) error {
	resp := utils.NewResponse()

	claims := jwt.GetJWTUserData(c)
	uid, _ := claims["id"].(string)
	if uid == "" {
		return resp.Fail(global.ResponseUnauthorized, "缺少用户身份").ToJSON(c)
	}
	if tickets == nil {
		return resp.Fail(global.ResponseInternalServerError, "实时通知凭证服务未初始化").ToJSON(c)
	}

	addresses := addressesFor(c.Context(), uid)
	ticket, expireAt, err := tickets.Issue(addresses)
	if err != nil {
		return resp.Fail(global.ResponseInternalServerError, err.Error()).ToJSON(c)
	}
	if logger != nil {
		logger.Info("WS ticket 颁发",
			zap.String("user_uid", uid),
			zap.Int("addresses", len(addresses)),
		)
	}
	return resp.Success(fiber.Map{"ticket": ticket, "expire_at": expireAt}).ToJSON(c)
}
