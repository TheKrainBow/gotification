package gotification

import "strings"

func mockTargetKey(channel Channel, provider string) string {
	provider = strings.TrimSpace(provider)
	return string(channel) + "|" + provider
}

func (d *Dispatcher) shouldMock(channel Channel, provider string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.mockTargets == nil {
		return false
	}
	v, ok := d.mockTargets[mockTargetKey(channel, provider)]
	return ok && v
}
