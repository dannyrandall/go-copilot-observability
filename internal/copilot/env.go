package copilot

import (
	"fmt"
	"os"
)

// ServiceName uses copilot injected environment variables to determine the name of the service.
// If any environment variables are missing, the name defaults to fallback.
func ServiceName(fallback string) string {
	app, ok := os.LookupEnv("COPILOT_APPLICATION_NAME")
	if !ok {
		return fallback
	}

	env, ok := os.LookupEnv("COPILOT_ENVIRONMENT_NAME")
	if !ok {
		return fallback
	}

	svc, ok := os.LookupEnv("COPILOT_SERVICE_NAME")
	if !ok {
		return fallback
	}

	return fmt.Sprintf("%s-%s-%s", app, env, svc)
}
