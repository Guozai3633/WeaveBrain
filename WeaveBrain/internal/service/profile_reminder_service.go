package service

import (
	"context"
	"errors"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

// UserProfileService handles long-term user profile data.
type UserProfileService struct {
	repos *repository.DBStore
}

func NewUserProfileService(repos *repository.DBStore) *UserProfileService {
	return &UserProfileService{repos: repos}
}

// Get retrieves a user profile.
func (s *UserProfileService) Get(ctx context.Context, userID uuid.UUID) (*entity.UserProfile, error) {
	return s.repos.UserProfile.GetByUserID(ctx, userID)
}

// Upsert updates or creates a user profile.
func (s *UserProfileService) Upsert(ctx context.Context, userID uuid.UUID, data map[string]any) (*entity.UserProfile, error) {
	p, err := s.repos.UserProfile.GetByUserID(ctx, userID)
	if err != nil {
		// Create new profile
		p = entity.NewUserProfile(userID)
		p.ProfileData = data
		p.LastUpdated = time.Now()
		if err := s.repos.UserProfile.Create(ctx, p); err != nil {
			return nil, err
		}
		return p, nil
	}

	// Merge data into existing profile
	for k, v := range data {
		p.ProfileData[k] = v
	}
	p.LastUpdated = time.Now()
	if err := s.repos.UserProfile.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete removes a user profile.
func (s *UserProfileService) Delete(ctx context.Context, userID uuid.UUID) error {
	return s.repos.UserProfile.Delete(ctx, userID)
}

// ReminderService handles scheduled notifications.
type ReminderService struct {
	repos *repository.DBStore
}

func NewReminderService(repos *repository.DBStore) *ReminderService {
	return &ReminderService{repos: repos}
}

// Create creates a new reminder.
func (s *ReminderService) Create(ctx context.Context, userID uuid.UUID, projectID *int64, triggerTime time.Time, message string) (*entity.Reminder, error) {
	if message == "" {
		return nil, errors.New("reminder message is required")
	}
	if triggerTime.IsZero() {
		return nil, errors.New("trigger time is required")
	}

	rem := &entity.Reminder{
		UserID:      userID,
		ProjectID:   projectID,
		TriggerTime: triggerTime,
		Message:     message,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.repos.Reminder.Create(ctx, rem); err != nil {
		return nil, err
	}
	return rem, nil
}

// GetByID retrieves a reminder.
func (s *ReminderService) GetByID(ctx context.Context, id int64) (*entity.Reminder, error) {
	return s.repos.Reminder.GetByID(ctx, id)
}

// GetPending returns pending reminders before the given time.
func (s *ReminderService) GetPending(ctx context.Context, before time.Time, limit int) ([]*entity.Reminder, error) {
	if limit < 1 {
		limit = 50
	}
	return s.repos.Reminder.GetPending(ctx, before, limit)
}

// Trigger marks a reminder as triggered.
func (s *ReminderService) Trigger(ctx context.Context, id int64) error {
	rem, err := s.repos.Reminder.GetByID(ctx, id)
	if err != nil {
		return err
	}
	rem.Status = "triggered"
	return s.repos.Reminder.Update(ctx, rem)
}

// Cancel marks a reminder as cancelled.
func (s *ReminderService) Cancel(ctx context.Context, id int64) error {
	rem, err := s.repos.Reminder.GetByID(ctx, id)
	if err != nil {
		return err
	}
	rem.Status = "cancelled"
	return s.repos.Reminder.Update(ctx, rem)
}

// Delete removes a reminder.
func (s *ReminderService) Delete(ctx context.Context, id int64) error {
	return s.repos.Reminder.Delete(ctx, id)
}
