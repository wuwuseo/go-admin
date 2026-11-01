// Package ws 实时消息通知装配层：Hub 单例、ticket 端点、地址 provider 与 Ent 消息 hook。
// 只依赖 cmf/ws 协议层与站内消息表，不 import 任何功能集。
package ws

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/wuwuseo/cmf/log"
	cmfws "github.com/wuwuseo/cmf/ws"

	"go-admin/internal/ent/message"
	"go-admin/internal/global"
)

var (
	hub            *cmfws.Hub
	tickets        *cmfws.TicketStore
	connectHandler fiber.Handler
	logger         log.Logger
)

// Init 装配实时通知：Hub、ticket 存储、未读统计与 ent 消息 hook
func Init(lg log.Logger) {
	logger = lg
	tickets = cmfws.NewTicketStore(2 * time.Minute)
	hub = cmfws.NewHub(cmfws.Config{
		Logger:        lg,
		Tickets:       tickets,
		UnreadCounter: unreadCount,
	})
	connectHandler = cmfws.NewConnectHandler(hub)
	go tickets.StartCleanup(context.Background())
	if global.ORM != nil {
		// hook 是「功能集零接入」的关键：任何功能集写一条 ent.Message 即自动获得实时提示
		global.ORM.Message.Use(MessageHook())
	}
}

// Shutdown 关闭全部 WebSocket 连接（未初始化时为空操作）
func Shutdown() {
	if hub != nil {
		hub.Shutdown()
	}
}

// RealtimeHub 返回进程内 Hub 单例，供功能集广播使用（未初始化时为 nil）
func RealtimeHub() *cmfws.Hub {
	return hub
}

// unreadCount 统计地址维度的未读数
// ponytail: 与 /messages/unread-count 的口径不同（后者把 user_uid="" 的广播也算进来），
// 管理端未读数仍以 SSE + 列表接口为准；需要统一时把 message.UserUIDEQ("") 并入查询即可
func unreadCount(ctx context.Context, addresses []string) (int, error) {
	if global.ORM == nil || len(addresses) == 0 {
		return 0, nil
	}
	return global.ORM.Message.Query().
		Where(message.UserUIDIn(addresses...), message.ReadAtIsNil()).
		Count(ctx)
}

// Connect WebSocket 连接端点：ticket 在升级前校验，失败返回 401 不升级
func Connect(c fiber.Ctx) error {
	if connectHandler == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("ws handler 未初始化")
	}
	return connectHandler(c)
}
