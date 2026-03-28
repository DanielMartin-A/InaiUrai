package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
)

type HeartbeatService struct {
	engagementRepo *repository.EngagementRepo
	taskMgr        *TaskManager
	lastFired      map[string]time.Time
}

func NewHeartbeatService(er *repository.EngagementRepo, tm *TaskManager) *HeartbeatService {
	return &HeartbeatService{engagementRepo: er, taskMgr: tm, lastFired: make(map[string]time.Time)}
}

type HeartbeatConfig struct {
	Schedule        string `json:"schedule"`
	TaskDescription string `json:"task_description"`
}

func (s *HeartbeatService) Run(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("heartbeat scheduler stopped")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *HeartbeatService) tick(ctx context.Context) {
	engagements, err := s.engagementRepo.GetActiveWithHeartbeats(ctx)
	if err != nil {
		slog.Error("heartbeat fetch failed", "error", err)
		return
	}

	now := time.Now()
	for _, eng := range engagements {
		var configs map[string]HeartbeatConfig
		if err := json.Unmarshal(eng.HeartbeatConfig, &configs); err != nil {
			continue
		}
		for roleSlug, cfg := range configs {
			if shouldFire(cfg.Schedule, now) {
				key := eng.ID.String() + ":" + roleSlug
				cooldown := scheduleCooldown(cfg.Schedule)
				if last, ok := s.lastFired[key]; ok && now.Sub(last) < cooldown {
					continue
				}
				slog.Info("firing heartbeat", "engagement_id", eng.ID, "role", roleSlug)
				if err := s.taskMgr.ExecuteHeartbeatTask(ctx, &eng, roleSlug, cfg.TaskDescription); err != nil {
					slog.Error("heartbeat task failed", "error", err, "engagement_id", eng.ID, "role", roleSlug)
				} else {
					s.lastFired[key] = now
				}
			}
		}
	}
}

func scheduleCooldown(schedule string) time.Duration {
	switch schedule {
	case "daily":
		return 20 * time.Hour
	case "weekly":
		return 6 * 24 * time.Hour
	case "biweekly":
		return 13 * 24 * time.Hour
	default:
		return 20 * time.Hour
	}
}

func shouldFire(schedule string, now time.Time) bool {
	hour := now.Hour()
	minute := now.Minute()
	weekday := now.Weekday()

	switch schedule {
	case "daily":
		return hour == 9 && minute < 15
	case "weekly":
		return weekday == time.Monday && hour == 9 && minute < 15
	case "biweekly":
		_, week := now.ISOWeek()
		return week%2 == 0 && weekday == time.Monday && hour == 9 && minute < 15
	default:
		return false
	}
}
