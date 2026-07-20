package app

import "fmt"

type State string

const (
	StateRunning            State = "running"
	StateDegraded           State = "degraded"
	StateConfigurationError State = "configuration_error"
)

type ComponentStatus struct {
	Name    string
	Enabled bool
	Running bool
	Error   error
}

func Aggregate(configurationError error, statuses []ComponentStatus) State {
	if configurationError != nil {
		return StateConfigurationError
	}
	for _, status := range statuses {
		if status.Enabled && (!status.Running || status.Error != nil) {
			return StateDegraded
		}
	}
	return StateRunning
}

func Tooltip(state State) string {
	switch state {
	case StateRunning:
		return "OmniSSHAgent - Running"
	case StateDegraded:
		return "OmniSSHAgent - Degraded"
	case StateConfigurationError:
		return "OmniSSHAgent - Configuration error"
	default:
		return fmt.Sprintf("OmniSSHAgent - %s", state)
	}
}
