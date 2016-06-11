package integration_tests

import (
	"testing"
	"io/ioutil"
	"github.com/stretchr/testify/assert"
	"github.com/gruntwork-io/boilerplate/cli"
	"fmt"
	"github.com/gruntwork-io/boilerplate/config"
	"os"
	"path"
)

func TestExamples(t *testing.T) {
	t.Parallel()

	examplesBasePath := "../examples"
	examplesExpectedOutputBasePath := "../test-fixtures/examples-expecrted-output"

	outputBasePath, err := ioutil.TempDir("", "boilerplate-test-output")
	assert.Nil(t, err)

	files, err := ioutil.ReadDir(examplesBasePath)
	assert.Nil(t, err)

	app := cli.CreateBoilerplateCli("test")

	for _, file := range files {
		if file.IsDir() {
			templateFolder := path.Join(examplesBasePath, file.Name())
			outputFolder := path.Join(outputBasePath, file.Name())

			err := app.Run(fmt.Sprintf("--%s %s --%s %s --%s", config.OPT_TEMPLATE_FOLDER, templateFolder, config.OPT_OUTPUT_FOLDER, outputFolder, config.OPT_NON_INTERACTIVE))
			assert.Nil(t, err)
		}
	}
}
