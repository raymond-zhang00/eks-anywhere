package e2e

import (
	"os"
	"regexp"

	e2etests "github.com/aws/eks-anywhere/test/framework"
)

const (
	nutanixFeatureGateEnvVar = "NUTANIX_PROVIDER"
	nutanixRegex             = `^.*Nutanix.*$`
)

func (e *E2ESession) setupNutanixEnv(testRegex string) error {
	re := regexp.MustCompile(nutanixRegex)
	if !re.MatchString(testRegex) {
		e.logger.V(2).Info("Not running Nutanix tests, skipping Env variable setup")
		return nil
	}

	requiredEnvVars := e2etests.RequiredNutanixEnvVars()
	for _, eVar := range requiredEnvVars {
		if val, ok := os.LookupEnv(eVar); ok {
			e.testEnvVars[eVar] = val
		}
	}

	// Since Nutanix Provider is feature gated, manually enable the feature gate for all Nutanix tests.
	e.testEnvVars[nutanixFeatureGateEnvVar] = "true"
	return nil
}