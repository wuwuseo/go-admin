package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"go-admin/internal/ent"
	"go-admin/internal/ent/migrate"
	"go-admin/internal/ent/systempermission"
	"go-admin/internal/global"

	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func setupSystemPermissionTest(t *testing.T) {
	db, err := sql.Open("sqlite", "file:ent?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	drv := entsql.OpenDB("sqlite3", db)
	client := ent.NewClient(ent.Driver(drv))
	if err := client.Schema.Create(context.Background(), migrate.WithForeignKeys(false)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	prevORM := global.ORM
	global.ORM = client
	t.Cleanup(func() { global.ORM = prevORM })
}

func TestSystemPermissionService_Create(t *testing.T) {
	setupSystemPermissionTest(t)
	svc := NewSystemPermissionService()
	ctx := withPermissionSystemOperator(context.Background())

	t.Run("成功创建菜单权限", func(t *testing.T) {
		req := &CreateSystemPermissionRequest{
			Domain: "default",
			Type:   SystemPermissionTypeMenu,
			Name:   "测试菜单",
			Code:   "test:menu",
			Status: 1,
		}
		resp, err := svc.Create(ctx, "", req)
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if resp.ID == 0 {
			t.Error("ID不应为0")
		}
		if resp.Name != "测试菜单" {
			t.Errorf("名称不匹配: 期望 '测试菜单', 得到 '%s'", resp.Name)
		}
		if resp.Code != "test:menu" {
			t.Errorf("代码不匹配: 期望 'test:menu', 得到 '%s'", resp.Code)
		}
		if resp.CreatedAt == "" {
			t.Error("创建时间不应为空")
		}
		if resp.UpdatedAt == "" {
			t.Error("更新时间不应为空")
		}
	})

	t.Run("重复Code+Domain返回错误", func(t *testing.T) {
		req1 := &CreateSystemPermissionRequest{
			Domain: "default",
			Type:   SystemPermissionTypeMenu,
			Name:   "权限A",
			Code:   "test:dup",
			Status: 1,
		}
		_, err := svc.Create(ctx, "", req1)
		if err != nil {
			t.Fatalf("第一次创建失败: %v", err)
		}

		req2 := &CreateSystemPermissionRequest{
			Domain: "default",
			Type:   SystemPermissionTypeMenu,
			Name:   "权限B",
			Code:   "test:dup",
			Status: 1,
		}
		_, err = svc.Create(ctx, "", req2)
		if err == nil {
			t.Fatal("期望重复代码错误，但没有收到错误")
		}
		if !strings.Contains(err.Error(), "权限代码已存在") {
			t.Errorf("错误信息不匹配: %v", err)
		}
	})

	t.Run("包含所有可选字段", func(t *testing.T) {
		req := &CreateSystemPermissionRequest{
			Domain:      "admin",
			Type:        SystemPermissionTypeAPI,
			Name:        "完整权限",
			Code:        "test:full",
			Path:        "/api/v1/test",
			Method:      "GET",
			MenuPath:    "/test",
			Icon:        "icon-test",
			Component:   "test/index",
			ParentID:    0,
			SortOrder:   10,
			IsCommon:    true,
			Status:      1,
			Description: "这是一个测试描述",
		}
		resp, err := svc.Create(ctx, "", req)
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if resp.Domain != "admin" {
			t.Errorf("域不匹配: 期望 'admin', 得到 '%s'", resp.Domain)
		}
		if resp.Type != SystemPermissionTypeAPI {
			t.Errorf("类型不匹配: 期望 'api', 得到 '%s'", resp.Type)
		}
		if resp.Path != "/api/v1/test" {
			t.Errorf("路径不匹配: 期望 '/api/v1/test', 得到 '%s'", resp.Path)
		}
		if resp.Method != "GET" {
			t.Errorf("方法不匹配: 期望 'GET', 得到 '%s'", resp.Method)
		}
		if resp.MenuPath != "/test" {
			t.Errorf("菜单路径不匹配: 期望 '/test', 得到 '%s'", resp.MenuPath)
		}
		if resp.Icon != "icon-test" {
			t.Errorf("图标不匹配: 期望 'icon-test', 得到 '%s'", resp.Icon)
		}
		if resp.Component != "test/index" {
			t.Errorf("组件不匹配: 期望 'test/index', 得到 '%s'", resp.Component)
		}
		if resp.SortOrder != 10 {
			t.Errorf("排序不匹配: 期望 10, 得到 %d", resp.SortOrder)
		}
		if !resp.IsCommon {
			t.Error("期望 IsCommon 为 true")
		}
		if resp.Description != "这是一个测试描述" {
			t.Errorf("描述不匹配: 期望 '这是一个测试描述', 得到 '%s'", resp.Description)
		}
	})
}

func TestSystemPermissionService_GetByID(t *testing.T) {
	setupSystemPermissionTest(t)
	svc := NewSystemPermissionService()
	ctx := withPermissionSystemOperator(context.Background())

	t.Run("查询存在的权限", func(t *testing.T) {
		createReq := &CreateSystemPermissionRequest{
			Domain:      "default",
			Type:        SystemPermissionTypeMenu,
			Name:        "查询测试",
			Code:        "test:get",
			Description: "测试描述",
			Status:      1,
		}
		created, err := svc.Create(ctx, "", createReq)
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		resp, err := svc.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if resp.ID != created.ID {
			t.Errorf("ID不匹配: 期望 %d, 得到 %d", created.ID, resp.ID)
		}
		if resp.Name != "查询测试" {
			t.Errorf("名称不匹配: 期望 '查询测试', 得到 '%s'", resp.Name)
		}
		if resp.Code != "test:get" {
			t.Errorf("代码不匹配: 期望 'test:get', 得到 '%s'", resp.Code)
		}
		if resp.Description != "测试描述" {
			t.Errorf("描述不匹配: 期望 '测试描述', 得到 '%s'", resp.Description)
		}
	})

	t.Run("查询不存在的权限返回错误", func(t *testing.T) {
		_, err := svc.GetByID(ctx, 99999)
		if err == nil {
			t.Fatal("期望权限不存在错误，但没有收到错误")
		}
		if !strings.Contains(err.Error(), "权限不存在") {
			t.Errorf("错误信息不匹配: %v", err)
		}
	})
}

func TestSystemPermissionService_List(t *testing.T) {
	setupSystemPermissionTest(t)
	svc := NewSystemPermissionService()
	ctx := withPermissionSystemOperator(context.Background())

	seedData := []struct {
		Domain   string
		Type     SystemPermissionType
		Name     string
		Code     string
		Status   int8
		IsCommon bool
	}{
		{"default", SystemPermissionTypeMenu, "仪表盘", "dashboard", 1, true},
		{"default", SystemPermissionTypeMenu, "系统管理", "system", 1, true},
		{"default", SystemPermissionTypeAPI, "获取用户", "user:get", 1, false},
		{"default", SystemPermissionTypeAPI, "创建用户", "user:create", 1, false},
		{"default", SystemPermissionTypeButton, "删除按钮", "user:delete:btn", 0, false},
		{"admin", SystemPermissionTypeMenu, "管理菜单", "admin:menu", 1, false},
		{"admin", SystemPermissionTypeAPI, "管理API", "admin:api", 1, false},
	}
	for _, s := range seedData {
		_, err := svc.Create(ctx, "", &CreateSystemPermissionRequest{
			Domain:   s.Domain,
			Type:     s.Type,
			Name:     s.Name,
			Code:     s.Code,
			Status:   s.Status,
			IsCommon: s.IsCommon,
		})
		if err != nil {
			t.Fatalf("创建种子数据失败: %v", err)
		}
	}

	t.Run("无筛选条件返回全部", func(t *testing.T) {
		resp, err := svc.List(ctx, "", &ListSystemPermissionsRequest{})
		if err != nil {
			t.Fatalf("查询列表失败: %v", err)
		}
		if resp.Total != 7 {
			t.Errorf("期望总数 7, 得到 %d", resp.Total)
		}
		if len(resp.Data) != 7 {
			t.Errorf("期望数据条数 7, 得到 %d", len(resp.Data))
		}
	})

	t.Run("域筛选同时返回IsCommon=true项", func(t *testing.T) {
		req := &ListSystemPermissionsRequest{
			Domain: "admin",
		}
		resp, err := svc.List(ctx, "", req)
		if err != nil {
			t.Fatalf("查询列表失败: %v", err)
		}
		for _, d := range resp.Data {
			if d.Domain != "admin" && !d.IsCommon {
				t.Errorf("期望域为 'admin' 或是公共权限, 得到域 '%s', IsCommon=%v", d.Domain, d.IsCommon)
			}
		}
	})

	t.Run("类型筛选", func(t *testing.T) {
		req := &ListSystemPermissionsRequest{
			Type: SystemPermissionTypeAPI,
		}
		resp, err := svc.List(ctx, "", req)
		if err != nil {
			t.Fatalf("查询列表失败: %v", err)
		}
		if resp.Total != 3 {
			t.Errorf("期望 API 类型总数 3, 得到 %d", resp.Total)
		}
		for _, d := range resp.Data {
			if d.Type != SystemPermissionTypeAPI {
				t.Errorf("期望类型 'api', 得到 '%s'", d.Type)
			}
		}
	})

	t.Run("关键字筛选匹配名称和代码", func(t *testing.T) {
		req := &ListSystemPermissionsRequest{
			ListRequest: ListRequest{
				Keyword: "user",
			},
		}
		resp, err := svc.List(ctx, "", req)
		if err != nil {
			t.Fatalf("查询列表失败: %v", err)
		}
		if resp.Total != 3 {
			t.Errorf("期望匹配 'user' 关键字结果 3 条, 得到 %d", resp.Total)
		}
		for _, d := range resp.Data {
			if !strings.Contains(d.Name, "user") && !strings.Contains(d.Code, "user") {
				t.Errorf("期望名称或代码包含 'user', 得到名称 '%s' 代码 '%s'", d.Name, d.Code)
			}
		}
	})

	t.Run("状态筛选启用权限", func(t *testing.T) {
		req := &ListSystemPermissionsRequest{
			Status: 1,
		}
		resp, err := svc.List(ctx, "", req)
		if err != nil {
			t.Fatalf("查询列表失败: %v", err)
		}
		if resp.Total != 6 {
			t.Errorf("期望状态1总数 6, 得到 %d", resp.Total)
		}
		for _, d := range resp.Data {
			if d.Status != 1 {
				t.Errorf("期望状态 1, 得到状态 %d, 代码 '%s'", d.Status, d.Code)
			}
		}
	})
}

func TestSystemPermissionService_Update(t *testing.T) {
	setupSystemPermissionTest(t)
	svc := NewSystemPermissionService()
	ctx := withPermissionSystemOperator(context.Background())

	t.Run("成功更新名称和排序", func(t *testing.T) {
		createReq := &CreateSystemPermissionRequest{
			Domain:    "default",
			Type:      SystemPermissionTypeMenu,
			Name:      "原始名称",
			Code:      "test:update",
			SortOrder: 5,
			Status:    1,
		}
		created, err := svc.Create(ctx, "", createReq)
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		sortOrder := 99
		updateReq := &UpdateSystemPermissionRequest{
			Name:      "新名称",
			SortOrder: &sortOrder,
		}
		err = svc.Update(ctx, "", created.ID, updateReq)
		if err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		updated, err := svc.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if updated.Name != "新名称" {
			t.Errorf("名称未更新: 期望 '新名称', 得到 '%s'", updated.Name)
		}
		if updated.SortOrder != 99 {
			t.Errorf("排序未更新: 期望 99, 得到 %d", updated.SortOrder)
		}
	})

	t.Run("更新不存在的权限返回错误", func(t *testing.T) {
		req := &UpdateSystemPermissionRequest{
			Name: "不存在",
		}
		err := svc.Update(ctx, "", 99999, req)
		if err == nil {
			t.Fatal("期望权限不存在错误，但没有收到错误")
		}
		if !strings.Contains(err.Error(), "权限不存在") {
			t.Errorf("错误信息不匹配: %v", err)
		}
	})
}

func TestSystemPermissionService_Delete(t *testing.T) {
	setupSystemPermissionTest(t)
	svc := NewSystemPermissionService()
	ctx := withPermissionSystemOperator(context.Background())

	t.Run("成功删除叶子权限", func(t *testing.T) {
		createReq := &CreateSystemPermissionRequest{
			Domain: "default",
			Type:   SystemPermissionTypeMenu,
			Name:   "待删除",
			Code:   "test:delete:leaf",
			Status: 1,
		}
		created, err := svc.Create(ctx, "", createReq)
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		err = svc.Delete(ctx, "", created.ID)
		if err != nil {
			t.Fatalf("删除失败: %v", err)
		}

		_, err = svc.GetByID(ctx, created.ID)
		if err == nil {
			t.Fatal("期望权限已删除，但仍能查到")
		}
		if !strings.Contains(err.Error(), "权限不存在") {
			t.Errorf("错误信息不匹配: %v", err)
		}
	})

	t.Run("有子权限时删除失败", func(t *testing.T) {
		parentReq := &CreateSystemPermissionRequest{
			Domain: "default",
			Type:   SystemPermissionTypeMenu,
			Name:   "父权限",
			Code:   "test:delete:parent",
			Status: 1,
		}
		parent, err := svc.Create(ctx, "", parentReq)
		if err != nil {
			t.Fatalf("创建父权限失败: %v", err)
		}

		childReq := &CreateSystemPermissionRequest{
			Domain:   "default",
			Type:     SystemPermissionTypeMenu,
			Name:     "子权限",
			Code:     "test:delete:child",
			ParentID: parent.ID,
			Status:   1,
		}
		_, err = svc.Create(ctx, "", childReq)
		if err != nil {
			t.Fatalf("创建子权限失败: %v", err)
		}

		err = svc.Delete(ctx, "", parent.ID)
		if err == nil {
			t.Fatal("期望删除失败，但成功了")
		}
		if !strings.Contains(err.Error(), "请先删除子权限") {
			t.Errorf("错误信息不匹配: %v", err)
		}
	})

	t.Run("删除不存在的权限返回错误", func(t *testing.T) {
		err := svc.Delete(ctx, "", 99999)
		if err == nil {
			t.Fatal("期望权限不存在错误，但没有收到错误")
		}
		if !strings.Contains(err.Error(), "权限不存在") {
			t.Errorf("错误信息不匹配: %v", err)
		}
	})
}

func TestSystemPermissionService_GetTree(t *testing.T) {
	setupSystemPermissionTest(t)
	svc := NewSystemPermissionService()
	ctx := withPermissionSystemOperator(context.Background())

	t.Run("构建正确的树结构", func(t *testing.T) {
		parentReq := &CreateSystemPermissionRequest{
			Domain:    "default",
			Type:      SystemPermissionTypeMenu,
			Name:      "父菜单",
			Code:      "tree:parent",
			SortOrder: 1,
			Status:    1,
		}
		parent, err := svc.Create(ctx, "", parentReq)
		if err != nil {
			t.Fatalf("创建父权限失败: %v", err)
		}

		child1Req := &CreateSystemPermissionRequest{
			Domain:    "default",
			Type:      SystemPermissionTypeMenu,
			Name:      "子菜单1",
			Code:      "tree:child1",
			ParentID:  parent.ID,
			SortOrder: 1,
			Status:    1,
		}
		_, err = svc.Create(ctx, "", child1Req)
		if err != nil {
			t.Fatalf("创建子权限1失败: %v", err)
		}

		child2Req := &CreateSystemPermissionRequest{
			Domain:    "default",
			Type:      SystemPermissionTypeMenu,
			Name:      "子菜单2",
			Code:      "tree:child2",
			ParentID:  parent.ID,
			SortOrder: 2,
			Status:    1,
		}
		_, err = svc.Create(ctx, "", child2Req)
		if err != nil {
			t.Fatalf("创建子权限2失败: %v", err)
		}

		orphanReq := &CreateSystemPermissionRequest{
			Domain:    "default",
			Type:      SystemPermissionTypeMenu,
			Name:      "独立菜单",
			Code:      "tree:orphan",
			SortOrder: 0,
			Status:    1,
		}
		_, err = svc.Create(ctx, "", orphanReq)
		if err != nil {
			t.Fatalf("创建独立权限失败: %v", err)
		}

		nodes, err := svc.GetTree(ctx, "", "default")
		if err != nil {
			t.Fatalf("获取树失败: %v", err)
		}

		if len(nodes) != 2 {
			t.Fatalf("期望根节点数 2, 得到 %d", len(nodes))
		}

		foundParent := false
		foundOrphan := false
		for _, node := range nodes {
			if node.Name == "父菜单" {
				foundParent = true
				if len(node.Children) != 2 {
					t.Errorf("父菜单期望 2 个子节点, 得到 %d", len(node.Children))
				}
			}
			if node.Name == "独立菜单" {
				foundOrphan = true
				if len(node.Children) != 0 {
					t.Errorf("独立菜单期望 0 个子节点, 得到 %d", len(node.Children))
				}
			}
		}
		if !foundParent {
			t.Error("未找到父菜单节点")
		}
		if !foundOrphan {
			t.Error("未找到独立菜单节点")
		}
	})

	t.Run("空域返回所有权限树", func(t *testing.T) {
		adminReq := &CreateSystemPermissionRequest{
			Domain:    "admin",
			Type:      SystemPermissionTypeMenu,
			Name:      "管理域菜单",
			Code:      "tree:all:admin",
			SortOrder: 3,
			Status:    1,
		}
		_, err := svc.Create(ctx, "", adminReq)
		if err != nil {
			t.Fatalf("创建admin域权限失败: %v", err)
		}

		nodes, err := svc.GetTree(ctx, "", "")
		if err != nil {
			t.Fatalf("获取树失败: %v", err)
		}

		foundDefault := false
		foundAdmin := false
		for _, node := range nodes {
			if node.Name == "父菜单" {
				foundDefault = true
			}
			if node.Name == "管理域菜单" {
				foundAdmin = true
			}
		}
		if !foundDefault {
			t.Error("空域查询未找到default域的父菜单节点")
		}
		if !foundAdmin {
			t.Error("空域查询未找到admin域的管理域菜单节点")
		}
	})
}

func TestSystemPermissionService_InitSeedPermissions(t *testing.T) {
	setupSystemPermissionTest(t)
	svc := NewSystemPermissionService()
	ctx := context.Background()

	t.Run("新种子权限被创建", func(t *testing.T) {
		seeds := []SeedPermission{
			{Domain: "default", Type: "menu", Name: "种子菜单1", Code: "seed:menu1", SortOrder: 1, Status: 1},
			{Domain: "default", Type: "menu", Name: "种子菜单2", Code: "seed:menu2", SortOrder: 2, Status: 1},
			{Domain: "admin", Type: "api", Name: "种子API", Code: "seed:api", Path: "/api/seed", Method: "GET", SortOrder: 3, Status: 1},
		}
		err := svc.InitSeedPermissions(ctx, seeds)
		if err != nil {
			t.Fatalf("初始化种子权限失败: %v", err)
		}

		resp, err := svc.List(ctx, "", &ListSystemPermissionsRequest{})
		if err != nil {
			t.Fatalf("查询列表失败: %v", err)
		}
		if resp.Total != 3 {
			t.Errorf("期望创建 3 条种子权限, 得到 %d", resp.Total)
		}

		foundCodes := make(map[string]bool)
		for _, d := range resp.Data {
			foundCodes[d.Code] = true
		}
		for _, seed := range seeds {
			if !foundCodes[seed.Code] {
				t.Errorf("未找到种子权限代码 '%s'", seed.Code)
			}
		}
	})

	t.Run("已存在的种子权限被跳过不产生重复", func(t *testing.T) {
		firstSeed := []SeedPermission{
			{Domain: "default", Type: "menu", Name: "已存在菜单", Code: "seed:skip", SortOrder: 10, Status: 1},
			{Domain: "default", Type: "menu", Name: "新菜单", Code: "seed:new", SortOrder: 11, Status: 1},
		}
		err := svc.InitSeedPermissions(ctx, firstSeed)
		if err != nil {
			t.Fatalf("首次初始化种子权限失败: %v", err)
		}

		secondSeed := []SeedPermission{
			{Domain: "default", Type: "menu", Name: "已存在菜单(再次)", Code: "seed:skip", SortOrder: 10, Status: 1},
			{Domain: "default", Type: "menu", Name: "另一个新菜单", Code: "seed:another", SortOrder: 12, Status: 1},
		}
		err = svc.InitSeedPermissions(ctx, secondSeed)
		if err != nil {
			t.Fatalf("再次初始化种子权限失败: %v", err)
		}

		countByCode := make(map[string]int)
		allResp, err := svc.List(ctx, "", &ListSystemPermissionsRequest{})
		if err != nil {
			t.Fatalf("查询列表失败: %v", err)
		}
		for _, d := range allResp.Data {
			countByCode[d.Code]++
		}
		if countByCode["seed:skip"] != 1 {
			t.Errorf("权限代码 'seed:skip' 出现 %d 次, 期望只出现1次(重复应被跳过)", countByCode["seed:skip"])
		}
		if countByCode["seed:new"] != 1 {
			t.Errorf("权限代码 'seed:new' 出现 %d 次, 期望只出现1次", countByCode["seed:new"])
		}
		if countByCode["seed:another"] != 1 {
			t.Errorf("权限代码 'seed:another' 出现 %d 次, 期望只出现1次", countByCode["seed:another"])
		}
	})
}

func TestSystemPermissionService_InitSeedPermissions_ResolvesSeedParentReferences(t *testing.T) {
	setupSystemPermissionTest(t)
	svc := NewSystemPermissionService()
	ctx := context.Background()

	_, err := global.ORM.SystemPermission.Create().
		SetDomain("default").
		SetType("menu").
		SetName("Unrelated").
		SetCode("seed:unrelated").
		SetStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatalf("创建无关权限失败: %v", err)
	}

	seeds := []SeedPermission{
		{ID: 100, Domain: "default", Type: "menu", Name: "Parent", Code: "seed:parent", Status: 1},
		{ID: 200, Domain: "default", Type: "menu", Name: "Child", Code: "seed:child", ParentID: 100, Status: 1},
	}
	if err := svc.InitSeedPermissions(ctx, seeds); err != nil {
		t.Fatalf("初始化种子权限失败: %v", err)
	}

	parent, err := global.ORM.SystemPermission.Query().
		Where(systempermission.DomainEQ("default"), systempermission.CodeEQ("seed:parent")).
		Only(ctx)
	if err != nil {
		t.Fatalf("查询父权限失败: %v", err)
	}
	child, err := global.ORM.SystemPermission.Query().
		Where(systempermission.DomainEQ("default"), systempermission.CodeEQ("seed:child")).
		Only(ctx)
	if err != nil {
		t.Fatalf("查询子权限失败: %v", err)
	}

	if child.ParentID != parent.ID {
		t.Fatalf("child parent_id = %d, want actual parent id %d", child.ParentID, parent.ID)
	}
}

func TestSystemPermissionService_InitSeedPermissions_RepairsExistingParentReferences(t *testing.T) {
	setupSystemPermissionTest(t)
	svc := NewSystemPermissionService()
	ctx := context.Background()

	oldParent, err := global.ORM.SystemPermission.Create().
		SetDomain("default").
		SetType("menu").
		SetName("Old Parent").
		SetCode("seed:old-parent").
		SetStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatalf("创建旧父权限失败: %v", err)
	}
	_, err = global.ORM.SystemPermission.Create().
		SetDomain("default").
		SetType("menu").
		SetName("Existing Child").
		SetCode("seed:existing-child").
		SetParentID(oldParent.ID).
		SetStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatalf("创建已有子权限失败: %v", err)
	}

	seeds := []SeedPermission{
		{ID: 10, Domain: "default", Type: "menu", Name: "New Parent", Code: "seed:new-parent", Status: 1},
		{ID: 11, Domain: "default", Type: "menu", Name: "Existing Child", Code: "seed:existing-child", ParentID: 10, Status: 1},
	}
	if err := svc.InitSeedPermissions(ctx, seeds); err != nil {
		t.Fatalf("初始化种子权限失败: %v", err)
	}

	newParent, err := global.ORM.SystemPermission.Query().
		Where(systempermission.DomainEQ("default"), systempermission.CodeEQ("seed:new-parent")).
		Only(ctx)
	if err != nil {
		t.Fatalf("查询新父权限失败: %v", err)
	}
	child, err := global.ORM.SystemPermission.Query().
		Where(systempermission.DomainEQ("default"), systempermission.CodeEQ("seed:existing-child")).
		Only(ctx)
	if err != nil {
		t.Fatalf("查询已有子权限失败: %v", err)
	}

	if child.ParentID != newParent.ID {
		t.Fatalf("existing child parent_id = %d, want repaired parent id %d", child.ParentID, newParent.ID)
	}
}

func TestInitSystemPermissions_IncludesPluginManagementSeeds(t *testing.T) {
	setupSystemPermissionTest(t)
	ctx := t.Context()

	if err := InitSystemPermissions(ctx); err != nil {
		t.Fatalf("InitSystemPermissions failed: %v", err)
	}

	expected := []seedPermissionExpectation{
		{Code: "plugin", Type: "menu", MenuPath: "/system/plugin", ParentCode: "system"},
		{Code: "plugin:list", Type: "api", Path: "/api/v1/admin/auth/plugins", Method: "GET", ParentCode: "plugin"},
		{Code: "plugin:get", Type: "api", Path: "/api/v1/admin/auth/plugins/{plugin_id}", Method: "GET", ParentCode: "plugin"},
		{Code: "plugin:install", Type: "api", Path: "/api/v1/admin/auth/plugins/install", Method: "POST", ParentCode: "plugin"},
		{Code: "plugin:enable", Type: "api", Path: "/api/v1/admin/auth/plugins/{plugin_id}/enable", Method: "POST", ParentCode: "plugin"},
		{Code: "plugin:disable", Type: "api", Path: "/api/v1/admin/auth/plugins/{plugin_id}/disable", Method: "POST", ParentCode: "plugin"},
		{Code: "plugin:uninstall", Type: "api", Path: "/api/v1/admin/auth/plugins/{plugin_id}", Method: "DELETE", ParentCode: "plugin"},
		{Code: "plugin:add", Type: "button", ParentCode: "plugin"},
		{Code: "plugin:edit", Type: "button", ParentCode: "plugin"},
		{Code: "plugin:delete", Type: "button", ParentCode: "plugin"},
		{Code: "plugin:import", Type: "button", ParentCode: "plugin"},
		{Code: "role:delete-api", Type: "api", Path: "/api/v1/admin/auth/roles/{id}", Method: "DELETE", ParentCode: "system:role"},
		{Code: "permission:delete-api", Type: "api", Path: "/api/v1/admin/auth/system-permissions/{id}", Method: "DELETE", ParentCode: "system:permission"},
		{Code: "role:delete", Type: "button", ParentCode: "system:role"},
		{Code: "permission:delete", Type: "button", ParentCode: "system:permission"},
	}

	for _, want := range expected {
		assertSeedPermissionExists(t, ctx, want)
	}
}

func TestInitSystemPermissionsIncludesCaptchaManagementSeeds(t *testing.T) {
	setupSystemPermissionTest(t)
	ctx := t.Context()

	if err := InitSystemPermissions(ctx); err != nil {
		t.Fatalf("InitSystemPermissions failed: %v", err)
	}

	expected := []seedPermissionExpectation{
		{Code: "system:captcha", Type: "menu", MenuPath: "/system/captcha", ParentCode: "system"},
		{Code: "captcha:settings", Type: "api", Path: "/api/v1/admin/auth/captcha/settings", Method: "GET", ParentCode: "system:captcha"},
		{Code: "captcha:scene:update", Type: "api", Path: "/api/v1/admin/auth/captcha/scenes/{scene}", Method: "PUT", ParentCode: "system:captcha"},
		{Code: "captcha:provider:update", Type: "api", Path: "/api/v1/admin/auth/captcha/providers/{provider}", Method: "PUT", ParentCode: "system:captcha"},
		{Code: "captcha:provider:test", Type: "api", Path: "/api/v1/admin/auth/captcha/providers/{provider}/test", Method: "POST", ParentCode: "system:captcha"},
		{Code: "captcha:edit", Type: "button", ParentCode: "system:captcha"},
	}
	for _, want := range expected {
		assertSeedPermissionExists(t, ctx, want)
	}
}

func TestInitSystemPermissions_CreatesAllDefaultSeeds(t *testing.T) {
	setupSystemPermissionTest(t)
	ctx := t.Context()

	if err := InitSystemPermissions(ctx); err != nil {
		t.Fatalf("InitSystemPermissions failed: %v", err)
	}

	count, err := global.ORM.SystemPermission.Query().
		Where(systempermission.DomainEQ("default")).
		Count(ctx)
	if err != nil {
		t.Fatalf("count system permissions failed: %v", err)
	}
	const wantCount = 205
	if count != wantCount {
		t.Fatalf("default system permission count = %d, want %d", count, wantCount)
	}
}

func TestInitSystemPermissions_IncludesSlideGroupSeeds(t *testing.T) {
	setupSystemPermissionTest(t)
	ctx := t.Context()

	if err := InitSystemPermissions(ctx); err != nil {
		t.Fatalf("InitSystemPermissions failed: %v", err)
	}

	expected := []seedPermissionExpectation{
		{Code: "system:slide", Type: "menu", MenuPath: "/system/slide", ParentCode: "system"},
		{Code: "slide-group:list", Type: "api", Path: "/api/v1/admin/auth/slide-groups", Method: "GET", ParentCode: "system:slide"},
		{Code: "slide-group:create", Type: "api", Path: "/api/v1/admin/auth/slide-groups", Method: "POST", ParentCode: "system:slide"},
		{Code: "slide-group:update", Type: "api", Path: "/api/v1/admin/auth/slide-groups/{id}", Method: "PUT", ParentCode: "system:slide"},
		{Code: "slide-group:delete", Type: "api", Path: "/api/v1/admin/auth/slide-groups/{id}", Method: "DELETE", ParentCode: "system:slide"},
		{Code: "slide:group", Type: "button", ParentCode: "system:slide"},
	}

	for _, want := range expected {
		assertSeedPermissionExists(t, ctx, want)
	}
}

func TestInitSystemPermissions_IncludesLoginLogSeeds(t *testing.T) {
	setupSystemPermissionTest(t)
	ctx := t.Context()

	if err := InitSystemPermissions(ctx); err != nil {
		t.Fatalf("InitSystemPermissions failed: %v", err)
	}

	expected := []seedPermissionExpectation{
		{Code: "system:login-log", Type: "menu", MenuPath: "/system/login-log", ParentCode: "system"},
		{Code: "login-log:list", Type: "api", Path: "/api/v1/admin/auth/login-logs", Method: "GET", ParentCode: "system:login-log"},
		{Code: "login-log:get", Type: "api", Path: "/api/v1/admin/auth/login-logs/{id}", Method: "GET", ParentCode: "system:login-log"},
	}

	for _, want := range expected {
		assertSeedPermissionExists(t, ctx, want)
	}
}

func TestInitSystemPermissions_IncludesQueueJobSeeds(t *testing.T) {
	setupSystemPermissionTest(t)
	ctx := t.Context()

	if err := InitSystemPermissions(ctx); err != nil {
		t.Fatalf("InitSystemPermissions failed: %v", err)
	}

	expected := []seedPermissionExpectation{
		{Code: "system:queue-job", Type: "menu", MenuPath: "/system/queue-job", ParentCode: "system"},
		{Code: "queue-job:list", Type: "api", Path: "/api/v1/admin/auth/queue-jobs", Method: "GET", ParentCode: "system:queue-job"},
		{Code: "queue-job:enqueue", Type: "api", Path: "/api/v1/admin/auth/queue-jobs", Method: "POST", ParentCode: "system:queue-job"},
		{Code: "queue-job:stats", Type: "api", Path: "/api/v1/admin/auth/queue-jobs/stats", Method: "GET", ParentCode: "system:queue-job"},
		{Code: "queue-job:handlers", Type: "api", Path: "/api/v1/admin/auth/queue-jobs/handlers", Method: "GET", ParentCode: "system:queue-job"},
		{Code: "queue-job:get", Type: "api", Path: "/api/v1/admin/auth/queue-jobs/{queue}/{id}", Method: "GET", ParentCode: "system:queue-job"},
		{Code: "queue-job:retry", Type: "api", Path: "/api/v1/admin/auth/queue-jobs/{queue}/{id}/retry", Method: "POST", ParentCode: "system:queue-job"},
		{Code: "queue-job:cancel", Type: "api", Path: "/api/v1/admin/auth/queue-jobs/{queue}/{id}/cancel", Method: "POST", ParentCode: "system:queue-job"},
		{Code: "queue-job:delete", Type: "api", Path: "/api/v1/admin/auth/queue-jobs/{queue}/{id}", Method: "DELETE", ParentCode: "system:queue-job"},
		{Code: "queue-job:capabilities", Type: "api", Path: "/api/v1/admin/auth/queue-jobs/capabilities", Method: "GET", ParentCode: "system:queue-job"},
		{Code: "queue-job:overview", Type: "api", Path: "/api/v1/admin/auth/queue-jobs/overview", Method: "GET", ParentCode: "system:queue-job"},
		{Code: "queue-job:pause", Type: "api", Path: "/api/v1/admin/auth/queue-jobs/{queue}/pause", Method: "POST", ParentCode: "system:queue-job"},
		{Code: "queue-job:resume", Type: "api", Path: "/api/v1/admin/auth/queue-jobs/{queue}/resume", Method: "POST", ParentCode: "system:queue-job"},
		{Code: "queue-job:raw-message", Type: "api", Path: "/api/v1/admin/auth/queue-jobs/{queue}/messages/{sequence}", Method: "GET", ParentCode: "system:queue-job"},
	}

	for _, want := range expected {
		assertSeedPermissionExists(t, ctx, want)
	}
}

func TestInitSystemPermissions_IncludesMessageCenterSeeds(t *testing.T) {
	setupSystemPermissionTest(t)
	ctx := t.Context()

	if err := InitSystemPermissions(ctx); err != nil {
		t.Fatalf("InitSystemPermissions failed: %v", err)
	}

	expected := []seedPermissionExpectation{
		{Code: "message", Type: "menu", MenuPath: "/message"},
		{Code: "messagecenter", Type: "menu", MenuPath: "/message/center", ParentCode: "message"},
		{Code: "messagemanage", Type: "menu", MenuPath: "/message/manage", ParentCode: "message"},
		{Code: "messagebroadcast", Type: "menu", MenuPath: "/message/broadcast", ParentCode: "message"},
		{Code: "messagepreference", Type: "menu", MenuPath: "/message/preference", ParentCode: "message"},
		{Code: "message:list", Type: "api", Path: "/api/v1/admin/auth/messages", Method: "GET", ParentCode: "messagecenter"},
		{Code: "message:get", Type: "api", Path: "/api/v1/admin/auth/messages/{id}", Method: "GET", ParentCode: "messagecenter"},
		{Code: "message:read", Type: "api", Path: "/api/v1/admin/auth/messages/read", Method: "POST", ParentCode: "messagecenter"},
		{Code: "message:read-all", Type: "api", Path: "/api/v1/admin/auth/messages/read-all", Method: "POST", ParentCode: "messagecenter"},
		{Code: "message:unread", Type: "api", Path: "/api/v1/admin/auth/messages/unread", Method: "POST", ParentCode: "messagecenter"},
		{Code: "message:unread-count", Type: "api", Path: "/api/v1/admin/auth/messages/unread-count", Method: "GET", ParentCode: "messagecenter"},
		{Code: "message:delete-api", Type: "api", Path: "/api/v1/admin/auth/messages/{id}", Method: "DELETE", ParentCode: "messagecenter"},
		{Code: "message:delete-batch", Type: "api", Path: "/api/v1/admin/auth/messages/delete-batch", Method: "POST", ParentCode: "messagemanage"},
		{Code: "message:clear", Type: "api", Path: "/api/v1/admin/auth/messages/clear", Method: "POST", ParentCode: "messagemanage"},
		{Code: "messagepreference:get", Type: "api", Path: "/api/v1/admin/auth/messages/preference", Method: "GET", ParentCode: "messagepreference"},
		{Code: "messagepreference:update", Type: "api", Path: "/api/v1/admin/auth/messages/preference", Method: "PUT", ParentCode: "messagepreference"},
		{Code: "messagebroadcast:send", Type: "api", Path: "/api/v1/admin/auth/messages/broadcast", Method: "POST", ParentCode: "messagebroadcast"},
		{Code: "message:test-push", Type: "api", Path: "/api/v1/admin/auth/messages/test-push", Method: "POST", ParentCode: "messagebroadcast"},
		{Code: "sse:ticket", Type: "api", Path: "/api/v1/admin/auth/sse/ticket", Method: "POST", ParentCode: "messagecenter"},
		{Code: "message:add", Type: "button", ParentCode: "message"},
		{Code: "message:edit", Type: "button", ParentCode: "message"},
		{Code: "message:delete", Type: "button", ParentCode: "message"},
		{Code: "messagecenter:add", Type: "button", ParentCode: "messagecenter"},
		{Code: "messagecenter:edit", Type: "button", ParentCode: "messagecenter"},
		{Code: "messagecenter:delete", Type: "button", ParentCode: "messagecenter"},
		{Code: "messagemanage:add", Type: "button", ParentCode: "messagemanage"},
		{Code: "messagemanage:edit", Type: "button", ParentCode: "messagemanage"},
		{Code: "messagemanage:delete", Type: "button", ParentCode: "messagemanage"},
		{Code: "messagebroadcast:add", Type: "button", ParentCode: "messagebroadcast"},
		{Code: "messagebroadcast:edit", Type: "button", ParentCode: "messagebroadcast"},
		{Code: "messagebroadcast:delete", Type: "button", ParentCode: "messagebroadcast"},
		{Code: "messagepreference:add", Type: "button", ParentCode: "messagepreference"},
		{Code: "messagepreference:edit", Type: "button", ParentCode: "messagepreference"},
		{Code: "messagepreference:delete", Type: "button", ParentCode: "messagepreference"},
	}

	for _, want := range expected {
		assertSeedPermissionExists(t, ctx, want)
	}
}

func TestInitSystemPermissions_IncludesCronJobRouteSeeds(t *testing.T) {
	setupSystemPermissionTest(t)
	ctx := t.Context()

	if err := InitSystemPermissions(ctx); err != nil {
		t.Fatalf("InitSystemPermissions failed: %v", err)
	}

	expected := []seedPermissionExpectation{
		{Code: "cron-job:list", Type: "api", Path: "/api/v1/admin/auth/cron-jobs", Method: "GET", ParentCode: "system:cron-job"},
		{Code: "cron-job:create", Type: "api", Path: "/api/v1/admin/auth/cron-jobs", Method: "POST", ParentCode: "system:cron-job"},
		{Code: "cron-job:get", Type: "api", Path: "/api/v1/admin/auth/cron-jobs/{id}", Method: "GET", ParentCode: "system:cron-job"},
		{Code: "cron-job:update", Type: "api", Path: "/api/v1/admin/auth/cron-jobs/{id}", Method: "PUT", ParentCode: "system:cron-job"},
		{Code: "cron-job:delete-api", Type: "api", Path: "/api/v1/admin/auth/cron-jobs/{id}", Method: "DELETE", ParentCode: "system:cron-job"},
		{Code: "cron-job:enable", Type: "api", Path: "/api/v1/admin/auth/cron-jobs/{id}/enable", Method: "POST", ParentCode: "system:cron-job"},
		{Code: "cron-job:disable", Type: "api", Path: "/api/v1/admin/auth/cron-jobs/{id}/disable", Method: "POST", ParentCode: "system:cron-job"},
		{Code: "cron-job:run", Type: "api", Path: "/api/v1/admin/auth/cron-jobs/{id}/run", Method: "POST", ParentCode: "system:cron-job"},
		{Code: "cron-job:logs", Type: "api", Path: "/api/v1/admin/auth/cron-jobs/{id}/logs", Method: "GET", ParentCode: "system:cron-job"},
		{Code: "cron-job:log-tail", Type: "api", Path: "/api/v1/admin/auth/cron-jobs/{id}/logs/tail", Method: "GET", ParentCode: "system:cron-job"},
		{Code: "cron-job:log-content", Type: "api", Path: "/api/v1/admin/auth/cron-jobs/{id}/logs/{filename}", Method: "GET", ParentCode: "system:cron-job"},
	}

	for _, want := range expected {
		assertSeedPermissionExists(t, ctx, want)
	}
}

func TestInitSystemPermissions_IncludesRequestedMissingAPISeeds(t *testing.T) {
	setupSystemPermissionTest(t)
	ctx := t.Context()

	if err := InitSystemPermissions(ctx); err != nil {
		t.Fatalf("InitSystemPermissions failed: %v", err)
	}

	expected := []seedPermissionExpectation{
		{Code: "config:groups", Type: "api", Path: "/api/v1/admin/auth/configs/groups", Method: "GET", ParentCode: "system:config"},
		{Code: "config:group", Type: "api", Path: "/api/v1/admin/auth/configs/group/{group}", Method: "GET", ParentCode: "system:config"},
		{Code: "config:create", Type: "api", Path: "/api/v1/admin/auth/configs", Method: "POST", ParentCode: "system:config"},
		{Code: "config:get", Type: "api", Path: "/api/v1/admin/auth/configs/{id}", Method: "GET", ParentCode: "system:config"},
		{Code: "config:update", Type: "api", Path: "/api/v1/admin/auth/configs/{id}", Method: "PUT", ParentCode: "system:config"},
		{Code: "config:delete-api", Type: "api", Path: "/api/v1/admin/auth/configs/{id}", Method: "DELETE", ParentCode: "system:config"},
		{Code: "attachment-group:list", Type: "api", Path: "/api/v1/admin/auth/attachment-groups", Method: "GET", ParentCode: "system:attachment"},
		{Code: "dict-data:create", Type: "api", Path: "/api/v1/admin/auth/dict-data", Method: "POST", ParentCode: "system:dict-data"},
		{Code: "dict-data:get", Type: "api", Path: "/api/v1/admin/auth/dict-data/{id}", Method: "GET", ParentCode: "system:dict-data"},
		{Code: "dict-data:update", Type: "api", Path: "/api/v1/admin/auth/dict-data/{id}", Method: "PUT", ParentCode: "system:dict-data"},
		{Code: "dict-data:delete-api", Type: "api", Path: "/api/v1/admin/auth/dict-data/{id}", Method: "DELETE", ParentCode: "system:dict-data"},
		{Code: "dict-data:type", Type: "api", Path: "/api/v1/admin/auth/dict-data/type/{type}", Method: "GET", ParentCode: "system:dict-data"},
		{Code: "role:permissions:get", Type: "api", Path: "/api/v1/admin/auth/roles/{id}/permissions", Method: "GET", ParentCode: "system:role"},
		{Code: "role:permissions:save", Type: "api", Path: "/api/v1/admin/auth/roles/{id}/permissions", Method: "POST", ParentCode: "system:role"},
	}

	for _, want := range expected {
		assertSeedPermissionExists(t, ctx, want)
	}
}

type seedPermissionExpectation struct {
	Code       string
	Type       string
	Path       string
	Method     string
	MenuPath   string
	ParentCode string
}

func assertSeedPermissionExists(t *testing.T, ctx context.Context, want seedPermissionExpectation) {
	t.Helper()

	query := global.ORM.SystemPermission.Query().Where(
		systempermission.DomainEQ("default"),
		systempermission.TypeEQ(want.Type),
		systempermission.CodeEQ(want.Code),
	)
	if want.Path != "" {
		query = query.Where(systempermission.PathEQ(want.Path))
	}
	if want.Method != "" {
		query = query.Where(systempermission.MethodEQ(want.Method))
	}
	if want.MenuPath != "" {
		query = query.Where(systempermission.MenuPathEQ(want.MenuPath))
	}

	perm, err := query.Only(ctx)
	if err != nil {
		t.Fatalf("query seed permission %s failed: %v", want.Code, err)
	}
	if want.ParentCode == "" {
		return
	}

	parent, err := global.ORM.SystemPermission.Query().Where(
		systempermission.DomainEQ("default"),
		systempermission.CodeEQ(want.ParentCode),
	).Only(ctx)
	if err != nil {
		t.Fatalf("query parent permission %s failed: %v", want.ParentCode, err)
	}
	if perm.ParentID != parent.ID {
		t.Fatalf("seed permission %s parent_id = %d, want parent %s id %d", want.Code, perm.ParentID, want.ParentCode, parent.ID)
	}
}

func TestSystemPermission_Update_RejectsSelfParent(t *testing.T) {
	setupSystemPermissionTest(t)
	ctx := withPermissionSystemOperator(t.Context())
	svc := NewSystemPermissionService()
	created, err := svc.Create(ctx, "", &CreateSystemPermissionRequest{
		Domain: "default", Type: SystemPermissionTypeMenu, Name: "P", Code: "cycle:self",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	selfID := created.ID
	err = svc.Update(ctx, "", selfID, &UpdateSystemPermissionRequest{ParentID: &selfID})
	if !errors.Is(err, ErrPermissionParentCycle) {
		t.Fatalf("期望 ErrPermissionParentCycle，got %v", err)
	}
}

func TestSystemPermission_Update_RejectsDescendantParent(t *testing.T) {
	setupSystemPermissionTest(t)
	ctx := withPermissionSystemOperator(t.Context())
	svc := NewSystemPermissionService()
	parent, err := svc.Create(ctx, "", &CreateSystemPermissionRequest{
		Domain: "default", Type: SystemPermissionTypeMenu, Name: "P", Code: "cycle:p",
	})
	if err != nil {
		t.Fatalf("create parent failed: %v", err)
	}
	child, err := svc.Create(ctx, "", &CreateSystemPermissionRequest{
		Domain: "default", Type: SystemPermissionTypeMenu, Name: "C", Code: "cycle:c", ParentID: parent.ID,
	})
	if err != nil {
		t.Fatalf("create child failed: %v", err)
	}
	err = svc.Update(ctx, "", parent.ID, &UpdateSystemPermissionRequest{ParentID: &child.ID})
	if !errors.Is(err, ErrPermissionParentCycle) {
		t.Fatalf("期望 ErrPermissionParentCycle，got %v", err)
	}
}

func TestInitSystemPermissions_IncludesAdminManagementSeeds(t *testing.T) {
	setupSystemPermissionTest(t)
	ctx := t.Context()
	if err := InitSystemPermissions(ctx); err != nil {
		t.Fatalf("InitSystemPermissions failed: %v", err)
	}
	expected := []seedPermissionExpectation{
		{Code: "system:admin", Type: "menu", MenuPath: "/system/admin", ParentCode: "system"},
		{Code: "admin:list", Type: "api", Path: "/api/v1/admin/auth/admins", Method: "GET", ParentCode: "system:admin"},
		{Code: "admin:create", Type: "api", Path: "/api/v1/admin/auth/admins", Method: "POST", ParentCode: "system:admin"},
		{Code: "admin:detail", Type: "api", Path: "/api/v1/admin/auth/admins/{id}", Method: "GET", ParentCode: "system:admin"},
		{Code: "admin:update", Type: "api", Path: "/api/v1/admin/auth/admins/{id}", Method: "PUT", ParentCode: "system:admin"},
		{Code: "admin:roles:get", Type: "api", Path: "/api/v1/admin/auth/admins/{id}/roles", Method: "GET", ParentCode: "system:admin"},
		{Code: "admin:roles:put", Type: "api", Path: "/api/v1/admin/auth/admins/{id}/roles", Method: "PUT", ParentCode: "system:admin"},
		{Code: "admin:activate", Type: "api", Path: "/api/v1/admin/auth/admins/{id}/activate", Method: "PUT", ParentCode: "system:admin"},
		{Code: "admin:deactivate", Type: "api", Path: "/api/v1/admin/auth/admins/{id}/deactivate", Method: "PUT", ParentCode: "system:admin"},
		{Code: "admin:add", Type: "button", ParentCode: "system:admin"},
		{Code: "admin:edit", Type: "button", ParentCode: "system:admin"},
		{Code: "admin:assign-role", Type: "button", ParentCode: "system:admin"},
		{Code: "admin:activate-btn", Type: "button", ParentCode: "system:admin"},
		{Code: "admin:deactivate-btn", Type: "button", ParentCode: "system:admin"},
	}
	for _, want := range expected {
		assertSeedPermissionExists(t, ctx, want)
	}
}

func TestInitSystemPermissions_IncludesOAuthManagementSeeds(t *testing.T) {
	setupSystemPermissionTest(t)
	ctx := t.Context()
	if err := InitSystemPermissions(ctx); err != nil {
		t.Fatalf("InitSystemPermissions failed: %v", err)
	}
	expected := []seedPermissionExpectation{
		{Code: "system:third-party-user", Type: "menu", MenuPath: "/system/third-party-user", ParentCode: "system"},
		{Code: "third-party-user:list", Type: "api", Path: "/api/v1/admin/auth/third-party-users", Method: "GET", ParentCode: "system:third-party-user"},
		{Code: "third-party-user:delete", Type: "api", Path: "/api/v1/admin/auth/third-party-users/{id}", Method: "DELETE", ParentCode: "system:third-party-user"},
		{Code: "oauth-config:list", Type: "api", Path: "/api/v1/admin/auth/oauth/providers", Method: "GET", ParentCode: "system:config"},
		{Code: "oauth-config:get", Type: "api", Path: "/api/v1/admin/auth/oauth/providers/{provider}", Method: "GET", ParentCode: "system:config"},
		{Code: "oauth-config:update", Type: "api", Path: "/api/v1/admin/auth/oauth/providers/{provider}", Method: "PUT", ParentCode: "system:config"},
		{Code: "oauth-config:delete", Type: "api", Path: "/api/v1/admin/auth/oauth/providers/{provider}", Method: "DELETE", ParentCode: "system:config"},
		{Code: "admin-oauth:list", Type: "api", Path: "/api/v1/admin/auth/third-party-admins", Method: "GET", ParentCode: "system:admin"},
		{Code: "admin-oauth:delete", Type: "api", Path: "/api/v1/admin/auth/third-party-admins/{id}", Method: "DELETE", ParentCode: "system:admin"},
	}
	for _, want := range expected {
		assertSeedPermissionExists(t, ctx, want)
	}
}
