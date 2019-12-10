package metrics

import "k8s.io/klog"

func HandleConsoleURL(oldURL, newURL string) {
	// if neither have been set, there is nothing to update
	if noHost(oldURL, newURL) {
		klog.V(4).Infof("metric console_url has no host")
		return
	}

	// only a new URL
	if isNewHost(oldURL, newURL) {
		klog.V(4).Infof("metric console_url new host: %s %s", oldURL, newURL)
		singleton.ConsoleURL.WithLabelValues(newURL).Set(1)
		return
	}

	// route or ingress update
	if isHostTransition(oldURL, newURL) {
		klog.V(4).Infof("metric console_url host transition: %s to %s", oldURL, newURL)
		singleton.ConsoleURL.WithLabelValues(oldURL).Set(0)
		singleton.ConsoleURL.WithLabelValues(newURL).Set(1)
		return
	}

	// something went wrong and we no longer have a route or ingress with a host
	if hostDied(oldURL, newURL) {
		klog.V(4).Infof("metric console_url host lost: %s %s", oldURL, newURL)
		singleton.ConsoleURL.WithLabelValues(oldURL).Set(0)
		return
	}
	klog.Error("metric console_url unhandled")
}

func noHost(old, new string) bool {
	return len(old) == 0 && len(new) == 0
}
func isNewHost(old, new string) bool {
	return len(old) == 0 && len(new) != 0
}
func isHostTransition(old, new string) bool {
	return len(old) != 0 && len(new) != 0
}
func hostDied(old, new string) bool {
	return len(old) != 0 && len(new) == 0
}
