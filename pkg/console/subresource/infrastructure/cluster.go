package infrastructure

import config "github.com/openshift/api/config/v1"

func GetAPIServerURL(infrastructure *config.Infrastructure) string {
	if infrastructure != nil {
		return infrastructure.Status.APIServerURL
	}
	return ""
}
