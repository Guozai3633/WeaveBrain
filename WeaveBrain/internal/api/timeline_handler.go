package api

import (
	"net/http"
	"strconv"

	"weavebrain/internal/db/repository"

	"github.com/gin-gonic/gin"
)

type TimelineHandler struct {
	store *repository.DBStore
}

func NewTimelineHandler(store *repository.DBStore) *TimelineHandler {
	return &TimelineHandler{store: store}
}

func (h *TimelineHandler) GetTimeline(c *gin.Context) {
	uid, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	events, total, err := h.store.Timeline.GetTimeline(c.Request.Context(), uid, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": events,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}
