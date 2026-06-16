package api

import (
	"net/http"
	"strconv"
	"strings"

	"weavebrain/internal/entity"

	"github.com/gin-gonic/gin"
)

func (s *Server) setupIdeaRoutes(group *gin.RouterGroup) {
	group.POST("", s.handleCreateIdea)
	group.GET("", s.handleListIdeas)
	group.GET("/:id", s.handleGetIdea)
	group.PUT("/:id", s.handleUpdateIdea)
	group.DELETE("/:id", s.handleDeleteIdea)
}

type CreateIdeaRequest struct {
	RawInput       string         `json:"raw_input" binding:"required"`
	StructuredData map[string]any  `json:"structured_data"`
	Tags           []string       `json:"tags"`
}

func (s *Server) handleCreateIdea(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateIdeaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// project_id from query param or default.
	projectID, err := strconv.ParseInt(c.Query("project_id"), 10, 64)
	if err != nil || projectID == 0 {
		// Get user's default project (placeholder: will be resolved via service)
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
		return
	}

	idea, err := s.services.Idea.Create(c.Request.Context(), projectID, userID, req.RawInput, req.StructuredData, req.Tags)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create idea"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"idea": toIdeaResponse(idea)})
}

func (s *Server) handleListIdeas(c *gin.Context) {
	_, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	projectID, err := strconv.ParseInt(c.Query("project_id"), 10, 64)
	if err != nil || projectID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	search := c.Query("search")
	tagsParam := c.Query("tags")

	var tags []string
	if tagsParam != "" {
		for _, t := range strings.Split(tagsParam, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	var ideas []*entity.Idea
	var total int64

	if search != "" || len(tags) > 0 {
		ideas, total, err = s.services.Idea.Search(c.Request.Context(), projectID, search, tags, page, limit)
	} else {
		ideas, total, err = s.services.Idea.ListByProject(c.Request.Context(), projectID, page, limit)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list ideas"})
		return
	}

	resp := make([]map[string]any, len(ideas))
	for i, idea := range ideas {
		resp[i] = toIdeaResponse(idea)
	}

	c.JSON(http.StatusOK, gin.H{
		"ideas":  resp,
		"total":  total,
		"page":   page,
		"limit":  limit,
	})
}

func (s *Server) handleGetIdea(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid idea id"})
		return
	}

	idea, err := s.services.Idea.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "idea not found"})
		return
	}

	if idea.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"idea": toIdeaResponse(idea)})
}

func (s *Server) handleUpdateIdea(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid idea id"})
		return
	}

	idea, err := s.services.Idea.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "idea not found"})
		return
	}

	if idea.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var req struct {
		RawInput       string         `json:"raw_input"`
		StructuredData map[string]any  `json:"structured_data"`
		Tags           []string       `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := s.services.Idea.Update(c.Request.Context(), id, req.RawInput, req.StructuredData, req.Tags); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update idea"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "idea updated"})
}

func (s *Server) handleDeleteIdea(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid idea id"})
		return
	}

	idea, err := s.services.Idea.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "idea not found"})
		return
	}

	if idea.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	if err := s.services.Idea.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete idea"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "idea deleted"})
}

func toIdeaResponse(i *entity.Idea) map[string]any {
	resp := map[string]any{
		"id":           i.ID,
		"project_id":   i.ProjectID,
		"user_id":      i.UserID.String(),
		"raw_input":    i.RawInput,
		"tags":         i.Tags,
		"created_at":   i.CreatedAt,
		"updated_at":   i.UpdatedAt,
	}
	if i.StructuredData != nil {
		resp["structured_data"] = i.StructuredData
	}
	return resp
}
