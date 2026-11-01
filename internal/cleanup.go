package internal

import (
	"context"
	"errors"
	"time"

	"go-admin/internal/global"
	adminqueue "go-admin/internal/queue"
	"go-admin/internal/service"
	"go-admin/internal/utils"
	"go-admin/internal/ws"

	"github.com/wuwuseo/cmf/log"
	"go.uber.org/zap"
)

func Cleanup() error {
	var pluginCloseErr error

	// 持久化 UID 计数器，确保重启后连续
	utils.SaveUID()
	log.Info("UID 计数器已持久化")

	// 优雅关闭前执行最后一次过期分片会话清理
	service.CleanupExpiredSessions()
	log.Info("已执行最后一次过期分片会话清理")

	// 停止分片上传会话清理协程
	service.StopChunkSessionCleanup()
	log.Info("分片上传会话清理协程已停止")

	// 停止定时任务管理器（在关闭 DB 之前）
	if global.CronManager != nil {
		global.CronManager.Stop()
		log.Info("定时任务管理器已停止")
	}

	// 优雅停止任务队列（在关闭 DB 之前；在途任务最长等待 30 秒）
	adminqueue.Shutdown(30 * time.Second)
	log.Info("任务队列已停止")

	if activePluginManager != nil {
		pluginCtx, pluginCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := activePluginManager.Close(pluginCtx); err != nil {
			pluginCloseErr = err
			log.Error("插件运行时关闭失败", zap.Error(err))
		} else {
			log.Info("插件运行时已停止")
		}
		pluginCancel()
		activePluginManager = nil
	}

	// 优雅关闭操作日志服务（在关闭 DB 之前）
	operationLogCtx, operationLogCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer operationLogCancel()
	if err := service.ShutdownDefaultOperationLogService(operationLogCtx); err != nil {
		log.Error("操作日志服务关闭失败", zap.Error(err))
		return errors.Join(pluginCloseErr, err)
	}
	log.Info("操作日志服务已停止")

	// 优雅关闭登录日志服务（在关闭 DB 之前）
	if err := service.ShutdownDefaultLoginLogService(operationLogCtx); err != nil {
		log.Error("登录日志服务关闭失败", zap.Error(err))
		return errors.Join(pluginCloseErr, err)
	}
	log.Info("登录日志服务已停止")

	// 关闭 WebSocket 实时通知连接（在关闭 DB 之前）
	ws.Shutdown()
	log.Info("WebSocket 实时通知已关闭")

	// 清理 Wire 注入的数据库连接池
	if appInstance != nil && appInstance.DB != nil {
		if err := appInstance.DB.Close(); err != nil {
			log.Error("关闭数据库连接池失败", zap.Error(err))
			return errors.Join(pluginCloseErr, err)
		}
		log.Info("数据库连接池已关闭")
	}

	// 清理 Ent client
	if global.ORM != nil {
		if err := global.ORM.Close(); err != nil {
			log.Error("关闭 Ent client 失败", zap.Error(err))
			return errors.Join(pluginCloseErr, err)
		}
		global.ORM = nil
		log.Info("Ent client 已关闭")
	}

	return pluginCloseErr
}
