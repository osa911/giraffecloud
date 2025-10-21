package handlers

import (
	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/api/dto/v1/contact"
	"giraffecloud/internal/service"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
)

type ContactHandler struct {
	telegramService  *service.TelegramService
	recaptchaService *service.RecaptchaService
}

func NewContactHandler() *ContactHandler {
	return &ContactHandler{
		telegramService:  service.NewTelegramService(),
		recaptchaService: service.NewRecaptchaService(),
	}
}

func (h *ContactHandler) Submit(c *gin.Context) {
	// Get contact data from context (set by validation middleware)
	contactData, exists := c.Get(constants.ContextKeyContact)
	if !exists {
		utils.HandleAPIError(c, nil, common.ErrCodeInternalServer, "Contact data not found in context")
		return
	}

	// Extract and convert to ContactRequest
	contactPtr, ok := contactData.(*contact.ContactRequest)
	if !ok {
		utils.HandleAPIError(c, nil, common.ErrCodeInternalServer, "Invalid contact data format")
		return
	}

	// Verify reCAPTCHA token (minimum score 0.5 for v3)
	isValid, err := h.recaptchaService.VerifyToken(contactPtr.RecaptchaToken, 0.5)
	if err != nil || !isValid {
		utils.HandleAPIError(c, err, common.ErrCodeBadRequest, "reCAPTCHA verification failed")
		return
	}

	// Send message to Telegram
	err = h.telegramService.SendContactMessage(
		contactPtr.Name,
		contactPtr.Email,
		contactPtr.Message,
	)
	if err != nil {
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to send message")
		return
	}

	// Return success response
	utils.HandleSuccess(c, contact.ContactResponse{
		Message: "Message sent successfully. We'll get back to you shortly.",
		Success: true,
	})
}
