package integrationtests_test

import (
	"errors"
	"os"
	"testing"

	"github.com/gruntwork-io/boilerplate/config"
	"github.com/stretchr/testify/require"
)

const (
	testVersion = "v1.33.7"
)

// TestVersionProvider implements VersionProvider for testing
type TestVersionProvider struct {
	version string
}

func (p TestVersionProvider) GetVersion() string {
	return p.version
}

func TestRequiredVersionMatchCase(t *testing.T) {
	t.Parallel()

	// Test the version enforcement logic directly using dependency injection
	boilerplateConfig, err := loadTestConfig("../test-fixtures/regression-test/required-version/match/boilerplate.yml")
	require.NoError(t, err)

	versionProvider := TestVersionProvider{version: testVersion}

	require.NoError(t, config.EnforceRequiredVersionWithProvider(boilerplateConfig, versionProvider))
}

func TestRequiredVersionOverTest(t *testing.T) {
	t.Parallel()

	// Test the version enforcement logic directly using dependency injection
	boilerplateConfig, err := loadTestConfig("../test-fixtures/regression-test/required-version/over-test/boilerplate.yml")
	require.NoError(t, err)

	versionProvider := TestVersionProvider{version: testVersion}

	err = config.EnforceRequiredVersionWithProvider(boilerplateConfig, versionProvider)
	require.Error(t, err)

	var invalidBoilerplateVersion config.InvalidBoilerplateVersion

	isInvalidVersionErr := errors.As(err, &invalidBoilerplateVersion)
	require.True(t, isInvalidVersionErr)
}

func TestRequiredVersionUnderTest(t *testing.T) {
	t.Parallel()

	// Test the version enforcement logic directly using dependency injection
	boilerplateConfig, err := loadTestConfig("../test-fixtures/regression-test/required-version/under-test/boilerplate.yml")
	require.NoError(t, err)

	versionProvider := TestVersionProvider{version: testVersion}

	require.NoError(t, config.EnforceRequiredVersionWithProvider(boilerplateConfig, versionProvider))
}

// loadTestConfig loads a boilerplate config file for testing purposes
func loadTestConfig(configPath string) (*config.BoilerplateConfig, error) {
	bytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	return config.ParseBoilerplateConfig(bytes)
}
