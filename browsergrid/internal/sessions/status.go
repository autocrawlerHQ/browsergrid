package sessions

func statusFromEvent(e SessionEventType) (SessionStatus, bool) {
	switch e {
	case EvtSessionCreated:
		return StatusPending, true
	case EvtResourceAllocated:
		return StatusPending, true
	case EvtSessionStarting, EvtContainerStarted:
		return StatusStarting, true
	case EvtBrowserStarted:
		return StatusStarting, true
	case EvtSessionAvailable, EvtPoolAdded:
		return StatusAvailable, true
	case EvtSessionClaimed:
		return StatusClaimed, true
	case EvtSessionAssigned, EvtSessionReady:
		return StatusRunning, true
	case EvtSessionActive:
		return StatusRunning, true
	case EvtSessionIdle:
		return StatusIdle, true
	case EvtSessionCompleted:
		return StatusCompleted, true
	case EvtSessionExpired:
		return StatusExpired, true
	case EvtSessionTimedOut:
		return StatusTimedOut, true
	case EvtSessionTerminated:
		return StatusTerminated, true
	case EvtStartupFailed, EvtResourceExhausted, EvtNetworkError:
		return StatusFailed, true
	case EvtBrowserCrashed, EvtContainerCrashed:
		return StatusCrashed, true
	case EvtHeartbeat, EvtStatusChanged, EvtConfigUpdated, EvtHealthCheck,
		EvtPoolRemoved, EvtPoolDrained:
		return "", false
	default:
		return "", false
	}
}

func shouldUpdateStatus(cur, next SessionStatus) bool {
	order := map[SessionStatus]int{
		StatusPending:    0,
		StatusStarting:   1,
		StatusAvailable:  2,
		StatusClaimed:    3,
		StatusRunning:    4,
		StatusIdle:       4,
		StatusCompleted:  5,
		StatusFailed:     5,
		StatusExpired:    5,
		StatusCrashed:    5,
		StatusTimedOut:   5,
		StatusTerminated: 5,
	}

	curOrder := order[cur]
	nextOrder := order[next]

	if (cur == StatusRunning && next == StatusIdle) ||
		(cur == StatusIdle && next == StatusRunning) {
		return true
	}

	if cur == StatusAvailable && next == StatusClaimed {
		return true
	}

	if cur == StatusClaimed && next == StatusRunning {
		return true
	}

	if cur == StatusAvailable && (next == StatusTerminated || next == StatusExpired) {
		return true
	}

	return nextOrder > curOrder
}

func IsTerminalStatus(status SessionStatus) bool {
	switch status {
	case StatusCompleted, StatusFailed, StatusExpired,
		StatusCrashed, StatusTimedOut, StatusTerminated:
		return true
	default:
		return false
	}
}

func IsPooledStatus(status SessionStatus) bool {
	return status == StatusAvailable
}

func IsClaimableStatus(status SessionStatus) bool {
	return status == StatusAvailable
}

func CanTransitionTo(current, target SessionStatus) bool {
	if IsTerminalStatus(current) {
		return false
	}

	return shouldUpdateStatus(current, target)
}

func GetValidTransitions(current SessionStatus) []SessionStatus {
	if IsTerminalStatus(current) {
		return []SessionStatus{}
	}

	var valid []SessionStatus
	allStatuses := []SessionStatus{
		StatusPending, StatusStarting, StatusAvailable, StatusClaimed,
		StatusRunning, StatusIdle, StatusCompleted, StatusFailed,
		StatusExpired, StatusCrashed, StatusTimedOut, StatusTerminated,
	}

	for _, status := range allStatuses {
		if CanTransitionTo(current, status) {
			valid = append(valid, status)
		}
	}

	return valid
}

func GetPoolTransitions(current SessionStatus) []SessionStatus {
	switch current {
	case StatusAvailable:
		return []SessionStatus{StatusClaimed, StatusTerminated, StatusExpired}
	case StatusClaimed:
		return []SessionStatus{StatusRunning, StatusTerminated}
	default:
		return []SessionStatus{}
	}
}
