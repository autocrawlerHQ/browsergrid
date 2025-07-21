package storage

import (
	"context"
	"io"
	"time"
)

type Storage interface {
	Initialize(ctx context.Context, resource *Resource) error
	Get(ctx context.Context, resourceID string) (*Resource, error)
	List(ctx context.Context, filter *ResourceFilter) ([]*Resource, error)
	Delete(ctx context.Context, resourceID string) error
	OpenReader(ctx context.Context, resourceID string, path string) (io.ReadCloser, error)
	OpenWriter(ctx context.Context, resourceID string, path string) (io.WriteCloser, error)
	GetMetadata(ctx context.Context, resourceID string) (*Metadata, error)
	UpdateMetadata(ctx context.Context, resourceID string, metadata *Metadata) error
	GetUsage(ctx context.Context, resourceID string) (*Usage, error)
}

type Resource struct {
	ID          string            `json:"id"`
	Type        ResourceType      `json:"type"`
	OwnerID     string            `json:"owner_id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
}

type ResourceType string

const (
	ResourceTypeProfile    ResourceType = "profile"
	ResourceTypeDeployment ResourceType = "deployment"
	ResourceTypeRecording  ResourceType = "recording"
	ResourceTypeArtifact   ResourceType = "artifact"
)

type ResourceFilter struct {
	Type    *ResourceType     `json:"type,omitempty"`
	OwnerID *string           `json:"owner_id,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
	Limit   int               `json:"limit,omitempty"`
	Offset  int               `json:"offset,omitempty"`
}

type Metadata struct {
	ContentType string            `json:"content_type,omitempty"`
	Size        int64             `json:"size,omitempty"`
	Checksum    string            `json:"checksum,omitempty"`
	Custom      map[string]string `json:"custom,omitempty"`
}

type Usage struct {
	BytesUsed  int64 `json:"bytes_used"`
	FilesCount int64 `json:"files_count"`
}

type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
	ReadOnly  bool   `json:"read_only"`
	SubPath   string `json:"sub_path,omitempty"`
}

type StreamOptions struct {
	BufferSize int   `json:"buffer_size,omitempty"`
	ChunkSize  int   `json:"chunk_size,omitempty"`
	Resume     bool  `json:"resume,omitempty"`
	Offset     int64 `json:"offset,omitempty"`
}
