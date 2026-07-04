package generation

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/response"
	"mime"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller handles generation HTTP requests.
type Controller struct {
	generationService service.GenerationService
}

// NewController creates a generation controller.
func NewController(generationService service.GenerationService) *Controller {
	return &Controller{generationService: generationService}
}

// Generate runs the supervisor generation service.
func (ctrl *Controller) Generate(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "user is not authenticated")
		return
	}

	var req request.GenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.generationService.Generate(c.Request.Context(), &service.GenerationRequest{
		UserID:       userID,
		NotebookID:   req.NotebookID,
		Markdown:     req.Markdown,
		Type:         service.GenerationType(req.Type),
		Prompt:       req.Prompt,
		Options:      req.Options,
		SourceIDs:    req.SourceIDs,
		UseWeb:       req.UseWeb,
		AllowDegrade: req.AllowDegrade,
	})
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// Export converts generated content into a downloadable attachment.
func (ctrl *Controller) Export(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "user is not authenticated")
		return
	}

	var req request.GenerationExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.generationService.Export(c.Request.Context(), &service.GenerationExportRequest{
		Type:     service.GenerationType(req.Type),
		Content:  req.Content,
		Title:    req.Title,
		Template: req.Template,
	})
	if err != nil {
		logger.Warn("generation export failed",
			zap.Uint("user_id", userID),
			zap.String("type", req.Type),
			zap.Int("content_len", len(req.Content)),
			zap.Bool("contains_section", strings.Contains(strings.ToLower(req.Content), "<section")),
			zap.String("template", req.Template),
			zap.Error(err),
		)
		response.BizError(c, err)
		return
	}
	logger.Info("generation export completed",
		zap.Uint("user_id", userID),
		zap.String("type", req.Type),
		zap.Int("content_len", len(req.Content)),
		zap.Int("output_len", len(resp.Data)),
		zap.String("content_type", resp.ContentType),
		zap.String("filename", resp.Filename),
	)

	c.Header("Content-Type", resp.ContentType)
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": resp.Filename}))
	c.Data(200, resp.ContentType, resp.Data)
}
