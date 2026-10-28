package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"go-admin/internal/ent"
	"go-admin/internal/ent/dictdata"
	"go-admin/internal/ent/slide"
	"go-admin/internal/global"
)

// SlideGroupDictType 幻灯片分组对应的字典类型编码
const SlideGroupDictType = "slide_group"

var (
	// ErrSlideGroupNotFound 幻灯片分组不存在
	ErrSlideGroupNotFound = errors.New("幻灯片分组不存在")
	// ErrSlideGroupInUse 幻灯片分组下仍有幻灯片
	ErrSlideGroupInUse = errors.New("幻灯片分组下仍有幻灯片")
	// ErrSlideGroupValueExists 幻灯片分组编码重复
	ErrSlideGroupValueExists = errors.New("分组编码已存在")
	// ErrSlideGroupValueInvalid 幻灯片分组编码格式非法
	ErrSlideGroupValueInvalid = errors.New("分组编码只能包含字母、数字、下划线和短横线")
)

// slideGroupValuePattern 分组编码格式:字母开头,允许字母数字下划线短横线
var slideGroupValuePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// SlideGroupService 幻灯片分组服务
// 分组数据复用字典表(dict_data),字典类型固定为 slide_group,
// 字典项的 label 为分组名称、value 为分组编码(即 slides.group_code)。
type SlideGroupService struct {
	*CRUDBase
}

// NewSlideGroupService 创建幻灯片分组服务实例
func NewSlideGroupService() *SlideGroupService {
	return &SlideGroupService{
		CRUDBase: &CRUDBase{Client: global.ORM},
	}
}

// CreateSlideGroupRequest 创建幻灯片分组请求
type CreateSlideGroupRequest struct {
	Label  string `json:"label" validate:"required,max=100"`
	Value  string `json:"value" validate:"required,max=100"`
	Sort   *int   `json:"sort"`
	Status *int   `json:"status"`
	Remark string `json:"remark" validate:"omitempty,max=500"`
}

// UpdateSlideGroupRequest 更新幻灯片分组请求
type UpdateSlideGroupRequest struct {
	Label  *string `json:"label" validate:"omitempty,max=100"`
	Value  *string `json:"value" validate:"omitempty,max=100"`
	Sort   *int    `json:"sort"`
	Status *int    `json:"status"`
	Remark *string `json:"remark" validate:"omitempty,max=500"`
}

// SlideGroupResponse 幻灯片分组响应
type SlideGroupResponse struct {
	ID         int       `json:"id"`
	Label      string    `json:"label"`
	Value      string    `json:"value"`
	Sort       int       `json:"sort"`
	Status     int       `json:"status"`
	Remark     string    `json:"remark,omitempty"`
	SlideCount int       `json:"slide_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// List 获取幻灯片分组列表(含分组下幻灯片数量),按 sort 升序
func (s *SlideGroupService) List(ctx context.Context) ([]*SlideGroupResponse, error) {
	items, err := s.Client.DictData.Query().
		Where(dictdata.DictTypeEQ(SlideGroupDictType)).
		Order(ent.Asc(dictdata.FieldSort), ent.Asc(dictdata.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询幻灯片分组失败: %w", err)
	}

	counts, err := s.countSlidesByGroup(ctx)
	if err != nil {
		return nil, err
	}

	data := make([]*SlideGroupResponse, 0, len(items))
	for _, item := range items {
		data = append(data, &SlideGroupResponse{
			ID:         item.ID,
			Label:      item.Label,
			Value:      item.Value,
			Sort:       item.Sort,
			Status:     item.Status,
			Remark:     item.Remark,
			SlideCount: counts[item.Value],
			CreatedAt:  item.CreatedAt,
			UpdatedAt:  item.UpdatedAt,
		})
	}

	return data, nil
}

// Create 创建幻灯片分组
func (s *SlideGroupService) Create(ctx context.Context, req *CreateSlideGroupRequest) (*SlideGroupResponse, error) {
	if !slideGroupValuePattern.MatchString(req.Value) {
		return nil, ErrSlideGroupValueInvalid
	}

	exists, err := s.Client.DictData.Query().
		Where(
			dictdata.DictTypeEQ(SlideGroupDictType),
			dictdata.ValueEQ(req.Value),
		).
		Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("检查分组编码失败: %w", err)
	}
	if exists {
		return nil, ErrSlideGroupValueExists
	}

	builder := s.Client.DictData.Create().
		SetLabel(req.Label).
		SetValue(req.Value).
		SetDictType(SlideGroupDictType)

	if req.Sort != nil {
		builder.SetSort(*req.Sort)
	}
	if req.Status != nil {
		builder.SetStatus(*req.Status)
	}
	if req.Remark != "" {
		builder.SetRemark(req.Remark)
	}

	item, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建幻灯片分组失败: %w", err)
	}

	return &SlideGroupResponse{
		ID:        item.ID,
		Label:     item.Label,
		Value:     item.Value,
		Sort:      item.Sort,
		Status:    item.Status,
		Remark:    item.Remark,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}, nil
}

// Update 更新幻灯片分组
// 修改分组编码时会同步迁移该分组下的所有幻灯片,避免出现孤儿数据。
func (s *SlideGroupService) Update(ctx context.Context, id int, req *UpdateSlideGroupRequest) error {
	item, err := s.Client.DictData.Query().
		Where(
			dictdata.IDEQ(id),
			dictdata.DictTypeEQ(SlideGroupDictType),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrSlideGroupNotFound
		}
		return fmt.Errorf("查询幻灯片分组失败: %w", err)
	}

	nextValue := item.Value
	if req.Value != nil && *req.Value != item.Value {
		if !slideGroupValuePattern.MatchString(*req.Value) {
			return ErrSlideGroupValueInvalid
		}
		exists, err := s.Client.DictData.Query().
			Where(
				dictdata.DictTypeEQ(SlideGroupDictType),
				dictdata.ValueEQ(*req.Value),
				dictdata.IDNEQ(id),
			).
			Exist(ctx)
		if err != nil {
			return fmt.Errorf("检查分组编码失败: %w", err)
		}
		if exists {
			return ErrSlideGroupValueExists
		}
		nextValue = *req.Value
	}

	tx, err := s.Client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	update := tx.DictData.UpdateOneID(item.ID)
	if req.Label != nil {
		update.SetLabel(*req.Label)
	}
	if req.Sort != nil {
		update.SetSort(*req.Sort)
	}
	if req.Status != nil {
		update.SetStatus(*req.Status)
	}
	if req.Remark != nil {
		update.SetRemark(*req.Remark)
	}
	if nextValue != item.Value {
		update.SetValue(nextValue)
	}

	if _, err := update.Save(ctx); err != nil {
		return rollbackSlideGroupTx(tx, fmt.Errorf("更新幻灯片分组失败: %w", err))
	}

	if nextValue != item.Value {
		if _, err := tx.Slide.Update().
			Where(slide.GroupCodeEQ(item.Value)).
			SetGroupCode(nextValue).
			Save(ctx); err != nil {
			return rollbackSlideGroupTx(tx, fmt.Errorf("迁移分组下幻灯片失败: %w", err))
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

// Delete 删除幻灯片分组
// 分组下仍有幻灯片时拒绝删除,避免幻灯片引用失效的分组编码。
func (s *SlideGroupService) Delete(ctx context.Context, id int) error {
	item, err := s.Client.DictData.Query().
		Where(
			dictdata.IDEQ(id),
			dictdata.DictTypeEQ(SlideGroupDictType),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrSlideGroupNotFound
		}
		return fmt.Errorf("查询幻灯片分组失败: %w", err)
	}

	used, err := s.Client.Slide.Query().
		Where(slide.GroupCodeEQ(item.Value)).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("检查分组占用情况失败: %w", err)
	}
	if used > 0 {
		return fmt.Errorf("%w: 分组「%s」下还有 %d 张幻灯片,请先移动或删除", ErrSlideGroupInUse, item.Label, used)
	}

	if err := s.Client.DictData.DeleteOneID(item.ID).Exec(ctx); err != nil {
		return fmt.Errorf("删除幻灯片分组失败: %w", err)
	}

	return nil
}

// countSlidesByGroup 统计各分组下的幻灯片数量
func (s *SlideGroupService) countSlidesByGroup(ctx context.Context) (map[string]int, error) {
	codes, err := s.Client.Slide.Query().
		Select(slide.FieldGroupCode).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("统计分组幻灯片数量失败: %w", err)
	}

	counts := make(map[string]int, len(codes))
	for _, code := range codes {
		counts[code]++
	}

	return counts, nil
}

// rollbackSlideGroupTx 回滚事务并返回原始错误
func rollbackSlideGroupTx(tx *ent.Tx, err error) error {
	if rbErr := tx.Rollback(); rbErr != nil {
		return fmt.Errorf("%w (回滚失败: %v)", err, rbErr)
	}
	return err
}
