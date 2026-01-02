package services

func (sm *ServiceManager) UpdateInstallLog(svcType, version, log string) {
	sm.statusMu.Lock()
	defer sm.statusMu.Unlock()
	key := svcType + "-" + version
	sm.installLogs[key] = log
}

// GetInstallLog returns the last log message for a service installation
func (sm *ServiceManager) GetInstallLog(svcType, version string) string {
	sm.statusMu.RLock()
	defer sm.statusMu.RUnlock()
	key := svcType + "-" + version
	return sm.installLogs[key]
}
