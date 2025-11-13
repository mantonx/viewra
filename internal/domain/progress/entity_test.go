package progress

import (
	"testing"
	"time"
)

func TestGetProgressPercentage(t *testing.T) {
	tests := []struct {
		name     string
		progress int
		duration int
		want     float64
	}{
		{
			name:     "Normal progress",
			progress: 45,
			duration: 100,
			want:     45.0,
		},
		{
			name:     "Zero duration",
			progress: 50,
			duration: 0,
			want:     0.0,
		},
		{
			name:     "Fully watched",
			progress: 100,
			duration: 100,
			want:     100.0,
		},
		{
			name:     "Over progress (capped at 100)",
			progress: 150,
			duration: 100,
			want:     100.0,
		},
		{
			name:     "Zero progress",
			progress: 0,
			duration: 100,
			want:     0.0,
		},
		{
			name:     "Partial progress",
			progress: 30,
			duration: 90,
			want:     33.33,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wp := &WatchProgress{
				ProgressSeconds: tt.progress,
				DurationSeconds: tt.duration,
			}
			got := wp.GetProgressPercentage()

			// Allow small floating point differences
			diff := got - tt.want
			if diff < -0.01 || diff > 0.01 {
				t.Errorf("GetProgressPercentage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldMarkWatched(t *testing.T) {
	tests := []struct {
		name     string
		progress int
		duration int
		want     bool
	}{
		{
			name:     "Exactly 90% - should mark watched",
			progress: 90,
			duration: 100,
			want:     true,
		},
		{
			name:     "89% - should NOT mark watched",
			progress: 89,
			duration: 100,
			want:     false,
		},
		{
			name:     "91% - should mark watched",
			progress: 91,
			duration: 100,
			want:     true,
		},
		{
			name:     "100% - should mark watched",
			progress: 100,
			duration: 100,
			want:     true,
		},
		{
			name:     "10% - should NOT mark watched",
			progress: 10,
			duration: 100,
			want:     false,
		},
		{
			name:     "Zero duration - should NOT mark watched",
			progress: 50,
			duration: 0,
			want:     false,
		},
		{
			name:     "Over 100% - should mark watched",
			progress: 150,
			duration: 100,
			want:     true,
		},
		{
			name:     "Exactly 89.99% - should NOT mark watched",
			progress: 8999,
			duration: 10000,
			want:     false,
		},
		{
			name:     "Exactly 90.00% - should mark watched",
			progress: 9000,
			duration: 10000,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wp := &WatchProgress{
				ProgressSeconds: tt.progress,
				DurationSeconds: tt.duration,
			}
			got := wp.ShouldMarkWatched()
			if got != tt.want {
				t.Errorf("ShouldMarkWatched() = %v, want %v (progress=%d, duration=%d, percentage=%.2f)",
					got, tt.want, tt.progress, tt.duration, wp.GetProgressPercentage())
			}
		})
	}
}

func TestUpdateProgress(t *testing.T) {
	tests := []struct {
		name            string
		initialProgress int
		initialWatched  bool
		newProgress     int
		duration        int
		wantProgress    int
		wantWatched     bool
	}{
		{
			name:            "Update normal progress",
			initialProgress: 30,
			initialWatched:  false,
			newProgress:     45,
			duration:        100,
			wantProgress:    45,
			wantWatched:     false,
		},
		{
			name:            "Update to 90% - auto marks watched",
			initialProgress: 50,
			initialWatched:  false,
			newProgress:     90,
			duration:        100,
			wantProgress:    90,
			wantWatched:     true,
		},
		{
			name:            "Update already watched - stays watched",
			initialProgress: 95,
			initialWatched:  true,
			newProgress:     50,
			duration:        100,
			wantProgress:    50,
			wantWatched:     true,
		},
		{
			name:            "Update below 90% - doesn't auto mark",
			initialProgress: 30,
			initialWatched:  false,
			newProgress:     89,
			duration:        100,
			wantProgress:    89,
			wantWatched:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wp := &WatchProgress{
				ProgressSeconds: tt.initialProgress,
				DurationSeconds: tt.duration,
				IsWatched:       tt.initialWatched,
			}

			wp.UpdateProgress(tt.newProgress)
			wp.DurationSeconds = tt.duration

			if wp.ProgressSeconds != tt.wantProgress {
				t.Errorf("ProgressSeconds = %v, want %v", wp.ProgressSeconds, tt.wantProgress)
			}
			if wp.IsWatched != tt.wantWatched {
				t.Errorf("IsWatched = %v, want %v", wp.IsWatched, tt.wantWatched)
			}
		})
	}
}

func TestMarkWatched(t *testing.T) {
	wp := &WatchProgress{
		ProgressSeconds: 50,
		DurationSeconds: 100,
		IsWatched:       false,
	}

	beforeTime := time.Now()
	wp.MarkWatched()
	afterTime := time.Now()

	if !wp.IsWatched {
		t.Error("MarkWatched() did not set IsWatched to true")
	}

	if wp.ProgressSeconds != 100 {
		t.Errorf("MarkWatched() set ProgressSeconds = %v, want 100", wp.ProgressSeconds)
	}

	if wp.LastWatchedAt.Before(beforeTime) || wp.LastWatchedAt.After(afterTime) {
		t.Error("MarkWatched() did not update LastWatchedAt correctly")
	}
}

func TestMarkUnwatched(t *testing.T) {
	wp := &WatchProgress{
		ProgressSeconds: 95,
		DurationSeconds: 100,
		IsWatched:       true,
		LastWatchedAt:   time.Now(),
	}

	wp.MarkUnwatched()

	if wp.IsWatched {
		t.Error("MarkUnwatched() did not set IsWatched to false")
	}

	if wp.ProgressSeconds != 0 {
		t.Errorf("MarkUnwatched() set ProgressSeconds = %v, want 0", wp.ProgressSeconds)
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name    string
		wp      WatchProgress
		wantErr bool
	}{
		{
			name: "Valid progress",
			wp: WatchProgress{
				MediaID:         1,
				UserID:          1,
				ProgressSeconds: 50,
				DurationSeconds: 100,
			},
			wantErr: false,
		},
		{
			name: "Invalid - zero MediaID",
			wp: WatchProgress{
				MediaID:         0,
				UserID:          1,
				ProgressSeconds: 50,
				DurationSeconds: 100,
			},
			wantErr: true,
		},
		{
			name: "Invalid - negative progress",
			wp: WatchProgress{
				MediaID:         1,
				UserID:          1,
				ProgressSeconds: -10,
				DurationSeconds: 100,
			},
			wantErr: true,
		},
		{
			name: "Invalid - negative duration",
			wp: WatchProgress{
				MediaID:         1,
				UserID:          1,
				ProgressSeconds: 50,
				DurationSeconds: -100,
			},
			wantErr: true,
		},
		{
			name: "Invalid - progress exceeds duration",
			wp: WatchProgress{
				MediaID:         1,
				UserID:          1,
				ProgressSeconds: 150,
				DurationSeconds: 100,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.wp.IsValid()
			if (err != nil) != tt.wantErr {
				t.Errorf("IsValid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
