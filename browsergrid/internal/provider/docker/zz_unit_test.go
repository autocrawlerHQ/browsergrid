package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNatSet(t *testing.T) {
	tests := []struct {
		name     string
		ports    []string
		expected int
	}{
		{
			name:     "empty ports",
			ports:    []string{},
			expected: 0,
		},
		{
			name:     "single port",
			ports:    []string{"8080/tcp"},
			expected: 1,
		},
		{
			name:     "multiple ports",
			ports:    []string{"8080/tcp", "9222/tcp", "3000/udp"},
			expected: 3,
		},
		{
			name:     "duplicate ports",
			ports:    []string{"8080/tcp", "8080/tcp"},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := natSet(tt.ports...)
			assert.Len(t, result, tt.expected)

			for _, port := range tt.ports {
				_, exists := result[nat.Port(port)]
				assert.True(t, exists, "port %s should exist in set", port)
			}
		})
	}
}

func TestNatMap(t *testing.T) {
	tests := []struct {
		name          string
		containerPort int
		hostPort      int
		expectedKey   string
		expectedHost  string
	}{
		{
			name:          "specific host port",
			containerPort: 8080,
			hostPort:      9000,
			expectedKey:   "8080/tcp",
			expectedHost:  "9000",
		},
		{
			name:          "random host port",
			containerPort: 3000,
			hostPort:      0,
			expectedKey:   "3000/tcp",
			expectedHost:  "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := natMap(tt.containerPort, tt.hostPort)

			expectedPort := nat.Port(tt.expectedKey)
			bindings, exists := result[expectedPort]
			require.True(t, exists, "port binding should exist")
			require.Len(t, bindings, 1, "should have exactly one binding")

			binding := bindings[0]
			assert.Equal(t, "0.0.0.0", binding.HostIP)
			assert.Equal(t, tt.expectedHost, binding.HostPort)
		})
	}
}

func TestStrPtr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "non-empty string",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "special characters",
			input:    "test@#$%^&*()",
			expected: "test@#$%^&*()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strPtr(tt.input)
			require.NotNil(t, result)
			assert.Equal(t, tt.expected, *result)
		})
	}
}

func TestInt64Ptr(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected int64
	}{
		{
			name:     "zero",
			input:    0,
			expected: 0,
		},
		{
			name:     "positive number",
			input:    12345,
			expected: 12345,
		},
		{
			name:     "negative number",
			input:    -9876,
			expected: -9876,
		},
		{
			name:     "max int64",
			input:    9223372036854775807,
			expected: 9223372036854775807,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := int64Ptr(tt.input)
			require.NotNil(t, result)
			assert.Equal(t, tt.expected, *result)
		})
	}
}

func TestDefaultStr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fallback string
		expected string
	}{
		{
			name:     "empty string uses default",
			input:    "",
			fallback: "default",
			expected: "default",
		},
		{
			name:     "non-empty string uses input",
			input:    "custom",
			fallback: "default",
			expected: "custom",
		},
		{
			name:     "whitespace is not empty",
			input:    " ",
			fallback: "default",
			expected: " ",
		},
		{
			name:     "both empty",
			input:    "",
			fallback: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := defaultStr(tt.input, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCpuPercentUnix(t *testing.T) {
	tests := []struct {
		name     string
		stats    container.StatsResponse
		expected float64
	}{
		{
			name: "normal cpu usage",
			stats: container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage:  2000000000,
						PercpuUsage: []uint64{500000000, 500000000, 500000000, 500000000},
					},
					SystemUsage: 20000000000,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 1000000000,
					},
					SystemUsage: 10000000000,
				},
			},
			expected: 40.0,
		},
		{
			name: "zero cpu usage",
			stats: container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage:  1000000000,
						PercpuUsage: []uint64{250000000, 250000000, 250000000, 250000000},
					},
					SystemUsage: 10000000000,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 1000000000,
					},
					SystemUsage: 10000000000,
				},
			},
			expected: 0.0,
		},
		{
			name: "system delta is zero",
			stats: container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage:  2000000000,
						PercpuUsage: []uint64{500000000, 500000000},
					},
					SystemUsage: 10000000000,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 1000000000,
					},
					SystemUsage: 10000000000,
				},
			},
			expected: 0.0,
		},
		{
			name: "cpu delta is zero",
			stats: container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage:  1000000000,
						PercpuUsage: []uint64{500000000, 500000000},
					},
					SystemUsage: 20000000000,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 1000000000,
					},
					SystemUsage: 10000000000,
				},
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cpuPercentUnix(tt.stats)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWsToHTTP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "websocket URL",
			input:    "ws://localhost:8080",
			expected: "localhost:8080",
		},
		{
			name:     "secure websocket URL",
			input:    "wss://example.com:443/path",
			expected: "wss://example.com:443/path",
		},
		{
			name:     "no ws prefix",
			input:    "http://localhost:8080",
			expected: "http://localhost:8080",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "just ws://",
			input:    "ws://",
			expected: "",
		},
		{
			name:     "ws in middle of string",
			input:    "some-ws://text",
			expected: "some-ws://text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wsToHTTP(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetType(t *testing.T) {
	provisioner := &DockerProvisioner{}
	assert.Equal(t, "docker", string(provisioner.GetType()))
}

func BenchmarkNatSet(b *testing.B) {
	ports := []string{"8080/tcp", "9222/tcp", "3000/tcp", "5432/tcp"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = natSet(ports...)
	}
}

func BenchmarkNatMap(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = natMap(8080, 0)
	}
}

func BenchmarkCpuPercentUnix(b *testing.B) {
	stats := container.StatsResponse{
		CPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage:  2000000000,
				PercpuUsage: []uint64{500000000, 500000000, 500000000, 500000000},
			},
			SystemUsage: 20000000000,
		},
		PreCPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage: 1000000000,
			},
			SystemUsage: 10000000000,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cpuPercentUnix(stats)
	}
}
