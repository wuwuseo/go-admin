package ws

import (
	"context"

	"go.uber.org/zap"

	cmfws "github.com/wuwuseo/cmf/ws"

	"go-admin/internal/ent"
)

// sendEnvelope 默认走进程内单例 Hub；测试中可替换以捕获事件
var sendEnvelope = func(address string, ev cmfws.Event) {
	if h := RealtimeHub(); h != nil {
		h.Send(address, ev)
	}
}

// MessageHook 消息落库后推送信封：只带 id / 分类 / 未读数，不传正文。
// ponytail: hook 在事务内触发，回滚时会多推一条提示；前端收到后重新拉列表，
// 列表里没有该条、未读数由服务端重算，用户无感。若将来要求严格一致，改为事务提交后推送。
// 已知边界：CreateBulk 不触发 hook，当前批量写入路径未接入实时提示。
func MessageHook() ent.Hook {
	return func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			value, err := next.Mutate(ctx, m)
			if err != nil {
				return value, err
			}
			if _, ok := m.(*ent.MessageMutation); !ok || !m.Op().Is(ent.OpCreate) {
				return value, nil
			}
			uid, _ := m.(*ent.MessageMutation).UserUID()
			// ponytail: 广播消息（user_uid=""）在进程内 Hub 里没有确定的接收地址，
			// 不推实时提示；前端靠列表接口兜底。需要时改为向「全地址」房间推送或按连接登记。
			if uid == "" {
				return value, nil
			}
			entity, ok := value.(*ent.Message)
			if !ok {
				return value, nil
			}

			unread := 0
			if n, countErr := unreadCount(ctx, []string{uid}); countErr == nil {
				unread = n
			} else if logger != nil {
				logger.Warn("WS 未读数统计失败", zap.Int("message_id", entity.ID), zap.Error(countErr))
			}

			sendEnvelope(uid, cmfws.Event{
				Type: cmfws.TypeMessageCreated,
				Data: map[string]any{
					"id":       entity.ID,
					"category": entity.Type,
					"unread":   unread,
				},
			})
			return value, nil
		})
	}
}
