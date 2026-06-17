package service

import (
	"strings"

	bizerrors "YoudaoNoteLm/pkg/errors"
)

// BuildSearchQuery 统一构建搜索 query。
func BuildSearchQuery(req *SearchRequest) (string, error) {
	if req == nil {
		return "", bizerrors.New(bizerrors.CodeInvalidParam, "搜索请求不能为空")
	}

	query := strings.Join(strings.Fields(strings.TrimSpace(req.Query)), " ")
	if query == "" {
		return "", bizerrors.New(bizerrors.CodeInvalidParam, "搜索 query 不能为空")
	}

	switch req.Scene {
	case SearchSceneGeneration, SearchSceneChat, SearchSceneImport, SearchSceneSourcePreview:
		return query, nil
	default:
		return "", bizerrors.New(bizerrors.CodeInvalidParam, "无效的搜索场景")
	}
}
