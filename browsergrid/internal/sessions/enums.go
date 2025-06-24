package sessions

// OperatingSystem represents supported operating systems
// @Description Supported operating systems for browser sessions
type OperatingSystem string //@name OperatingSystem

const (
	OSWindows OperatingSystem = "windows"
	OSMacOS   OperatingSystem = "macos"
	OSLinux   OperatingSystem = "linux"
)

// Browser represents supported browser types
// @Description Supported browser types
type Browser string //@name Browser

const (
	BrowserChrome   Browser = "chrome"
	BrowserChromium Browser = "chromium"
	BrowserFirefox  Browser = "firefox"
	BrowserEdge     Browser = "edge"
	BrowserOpera    Browser = "webkit"
	BrowserSafari   Browser = "safari"
)

// BrowserVersion represents browser version types
// @Description Browser version types (latest, stable, canary, dev)
type BrowserVersion string //@name BrowserVersion

const (
	VerLatest BrowserVersion = "latest"
	VerStable BrowserVersion = "stable"
	VerCanary BrowserVersion = "canary"
	VerDev    BrowserVersion = "dev"
)

// SessionStatus represents the current status of a browser session
// @Description Current status of a browser session
type SessionStatus string //@name SessionStatus

const (
	StatusPending    SessionStatus = "pending"
	StatusStarting   SessionStatus = "starting"
	StatusAvailable  SessionStatus = "available"
	StatusClaimed    SessionStatus = "claimed"
	StatusRunning    SessionStatus = "running"
	StatusIdle       SessionStatus = "idle"
	StatusCompleted  SessionStatus = "completed"
	StatusFailed     SessionStatus = "failed"
	StatusExpired    SessionStatus = "expired"
	StatusCrashed    SessionStatus = "crashed"
	StatusTimedOut   SessionStatus = "timed_out"
	StatusTerminated SessionStatus = "terminated"
)

// SessionEventType represents different types of session events
// @Description Types of events that can occur during a session
type SessionEventType string //@name SessionEventType

const (
	EvtSessionCreated    SessionEventType = "session_created"
	EvtResourceAllocated SessionEventType = "resource_allocated"

	EvtSessionStarting  SessionEventType = "session_starting"
	EvtContainerStarted SessionEventType = "container_started"
	EvtBrowserStarted   SessionEventType = "browser_started"
	EvtSessionAvailable SessionEventType = "session_available"

	EvtSessionClaimed  SessionEventType = "session_claimed"
	EvtSessionAssigned SessionEventType = "session_assigned"
	EvtSessionReady    SessionEventType = "session_ready"

	EvtSessionActive SessionEventType = "session_active"
	EvtSessionIdle   SessionEventType = "session_idle"
	EvtHeartbeat     SessionEventType = "heartbeat"

	EvtPoolAdded   SessionEventType = "pool_added"
	EvtPoolRemoved SessionEventType = "pool_removed"
	EvtPoolDrained SessionEventType = "pool_drained"

	EvtSessionCompleted  SessionEventType = "session_completed"
	EvtSessionExpired    SessionEventType = "session_expired"
	EvtSessionTimedOut   SessionEventType = "session_timed_out"
	EvtSessionTerminated SessionEventType = "session_terminated"

	EvtStartupFailed     SessionEventType = "startup_failed"
	EvtBrowserCrashed    SessionEventType = "browser_crashed"
	EvtContainerCrashed  SessionEventType = "container_crashed"
	EvtResourceExhausted SessionEventType = "resource_exhausted"
	EvtNetworkError      SessionEventType = "network_error"

	EvtStatusChanged SessionEventType = "status_changed"
	EvtConfigUpdated SessionEventType = "config_updated"
	EvtHealthCheck   SessionEventType = "health_check"
)
