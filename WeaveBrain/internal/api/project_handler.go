package api

import (
	"net/http"
	"strconv"

	"weavebrain/internal/entity"

	"github.com/gin-gonic/gin"
)

func (s *Server) setupProjectRoutes(group *gin.RouterGroup) {
	group.POST("", s.handleCreateProject)
	group.GET("", s.handleListProjects)
	group.GET("/:id", s.handleGetProject)
	group.PUT("/:id", s.handleUpdateProject)
	group.DELETE("/:id", s.handleDeleteProject)
}

type CreateProjectRequest struct {
	Name         string `json:"name" binding:"required"`
	DefaultProject bool `json:"default_project"`
}

func (s *Server) handleCreateProject(c *gin.Context) {
	// Extract user ID from auth middleware context (placeholder).
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	project, err := s.services.Project.CreateProject(c.Request.Context(), userID, req.Name, req.DefaultProject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create project"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"project": toProjectResponse(project)})
}

func (s *Server) handleListProjects(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	projects, err := s.services.Project.ListByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list projects"})
		return
	}

	resp := make([]map[string]any, len(projects))
	for i, p := range projects {
		resp[i] = toProjectResponse(p)
	}

	c.JSON(http.StatusOK, gin.H{"projects": resp})
}

func (s *Server) handleGetProject(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	project, err := s.services.Project.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"project": toProjectResponse(project)})
}

func (s *Server) handleUpdateProject(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	project, err := s.services.Project.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var req struct {
		Name           string `json:"name"`
		DefaultProject bool   `json:"default_project"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := s.services.Project.Update(c.Request.Context(), id, req.Name, req.DefaultProject); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update project"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "project updated"})
}

func (s *Server) handleDeleteProject(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	project, err := s.services.Project.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	if err := s.services.Project.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete project"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "project deleted"})
}

func toProjectResponse(p *entity.Project) map[string]any {
	return map[string]any{
		"id":              p.ID,
		"user_id":         p.UserID.String(),
		"name":            p.Name,
		"default_project": p.DefaultProject,
		"created_at":      p.CreatedAt,
		"updated_at":      p.UpdatedAt,
	}
}
