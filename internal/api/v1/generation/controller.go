package generation

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
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
