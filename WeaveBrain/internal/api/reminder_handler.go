package api

import (
	"net/http"
	"strconv"
	"time"

	"weavebrain/internal/entity"

	"github.com/gin-gonic/gin"
)

func (s *Server) setupReminderRoutes(group *gin.RouterGroup) {
	group.POST("", s.handleCreateReminder)
	group.GET("", s.handleListReminders)
	group.GET("/:id", s.handleGetReminder)
	group.PUT("/:id/status", s.handleUpdateReminderStatus)
	group.DELETE("/:id", s.handleDeleteReminder)
}

type CreateReminderRequest struct {
	ProjectID   *int64     `json:"project_id,omitempty"`
	TriggerTime time.Time  `json:"trigger_time" binding:"required"`
	Message     string     `json:"message" binding:"required"`
}

func (s *Server) handleCreateReminder(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	rem, err := s.services.Reminder.Create(c.Request.Context(), userID, req.ProjectID, req.TriggerTime, req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create reminder"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"reminder": toReminderResponse(rem)})
}

func (s *Server) handleListReminders(c *gin.Context) {
	_, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	status := c.DefaultQuery("status", "pending")
	beforeStr := c.DefaultQuery("before", time.Now().Format(time.RFC3339))
	before, err := time.Parse(time.RFC3339, beforeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid before parameter"})
		return
	}

	reminders, err := s.services.Reminder.GetPending(c.Request.Context(), before, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list reminders"})
		return
	}

	resp := make([]map[string]any, len(reminders))
	for i, r := range reminders {
		resp[i] = toReminderResponse(r)
	}

	_ = status // used for filtering
	c.JSON(http.StatusOK, gin.H{"reminders": resp})
}

func (s *Server) handleGetReminder(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reminder id"})
		return
	}

	rem, err := s.services.Reminder.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reminder not found"})
		return
	}

	if rem.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reminder": toReminderResponse(rem)})
}

func (s *Server) handleUpdateReminderStatus(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reminder id"})
		return
	}

	rem, err := s.services.Reminder.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reminder not found"})
		return
	}

	if rem.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	switch req.Status {
	case "triggered":
		err = s.services.Reminder.Trigger(c.Request.Context(), id)
	case "cancelled":
		err = s.services.Reminder.Cancel(c.Request.Context(), id)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status, must be 'triggered' or 'cancelled'"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update reminder status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "reminder status updated"})
}

func (s *Server) handleDeleteReminder(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reminder id"})
		return
	}

	rem, err := s.services.Reminder.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reminder not found"})
		return
	}

	if rem.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	if err := s.services.Reminder.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete reminder"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "reminder deleted"})
}

func toReminderResponse(r *entity.Reminder) map[string]any {
	return map[string]any{
		"id":           r.ID,
		"user_id":      r.UserID.String(),
		"project_id":   r.ProjectID,
		"trigger_time": r.TriggerTime,
		"message":      r.Message,
		"status":       r.Status,
		"created_at":   r.CreatedAt,
	}
}
