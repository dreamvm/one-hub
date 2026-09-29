package midjourney

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"one-api/common"
	"one-api/common/image"
	"one-api/controller"
	"one-api/model"
	provider "one-api/providers/midjourney"
)

func RelayMidjourneyImage(c *gin.Context) {
	task := model.GetByOnlyMJId(c.Param("id"))
	if task == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "midjourney_task_not_found"})
		return
	}
	response, err := image.RequestPublicFile(c.Request.Context(), task.ImageUrl)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "http_get_image_failed"})
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": "http_get_image_failed"})
		return
	}
	// Read the bounded body before sending a success status. Failed or oversized
	// downloads must not become a partial 200 response or expose upstream errors.
	body, err := io.ReadAll(response.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "http_get_image_failed"})
		return
	}
	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, contentType, body)
}

func RelayMidjourneyNotify(c *gin.Context) *provider.MidjourneyResponse {
	var notification struct {
		ID string `json:"id"`
	}
	if err := common.UnmarshalBodyReusable(c, &notification); err != nil {
		return provider.MidjourneyErrorWrapper(4, "bind_request_body_failed")
	}
	userID := c.GetInt("id")
	if userID <= 0 || notification.ID == "" {
		return provider.MidjourneyErrorWrapper(4, "midjourney_task_not_found")
	}
	task := model.GetByMJId(userID, notification.ID)
	if task == nil {
		return provider.MidjourneyErrorWrapper(4, "midjourney_task_not_found")
	}
	// mj-api-secret authenticates a user token, not an upstream provider. Treat
	// notification data only as a wakeup hint; bound-channel polling owns updates.
	if task.Progress != "100%" {
		controller.ActivateUpdateMidjourneyTaskBulk()
	}
	return nil
}
