package core

const Version = "1.3.3"

const (
	SeverityInfo      = "INFO"
	SeverityWarn      = "WARNING"
	SeverityCritical  = "CRITICAL"
	SeverityRecovered = "RECOVERED"
)

const (
	SourceSSH         = "ssh"
	SourcSudo         = "sudo"
	SourceDocker      = "docker"
	SourceSystemd     = "systemd"
	SourceNetwork     = "network"
	SourceShutdown    = "shutdown"
	SourceCPU         = "cpu"
	SourceRAM         = "ram"
	SourceDisk        = "disk"
	SourceTemperature = "temperature"
	SourceDaemon      = "daemon"
)

const (
	EventSSHLogin     = "SSH_LOGIN"
	EventSSHLogout    = "SSH_LOGOUT"
	EventSSHBruteforce = "SSH_BRUTEFORCE"
	EventSudoUsed     = "SUDO_USED"

	EventContainerStarted   = "CONTAINER_STARTED"
	EventContainerStopped   = "CONTAINER_STOPPED"
	EventContainerDied      = "CONTAINER_DIED"
	EventContainerRestarted = "CONTAINER_RESTARTED"

	EventServiceStarted   = "SERVICE_STARTED"
	EventServiceStopped   = "SERVICE_STOPPED"
	EventServiceFailed    = "SERVICE_FAILED"
	EventServiceRestarted = "SERVICE_RESTARTED"

	EventInterfaceUp   = "INTERFACE_UP"
	EventInterfaceDown = "INTERFACE_DOWN"

	EventDaemonStart     = "DAEMON_START"
	EventShutdownStarted = "SHUTDOWN_STARTED"
	EventRebootStarted   = "REBOOT_STARTED"

	EventTempWarn     = "TEMP_WARN"
	EventTempCritical = "TEMP_CRITICAL"
	EventTempRecovered = "TEMP_RECOVERED"

	EventCPUHigh     = "CPU_HIGH"
	EventCPURecovered = "CPU_RECOVERED"

	EventRAMHigh     = "RAM_HIGH"
	EventRAMRecovered = "RAM_RECOVERED"

	EventDiskWarn     = "DISK_WARN"
	EventDiskCritical = "DISK_CRITICAL"
	EventDiskRecovered = "DISK_RECOVERED"
)

const (
	DefaultCPUInterval       = 15
	DefaultRAMInterval       = 30
	DefaultTempInterval      = 30
	DefaultDiskInterval      = 300

	DefaultCPUWarn     = 80.0
	DefaultCPUCritical = 95.0
	DefaultRAMWarn     = 85.0
	DefaultRAMCritical = 95.0
	DefaultDiskWarn    = 85.0
	DefaultDiskCritical = 95.0
	DefaultTempWarn    = 75.0
	DefaultTempCritical = 85.0
)

const (
	CooldownSSHLogin     = 0
	CooldownSSHLogout    = 0
	CooldownSSHBruteforce = 300
	CooldownSudoUsed     = 0
	CooldownCPUHigh      = 300
	CooldownCPURecovered = 300
	CooldownTempWarn     = 300
	CooldownTempCritical = 300
	CooldownDiskWarn     = 3600
	CooldownDiskCritical = 3600
	CooldownRAMHigh      = 300
)

const DispatcherQueueTimeout = 1.0
