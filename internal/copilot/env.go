package copilot

import (
	"fmt"
	"os"
)

const defaultServiceName = "movies"

func ServiceName() string {
	app, ok := os.LookupEnv("COPILOT_APPLICATION_NAME")
	if !ok {
		return defaultServiceName
	}

	env, ok := os.LookupEnv("COPILOT_ENVIRONMENT_NAME")
	if !ok {
		return defaultServiceName
	}

	svc, ok := os.LookupEnv("COPILOT_SERVICE_NAME")
	if !ok {
		return defaultServiceName
	}

	return fmt.Sprintf("%s-%s-%s", app, env, svc)
}
