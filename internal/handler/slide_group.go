package handler

import (
	"errors"
	"fmt"
	"strconv"

	"go-admin/internal/global"
	"go-admin/internal/service"
	"go-admin/internal/utils"

	"github.com/gofiber/fiber/v3"
	"github.com/wuwuseo/cmf/validate"
)

// ListSlideGroups 获取幻灯片分组列表
// @Summary 获取幻灯片分组列表
// @Description 获取幻灯片分组列表(字典类型 slide_group),含每个分组下的幻灯片数量
// @Tags 幻灯片管理
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=[]service.SlideGroupResponse}
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/admin/auth/slide-groups [get]
func ListSlideGroups(c fiber.Ctx) error {
	resp := c.Locals("utils.Response").(*utils.Response)
	svc := c.Locals("service.SlideGroupService").(*service.SlideGroupService)

	items, err := svc.List(c.Context())
	if err != nil {
		return resp.Fail(global.ResponseInternalServerError, "获取幻灯片分组失败").ToJSON(c)
	}

	return resp.Success(items, global.ResponseSuccessCode, "获取幻灯片分组成功").ToJSON(c)
}

// CreateSlideGroup 创建幻灯片分组
// @Summary 创建幻灯片分组
// @Description 新增幻灯片分组,写入字典类型 slide_group
// @Tags 幻灯片管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.CreateSlideGroupRequest true "分组信息"
// @Success 200 {object} utils.Response{data=service.SlideGroupResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/admin/auth/slide-groups [post]
func CreateSlideGroup(c fiber.Ctx) error {
	resp := c.Locals("utils.Response").(*utils.Response)
	svc := c.Locals("service.SlideGroupService").(*service.SlideGroupService)

	req, err := validate.ParseAndValidate[service.CreateSlideGroupRequest](c)
	if err != nil {
		if validate.IsValidationError(err) {
			return resp.Fail(global.ResponseErrorCode, validate.FirstErrorMessage(err)).ToJSON(c)
		}
		return resp.Fail(global.ResponseErrorCode, "请求参数无效").ToJSON(c)
	}

	item, err := svc.Create(c.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSlideGroupValueInvalid),
			errors.Is(err, service.ErrSlideGroupValueExists):
			return resp.Fail(global.ResponseErrorCode, err.Error()).ToJSON(c)
		}
		return resp.Fail(global.ResponseInternalServerError, "创建幻灯片分组失败").ToJSON(c)
	}

	logService := c.Locals("service.OperationLogService").(service.OperationLogServiceInterface)
	logOperation(c, logService, "system", "创建", fmt.Sprintf("创建幻灯片分组: %s(%s)", req.Label, req.Value))

	return resp.Success(item, global.ResponseSuccessCode, "幻灯片分组创建成功").ToJSON(c)
}

// UpdateSlideGroup 更新幻灯片分组
// @Summary 更新幻灯片分组
// @Description 更新幻灯片分组;修改分组编码会同步迁移该分组下的幻灯片
// @Tags 幻灯片管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "分组ID"
// @Param body body service.UpdateSlideGroupRequest true "更新信息"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/admin/auth/slide-groups/{id} [put]
func UpdateSlideGroup(c fiber.Ctx) error {
	resp := c.Locals("utils.Response").(*utils.Response)
	svc := c.Locals("service.SlideGroupService").(*service.SlideGroupService)

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return resp.Fail(global.ResponseErrorCode, "无效的分组ID").ToJSON(c)
	}

	req, err := validate.ParseAndValidate[service.UpdateSlideGroupRequest](c)
	if err != nil {
		if validate.IsValidationError(err) {
			return resp.Fail(global.ResponseErrorCode, validate.FirstErrorMessage(err)).ToJSON(c)
		}
		return resp.Fail(global.ResponseErrorCode, "请求参数无效").ToJSON(c)
	}

	if err := svc.Update(c.Context(), id, req); err != nil {
		switch {
		case errors.Is(err, service.ErrSlideGroupNotFound):
			return resp.Fail(global.ResponseNotFound, "幻灯片分组不存在").ToJSON(c)
		case errors.Is(err, service.ErrSlideGroupValueInvalid),
			errors.Is(err, service.ErrSlideGroupValueExists):
			return resp.Fail(global.ResponseErrorCode, err.Error()).ToJSON(c)
		}
		return resp.Fail(global.ResponseInternalServerError, "更新幻灯片分组失败").ToJSON(c)
	}

	logService := c.Locals("service.OperationLogService").(service.OperationLogServiceInterface)
	logOperation(c, logService, "system", "更新", fmt.Sprintf("更新幻灯片分组 ID:%d", id))

	return resp.Success(nil, global.ResponseSuccessCode, "幻灯片分组更新成功").ToJSON(c)
}

// DeleteSlideGroup 删除幻灯片分组
// @Summary 删除幻灯片分组
// @Description 删除幻灯片分组;分组下仍有幻灯片时拒绝删除
// @Tags 幻灯片管理
// @Produce json
// @Security BearerAuth
// @Param id path int true "分组ID"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/admin/auth/slide-groups/{id} [delete]
func DeleteSlideGroup(c fiber.Ctx) error {
	resp := c.Locals("utils.Response").(*utils.Response)
	svc := c.Locals("service.SlideGroupService").(*service.SlideGroupService)

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return resp.Fail(global.ResponseErrorCode, "无效的分组ID").ToJSON(c)
	}

	if err := svc.Delete(c.Context(), id); err != nil {
		switch {
		case errors.Is(err, service.ErrSlideGroupNotFound):
			return resp.Fail(global.ResponseNotFound, "幻灯片分组不存在").ToJSON(c)
		case errors.Is(err, service.ErrSlideGroupInUse):
			return resp.Fail(global.ResponseErrorCode, err.Error()).ToJSON(c)
		}
		return resp.Fail(global.ResponseInternalServerError, "删除幻灯片分组失败").ToJSON(c)
	}

	logService := c.Locals("service.OperationLogService").(service.OperationLogServiceInterface)
	logOperation(c, logService, "system", "删除", fmt.Sprintf("删除幻灯片分组 ID:%d", id))

	return resp.Success(nil, global.ResponseSuccessCode, "幻灯片分组删除成功").ToJSON(c)
}
