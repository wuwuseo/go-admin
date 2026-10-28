package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"go-admin/internal/ent/enttest"
	"go-admin/internal/ent/migrate"
	"go-admin/internal/global"

	_ "github.com/mattn/go-sqlite3"
)

func TestSlideGroupService_CreateAndList(t *testing.T) {
	slideSvc, groupSvc, cleanup := setupTestSlideGroupDB(t)
	defer cleanup()
	ctx := t.Context()

	group, err := groupSvc.Create(ctx, &CreateSlideGroupRequest{
		Label: "活动页",
		Value: "activity",
	})
	if err != nil {
		t.Fatalf("创建分组失败: %v", err)
	}
	if group.SlideCount != 0 {
		t.Fatalf("新建分组幻灯片数量 = %d, want 0", group.SlideCount)
	}

	if _, err := slideSvc.Create(ctx, &CreateSlideRequest{
		Title:     "活动横幅",
		Image:     "/uploads/slides/a.png",
		GroupCode: "activity",
	}); err != nil {
		t.Fatalf("创建幻灯片失败: %v", err)
	}

	groups, err := groupSvc.List(ctx)
	if err != nil {
		t.Fatalf("获取分组列表失败: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("分组数量 = %d, want 1", len(groups))
	}
	if groups[0].Value != "activity" || groups[0].SlideCount != 1 {
		t.Fatalf("分组数据异常: %+v", groups[0])
	}
}

func TestSlideGroupService_CreateRejectsInvalidAndDuplicateValue(t *testing.T) {
	_, groupSvc, cleanup := setupTestSlideGroupDB(t)
	defer cleanup()
	ctx := t.Context()

	if _, err := groupSvc.Create(ctx, &CreateSlideGroupRequest{
		Label: "非法编码",
		Value: "1中文",
	}); !errors.Is(err, ErrSlideGroupValueInvalid) {
		t.Fatalf("非法编码错误 = %v, want ErrSlideGroupValueInvalid", err)
	}

	if _, err := groupSvc.Create(ctx, &CreateSlideGroupRequest{
		Label: "首页",
		Value: "home",
	}); err != nil {
		t.Fatalf("创建分组失败: %v", err)
	}

	if _, err := groupSvc.Create(ctx, &CreateSlideGroupRequest{
		Label: "首页重复",
		Value: "home",
	}); !errors.Is(err, ErrSlideGroupValueExists) {
		t.Fatalf("重复编码错误 = %v, want ErrSlideGroupValueExists", err)
	}
}

func TestSlideGroupService_UpdateValueMigratesSlides(t *testing.T) {
	slideSvc, groupSvc, cleanup := setupTestSlideGroupDB(t)
	defer cleanup()
	ctx := t.Context()

	group, err := groupSvc.Create(ctx, &CreateSlideGroupRequest{
		Label: "首页",
		Value: "home",
	})
	if err != nil {
		t.Fatalf("创建分组失败: %v", err)
	}

	slide, err := slideSvc.Create(ctx, &CreateSlideRequest{
		Title:     "首页横幅",
		Image:     "/uploads/slides/a.png",
		GroupCode: "home",
	})
	if err != nil {
		t.Fatalf("创建幻灯片失败: %v", err)
	}

	newValue := "homepage"
	newLabel := "首页(新)"
	if err := groupSvc.Update(ctx, group.ID, &UpdateSlideGroupRequest{
		Label: &newLabel,
		Value: &newValue,
	}); err != nil {
		t.Fatalf("更新分组失败: %v", err)
	}

	stored, err := slideSvc.Get(ctx, slide.ID)
	if err != nil {
		t.Fatalf("读取幻灯片失败: %v", err)
	}
	if stored.GroupCode != "homepage" {
		t.Fatalf("幻灯片分组编码 = %q, want homepage", stored.GroupCode)
	}

	groups, err := groupSvc.List(ctx)
	if err != nil {
		t.Fatalf("获取分组列表失败: %v", err)
	}
	if len(groups) != 1 || groups[0].Value != "homepage" || groups[0].SlideCount != 1 {
		t.Fatalf("分组数据异常: %+v", groups)
	}
}

func TestSlideGroupService_UpdateRejectsDuplicateValue(t *testing.T) {
	_, groupSvc, cleanup := setupTestSlideGroupDB(t)
	defer cleanup()
	ctx := t.Context()

	first, err := groupSvc.Create(ctx, &CreateSlideGroupRequest{Label: "首页", Value: "home"})
	if err != nil {
		t.Fatalf("创建分组失败: %v", err)
	}
	if _, err := groupSvc.Create(ctx, &CreateSlideGroupRequest{Label: "活动", Value: "activity"}); err != nil {
		t.Fatalf("创建分组失败: %v", err)
	}

	dup := "activity"
	if err := groupSvc.Update(ctx, first.ID, &UpdateSlideGroupRequest{Value: &dup}); !errors.Is(err, ErrSlideGroupValueExists) {
		t.Fatalf("重复编码错误 = %v, want ErrSlideGroupValueExists", err)
	}
}

func TestSlideGroupService_DeleteRejectsGroupInUse(t *testing.T) {
	slideSvc, groupSvc, cleanup := setupTestSlideGroupDB(t)
	defer cleanup()
	ctx := t.Context()

	group, err := groupSvc.Create(ctx, &CreateSlideGroupRequest{Label: "首页", Value: "home"})
	if err != nil {
		t.Fatalf("创建分组失败: %v", err)
	}
	slide, err := slideSvc.Create(ctx, &CreateSlideRequest{
		Title:     "首页横幅",
		Image:     "/uploads/slides/a.png",
		GroupCode: "home",
	})
	if err != nil {
		t.Fatalf("创建幻灯片失败: %v", err)
	}

	if err := groupSvc.Delete(ctx, group.ID); !errors.Is(err, ErrSlideGroupInUse) {
		t.Fatalf("删除被占用分组错误 = %v, want ErrSlideGroupInUse", err)
	}

	if err := slideSvc.Delete(ctx, slide.ID); err != nil {
		t.Fatalf("删除幻灯片失败: %v", err)
	}
	if err := groupSvc.Delete(ctx, group.ID); err != nil {
		t.Fatalf("删除分组失败: %v", err)
	}

	groups, err := groupSvc.List(ctx)
	if err != nil {
		t.Fatalf("获取分组列表失败: %v", err)
	}
	if len(groups) != 0 {
		t.Fatalf("删除后分组数量 = %d, want 0", len(groups))
	}
}

func TestSlideGroupService_UpdateMissingGroup(t *testing.T) {
	_, groupSvc, cleanup := setupTestSlideGroupDB(t)
	defer cleanup()
	ctx := t.Context()

	label := "不存在"
	if err := groupSvc.Update(ctx, 9999, &UpdateSlideGroupRequest{Label: &label}); !errors.Is(err, ErrSlideGroupNotFound) {
		t.Fatalf("更新不存在分组错误 = %v, want ErrSlideGroupNotFound", err)
	}
	if err := groupSvc.Delete(ctx, 9999); !errors.Is(err, ErrSlideGroupNotFound) {
		t.Fatalf("删除不存在分组错误 = %v, want ErrSlideGroupNotFound", err)
	}
}

func setupTestSlideGroupDB(t *testing.T) (*SlideService, *SlideGroupService, func()) {
	t.Helper()
	// 每个用例使用独立的内存库名,避免共享缓存导致用例间数据串味
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1",
		strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	client := enttest.Open(t, "sqlite3", dsn,
		enttest.WithMigrateOptions(migrate.WithForeignKeys(false)),
	)

	prevORM := global.ORM
	global.ORM = client

	cleanup := func() {
		global.ORM = prevORM
		client.Close()
	}
	return NewSlideService(), NewSlideGroupService(), cleanup
}
