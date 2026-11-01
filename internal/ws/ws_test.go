package ws

import (
	"context"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	cmfws "github.com/wuwuseo/cmf/ws"

	"go-admin/internal/ent/enttest"
	"go-admin/internal/ent/migrate"
)

// TestMessageHookPushesEnvelope 验证：单条消息落库后推送 message.created，广播消息不推
func TestMessageHookPushesEnvelope(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ws_hook?mode=memory&cache=shared&_fk=1",
		enttest.WithMigrateOptions(migrate.WithForeignKeys(false)))
	defer client.Close()

	var captured []string
	previous := sendEnvelope
	sendEnvelope = func(address string, ev cmfws.Event) {
		captured = append(captured, address)
	}
	t.Cleanup(func() { sendEnvelope = previous })

	client.Message.Use(MessageHook())

	msg, err := client.Message.Create().
		SetUserUID("u1").
		SetType("system").
		SetTitle("测试").
		SetContent("内容").
		Save(context.Background())
	if err != nil {
		t.Fatalf("创建消息失败: %v", err)
	}
	if len(captured) != 1 || captured[0] != "u1" {
		t.Fatalf("推送地址 = %+v, 期望 [u1]", captured)
	}
	if msg.ID == 0 || msg.Type != "system" {
		t.Fatalf("消息落库异常: %+v", msg)
	}

	captured = nil
	if _, err := client.Message.Create().
		SetType("system").
		SetTitle("广播").
		SetContent("内容").
		Save(context.Background()); err != nil {
		t.Fatalf("创建广播消息失败: %v", err)
	}
	if len(captured) != 0 {
		t.Fatalf("广播消息不应推送, got %+v", captured)
	}
}

// TestTicketAddressesIncludeProvider 验证 ticket 地址 = 自身 uid + provider 返回值
func TestTicketAddressesIncludeProvider(t *testing.T) {
	RegisterAddressProvider(func(ctx context.Context, uid string) ([]string, error) {
		return []string{"customer:1", uid}, nil
	})

	got := addressesFor(context.Background(), "u1")
	if len(got) != 2 || got[0] != "u1" || got[1] != "customer:1" {
		t.Fatalf("地址汇总 = %+v, 期望 [u1 customer:1]", got)
	}
	if got := addressesFor(context.Background(), ""); len(got) != 0 {
		t.Fatalf("空 uid 不应返回地址, got %+v", got)
	}
}
