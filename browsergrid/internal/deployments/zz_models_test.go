package deployments

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestDeployment_BeforeCreate(t *testing.T) {
	tests := []struct {
		name       string
		deployment Deployment
		checkFunc  func(t *testing.T, d *Deployment)
	}{
		{
			name: "generates UUID when nil",
			deployment: Deployment{
				Name:        "test-deployment",
				Version:     "1.0.0",
				Runtime:     RuntimeNode,
				PackageURL:  "https://example.com/package.zip",
				PackageHash: "hash123",
			},
			checkFunc: func(t *testing.T, d *Deployment) {
				assert.NotEqual(t, uuid.Nil, d.ID)
				assert.NotZero(t, d.CreatedAt)
				assert.NotZero(t, d.UpdatedAt)
				assert.Equal(t, StatusActive, d.Status)
				assert.NotNil(t, d.Config)
			},
		},
		{
			name: "preserves existing UUID",
			deployment: Deployment{
				ID:          uuid.New(),
				Name:        "test-deployment",
				Version:     "1.0.0",
				Runtime:     RuntimeNode,
				PackageURL:  "https://example.com/package.zip",
				PackageHash: "hash123",
			},
			checkFunc: func(t *testing.T, d *Deployment) {
				assert.NotEqual(t, uuid.Nil, d.ID)
			},
		},
		{
			name: "sets default config when nil",
			deployment: Deployment{
				Name:        "test-deployment",
				Version:     "1.0.0",
				Runtime:     RuntimeNode,
				PackageURL:  "https://example.com/package.zip",
				PackageHash: "hash123",
			},
			checkFunc: func(t *testing.T, d *Deployment) {
				assert.Equal(t, datatypes.JSON("{}"), d.Config)
			},
		},
		{
			name: "sets default status when empty",
			deployment: Deployment{
				Name:        "test-deployment",
				Version:     "1.0.0",
				Runtime:     RuntimeNode,
				PackageURL:  "https://example.com/package.zip",
				PackageHash: "hash123",
			},
			checkFunc: func(t *testing.T, d *Deployment) {
				assert.Equal(t, StatusActive, d.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &gorm.DB{}
			err := tt.deployment.BeforeCreate(db)
			require.NoError(t, err)
			tt.checkFunc(t, &tt.deployment)
		})
	}
}

func TestDeployment_BeforeUpdate(t *testing.T) {
	deployment := Deployment{
		ID:          uuid.New(),
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		CreatedAt:   time.Now().Add(-1 * time.Hour),
		UpdatedAt:   time.Now().Add(-1 * time.Hour),
	}

	originalUpdatedAt := deployment.UpdatedAt
	time.Sleep(1 * time.Millisecond) // Ensure time difference

	db := &gorm.DB{}
	err := deployment.BeforeUpdate(db)
	require.NoError(t, err)

	assert.True(t, deployment.UpdatedAt.After(originalUpdatedAt))
}

func TestDeployment_TableName(t *testing.T) {
	deployment := Deployment{}
	assert.Equal(t, "deployments", deployment.TableName())
}

func TestDeploymentRun_BeforeCreate(t *testing.T) {
	tests := []struct {
		name      string
		run       DeploymentRun
		checkFunc func(t *testing.T, dr *DeploymentRun)
	}{
		{
			name: "generates UUID when nil",
			run: DeploymentRun{
				DeploymentID: uuid.New(),
			},
			checkFunc: func(t *testing.T, dr *DeploymentRun) {
				assert.NotEqual(t, uuid.Nil, dr.ID)
				assert.NotZero(t, dr.CreatedAt)
				assert.NotZero(t, dr.UpdatedAt)
				assert.NotZero(t, dr.StartedAt)
				assert.Equal(t, RunStatusPending, dr.Status)
				assert.NotNil(t, dr.Output)
			},
		},
		{
			name: "preserves existing UUID",
			run: DeploymentRun{
				ID:           uuid.New(),
				DeploymentID: uuid.New(),
			},
			checkFunc: func(t *testing.T, dr *DeploymentRun) {
				assert.NotEqual(t, uuid.Nil, dr.ID)
			},
		},
		{
			name: "sets default output when nil",
			run: DeploymentRun{
				DeploymentID: uuid.New(),
			},
			checkFunc: func(t *testing.T, dr *DeploymentRun) {
				assert.Equal(t, datatypes.JSON("{}"), dr.Output)
			},
		},
		{
			name: "sets default status when empty",
			run: DeploymentRun{
				DeploymentID: uuid.New(),
			},
			checkFunc: func(t *testing.T, dr *DeploymentRun) {
				assert.Equal(t, RunStatusPending, dr.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &gorm.DB{}
			err := tt.run.BeforeCreate(db)
			require.NoError(t, err)
			tt.checkFunc(t, &tt.run)
		})
	}
}

func TestDeploymentRun_BeforeUpdate(t *testing.T) {
	run := DeploymentRun{
		ID:           uuid.New(),
		DeploymentID: uuid.New(),
		CreatedAt:    time.Now().Add(-1 * time.Hour),
		UpdatedAt:    time.Now().Add(-1 * time.Hour),
	}

	originalUpdatedAt := run.UpdatedAt
	time.Sleep(1 * time.Millisecond) // Ensure time difference

	db := &gorm.DB{}
	err := run.BeforeUpdate(db)
	require.NoError(t, err)

	assert.True(t, run.UpdatedAt.After(originalUpdatedAt))
}

func TestDeploymentRun_AfterFind(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-5 * time.Minute)
	completedAt := now

	tests := []struct {
		name     string
		run      DeploymentRun
		expected *int64
	}{
		{
			name: "calculates duration when completed",
			run: DeploymentRun{
				StartedAt:   startedAt,
				CompletedAt: &completedAt,
			},
			expected: func() *int64 {
				duration := int64(completedAt.Sub(startedAt).Seconds())
				return &duration
			}(),
		},
		{
			name: "no duration when not completed",
			run: DeploymentRun{
				StartedAt:   startedAt,
				CompletedAt: nil,
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &gorm.DB{}
			err := tt.run.AfterFind(db)
			require.NoError(t, err)

			if tt.expected == nil {
				assert.Nil(t, tt.run.Duration)
			} else {
				require.NotNil(t, tt.run.Duration)
				assert.Equal(t, *tt.expected, *tt.run.Duration)
			}
		})
	}
}

func TestDeploymentRun_TableName(t *testing.T) {
	run := DeploymentRun{}
	assert.Equal(t, "deployment_runs", run.TableName())
}

func TestDeploymentRun_IsTerminal(t *testing.T) {
	tests := []struct {
		status   RunStatus
		expected bool
	}{
		{RunStatusPending, false},
		{RunStatusRunning, false},
		{RunStatusCompleted, true},
		{RunStatusFailed, true},
		{RunStatusCancelled, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			run := DeploymentRun{Status: tt.status}
			assert.Equal(t, tt.expected, run.IsTerminal())
		})
	}
}

func TestDeploymentRun_IsRunning(t *testing.T) {
	tests := []struct {
		status   RunStatus
		expected bool
	}{
		{RunStatusPending, false},
		{RunStatusRunning, true},
		{RunStatusCompleted, false},
		{RunStatusFailed, false},
		{RunStatusCancelled, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			run := DeploymentRun{Status: tt.status}
			assert.Equal(t, tt.expected, run.IsRunning())
		})
	}
}

func TestDeploymentConfig_Serialization(t *testing.T) {
	config := DeploymentConfig{
		Concurrency:    2,
		MaxRetries:     3,
		TimeoutSeconds: 300,
		Environment: map[string]string{
			"NODE_ENV": "production",
			"DEBUG":    "false",
		},
		Schedule: "0 */6 * * *",
		ResourceLimits: map[string]interface{}{
			"cpu":    2.0,
			"memory": "2GB",
		},
		TriggerEvents: []string{"webhook", "schedule"},
		BrowserRequests: []BrowserRequest{
			{
				Browser:         "chrome",
				Version:         "latest",
				Headless:        true,
				OperatingSystem: "linux",
				Screen: map[string]interface{}{
					"width":  1920,
					"height": 1080,
				},
				Environment: map[string]string{
					"DISPLAY": ":99",
				},
			},
		},
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(config)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON deserialization
	var deserializedConfig DeploymentConfig
	err = json.Unmarshal(jsonData, &deserializedConfig)
	require.NoError(t, err)

	assert.Equal(t, config.Concurrency, deserializedConfig.Concurrency)
	assert.Equal(t, config.MaxRetries, deserializedConfig.MaxRetries)
	assert.Equal(t, config.TimeoutSeconds, deserializedConfig.TimeoutSeconds)
	assert.Equal(t, config.Environment, deserializedConfig.Environment)
	assert.Equal(t, config.Schedule, deserializedConfig.Schedule)
	assert.Equal(t, config.TriggerEvents, deserializedConfig.TriggerEvents)
	assert.Len(t, deserializedConfig.BrowserRequests, 1)
	assert.Equal(t, config.BrowserRequests[0].Browser, deserializedConfig.BrowserRequests[0].Browser)
}

func TestBrowserRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request BrowserRequest
		valid   bool
	}{
		{
			name: "valid browser request",
			request: BrowserRequest{
				Browser:         "chrome",
				Version:         "latest",
				Headless:        true,
				OperatingSystem: "linux",
				Screen: map[string]interface{}{
					"width":  1920,
					"height": 1080,
				},
			},
			valid: true,
		},
		{
			name: "browser request with profile",
			request: BrowserRequest{
				Browser:         "firefox",
				Version:         "stable",
				Headless:        false,
				OperatingSystem: "windows",
				ProfileID:       func() *uuid.UUID { id := uuid.New(); return &id }(),
			},
			valid: true,
		},
		{
			name: "minimal browser request",
			request: BrowserRequest{
				Browser: "chrome",
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation - ensure required fields are present if valid
			if tt.valid {
				assert.NotEmpty(t, tt.request.Browser)
			}

			// Test JSON serialization
			jsonData, err := json.Marshal(tt.request)
			require.NoError(t, err)
			assert.NotEmpty(t, jsonData)

			// Test JSON deserialization
			var deserializedRequest BrowserRequest
			err = json.Unmarshal(jsonData, &deserializedRequest)
			require.NoError(t, err)
			assert.Equal(t, tt.request.Browser, deserializedRequest.Browser)
		})
	}
}

func TestRuntimeValidation(t *testing.T) {
	tests := []struct {
		runtime Runtime
		valid   bool
	}{
		{RuntimeNode, true},
		{RuntimePython, true},
		{Runtime("java"), false}, // Not supported yet
		{Runtime(""), false},
	}

	supportedRuntimes := []Runtime{RuntimeNode, RuntimePython}

	for _, tt := range tests {
		t.Run(string(tt.runtime), func(t *testing.T) {
			found := false
			for _, supported := range supportedRuntimes {
				if tt.runtime == supported {
					found = true
					break
				}
			}
			assert.Equal(t, tt.valid, found)
		})
	}
}

func TestDeploymentStatusValidation(t *testing.T) {
	tests := []struct {
		status DeploymentStatus
		valid  bool
	}{
		{StatusActive, true},
		{StatusInactive, true},
		{StatusDeploying, true},
		{StatusFailed, true},
		{StatusDeprecated, true},
		{DeploymentStatus("invalid"), false},
	}

	validStatuses := []DeploymentStatus{
		StatusActive, StatusInactive, StatusDeploying, StatusFailed, StatusDeprecated,
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			found := false
			for _, valid := range validStatuses {
				if tt.status == valid {
					found = true
					break
				}
			}
			assert.Equal(t, tt.valid, found)
		})
	}
}

func TestRunStatusValidation(t *testing.T) {
	tests := []struct {
		status RunStatus
		valid  bool
	}{
		{RunStatusPending, true},
		{RunStatusRunning, true},
		{RunStatusCompleted, true},
		{RunStatusFailed, true},
		{RunStatusCancelled, true},
		{RunStatus("invalid"), false},
	}

	validStatuses := []RunStatus{
		RunStatusPending, RunStatusRunning, RunStatusCompleted, RunStatusFailed, RunStatusCancelled,
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			found := false
			for _, valid := range validStatuses {
				if tt.status == valid {
					found = true
					break
				}
			}
			assert.Equal(t, tt.valid, found)
		})
	}
}

func TestDeploymentComputedFields(t *testing.T) {
	deployment := Deployment{
		ID:             uuid.New(),
		Name:           "test-deployment",
		Version:        "1.0.0",
		Runtime:        RuntimeNode,
		PackageURL:     "https://example.com/package.zip",
		PackageHash:    "hash123",
		Status:         StatusActive,
		TotalRuns:      10,
		SuccessfulRuns: 8,
		FailedRuns:     2,
		LastRunAt:      func() *time.Time { t := time.Now(); return &t }(),
	}

	assert.Equal(t, 10, deployment.TotalRuns)
	assert.Equal(t, 8, deployment.SuccessfulRuns)
	assert.Equal(t, 2, deployment.FailedRuns)
	assert.NotNil(t, deployment.LastRunAt)
}

func TestDeploymentRun_DurationCalculation(t *testing.T) {
	startedAt := time.Now()
	completedAt := startedAt.Add(5 * time.Minute)

	run := DeploymentRun{
		StartedAt:   startedAt,
		CompletedAt: &completedAt,
	}

	// Simulate AfterFind hook
	err := run.AfterFind(&gorm.DB{})
	require.NoError(t, err)

	expectedDuration := int64(completedAt.Sub(startedAt).Seconds())
	require.NotNil(t, run.Duration)
	assert.Equal(t, expectedDuration, *run.Duration)
}
