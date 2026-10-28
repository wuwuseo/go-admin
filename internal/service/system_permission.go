package service

import (
	"context"
	"errors"
	"fmt"

	"go-admin/internal/ent"
	"go-admin/internal/ent/systempermission"
	"go-admin/internal/global"
)

// ErrPermissionParentCycle 系统权限父子环
var ErrPermissionParentCycle = errors.New("permission parent cycle detected")

// isSensitiveAPIPermission 判断系统权限是否为高风险 API 权限
func isSensitiveAPIPermission(p *ent.SystemPermission) bool {
	if p.Type != "api" {
		return false
	}
	return IsSensitivePermissionObject(p.Path)
}

// shouldFilterSensitive 判断是否需要过滤高风险权限（非超级管理员需要过滤）
func (s *SystemPermissionService) shouldFilterSensitive(ctx context.Context, operatorUID string) (bool, error) {
	if operatorUID == "" {
		return !isPermissionSystemOperator(ctx), nil
	}
	// 查询操作者是否拥有超级管理员角色
	isSuperAdmin, err := isUserSuperAdmin(ctx, s.client, operatorUID)
	if err != nil {
		return false, err
	}
	return !isSuperAdmin, nil
}

// SystemPermissionType 权限类型
type SystemPermissionType string

const (
	SystemPermissionTypeAPI    SystemPermissionType = "api"
	SystemPermissionTypeMenu   SystemPermissionType = "menu"
	SystemPermissionTypeButton SystemPermissionType = "button"
)

// CreateSystemPermissionRequest 创建权限请求
type CreateSystemPermissionRequest struct {
	Domain      string               `json:"domain" validate:"required,max=100"`
	Type        SystemPermissionType `json:"type" validate:"required,oneof=api menu button"`
	Name        string               `json:"name" validate:"required,max=100"`
	Code        string               `json:"code" validate:"required,max=200"`
	Path        string               `json:"path" validate:"max=500"`
	Method      string               `json:"method" validate:"max=20"`
	MenuPath    string               `json:"menu_path" validate:"max=500"`
	Icon        string               `json:"icon" validate:"max=100"`
	Component   string               `json:"component" validate:"max=200"`
	ParentID    int                  `json:"parent_id"`
	SortOrder   int                  `json:"sort_order"`
	IsCommon    bool                 `json:"is_common"`
	Status      int8                 `json:"status"`
	Description string               `json:"description" validate:"max=500"`
}

// UpdateSystemPermissionRequest 更新权限请求
type UpdateSystemPermissionRequest struct {
	Type        SystemPermissionType `json:"type" validate:"omitempty,oneof=api menu button"`
	Name        string               `json:"name" validate:"omitempty,max=100"`
	Path        string               `json:"path" validate:"omitempty,max=500"`
	Method      string               `json:"method" validate:"omitempty,max=20"`
	MenuPath    string               `json:"menu_path" validate:"omitempty,max=500"`
	Icon        string               `json:"icon" validate:"omitempty,max=100"`
	Component   string               `json:"component" validate:"omitempty,max=200"`
	ParentID    *int                 `json:"parent_id"`
	SortOrder   *int                 `json:"sort_order"`
	IsCommon    *bool                `json:"is_common"`
	Status      *int8                `json:"status" validate:"omitempty,oneof=0 1"`
	Description string               `json:"description" validate:"omitempty,max=500"`
}

// SystemPermissionResponse 权限响应
type SystemPermissionResponse struct {
	ID          int                  `json:"id"`
	Domain      string               `json:"domain"`
	Type        SystemPermissionType `json:"type"`
	Name        string               `json:"name"`
	Code        string               `json:"code"`
	Path        string               `json:"path"`
	Method      string               `json:"method"`
	MenuPath    string               `json:"menu_path"`
	Icon        string               `json:"icon"`
	Component   string               `json:"component"`
	ParentID    int                  `json:"parent_id"`
	SortOrder   int                  `json:"sort_order"`
	IsCommon    bool                 `json:"is_common"`
	Status      int8                 `json:"status"`
	Description string               `json:"description"`
	CreatedAt   string               `json:"created_at"`
	UpdatedAt   string               `json:"updated_at"`
}

// SystemPermissionTreeNode 权限树节点
type SystemPermissionTreeNode struct {
	ID          int                         `json:"id"`
	Type        SystemPermissionType        `json:"type"`
	Name        string                      `json:"name"`
	Code        string                      `json:"code"`
	Path        string                      `json:"path"`
	Method      string                      `json:"method"`
	MenuPath    string                      `json:"menu_path"`
	Icon        string                      `json:"icon"`
	Component   string                      `json:"component"`
	ParentID    int                         `json:"parent_id"`
	SortOrder   int                         `json:"sort_order"`
	IsCommon    bool                        `json:"is_common"`
	Status      int8                        `json:"status"`
	Description string                      `json:"description"`
	Children    []*SystemPermissionTreeNode `json:"children,omitempty"`
}

// ListSystemPermissionsRequest 权限列表请求
type ListSystemPermissionsRequest struct {
	ListRequest
	Domain string               `query:"domain" json:"domain"`
	Type   SystemPermissionType `query:"type" json:"type"`
	Status int8                 `query:"status" json:"status"`
}

// ListSystemPermissionsResponse 权限列表响应
type ListSystemPermissionsResponse = ListResponse[*SystemPermissionResponse]

// SeedPermission 种子权限结构
type SeedPermission struct {
	ID          int
	Domain      string
	Type        string
	Name        string
	Code        string
	Path        string
	Method      string
	MenuPath    string
	Icon        string
	Component   string
	ParentID    int
	SortOrder   int
	IsCommon    bool
	Status      int8
	Description string
}

// SystemPermissionServiceInterface 权限定义服务接口
type SystemPermissionServiceInterface interface {
	Create(ctx context.Context, operatorUID string, req *CreateSystemPermissionRequest) (*SystemPermissionResponse, error)
	Update(ctx context.Context, operatorUID string, id int, req *UpdateSystemPermissionRequest) error
	Delete(ctx context.Context, operatorUID string, id int) error
	GetByID(ctx context.Context, id int) (*SystemPermissionResponse, error)
	List(ctx context.Context, operatorUID string, req *ListSystemPermissionsRequest) (*ListSystemPermissionsResponse, error)
	GetTree(ctx context.Context, operatorUID string, domain string) ([]*SystemPermissionTreeNode, error)
	InitSeedPermissions(ctx context.Context, seeds []SeedPermission) error
}

type SystemPermissionService struct {
	client *ent.Client
}

// ensureSuperAdminOrSystem 确保操作者是超级管理员或系统级调用
// 系统权限定义的增删改属于高风险操作，仅超级管理员可执行
func (s *SystemPermissionService) ensureSuperAdminOrSystem(ctx context.Context, operatorUID string) error {
	if isPermissionSystemOperator(ctx) {
		return nil
	}
	if operatorUID == "" {
		return ErrPermissionOperatorRequired
	}
	isSuper, err := isUserSuperAdmin(ctx, s.client, operatorUID)
	if err != nil {
		return fmt.Errorf("检查超级管理员身份失败: %w", err)
	}
	if !isSuper {
		return ErrSuperAdminProtected
	}
	return nil
}

func NewSystemPermissionService() *SystemPermissionService {
	return NewSystemPermissionServiceWithClient(global.ORM)
}

func NewSystemPermissionServiceWithClient(client *ent.Client) *SystemPermissionService {
	return &SystemPermissionService{
		client: client,
	}
}

// Create 创建权限
func (s *SystemPermissionService) Create(ctx context.Context, operatorUID string, req *CreateSystemPermissionRequest) (*SystemPermissionResponse, error) {
	if err := s.ensureSuperAdminOrSystem(ctx, operatorUID); err != nil {
		return nil, err
	}

	exists, err := s.client.SystemPermission.Query().
		Where(
			systempermission.DomainEQ(req.Domain),
			systempermission.CodeEQ(req.Code),
		).Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("检查权限代码失败: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("权限代码已存在: %s", req.Code)
	}

	builder := s.client.SystemPermission.Create().
		SetDomain(req.Domain).
		SetType(string(req.Type)).
		SetName(req.Name).
		SetCode(req.Code).
		SetParentID(req.ParentID).
		SetSortOrder(req.SortOrder).
		SetIsCommon(req.IsCommon).
		SetStatus(req.Status)

	if req.Path != "" {
		builder.SetPath(req.Path)
	}
	if req.Method != "" {
		builder.SetMethod(req.Method)
	}
	if req.MenuPath != "" {
		builder.SetMenuPath(req.MenuPath)
	}
	if req.Icon != "" {
		builder.SetIcon(req.Icon)
	}
	if req.Component != "" {
		builder.SetComponent(req.Component)
	}
	if req.Description != "" {
		builder.SetDescription(req.Description)
	}

	perm, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建权限失败: %w", err)
	}

	return s.toResponse(perm), nil
}

// Update 更新权限
func (s *SystemPermissionService) Update(ctx context.Context, operatorUID string, id int, req *UpdateSystemPermissionRequest) error {
	if err := s.ensureSuperAdminOrSystem(ctx, operatorUID); err != nil {
		return err
	}

	current, err := s.client.SystemPermission.Query().Where(systempermission.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("权限不存在")
		}
		return fmt.Errorf("查询权限失败: %w", err)
	}

	// 启动事务以确保父权限更新和级联操作的数据一致性
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("启动事务失败: %w", err)
	}
	mtx := NewManagedTx(tx)
	defer mtx.RollbackIfNotCommitted()

	update := mtx.Tx().SystemPermission.UpdateOneID(id)

	if req.Type != "" {
		update.SetType(string(req.Type))
	}
	if req.Name != "" {
		update.SetName(req.Name)
	}
	if req.Path != "" {
		update.SetPath(req.Path)
	}
	if req.Method != "" {
		update.SetMethod(req.Method)
	}
	if req.MenuPath != "" {
		update.SetMenuPath(req.MenuPath)
	}
	if req.Icon != "" {
		update.SetIcon(req.Icon)
	}
	if req.Component != "" {
		update.SetComponent(req.Component)
	}
	if req.ParentID != nil && *req.ParentID != 0 {
		if *req.ParentID == id {
			return ErrPermissionParentCycle
		}
		isDescendant, err := s.isDescendantOf(ctx, *req.ParentID, id)
		if err != nil {
			return fmt.Errorf("检查父级环依赖失败: %w", err)
		}
		if isDescendant {
			return ErrPermissionParentCycle
		}
	}
	if req.ParentID != nil {
		update.SetParentID(*req.ParentID)
	}
	if req.SortOrder != nil {
		update.SetSortOrder(*req.SortOrder)
	}
	if req.IsCommon != nil {
		update.SetIsCommon(*req.IsCommon)
	}
	if req.Status != nil {
		update.SetStatus(*req.Status)
	}
	if req.Description != "" {
		update.SetDescription(req.Description)
	}

	_, err = update.Save(ctx)
	if err != nil {
		return fmt.Errorf("更新权限失败: %w", err)
	}

	// 父级被禁用时，级联禁用整个子树（包括所有后代）
	if req.Status != nil && *req.Status == 0 && current.Status == 1 {
		if err := s.cascadeDisableChildrenTx(ctx, mtx.Tx(), id); err != nil {
			return fmt.Errorf("级联禁用子权限失败: %w", err)
		}
	}

	// 父级被启用时，级联启用整个子树（包括所有后代）
	if req.Status != nil && *req.Status == 1 && current.Status == 0 {
		if err := s.cascadeEnableChildrenTx(ctx, mtx.Tx(), id); err != nil {
			return fmt.Errorf("级联启用子权限失败: %w", err)
		}
	}

	if err := mtx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

func (s *SystemPermissionService) cascadeSetChildrenStatus(ctx context.Context, client *ent.SystemPermissionClient, parentID int, fromStatus, targetStatus int8) error {
	children, err := client.Query().
		Where(systempermission.ParentIDEQ(parentID)).
		All(ctx)
	if err != nil {
		return err
	}
	for _, child := range children {
		if child.Status == fromStatus {
			_, err := client.UpdateOneID(child.ID).
				SetStatus(targetStatus).
				Save(ctx)
			if err != nil {
				return err
			}
		}
		if err := s.cascadeSetChildrenStatus(ctx, client, child.ID, fromStatus, targetStatus); err != nil {
			return err
		}
	}
	return nil
}

// cascadeDisableChildren 递归禁用所有子权限
func (s *SystemPermissionService) cascadeDisableChildren(ctx context.Context, parentID int) error {
	return s.cascadeSetChildrenStatus(ctx, s.client.SystemPermission, parentID, 1, 0)
}

// cascadeEnableChildren 递归启用所有子权限
func (s *SystemPermissionService) cascadeEnableChildren(ctx context.Context, parentID int) error {
	return s.cascadeSetChildrenStatus(ctx, s.client.SystemPermission, parentID, 0, 1)
}

// cascadeDisableChildrenTx 递归禁用所有子权限（事务版本）
func (s *SystemPermissionService) cascadeDisableChildrenTx(ctx context.Context, tx *ent.Tx, parentID int) error {
	return s.cascadeSetChildrenStatus(ctx, tx.SystemPermission, parentID, 1, 0)
}

// cascadeEnableChildrenTx 递归启用所有子权限（事务版本）
func (s *SystemPermissionService) cascadeEnableChildrenTx(ctx context.Context, tx *ent.Tx, parentID int) error {
	return s.cascadeSetChildrenStatus(ctx, tx.SystemPermission, parentID, 0, 1)
}

// Delete 删除权限
func (s *SystemPermissionService) Delete(ctx context.Context, operatorUID string, id int) error {
	if err := s.ensureSuperAdminOrSystem(ctx, operatorUID); err != nil {
		return err
	}

	_, err := s.client.SystemPermission.Query().Where(systempermission.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("权限不存在")
		}
		return fmt.Errorf("查询权限失败: %w", err)
	}

	hasChildren, err := s.client.SystemPermission.Query().
		Where(systempermission.ParentIDEQ(id)).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("检查子权限失败: %w", err)
	}
	if hasChildren {
		return fmt.Errorf("请先删除子权限")
	}

	err = s.client.SystemPermission.DeleteOneID(id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("删除权限失败: %w", err)
	}

	return nil
}

// GetByID 获取权限详情
func (s *SystemPermissionService) GetByID(ctx context.Context, id int) (*SystemPermissionResponse, error) {
	perm, err := s.client.SystemPermission.Query().Where(systempermission.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("权限不存在")
		}
		return nil, fmt.Errorf("查询权限失败: %w", err)
	}

	return s.toResponse(perm), nil
}

// List 获取权限列表
func (s *SystemPermissionService) List(ctx context.Context, operatorUID string, req *ListSystemPermissionsRequest) (*ListSystemPermissionsResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}

	query := s.client.SystemPermission.Query()

	if req.Domain != "" {
		query = query.Where(systempermission.Or(
			systempermission.DomainEQ(req.Domain),
			systempermission.IsCommonEQ(true),
		))
	}
	if req.Type != "" {
		query = query.Where(systempermission.TypeEQ(string(req.Type)))
	}
	if req.Keyword != "" {
		query = query.Where(
			systempermission.Or(
				systempermission.NameContains(req.Keyword),
				systempermission.CodeContains(req.Keyword),
			),
		)
	}
	if req.Status != 0 {
		query = query.Where(systempermission.StatusEQ(req.Status))
	}

	perms, err := query.
		Order(ent.Asc(systempermission.FieldSortOrder)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取权限列表失败: %w", err)
	}

	perms, err = s.filterByOperatorScope(ctx, operatorUID, req.Domain, perms, false)
	if err != nil {
		return nil, fmt.Errorf("check operator permission scope: %w", err)
	}

	total := len(perms)

	if req.PageSize != 0 {
		if req.PageSize > 100 {
			req.PageSize = 100
		}
		offset := (req.Page - 1) * req.PageSize
		if offset >= len(perms) {
			perms = []*ent.SystemPermission{}
		} else {
			end := offset + req.PageSize
			if end > len(perms) {
				end = len(perms)
			}
			perms = perms[offset:end]
		}
	}

	data := make([]*SystemPermissionResponse, 0, len(perms))
	for _, p := range perms {
		data = append(data, s.toResponse(p))
	}

	return &ListSystemPermissionsResponse{
		Total: total,
		Page:  req.Page,
		Data:  data,
	}, nil
}

// GetTree 获取权限树
func (s *SystemPermissionService) GetTree(ctx context.Context, operatorUID string, domain string) ([]*SystemPermissionTreeNode, error) {
	query := s.client.SystemPermission.Query()

	if domain != "" {
		query = query.Where(systempermission.Or(
			systempermission.DomainEQ(domain),
			systempermission.IsCommonEQ(true),
		))
	}

	perms, err := query.
		Order(ent.Asc(systempermission.FieldSortOrder)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取权限列表失败: %w", err)
	}

	filteredPerms, err := s.filterByOperatorScope(ctx, operatorUID, domain, perms, true)
	if err != nil {
		return nil, fmt.Errorf("check operator permission scope: %w", err)
	}

	nodeMap := make(map[int]*SystemPermissionTreeNode)
	rootNodes := make([]*SystemPermissionTreeNode, 0)

	for _, p := range filteredPerms {
		node := &SystemPermissionTreeNode{
			ID:          p.ID,
			Type:        SystemPermissionType(p.Type),
			Name:        p.Name,
			Code:        p.Code,
			Path:        p.Path,
			Method:      p.Method,
			MenuPath:    p.MenuPath,
			Icon:        p.Icon,
			Component:   p.Component,
			ParentID:    p.ParentID,
			SortOrder:   p.SortOrder,
			IsCommon:    p.IsCommon,
			Status:      p.Status,
			Description: p.Description,
			Children:    make([]*SystemPermissionTreeNode, 0),
		}
		nodeMap[p.ID] = node
	}

	for _, node := range nodeMap {
		if node.ParentID == 0 {
			rootNodes = append(rootNodes, node)
		} else {
			if parent, ok := nodeMap[node.ParentID]; ok {
				parent.Children = append(parent.Children, node)
			} else {
				rootNodes = append(rootNodes, node)
			}
		}
	}

	return rootNodes, nil
}

func (s *SystemPermissionService) filterByOperatorScope(ctx context.Context, operatorUID, domain string, perms []*ent.SystemPermission, includeAncestors bool) ([]*ent.SystemPermission, error) {
	permissionSvc := &PermissionService{client: s.client}
	scope, err := permissionSvc.getOperatorPermissionScope(ctx, operatorUID, domain)
	if err != nil {
		return nil, err
	}
	if scope.includeAll {
		return perms, nil
	}

	if !includeAncestors {
		filtered := make([]*ent.SystemPermission, 0, len(perms))
		for _, p := range perms {
			if scope.allowsSystemPermission(p) {
				filtered = append(filtered, p)
			}
		}
		return filtered, nil
	}

	permissionByID := make(map[int]*ent.SystemPermission, len(perms))
	for _, p := range perms {
		permissionByID[p.ID] = p
	}

	included := make(map[int]struct{})
	for _, p := range perms {
		if !scope.allowsSystemPermission(p) {
			continue
		}
		for current := p; current != nil; current = permissionByID[current.ParentID] {
			included[current.ID] = struct{}{}
			if current.ParentID == 0 {
				break
			}
		}
	}

	filtered := make([]*ent.SystemPermission, 0, len(included))
	for _, p := range perms {
		if _, ok := included[p.ID]; ok {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

// InitSeedPermissions 初始化种子权限（菜单、API、按钮）
func (s *SystemPermissionService) InitSeedPermissions(ctx context.Context, seeds []SeedPermission) error {
	seedByID := make(map[int]SeedPermission, len(seeds))
	actualIDBySeedID := make(map[int]int, len(seeds))
	for _, seed := range seeds {
		if seed.ID > 0 {
			seedByID[seed.ID] = seed
		}
	}

	for _, seed := range seeds {
		parentID, err := s.resolveSeedParentID(ctx, seed, seedByID, actualIDBySeedID)
		if err != nil {
			return err
		}

		existing, err := s.client.SystemPermission.Query().
			Where(
				systempermission.DomainEQ(seed.Domain),
				systempermission.CodeEQ(seed.Code),
			).Only(ctx)
		if err != nil {
			if !ent.IsNotFound(err) {
				return fmt.Errorf("检查权限失败: %w", err)
			}
		}

		if existing != nil {
			_, err = existing.Update().
				SetType(seed.Type).
				SetName(seed.Name).
				SetPath(seed.Path).
				SetMethod(seed.Method).
				SetMenuPath(seed.MenuPath).
				SetIcon(seed.Icon).
				SetComponent(seed.Component).
				SetParentID(parentID).
				SetSortOrder(seed.SortOrder).
				SetIsCommon(seed.IsCommon).
				SetStatus(seed.Status).
				SetDescription(seed.Description).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("更新种子权限失败: %w", err)
			}
			if seed.ID > 0 {
				actualIDBySeedID[seed.ID] = existing.ID
			}
			continue
		}

		create := s.client.SystemPermission.Create().
			SetDomain(seed.Domain).
			SetType(seed.Type).
			SetName(seed.Name).
			SetCode(seed.Code).
			SetPath(seed.Path).
			SetMethod(seed.Method).
			SetMenuPath(seed.MenuPath).
			SetIcon(seed.Icon).
			SetComponent(seed.Component).
			SetSortOrder(seed.SortOrder).
			SetIsCommon(seed.IsCommon).
			SetStatus(seed.Status).
			SetDescription(seed.Description)

		create.SetParentID(parentID)

		created, err := create.Save(ctx)
		if err != nil {
			return fmt.Errorf("创建权限失败: %w", err)
		}
		if seed.ID > 0 {
			actualIDBySeedID[seed.ID] = created.ID
		}
	}

	return nil
}

func (s *SystemPermissionService) resolveSeedParentID(ctx context.Context, seed SeedPermission, seedByID map[int]SeedPermission, actualIDBySeedID map[int]int) (int, error) {
	if seed.ParentID <= 0 {
		return 0, nil
	}

	if actualID, ok := actualIDBySeedID[seed.ParentID]; ok {
		return actualID, nil
	}

	parentSeed, ok := seedByID[seed.ParentID]
	if !ok {
		return seed.ParentID, nil
	}

	parent, err := s.client.SystemPermission.Query().
		Where(
			systempermission.DomainEQ(parentSeed.Domain),
			systempermission.CodeEQ(parentSeed.Code),
		).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return seed.ParentID, nil
		}
		return 0, fmt.Errorf("查询父级种子权限失败: %w", err)
	}

	actualIDBySeedID[seed.ParentID] = parent.ID
	return parent.ID, nil
}

// InitSystemPermissions 系统启动时初始化所有权限（包含菜单、按钮、API）
func InitSystemPermissions(ctx context.Context) error {
	svc := NewSystemPermissionService()

	seedData := []SeedPermission{
		{ID: 1, Domain: "default", Type: "menu", Name: "仪表盘", Code: "dashboard", MenuPath: "/dashboard/workplace", Component: "dashboard/workplace/index", Icon: "icon-dashboard", ParentID: 0, SortOrder: 0, IsCommon: true, Status: 1, Description: "系统仪表盘首页"},
		{ID: 2, Domain: "default", Type: "menu", Name: "工作台", Code: "workplace", MenuPath: "/dashboard/workplace", Component: "dashboard/workplace/index", ParentID: 1, SortOrder: 0, IsCommon: true, Status: 1, Description: "工作台页面"},
		{ID: 3, Domain: "default", Type: "menu", Name: "系统管理", Code: "system", MenuPath: "/system", Icon: "icon-settings", ParentID: 0, SortOrder: 2, IsCommon: true, Status: 1, Description: "系统管理模块"},
		{ID: 4, Domain: "default", Type: "menu", Name: "角色管理", Code: "system:role", MenuPath: "/system/role", Component: "system/role/index", ParentID: 3, SortOrder: 0, IsCommon: false, Status: 1, Description: "角色管理页面"},
		{ID: 5, Domain: "default", Type: "menu", Name: "权限定义", Code: "system:permission", MenuPath: "/system/permission", Component: "system/permission/index", ParentID: 3, SortOrder: 1, IsCommon: false, Status: 1, Description: "权限定义管理页面"},
		{ID: 6, Domain: "default", Type: "menu", Name: "附件管理", Code: "system:attachment", MenuPath: "/system/attachment", Component: "system/attachment/index", ParentID: 3, SortOrder: 2, IsCommon: false, Status: 1, Description: "附件管理页面"},
		{ID: 7, Domain: "default", Type: "menu", Name: "定时任务", Code: "system:cron-job", MenuPath: "/system/cron-job", Component: "system/cron-job/index", ParentID: 3, SortOrder: 3, IsCommon: false, Status: 1, Description: "定时任务管理页面"},
		{ID: 8, Domain: "default", Type: "menu", Name: "操作日志", Code: "system:operation-log", MenuPath: "/system/operation-log", Component: "system/operation-log/index", ParentID: 3, SortOrder: 4, IsCommon: false, Status: 1, Description: "操作日志页面"},
		{ID: 9, Domain: "default", Type: "menu", Name: "字典数据", Code: "system:dict-data", MenuPath: "/system/dict-data", Component: "system/dict-data/index", ParentID: 3, SortOrder: 5, IsCommon: false, Status: 1, Description: "字典数据管理页面"},
		{ID: 10, Domain: "default", Type: "menu", Name: "系统配置", Code: "system:config", MenuPath: "/system/config", Component: "system/config/index", ParentID: 3, SortOrder: 6, IsCommon: false, Status: 1, Description: "系统配置管理页面"},
		{ID: 11, Domain: "default", Type: "api", Name: "获取角色列表", Code: "role:list", Path: "/api/v1/admin/auth/roles", Method: "GET", ParentID: 4, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取角色列表API"},
		{ID: 12, Domain: "default", Type: "api", Name: "获取角色详情", Code: "role:get", Path: "/api/v1/admin/auth/roles/{id}", Method: "GET", ParentID: 4, SortOrder: 1, IsCommon: false, Status: 1, Description: "获取角色详情API"},
		{ID: 13, Domain: "default", Type: "api", Name: "创建角色", Code: "role:create", Path: "/api/v1/admin/auth/roles", Method: "POST", ParentID: 4, SortOrder: 2, IsCommon: false, Status: 1, Description: "创建角色API"},
		{ID: 14, Domain: "default", Type: "api", Name: "更新角色", Code: "role:update", Path: "/api/v1/admin/auth/roles/{id}", Method: "PUT", ParentID: 4, SortOrder: 3, IsCommon: false, Status: 1, Description: "更新角色API"},
		{ID: 15, Domain: "default", Type: "api", Name: "删除角色", Code: "role:delete-api", Path: "/api/v1/admin/auth/roles/{id}", Method: "DELETE", ParentID: 4, SortOrder: 4, IsCommon: false, Status: 1, Description: "删除角色API"},
		{ID: 118, Domain: "default", Type: "api", Name: "获取角色权限", Code: "role:permissions:get", Path: "/api/v1/admin/auth/roles/{id}/permissions", Method: "GET", ParentID: 4, SortOrder: 5, IsCommon: false, Status: 1, Description: "获取角色权限API"},
		{ID: 119, Domain: "default", Type: "api", Name: "保存角色权限", Code: "role:permissions:save", Path: "/api/v1/admin/auth/roles/{id}/permissions", Method: "POST", ParentID: 4, SortOrder: 6, IsCommon: false, Status: 1, Description: "保存角色权限API"},
		{ID: 16, Domain: "default", Type: "api", Name: "获取权限列表", Code: "permission:list", Path: "/api/v1/admin/auth/system-permissions", Method: "GET", ParentID: 5, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取权限列表API"},
		{ID: 17, Domain: "default", Type: "api", Name: "获取权限详情", Code: "permission:get", Path: "/api/v1/admin/auth/system-permissions/{id}", Method: "GET", ParentID: 5, SortOrder: 1, IsCommon: false, Status: 1, Description: "获取权限详情API"},
		{ID: 18, Domain: "default", Type: "api", Name: "创建权限", Code: "permission:create", Path: "/api/v1/admin/auth/system-permissions", Method: "POST", ParentID: 5, SortOrder: 2, IsCommon: false, Status: 1, Description: "创建权限API"},
		{ID: 19, Domain: "default", Type: "api", Name: "更新权限", Code: "permission:update", Path: "/api/v1/admin/auth/system-permissions/{id}", Method: "PUT", ParentID: 5, SortOrder: 3, IsCommon: false, Status: 1, Description: "更新权限API"},
		{ID: 20, Domain: "default", Type: "api", Name: "删除权限", Code: "permission:delete-api", Path: "/api/v1/admin/auth/system-permissions/{id}", Method: "DELETE", ParentID: 5, SortOrder: 4, IsCommon: false, Status: 1, Description: "删除权限API"},
		{ID: 21, Domain: "default", Type: "api", Name: "获取权限树", Code: "permission:tree", Path: "/api/v1/admin/auth/system-permissions/tree", Method: "GET", ParentID: 5, SortOrder: 5, IsCommon: false, Status: 1, Description: "获取权限树API"},
		{ID: 22, Domain: "default", Type: "api", Name: "获取附件列表", Code: "attachment:list", Path: "/api/v1/admin/auth/attachments", Method: "GET", ParentID: 6, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取附件列表API"},
		{ID: 23, Domain: "default", Type: "api", Name: "上传附件", Code: "attachment:upload", Path: "/api/v1/admin/auth/attachments/upload", Method: "POST", ParentID: 6, SortOrder: 1, IsCommon: false, Status: 1, Description: "上传附件API"},
		{ID: 24, Domain: "default", Type: "api", Name: "删除附件", Code: "attachment:delete", Path: "/api/v1/admin/auth/attachments/{id}", Method: "DELETE", ParentID: 6, SortOrder: 2, IsCommon: false, Status: 1, Description: "删除附件API"},
		{ID: 25, Domain: "default", Type: "api", Name: "获取定时任务列表", Code: "cron-job:list", Path: "/api/v1/admin/auth/cron-jobs", Method: "GET", ParentID: 7, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取定时任务列表API"},
		{ID: 26, Domain: "default", Type: "api", Name: "创建定时任务", Code: "cron-job:create", Path: "/api/v1/admin/auth/cron-jobs", Method: "POST", ParentID: 7, SortOrder: 1, IsCommon: false, Status: 1, Description: "创建定时任务API"},
		{ID: 27, Domain: "default", Type: "api", Name: "执行定时任务", Code: "cron-job:run", Path: "/api/v1/admin/auth/cron-jobs/{id}/run", Method: "POST", ParentID: 7, SortOrder: 7, IsCommon: false, Status: 1, Description: "执行定时任务API"},
		{ID: 122, Domain: "default", Type: "api", Name: "获取定时任务详情", Code: "cron-job:get", Path: "/api/v1/admin/auth/cron-jobs/{id}", Method: "GET", ParentID: 7, SortOrder: 2, IsCommon: false, Status: 1, Description: "获取定时任务详情API"},
		{ID: 123, Domain: "default", Type: "api", Name: "更新定时任务", Code: "cron-job:update", Path: "/api/v1/admin/auth/cron-jobs/{id}", Method: "PUT", ParentID: 7, SortOrder: 3, IsCommon: false, Status: 1, Description: "更新定时任务API"},
		{ID: 124, Domain: "default", Type: "api", Name: "删除定时任务", Code: "cron-job:delete-api", Path: "/api/v1/admin/auth/cron-jobs/{id}", Method: "DELETE", ParentID: 7, SortOrder: 4, IsCommon: false, Status: 1, Description: "删除定时任务API"},
		{ID: 125, Domain: "default", Type: "api", Name: "启用定时任务", Code: "cron-job:enable", Path: "/api/v1/admin/auth/cron-jobs/{id}/enable", Method: "POST", ParentID: 7, SortOrder: 5, IsCommon: false, Status: 1, Description: "启用定时任务API"},
		{ID: 126, Domain: "default", Type: "api", Name: "禁用定时任务", Code: "cron-job:disable", Path: "/api/v1/admin/auth/cron-jobs/{id}/disable", Method: "POST", ParentID: 7, SortOrder: 6, IsCommon: false, Status: 1, Description: "禁用定时任务API"},
		{ID: 127, Domain: "default", Type: "api", Name: "获取定时任务日志列表", Code: "cron-job:logs", Path: "/api/v1/admin/auth/cron-jobs/{id}/logs", Method: "GET", ParentID: 7, SortOrder: 8, IsCommon: false, Status: 1, Description: "获取定时任务日志列表API"},
		{ID: 128, Domain: "default", Type: "api", Name: "读取定时任务日志尾部", Code: "cron-job:log-tail", Path: "/api/v1/admin/auth/cron-jobs/{id}/logs/tail", Method: "GET", ParentID: 7, SortOrder: 9, IsCommon: false, Status: 1, Description: "读取定时任务日志尾部API"},
		{ID: 129, Domain: "default", Type: "api", Name: "获取定时任务日志内容", Code: "cron-job:log-content", Path: "/api/v1/admin/auth/cron-jobs/{id}/logs/{filename}", Method: "GET", ParentID: 7, SortOrder: 10, IsCommon: false, Status: 1, Description: "获取定时任务日志内容API"},
		{ID: 28, Domain: "default", Type: "api", Name: "获取操作日志", Code: "operation-log:list", Path: "/api/v1/admin/auth/operation-logs", Method: "GET", ParentID: 8, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取操作日志API"},
		{ID: 29, Domain: "default", Type: "api", Name: "获取字典数据", Code: "dict-data:list", Path: "/api/v1/admin/auth/dict-data", Method: "GET", ParentID: 9, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取字典数据API"},
		{ID: 30, Domain: "default", Type: "api", Name: "获取系统配置", Code: "config:list", Path: "/api/v1/admin/auth/configs", Method: "GET", ParentID: 10, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取系统配置API"},
		{ID: 31, Domain: "default", Type: "button", Name: "角色管理-新增按钮", Code: "role:add", ParentID: 4, SortOrder: 0, IsCommon: false, Status: 1, Description: "角色管理页面新增按钮"},
		{ID: 32, Domain: "default", Type: "button", Name: "角色管理-编辑按钮", Code: "role:edit", ParentID: 4, SortOrder: 1, IsCommon: false, Status: 1, Description: "角色管理页面编辑按钮"},
		{ID: 33, Domain: "default", Type: "button", Name: "角色管理-删除按钮", Code: "role:delete", ParentID: 4, SortOrder: 2, IsCommon: false, Status: 1, Description: "角色管理页面删除按钮"},
		{ID: 34, Domain: "default", Type: "button", Name: "角色管理-权限分配按钮", Code: "role:assign-permission", ParentID: 4, SortOrder: 3, IsCommon: false, Status: 1, Description: "角色管理页面权限分配按钮"},
		{ID: 35, Domain: "default", Type: "button", Name: "权限管理-新增按钮", Code: "permission:add", ParentID: 5, SortOrder: 0, IsCommon: false, Status: 1, Description: "权限管理页面新增按钮"},
		{ID: 36, Domain: "default", Type: "button", Name: "权限管理-编辑按钮", Code: "permission:edit", ParentID: 5, SortOrder: 1, IsCommon: false, Status: 1, Description: "权限管理页面编辑按钮"},
		{ID: 37, Domain: "default", Type: "button", Name: "权限管理-删除按钮", Code: "permission:delete", ParentID: 5, SortOrder: 2, IsCommon: false, Status: 1, Description: "权限管理页面删除按钮"},
		{ID: 38, Domain: "default", Type: "button", Name: "附件管理-上传按钮", Code: "attachment:upload-btn", ParentID: 6, SortOrder: 0, IsCommon: false, Status: 1, Description: "附件管理页面上传按钮"},
		{ID: 39, Domain: "default", Type: "button", Name: "附件管理-删除按钮", Code: "attachment:delete-btn", ParentID: 6, SortOrder: 1, IsCommon: false, Status: 1, Description: "附件管理页面删除按钮"},
		{ID: 40, Domain: "default", Type: "button", Name: "定时任务-新增按钮", Code: "cron-job:add", ParentID: 7, SortOrder: 0, IsCommon: false, Status: 1, Description: "定时任务页面新增按钮"},
		{ID: 41, Domain: "default", Type: "button", Name: "定时任务-编辑按钮", Code: "cron-job:edit", ParentID: 7, SortOrder: 1, IsCommon: false, Status: 1, Description: "定时任务页面编辑按钮"},
		{ID: 42, Domain: "default", Type: "button", Name: "定时任务-删除按钮", Code: "cron-job:delete", ParentID: 7, SortOrder: 2, IsCommon: false, Status: 1, Description: "定时任务页面删除按钮"},
		{ID: 43, Domain: "default", Type: "button", Name: "定时任务-执行按钮", Code: "cron-job:execute", ParentID: 7, SortOrder: 3, IsCommon: false, Status: 1, Description: "定时任务页面执行按钮"},
		{ID: 44, Domain: "default", Type: "button", Name: "字典管理-新增按钮", Code: "dict-data:add", ParentID: 9, SortOrder: 0, IsCommon: false, Status: 1, Description: "字典管理页面新增按钮"},
		{ID: 45, Domain: "default", Type: "button", Name: "字典管理-编辑按钮", Code: "dict-data:edit", ParentID: 9, SortOrder: 1, IsCommon: false, Status: 1, Description: "字典管理页面编辑按钮"},
		{ID: 46, Domain: "default", Type: "button", Name: "字典管理-删除按钮", Code: "dict-data:delete", ParentID: 9, SortOrder: 2, IsCommon: false, Status: 1, Description: "字典管理页面删除按钮"},
		{ID: 47, Domain: "default", Type: "button", Name: "配置管理-编辑按钮", Code: "config:edit", ParentID: 10, SortOrder: 0, IsCommon: false, Status: 1, Description: "配置管理页面编辑按钮"},
		{ID: 48, Domain: "default", Type: "menu", Name: "插件管理", Code: "plugin", MenuPath: "/system/plugin", Component: "system/plugin/index", ParentID: 3, SortOrder: 7, IsCommon: false, Status: 1, Description: "插件管理页面"},
		{ID: 49, Domain: "default", Type: "api", Name: "获取插件列表", Code: "plugin:list", Path: "/api/v1/admin/auth/plugins", Method: "GET", ParentID: 48, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取插件列表API"},
		{ID: 50, Domain: "default", Type: "api", Name: "获取插件详情", Code: "plugin:get", Path: "/api/v1/admin/auth/plugins/{plugin_id}", Method: "GET", ParentID: 48, SortOrder: 1, IsCommon: false, Status: 1, Description: "获取插件详情API"},
		{ID: 51, Domain: "default", Type: "api", Name: "安装插件", Code: "plugin:install", Path: "/api/v1/admin/auth/plugins/install", Method: "POST", ParentID: 48, SortOrder: 2, IsCommon: false, Status: 1, Description: "安装插件API"},
		{ID: 52, Domain: "default", Type: "api", Name: "启用插件", Code: "plugin:enable", Path: "/api/v1/admin/auth/plugins/{plugin_id}/enable", Method: "POST", ParentID: 48, SortOrder: 3, IsCommon: false, Status: 1, Description: "启用插件API"},
		{ID: 53, Domain: "default", Type: "api", Name: "禁用插件", Code: "plugin:disable", Path: "/api/v1/admin/auth/plugins/{plugin_id}/disable", Method: "POST", ParentID: 48, SortOrder: 4, IsCommon: false, Status: 1, Description: "禁用插件API"},
		{ID: 54, Domain: "default", Type: "api", Name: "卸载插件", Code: "plugin:uninstall", Path: "/api/v1/admin/auth/plugins/{plugin_id}", Method: "DELETE", ParentID: 48, SortOrder: 5, IsCommon: false, Status: 1, Description: "卸载插件API"},
		{ID: 55, Domain: "default", Type: "button", Name: "插件管理-新增按钮", Code: "plugin:add", ParentID: 48, SortOrder: 0, IsCommon: false, Status: 1, Description: "插件管理页面新增按钮"},
		{ID: 56, Domain: "default", Type: "button", Name: "插件管理-编辑按钮", Code: "plugin:edit", ParentID: 48, SortOrder: 1, IsCommon: false, Status: 1, Description: "插件管理页面编辑按钮"},
		{ID: 57, Domain: "default", Type: "button", Name: "插件管理-删除按钮", Code: "plugin:delete", ParentID: 48, SortOrder: 2, IsCommon: false, Status: 1, Description: "插件管理页面删除按钮"},
		{ID: 58, Domain: "default", Type: "button", Name: "插件管理-导入按钮", Code: "plugin:import", ParentID: 48, SortOrder: 3, IsCommon: false, Status: 1, Description: "插件管理页面导入按钮"},
		{ID: 59, Domain: "default", Type: "menu", Name: "消息中心", Code: "message", MenuPath: "/message", Icon: "icon-message", ParentID: 0, SortOrder: 3, IsCommon: false, Status: 1, Description: "消息中心模块"},
		{ID: 60, Domain: "default", Type: "menu", Name: "消息列表", Code: "messagecenter", MenuPath: "/message/center", Component: "message/index", ParentID: 59, SortOrder: 0, IsCommon: false, Status: 1, Description: "消息列表页面"},
		{ID: 61, Domain: "default", Type: "menu", Name: "消息管理", Code: "messagemanage", MenuPath: "/message/manage", Component: "message/manage", ParentID: 59, SortOrder: 1, IsCommon: false, Status: 1, Description: "消息管理页面"},
		{ID: 62, Domain: "default", Type: "menu", Name: "发送消息", Code: "messagebroadcast", MenuPath: "/message/broadcast", Component: "message/broadcast", ParentID: 59, SortOrder: 2, IsCommon: false, Status: 1, Description: "发送消息页面"},
		{ID: 63, Domain: "default", Type: "menu", Name: "消息偏好", Code: "messagepreference", MenuPath: "/message/preference", Component: "message/preference", ParentID: 59, SortOrder: 3, IsCommon: false, Status: 1, Description: "消息偏好页面"},
		{ID: 64, Domain: "default", Type: "api", Name: "获取消息列表", Code: "message:list", Path: "/api/v1/admin/auth/messages", Method: "GET", ParentID: 60, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取消息列表API"},
		{ID: 65, Domain: "default", Type: "api", Name: "获取消息详情", Code: "message:get", Path: "/api/v1/admin/auth/messages/{id}", Method: "GET", ParentID: 60, SortOrder: 1, IsCommon: false, Status: 1, Description: "获取消息详情API"},
		{ID: 66, Domain: "default", Type: "api", Name: "标记消息已读", Code: "message:read", Path: "/api/v1/admin/auth/messages/read", Method: "POST", ParentID: 60, SortOrder: 2, IsCommon: false, Status: 1, Description: "标记消息已读API"},
		{ID: 67, Domain: "default", Type: "api", Name: "全部标记已读", Code: "message:read-all", Path: "/api/v1/admin/auth/messages/read-all", Method: "POST", ParentID: 60, SortOrder: 3, IsCommon: false, Status: 1, Description: "全部标记已读API"},
		{ID: 68, Domain: "default", Type: "api", Name: "标记消息未读", Code: "message:unread", Path: "/api/v1/admin/auth/messages/unread", Method: "POST", ParentID: 60, SortOrder: 4, IsCommon: false, Status: 1, Description: "标记消息未读API"},
		{ID: 69, Domain: "default", Type: "api", Name: "获取未读数量", Code: "message:unread-count", Path: "/api/v1/admin/auth/messages/unread-count", Method: "GET", ParentID: 60, SortOrder: 5, IsCommon: false, Status: 1, Description: "获取未读数量API"},
		{ID: 70, Domain: "default", Type: "api", Name: "删除消息", Code: "message:delete-api", Path: "/api/v1/admin/auth/messages/{id}", Method: "DELETE", ParentID: 60, SortOrder: 6, IsCommon: false, Status: 1, Description: "删除消息API"},
		{ID: 71, Domain: "default", Type: "api", Name: "批量删除消息", Code: "message:delete-batch", Path: "/api/v1/admin/auth/messages/delete-batch", Method: "POST", ParentID: 61, SortOrder: 0, IsCommon: false, Status: 1, Description: "批量删除消息API"},
		{ID: 72, Domain: "default", Type: "api", Name: "清空消息", Code: "message:clear", Path: "/api/v1/admin/auth/messages/clear", Method: "POST", ParentID: 61, SortOrder: 1, IsCommon: false, Status: 1, Description: "清空消息API"},
		{ID: 73, Domain: "default", Type: "api", Name: "获取消息偏好", Code: "messagepreference:get", Path: "/api/v1/admin/auth/messages/preference", Method: "GET", ParentID: 63, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取消息偏好API"},
		{ID: 74, Domain: "default", Type: "api", Name: "更新消息偏好", Code: "messagepreference:update", Path: "/api/v1/admin/auth/messages/preference", Method: "PUT", ParentID: 63, SortOrder: 1, IsCommon: false, Status: 1, Description: "更新消息偏好API"},
		{ID: 75, Domain: "default", Type: "api", Name: "发送广播消息", Code: "messagebroadcast:send", Path: "/api/v1/admin/auth/messages/broadcast", Method: "POST", ParentID: 62, SortOrder: 0, IsCommon: false, Status: 1, Description: "发送广播消息API"},
		{ID: 76, Domain: "default", Type: "api", Name: "测试推送消息", Code: "message:test-push", Path: "/api/v1/admin/auth/messages/test-push", Method: "POST", ParentID: 62, SortOrder: 1, IsCommon: false, Status: 1, Description: "测试推送消息API"},
		{ID: 77, Domain: "default", Type: "api", Name: "签发SSE票据", Code: "sse:ticket", Path: "/api/v1/admin/auth/sse/ticket", Method: "POST", ParentID: 60, SortOrder: 7, IsCommon: false, Status: 1, Description: "签发SSE票据API"},
		{ID: 78, Domain: "default", Type: "button", Name: "消息中心-新增按钮", Code: "message:add", ParentID: 59, SortOrder: 0, IsCommon: false, Status: 1, Description: "消息中心新增按钮"},
		{ID: 79, Domain: "default", Type: "button", Name: "消息中心-编辑按钮", Code: "message:edit", ParentID: 59, SortOrder: 1, IsCommon: false, Status: 1, Description: "消息中心编辑按钮"},
		{ID: 80, Domain: "default", Type: "button", Name: "消息中心-删除按钮", Code: "message:delete", ParentID: 59, SortOrder: 2, IsCommon: false, Status: 1, Description: "消息中心删除按钮"},
		{ID: 81, Domain: "default", Type: "button", Name: "消息列表-新增按钮", Code: "messagecenter:add", ParentID: 60, SortOrder: 0, IsCommon: false, Status: 1, Description: "消息列表新增按钮"},
		{ID: 82, Domain: "default", Type: "button", Name: "消息列表-编辑按钮", Code: "messagecenter:edit", ParentID: 60, SortOrder: 1, IsCommon: false, Status: 1, Description: "消息列表编辑按钮"},
		{ID: 83, Domain: "default", Type: "button", Name: "消息列表-删除按钮", Code: "messagecenter:delete", ParentID: 60, SortOrder: 2, IsCommon: false, Status: 1, Description: "消息列表删除按钮"},
		{ID: 84, Domain: "default", Type: "button", Name: "消息管理-新增按钮", Code: "messagemanage:add", ParentID: 61, SortOrder: 0, IsCommon: false, Status: 1, Description: "消息管理新增按钮"},
		{ID: 85, Domain: "default", Type: "button", Name: "消息管理-编辑按钮", Code: "messagemanage:edit", ParentID: 61, SortOrder: 1, IsCommon: false, Status: 1, Description: "消息管理编辑按钮"},
		{ID: 86, Domain: "default", Type: "button", Name: "消息管理-删除按钮", Code: "messagemanage:delete", ParentID: 61, SortOrder: 2, IsCommon: false, Status: 1, Description: "消息管理删除按钮"},
		{ID: 87, Domain: "default", Type: "button", Name: "发送消息-新增按钮", Code: "messagebroadcast:add", ParentID: 62, SortOrder: 0, IsCommon: false, Status: 1, Description: "发送消息新增按钮"},
		{ID: 88, Domain: "default", Type: "button", Name: "发送消息-编辑按钮", Code: "messagebroadcast:edit", ParentID: 62, SortOrder: 1, IsCommon: false, Status: 1, Description: "发送消息编辑按钮"},
		{ID: 89, Domain: "default", Type: "button", Name: "发送消息-删除按钮", Code: "messagebroadcast:delete", ParentID: 62, SortOrder: 2, IsCommon: false, Status: 1, Description: "发送消息删除按钮"},
		{ID: 90, Domain: "default", Type: "button", Name: "消息偏好-新增按钮", Code: "messagepreference:add", ParentID: 63, SortOrder: 0, IsCommon: false, Status: 1, Description: "消息偏好新增按钮"},
		{ID: 91, Domain: "default", Type: "button", Name: "消息偏好-编辑按钮", Code: "messagepreference:edit", ParentID: 63, SortOrder: 1, IsCommon: false, Status: 1, Description: "消息偏好编辑按钮"},
		{ID: 92, Domain: "default", Type: "button", Name: "消息偏好-删除按钮", Code: "messagepreference:delete", ParentID: 63, SortOrder: 2, IsCommon: false, Status: 1, Description: "消息偏好删除按钮"},
		{ID: 93, Domain: "default", Type: "api", Name: "获取配置分组", Code: "config:groups", Path: "/api/v1/admin/auth/configs/groups", Method: "GET", ParentID: 10, SortOrder: 1, IsCommon: false, Status: 1, Description: "获取配置分组API"},
		{ID: 120, Domain: "default", Type: "api", Name: "获取配置分组详情", Code: "config:group", Path: "/api/v1/admin/auth/configs/group/{group}", Method: "GET", ParentID: 10, SortOrder: 2, IsCommon: false, Status: 1, Description: "获取配置分组详情API"},
		{ID: 121, Domain: "default", Type: "api", Name: "按键获取配置", Code: "config:by-key", Path: "/api/v1/admin/auth/configs/by-key/{key}", Method: "GET", ParentID: 10, SortOrder: 3, IsCommon: false, Status: 1, Description: "按键获取配置API"},
		{ID: 130, Domain: "default", Type: "api", Name: "创建系统配置", Code: "config:create", Path: "/api/v1/admin/auth/configs", Method: "POST", ParentID: 10, SortOrder: 4, IsCommon: false, Status: 1, Description: "创建系统配置API"},
		{ID: 131, Domain: "default", Type: "api", Name: "获取系统配置详情", Code: "config:get", Path: "/api/v1/admin/auth/configs/{id}", Method: "GET", ParentID: 10, SortOrder: 5, IsCommon: false, Status: 1, Description: "获取系统配置详情API"},
		{ID: 132, Domain: "default", Type: "api", Name: "更新系统配置", Code: "config:update", Path: "/api/v1/admin/auth/configs/{id}", Method: "PUT", ParentID: 10, SortOrder: 6, IsCommon: false, Status: 1, Description: "更新系统配置API"},
		{ID: 133, Domain: "default", Type: "api", Name: "删除系统配置", Code: "config:delete-api", Path: "/api/v1/admin/auth/configs/{id}", Method: "DELETE", ParentID: 10, SortOrder: 7, IsCommon: false, Status: 1, Description: "删除系统配置API"},
		{ID: 144, Domain: "default", Type: "api", Name: "生成附件访问地址", Code: "attachment:access-url", Path: "/api/v1/admin/auth/attachments/{id}/access-url", Method: "GET", ParentID: 6, SortOrder: 3, IsCommon: false, Status: 1, Description: "生成附件访问地址API"},
		{ID: 94, Domain: "default", Type: "api", Name: "获取附件分组列表", Code: "attachment-group:list", Path: "/api/v1/admin/auth/attachment-groups", Method: "GET", ParentID: 6, SortOrder: 3, IsCommon: false, Status: 1, Description: "获取附件分组列表API"},
		{ID: 95, Domain: "default", Type: "api", Name: "按类型获取字典数据", Code: "dict-data:type", Path: "/api/v1/admin/auth/dict-data/type/{type}", Method: "GET", ParentID: 9, SortOrder: 1, IsCommon: false, Status: 1, Description: "按类型获取字典数据API"},
		{ID: 134, Domain: "default", Type: "api", Name: "创建字典数据", Code: "dict-data:create", Path: "/api/v1/admin/auth/dict-data", Method: "POST", ParentID: 9, SortOrder: 2, IsCommon: false, Status: 1, Description: "创建字典数据API"},
		{ID: 135, Domain: "default", Type: "api", Name: "获取字典数据详情", Code: "dict-data:get", Path: "/api/v1/admin/auth/dict-data/{id}", Method: "GET", ParentID: 9, SortOrder: 3, IsCommon: false, Status: 1, Description: "获取字典数据详情API"},
		{ID: 136, Domain: "default", Type: "api", Name: "更新字典数据", Code: "dict-data:update", Path: "/api/v1/admin/auth/dict-data/{id}", Method: "PUT", ParentID: 9, SortOrder: 4, IsCommon: false, Status: 1, Description: "更新字典数据API"},
		{ID: 137, Domain: "default", Type: "api", Name: "删除字典数据", Code: "dict-data:delete-api", Path: "/api/v1/admin/auth/dict-data/{id}", Method: "DELETE", ParentID: 9, SortOrder: 5, IsCommon: false, Status: 1, Description: "删除字典数据API"},
		// 部门管理菜单
		{ID: 96, Domain: "default", Type: "menu", Name: "部门管理", Code: "system:department", MenuPath: "/system/department", Component: "system/department/index", ParentID: 3, SortOrder: 8, IsCommon: false, Status: 1, Description: "部门管理页面"},
		// 部门管理API
		{ID: 97, Domain: "default", Type: "api", Name: "创建部门", Code: "department:create", Path: "/api/v1/admin/auth/departments", Method: "POST", ParentID: 96, SortOrder: 0, IsCommon: false, Status: 1, Description: "创建部门API"},
		{ID: 98, Domain: "default", Type: "api", Name: "获取部门树", Code: "department:tree", Path: "/api/v1/admin/auth/departments/tree", Method: "GET", ParentID: 96, SortOrder: 1, IsCommon: false, Status: 1, Description: "获取部门树API"},
		{ID: 99, Domain: "default", Type: "api", Name: "获取部门详情", Code: "department:get", Path: "/api/v1/admin/auth/departments/{id}", Method: "GET", ParentID: 96, SortOrder: 2, IsCommon: false, Status: 1, Description: "获取部门详情API"},
		{ID: 100, Domain: "default", Type: "api", Name: "更新部门", Code: "department:update", Path: "/api/v1/admin/auth/departments/{id}", Method: "PUT", ParentID: 96, SortOrder: 3, IsCommon: false, Status: 1, Description: "更新部门API"},
		{ID: 101, Domain: "default", Type: "api", Name: "删除部门", Code: "department:delete-api", Path: "/api/v1/admin/auth/departments/{id}", Method: "DELETE", ParentID: 96, SortOrder: 4, IsCommon: false, Status: 1, Description: "删除部门API"},
		// 部门管理按钮
		{ID: 102, Domain: "default", Type: "button", Name: "部门管理-新增按钮", Code: "department:add", ParentID: 96, SortOrder: 0, IsCommon: false, Status: 1, Description: "部门管理页面新增按钮"},
		{ID: 103, Domain: "default", Type: "button", Name: "部门管理-编辑按钮", Code: "department:edit", ParentID: 96, SortOrder: 1, IsCommon: false, Status: 1, Description: "部门管理页面编辑按钮"},
		{ID: 104, Domain: "default", Type: "button", Name: "部门管理-删除按钮", Code: "department:delete", ParentID: 96, SortOrder: 2, IsCommon: false, Status: 1, Description: "部门管理页面删除按钮"},
		// 管理员管理菜单
		{ID: 138, Domain: "default", Type: "menu", Name: "管理员管理", Code: "system:admin", MenuPath: "/system/admin", Component: "system/admin/index", ParentID: 3, SortOrder: 9, IsCommon: false, Status: 1, Description: "管理员管理页面"},
		// 管理员管理 API（数据权限闭环新增）
		{ID: 110, Domain: "default", Type: "api", Name: "管理员列表", Code: "admin:list", Path: "/api/v1/admin/auth/admins", Method: "GET", ParentID: 138, SortOrder: 0, IsCommon: false, Status: 1, Description: "管理员列表API"},
		{ID: 111, Domain: "default", Type: "api", Name: "创建管理员", Code: "admin:create", Path: "/api/v1/admin/auth/admins", Method: "POST", ParentID: 138, SortOrder: 1, IsCommon: false, Status: 1, Description: "创建管理员API"},
		{ID: 112, Domain: "default", Type: "api", Name: "管理员详情", Code: "admin:detail", Path: "/api/v1/admin/auth/admins/{id}", Method: "GET", ParentID: 138, SortOrder: 2, IsCommon: false, Status: 1, Description: "管理员详情API"},
		{ID: 113, Domain: "default", Type: "api", Name: "更新管理员", Code: "admin:update", Path: "/api/v1/admin/auth/admins/{id}", Method: "PUT", ParentID: 138, SortOrder: 3, IsCommon: false, Status: 1, Description: "更新管理员API"},
		{ID: 114, Domain: "default", Type: "api", Name: "查看管理员角色", Code: "admin:roles:get", Path: "/api/v1/admin/auth/admins/{id}/roles", Method: "GET", ParentID: 138, SortOrder: 4, IsCommon: false, Status: 1, Description: "查看管理员角色API"},
		{ID: 115, Domain: "default", Type: "api", Name: "分配管理员角色", Code: "admin:roles:put", Path: "/api/v1/admin/auth/admins/{id}/roles", Method: "PUT", ParentID: 138, SortOrder: 5, IsCommon: false, Status: 1, Description: "分配管理员角色API"},
		{ID: 116, Domain: "default", Type: "api", Name: "启用管理员", Code: "admin:activate", Path: "/api/v1/admin/auth/admins/{id}/activate", Method: "PUT", ParentID: 138, SortOrder: 6, IsCommon: false, Status: 1, Description: "启用管理员API"},
		{ID: 117, Domain: "default", Type: "api", Name: "禁用管理员", Code: "admin:deactivate", Path: "/api/v1/admin/auth/admins/{id}/deactivate", Method: "PUT", ParentID: 138, SortOrder: 7, IsCommon: false, Status: 1, Description: "禁用管理员API"},
		// 管理员管理按钮
		{ID: 139, Domain: "default", Type: "button", Name: "管理员管理-新增按钮", Code: "admin:add", ParentID: 138, SortOrder: 0, IsCommon: false, Status: 1, Description: "管理员管理页面新增按钮"},
		{ID: 140, Domain: "default", Type: "button", Name: "管理员管理-编辑按钮", Code: "admin:edit", ParentID: 138, SortOrder: 1, IsCommon: false, Status: 1, Description: "管理员管理页面编辑按钮"},
		{ID: 141, Domain: "default", Type: "button", Name: "管理员管理-分配角色按钮", Code: "admin:assign-role", ParentID: 138, SortOrder: 2, IsCommon: false, Status: 1, Description: "管理员管理页面分配角色按钮"},
		{ID: 142, Domain: "default", Type: "button", Name: "管理员管理-启用按钮", Code: "admin:activate-btn", ParentID: 138, SortOrder: 3, IsCommon: false, Status: 1, Description: "管理员管理页面启用按钮"},
		{ID: 143, Domain: "default", Type: "button", Name: "管理员管理-禁用按钮", Code: "admin:deactivate-btn", ParentID: 138, SortOrder: 4, IsCommon: false, Status: 1, Description: "管理员管理页面禁用按钮"},
		{ID: 145, Domain: "default", Type: "api", Name: "OAuth配置列表", Code: "oauth-config:list", Path: "/api/v1/admin/auth/oauth/providers", Method: "GET", ParentID: 10, SortOrder: 8, IsCommon: false, Status: 1, Description: "获取OAuth Provider配置列表"},
		{ID: 146, Domain: "default", Type: "api", Name: "OAuth配置详情", Code: "oauth-config:get", Path: "/api/v1/admin/auth/oauth/providers/{provider}", Method: "GET", ParentID: 10, SortOrder: 9, IsCommon: false, Status: 1, Description: "获取OAuth Provider配置详情"},
		{ID: 147, Domain: "default", Type: "api", Name: "更新OAuth配置", Code: "oauth-config:update", Path: "/api/v1/admin/auth/oauth/providers/{provider}", Method: "PUT", ParentID: 10, SortOrder: 10, IsCommon: false, Status: 1, Description: "创建或更新OAuth Provider配置"},
		{ID: 148, Domain: "default", Type: "api", Name: "删除OAuth配置", Code: "oauth-config:delete", Path: "/api/v1/admin/auth/oauth/providers/{provider}", Method: "DELETE", ParentID: 10, SortOrder: 11, IsCommon: false, Status: 1, Description: "删除OAuth Provider配置"},
		{ID: 149, Domain: "default", Type: "api", Name: "管理员OAuth身份列表", Code: "admin-oauth:list", Path: "/api/v1/admin/auth/third-party-admins", Method: "GET", ParentID: 138, SortOrder: 8, IsCommon: false, Status: 1, Description: "获取管理员第三方身份列表"},
		{ID: 150, Domain: "default", Type: "api", Name: "删除管理员OAuth身份", Code: "admin-oauth:delete", Path: "/api/v1/admin/auth/third-party-admins/{id}", Method: "DELETE", ParentID: 138, SortOrder: 9, IsCommon: false, Status: 1, Description: "解绑管理员第三方身份"},
		{ID: 151, Domain: "default", Type: "menu", Name: "第三方用户", Code: "system:third-party-user", MenuPath: "/system/third-party-user", Component: "system/third-party-user/index", ParentID: 3, SortOrder: 11, IsCommon: false, Status: 1, Description: "普通用户第三方身份管理页面"},
		{ID: 152, Domain: "default", Type: "api", Name: "第三方用户列表", Code: "third-party-user:list", Path: "/api/v1/admin/auth/third-party-users", Method: "GET", ParentID: 151, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取普通用户第三方身份列表"},
		{ID: 153, Domain: "default", Type: "api", Name: "删除第三方用户身份", Code: "third-party-user:delete", Path: "/api/v1/admin/auth/third-party-users/{id}", Method: "DELETE", ParentID: 151, SortOrder: 1, IsCommon: false, Status: 1, Description: "解绑普通用户第三方身份"},
	}

	seedData = append(seedData,
		SeedPermission{
			ID: 145, Domain: "default", Type: "menu", Name: "验证码管理",
			Code: "system:captcha", MenuPath: "/system/captcha",
			Component: "system/captcha/index", ParentID: 3, SortOrder: 10,
			IsCommon: false, Status: 1, Description: "验证码场景和 Provider 配置管理",
		},
		SeedPermission{
			ID: 146, Domain: "default", Type: "api", Name: "获取验证码设置",
			Code: "captcha:settings", Path: "/api/v1/admin/auth/captcha/settings",
			Method: "GET", ParentID: 145, SortOrder: 0,
			IsCommon: false, Status: 1, Description: "获取验证码设置 API",
		},
		SeedPermission{
			ID: 147, Domain: "default", Type: "api", Name: "更新验证码场景",
			Code: "captcha:scene:update", Path: "/api/v1/admin/auth/captcha/scenes/{scene}",
			Method: "PUT", ParentID: 145, SortOrder: 1,
			IsCommon: false, Status: 1, Description: "更新验证码场景策略 API",
		},
		SeedPermission{
			ID: 148, Domain: "default", Type: "api", Name: "更新验证码 Provider",
			Code: "captcha:provider:update", Path: "/api/v1/admin/auth/captcha/providers/{provider}",
			Method: "PUT", ParentID: 145, SortOrder: 2,
			IsCommon: false, Status: 1, Description: "更新验证码 Provider 配置 API",
		},
		SeedPermission{
			ID: 149, Domain: "default", Type: "api", Name: "校验验证码 Provider",
			Code: "captcha:provider:test", Path: "/api/v1/admin/auth/captcha/providers/{provider}/test",
			Method: "POST", ParentID: 145, SortOrder: 3,
			IsCommon: false, Status: 1, Description: "校验验证码 Provider 配置 API",
		},
		SeedPermission{
			ID: 150, Domain: "default", Type: "button", Name: "验证码管理-编辑按钮",
			Code: "captcha:edit", ParentID: 145, SortOrder: 0,
			IsCommon: false, Status: 1, Description: "编辑验证码设置按钮",
		},
	)

	// 用户管理权限（S2-5：此前无任何角色可被授予用户管理入口）
	seedData = append(seedData,
		SeedPermission{
			ID: 160, Domain: "default", Type: "menu", Name: "用户管理",
			Code: "system:user", MenuPath: "/system/user",
			Component: "system/user/index", ParentID: 3, SortOrder: 12,
			IsCommon: false, Status: 1, Description: "普通用户管理页面",
		},
		SeedPermission{ID: 161, Domain: "default", Type: "api", Name: "用户列表", Code: "user:list", Path: "/api/v1/admin/auth/users", Method: "GET", ParentID: 160, SortOrder: 0, IsCommon: false, Status: 1, Description: "用户列表API"},
		SeedPermission{ID: 162, Domain: "default", Type: "api", Name: "创建用户", Code: "user:create", Path: "/api/v1/admin/auth/users", Method: "POST", ParentID: 160, SortOrder: 1, IsCommon: false, Status: 1, Description: "创建用户API"},
		SeedPermission{ID: 163, Domain: "default", Type: "api", Name: "导出用户", Code: "user:export", Path: "/api/v1/admin/auth/users/export", Method: "POST", ParentID: 160, SortOrder: 2, IsCommon: false, Status: 1, Description: "导出用户API"},
		SeedPermission{ID: 164, Domain: "default", Type: "api", Name: "用户详情", Code: "user:detail", Path: "/api/v1/admin/auth/users/{id}", Method: "GET", ParentID: 160, SortOrder: 3, IsCommon: false, Status: 1, Description: "用户详情API"},
		SeedPermission{ID: 165, Domain: "default", Type: "api", Name: "更新用户", Code: "user:update", Path: "/api/v1/admin/auth/users/{id}", Method: "PUT", ParentID: 160, SortOrder: 4, IsCommon: false, Status: 1, Description: "更新用户API"},
		SeedPermission{ID: 166, Domain: "default", Type: "api", Name: "删除用户", Code: "user:delete-api", Path: "/api/v1/admin/auth/users/{id}", Method: "DELETE", ParentID: 160, SortOrder: 5, IsCommon: false, Status: 1, Description: "删除用户API"},
		SeedPermission{ID: 167, Domain: "default", Type: "api", Name: "启用用户", Code: "user:activate", Path: "/api/v1/admin/auth/users/{id}/activate", Method: "PUT", ParentID: 160, SortOrder: 6, IsCommon: false, Status: 1, Description: "启用用户API"},
		SeedPermission{ID: 168, Domain: "default", Type: "api", Name: "禁用用户", Code: "user:deactivate", Path: "/api/v1/admin/auth/users/{id}/deactivate", Method: "PUT", ParentID: 160, SortOrder: 7, IsCommon: false, Status: 1, Description: "禁用用户API"},
		SeedPermission{ID: 169, Domain: "default", Type: "api", Name: "加入黑名单", Code: "user:blacklist-add", Path: "/api/v1/admin/auth/users/{id}/blacklist", Method: "POST", ParentID: 160, SortOrder: 8, IsCommon: false, Status: 1, Description: "用户加入黑名单API"},
		SeedPermission{ID: 170, Domain: "default", Type: "api", Name: "移出黑名单", Code: "user:blacklist-remove", Path: "/api/v1/admin/auth/users/{id}/blacklist", Method: "DELETE", ParentID: 160, SortOrder: 9, IsCommon: false, Status: 1, Description: "用户移出黑名单API"},
		SeedPermission{ID: 171, Domain: "default", Type: "api", Name: "查看用户角色", Code: "user:roles-get", Path: "/api/v1/admin/auth/users/{id}/roles", Method: "GET", ParentID: 160, SortOrder: 10, IsCommon: false, Status: 1, Description: "查看用户角色API"},
		SeedPermission{ID: 172, Domain: "default", Type: "api", Name: "分配用户角色", Code: "user:roles-put", Path: "/api/v1/admin/auth/users/{id}/roles", Method: "PUT", ParentID: 160, SortOrder: 11, IsCommon: false, Status: 1, Description: "分配用户角色API"},
		SeedPermission{ID: 173, Domain: "default", Type: "api", Name: "用户第三方身份", Code: "user:third-party", Path: "/api/v1/admin/auth/users/{id}/third-party", Method: "GET", ParentID: 160, SortOrder: 12, IsCommon: false, Status: 1, Description: "用户第三方身份列表API"},
		SeedPermission{ID: 174, Domain: "default", Type: "button", Name: "用户管理-新增按钮", Code: "user:add", ParentID: 160, SortOrder: 0, IsCommon: false, Status: 1, Description: "用户管理页面新增按钮"},
		SeedPermission{ID: 175, Domain: "default", Type: "button", Name: "用户管理-编辑按钮", Code: "user:edit", ParentID: 160, SortOrder: 1, IsCommon: false, Status: 1, Description: "用户管理页面编辑按钮"},
		SeedPermission{ID: 176, Domain: "default", Type: "button", Name: "用户管理-删除按钮", Code: "user:delete", ParentID: 160, SortOrder: 2, IsCommon: false, Status: 1, Description: "用户管理页面删除按钮"},
		SeedPermission{ID: 177, Domain: "default", Type: "button", Name: "用户管理-分配角色按钮", Code: "user:assign-role", ParentID: 160, SortOrder: 3, IsCommon: false, Status: 1, Description: "用户管理页面分配角色按钮"},
		SeedPermission{ID: 178, Domain: "default", Type: "button", Name: "用户管理-启用按钮", Code: "user:activate-btn", ParentID: 160, SortOrder: 4, IsCommon: false, Status: 1, Description: "用户管理页面启用按钮"},
		SeedPermission{ID: 179, Domain: "default", Type: "button", Name: "用户管理-禁用按钮", Code: "user:deactivate-btn", ParentID: 160, SortOrder: 5, IsCommon: false, Status: 1, Description: "用户管理页面禁用按钮"},
		// 登录日志（菜单/API）
		SeedPermission{ID: 180, Domain: "default", Type: "menu", Name: "登录日志", Code: "system:login-log", MenuPath: "/system/login-log", Component: "system/login-log/index", ParentID: 3, SortOrder: 13, IsCommon: false, Status: 1, Description: "登录日志页面"},
		SeedPermission{ID: 181, Domain: "default", Type: "api", Name: "获取登录日志", Code: "login-log:list", Path: "/api/v1/admin/auth/login-logs", Method: "GET", ParentID: 180, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取登录日志列表API"},
		SeedPermission{ID: 182, Domain: "default", Type: "api", Name: "获取登录日志详情", Code: "login-log:get", Path: "/api/v1/admin/auth/login-logs/{id}", Method: "GET", ParentID: 180, SortOrder: 1, IsCommon: false, Status: 1, Description: "获取登录日志详情API"},
		// 任务队列（敏感基线同 cron-job：api 权限不自动授予 normal_admin）
		SeedPermission{ID: 183, Domain: "default", Type: "menu", Name: "任务队列", Code: "system:queue-job", MenuPath: "/system/queue-job", Component: "system/queue-job/index", ParentID: 3, SortOrder: 14, IsCommon: false, Status: 1, Description: "任务队列管理页面"},
		SeedPermission{ID: 184, Domain: "default", Type: "api", Name: "获取队列任务列表", Code: "queue-job:list", Path: "/api/v1/admin/auth/queue-jobs", Method: "GET", ParentID: 183, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取队列任务列表API"},
		SeedPermission{ID: 185, Domain: "default", Type: "api", Name: "手动入队任务", Code: "queue-job:enqueue", Path: "/api/v1/admin/auth/queue-jobs", Method: "POST", ParentID: 183, SortOrder: 1, IsCommon: false, Status: 1, Description: "手动入队队列任务API"},
		SeedPermission{ID: 186, Domain: "default", Type: "api", Name: "获取队列统计", Code: "queue-job:stats", Path: "/api/v1/admin/auth/queue-jobs/stats", Method: "GET", ParentID: 183, SortOrder: 2, IsCommon: false, Status: 1, Description: "获取队列统计API"},
		SeedPermission{ID: 187, Domain: "default", Type: "api", Name: "获取队列处理器", Code: "queue-job:handlers", Path: "/api/v1/admin/auth/queue-jobs/handlers", Method: "GET", ParentID: 183, SortOrder: 3, IsCommon: false, Status: 1, Description: "获取已注册处理器列表API"},
		SeedPermission{ID: 188, Domain: "default", Type: "api", Name: "获取队列任务详情", Code: "queue-job:get", Path: "/api/v1/admin/auth/queue-jobs/{queue}/{id}", Method: "GET", ParentID: 183, SortOrder: 4, IsCommon: false, Status: 1, Description: "获取队列任务详情API"},
		SeedPermission{ID: 189, Domain: "default", Type: "api", Name: "重试队列任务", Code: "queue-job:retry", Path: "/api/v1/admin/auth/queue-jobs/{queue}/{id}/retry", Method: "POST", ParentID: 183, SortOrder: 5, IsCommon: false, Status: 1, Description: "重试队列任务API"},
		SeedPermission{ID: 190, Domain: "default", Type: "api", Name: "取消队列任务", Code: "queue-job:cancel", Path: "/api/v1/admin/auth/queue-jobs/{queue}/{id}/cancel", Method: "POST", ParentID: 183, SortOrder: 6, IsCommon: false, Status: 1, Description: "取消执行中队列任务API"},
		SeedPermission{ID: 191, Domain: "default", Type: "api", Name: "删除队列任务", Code: "queue-job:delete", Path: "/api/v1/admin/auth/queue-jobs/{queue}/{id}", Method: "DELETE", ParentID: 183, SortOrder: 7, IsCommon: false, Status: 1, Description: "删除队列任务API"},
		SeedPermission{ID: 201, Domain: "default", Type: "api", Name: "获取队列能力", Code: "queue-job:capabilities", Path: "/api/v1/admin/auth/queue-jobs/capabilities", Method: "GET", ParentID: 183, SortOrder: 8, IsCommon: false, Status: 1},
		SeedPermission{ID: 202, Domain: "default", Type: "api", Name: "获取队列概览", Code: "queue-job:overview", Path: "/api/v1/admin/auth/queue-jobs/overview", Method: "GET", ParentID: 183, SortOrder: 9, IsCommon: false, Status: 1},
		SeedPermission{ID: 203, Domain: "default", Type: "api", Name: "暂停队列", Code: "queue-job:pause", Path: "/api/v1/admin/auth/queue-jobs/{queue}/pause", Method: "POST", ParentID: 183, SortOrder: 10, IsCommon: false, Status: 1},
		SeedPermission{ID: 204, Domain: "default", Type: "api", Name: "恢复队列", Code: "queue-job:resume", Path: "/api/v1/admin/auth/queue-jobs/{queue}/resume", Method: "POST", ParentID: 183, SortOrder: 11, IsCommon: false, Status: 1},
		SeedPermission{ID: 205, Domain: "default", Type: "api", Name: "查看原始消息", Code: "queue-job:raw-message", Path: "/api/v1/admin/auth/queue-jobs/{queue}/messages/{sequence}", Method: "GET", ParentID: 183, SortOrder: 12, IsCommon: false, Status: 1},
	)
	// 幻灯片管理权限(2026-09-25 幻灯片模块)
	seedData = append(seedData,
		SeedPermission{ID: 192, Domain: "default", Type: "menu", Name: "幻灯片管理", Code: "system:slide", MenuPath: "/system/slide", Component: "system/slide/index", ParentID: 3, SortOrder: 15, IsCommon: false, Status: 1, Description: "幻灯片管理页面"},
		SeedPermission{ID: 193, Domain: "default", Type: "api", Name: "幻灯片列表", Code: "slide:list", Path: "/api/v1/admin/auth/slides", Method: "GET", ParentID: 192, SortOrder: 0, IsCommon: false, Status: 1, Description: "获取幻灯片列表API"},
		SeedPermission{ID: 194, Domain: "default", Type: "api", Name: "创建幻灯片", Code: "slide:create", Path: "/api/v1/admin/auth/slides", Method: "POST", ParentID: 192, SortOrder: 1, IsCommon: false, Status: 1, Description: "创建幻灯片API"},
		SeedPermission{ID: 195, Domain: "default", Type: "api", Name: "幻灯片详情", Code: "slide:get", Path: "/api/v1/admin/auth/slides/{id}", Method: "GET", ParentID: 192, SortOrder: 2, IsCommon: false, Status: 1, Description: "获取幻灯片详情API"},
		SeedPermission{ID: 196, Domain: "default", Type: "api", Name: "更新幻灯片", Code: "slide:update", Path: "/api/v1/admin/auth/slides/{id}", Method: "PUT", ParentID: 192, SortOrder: 3, IsCommon: false, Status: 1, Description: "更新幻灯片API"},
		SeedPermission{ID: 197, Domain: "default", Type: "api", Name: "删除幻灯片", Code: "slide:delete-api", Path: "/api/v1/admin/auth/slides/{id}", Method: "DELETE", ParentID: 192, SortOrder: 4, IsCommon: false, Status: 1, Description: "删除幻灯片API"},
		SeedPermission{ID: 198, Domain: "default", Type: "button", Name: "幻灯片管理-新增按钮", Code: "slide:add", ParentID: 192, SortOrder: 0, IsCommon: false, Status: 1, Description: "幻灯片管理页面新增按钮"},
		SeedPermission{ID: 199, Domain: "default", Type: "button", Name: "幻灯片管理-编辑按钮", Code: "slide:edit", ParentID: 192, SortOrder: 1, IsCommon: false, Status: 1, Description: "幻灯片管理页面编辑按钮"},
		SeedPermission{ID: 200, Domain: "default", Type: "button", Name: "幻灯片管理-删除按钮", Code: "slide:delete", ParentID: 192, SortOrder: 2, IsCommon: false, Status: 1, Description: "幻灯片管理页面删除按钮"},
	)
	// 幻灯片分组管理权限(分组数据复用 slide_group 字典)
	seedData = append(seedData,
		SeedPermission{ID: 206, Domain: "default", Type: "api", Name: "幻灯片分组列表", Code: "slide-group:list", Path: "/api/v1/admin/auth/slide-groups", Method: "GET", ParentID: 192, SortOrder: 5, IsCommon: false, Status: 1, Description: "获取幻灯片分组列表API"},
		SeedPermission{ID: 207, Domain: "default", Type: "api", Name: "创建幻灯片分组", Code: "slide-group:create", Path: "/api/v1/admin/auth/slide-groups", Method: "POST", ParentID: 192, SortOrder: 6, IsCommon: false, Status: 1, Description: "创建幻灯片分组API"},
		SeedPermission{ID: 208, Domain: "default", Type: "api", Name: "更新幻灯片分组", Code: "slide-group:update", Path: "/api/v1/admin/auth/slide-groups/{id}", Method: "PUT", ParentID: 192, SortOrder: 7, IsCommon: false, Status: 1, Description: "更新幻灯片分组API"},
		SeedPermission{ID: 209, Domain: "default", Type: "api", Name: "删除幻灯片分组", Code: "slide-group:delete", Path: "/api/v1/admin/auth/slide-groups/{id}", Method: "DELETE", ParentID: 192, SortOrder: 8, IsCommon: false, Status: 1, Description: "删除幻灯片分组API"},
		SeedPermission{ID: 210, Domain: "default", Type: "button", Name: "幻灯片管理-分组管理按钮", Code: "slide:group", ParentID: 192, SortOrder: 3, IsCommon: false, Status: 1, Description: "幻灯片管理页面分组管理按钮"},
	)
	for i := range seedData {
		seedData[i].Domain = configuredPermissionDomain()
	}

	if err := svc.InitSeedPermissions(ctx, seedData); err != nil {
		return fmt.Errorf("初始化种子权限失败: %w", err)
	}

	return nil
}

// toResponse 转换为响应格式
func (s *SystemPermissionService) toResponse(p *ent.SystemPermission) *SystemPermissionResponse {
	return &SystemPermissionResponse{
		ID:          p.ID,
		Domain:      p.Domain,
		Type:        SystemPermissionType(p.Type),
		Name:        p.Name,
		Code:        p.Code,
		Path:        p.Path,
		Method:      p.Method,
		MenuPath:    p.MenuPath,
		Icon:        p.Icon,
		Component:   p.Component,
		ParentID:    p.ParentID,
		SortOrder:   p.SortOrder,
		IsCommon:    p.IsCommon,
		Status:      p.Status,
		Description: p.Description,
		CreatedAt:   p.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   p.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

// isDescendantOf 检查 candidateID 是否是 ancestorID 的后代（用于防止父子环依赖）
// 通过自下而上遍历 parent_id 链实现，带 maxDepth 与 visited 双重防御
func (s *SystemPermissionService) isDescendantOf(ctx context.Context, candidateID, ancestorID int) (bool, error) {
	const maxDepth = 100
	currentID := candidateID
	visited := make(map[int]bool, maxDepth)
	for i := 0; i < maxDepth; i++ {
		if currentID == 0 {
			return false, nil
		}
		if currentID == ancestorID {
			return true, nil
		}
		if visited[currentID] {
			return true, nil
		}
		visited[currentID] = true

		perm, err := s.client.SystemPermission.Query().
			Where(systempermission.IDEQ(currentID)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		currentID = perm.ParentID
	}
	return false, fmt.Errorf("权限层级超过最大深度 %d", maxDepth)
}
