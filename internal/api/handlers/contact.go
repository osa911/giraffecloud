package handlers

import (
	"github.com/osa911/giraffecloud/internal/api/constants"
	"github.com/osa911/giraffecloud/internal/api/dto/common"
	"github.com/osa911/giraffecloud/internal/api/dto/v1/contact"
	"github.com/osa911/giraffecloud/internal/db/ent"
	"github.com/osa911/giraffecloud/internal/service"
	"github.com/osa911/giraffecloud/internal/utils"

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

	// Gather additional information about the submission
	messageInfo := &service.ContactMessageInfo{
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Referrer:  c.Request.Referer(),
	}

	// Check if user is authenticated (optional - contact form is public)
	if userID, exists := c.Get(constants.ContextKeyUserID); exists {
		if id, ok := userID.(int); ok {
			messageInfo.UserID = &id
			messageInfo.IsAuthenticated = true

			// Try to get user's registered email from context
			if userData, userExists := c.Get(constants.ContextKeyUser); userExists {
				if user, ok := userData.(*ent.User); ok && user.Email != "" {
					messageInfo.UserEmail = &user.Email
				}
			}
		}
	}

	// Send message to Telegram
	err = h.telegramService.SendContactMessage(
		contactPtr.Name,
		contactPtr.Email,
		contactPtr.Message,
		messageInfo,
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
