package ws

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

var (
	providersMu sync.RWMutex
	providers   []func(ctx context.Context, uid string) ([]string, error)
)

// RegisterAddressProvider 注册地址提供者：功能集把自己的身份前缀（如 customer: / player:）拼进来。
// address 对框架层是不透明字符串；单条连接一次性绑定全部地址。
func RegisterAddressProvider(fn func(ctx context.Context, uid string) ([]string, error)) {
	if fn == nil {
		return
	}
	providersMu.Lock()
	defer providersMu.Unlock()
	providers = append(providers, fn)
}

// addressesFor 汇总 uid 自身与全部 provider 返回的地址（去重、去空）
func addressesFor(ctx context.Context, uid string) []string {
	if uid == "" {
		return nil
	}
	list := []string{uid}
	providersMu.RLock()
	fns := providers
	providersMu.RUnlock()
	for _, fn := range fns {
		addrs, err := fn(ctx, uid)
		if err != nil {
			// 单个 provider 失败不影响其它身份，降级为只用自己的 uid
			if logger != nil {
				logger.Warn("WS 地址解析失败", zap.String("uid", uid), zap.Error(err))
			}
			continue
		}
		list = append(list, addrs...)
	}

	seen := make(map[string]struct{}, len(list))
	out := make([]string, 0, len(list))
	for _, addr := range list {
		if addr == "" {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		out = append(out, addr)
	}
	return out
}
