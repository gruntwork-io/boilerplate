package getter_helper

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	getter "github.com/hashicorp/go-getter"
	urlhelper "github.com/hashicorp/go-getter/helper/url"

	"github.com/gruntwork-io/boilerplate/errors"
)

var forcedRegexp = regexp.MustCompile(`^([A-Za-z0-9]+)::(.+)$`)

// ValidateTemplateUrl returns an error if the template URL is not following one of the supported detector patterns.
func ValidateTemplateUrl(templateUrl string) error {
	_, err := ParseGetterUrl(templateUrl)
	return err
}

func ParseGetterUrl(templateUrl string) (*url.URL, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}
	getterURLWithGetter, err := getter.Detect(templateUrl, pwd, getter.Detectors)
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}
	return urlParseGetterUrl(getterURLWithGetter)
}

// Parse the given source URL into a URL struct. This method can handle source URLs that include go-getter's "forced
// getter" prefixes, such as git::.
// The following routine was obtained from terragrunt.
func urlParseGetterUrl(rawGetterUrlStr string) (*url.URL, error) {
	forcedGetter, getterUrlStr := getForcedGetter(rawGetterUrlStr)

	// Parse the URL without the getter prefix
	canonicalGetterUrl, err := urlhelper.Parse(getterUrlStr)
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	// Reattach the "getter" prefix as part of the scheme
	if forcedGetter != "" {
		canonicalGetterUrl.Scheme = fmt.Sprintf("%s::%s", forcedGetter, canonicalGetterUrl.Scheme)
	}

	return canonicalGetterUrl, nil
}

// Terraform source URLs can contain a "getter" prefix that specifies the type of protocol to use to download that URL,
// such as "git::", which means Git should be used to download the URL. This method returns the getter prefix and the
// rest of the URL. This code is copied from the getForcedGetter method of go-getter/get.go, as that method is not
// exported publicly.
func getForcedGetter(sourceUrl string) (string, string) {
	if matches := forcedRegexp.FindStringSubmatch(sourceUrl); matches != nil && len(matches) > 2 {
		return matches[1], matches[2]
	}

	return "", sourceUrl
}

// We use this code to force go-getter to copy files instead of creating symlinks.
func NewGetterClient(src string, dst string) (*getter.Client, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	client := &getter.Client{
		Ctx:  context.Background(),
		Src:  src,
		Dst:  dst,
		Pwd:  pwd,
		Mode: getter.ClientModeAny,
	}

	// We copy all the default getters from the go-getter library, but replace the "file" getter. We shallow clone the
	// getter map here rather than using getter.Getters directly because we shouldn't change the original,
	// globally-shared getter.Getters map.
	client.Getters = map[string]getter.Getter{}
	for getterName, getterValue := range getter.Getters {
		if getterName == "file" {
			client.Getters[getterName] = &FileCopyGetter{}
		} else {
			client.Getters[getterName] = getterValue
		}
	}

	return client, nil
}

// DownloadTemplatesToTemporaryFolder uses the go-getter library to fetch the templates from the configured URL to a
// temporary folder and returns the path to that folder. If there is a subdir in the template URL, return the combined
// path as well.
func DownloadTemplatesToTemporaryFolder(templateUrl string, logger *slog.Logger) (string, string, error) {
	workingDir, err := getTempFolder()
	if err != nil {
		return workingDir, workingDir, errors.WithStackTrace(err)
	}

	// Always set a subdir path because go-getter can not clone into an existing dir.
	cloneDir := filepath.Join(workingDir, "wd")

	logger.Info(fmt.Sprintf("Downloading templates from %s to %s", templateUrl, workingDir))

	// If there is a subdir component, we download everything and combine the path at the end to return the working path
	mainPath, subDir := getter.SourceDirSubdir(templateUrl)
	outDir := filepath.Clean(filepath.Join(cloneDir, subDir))

	client, err := NewGetterClient(mainPath, cloneDir)
	if err != nil {
		return workingDir, outDir, err
	}

	if err := client.Get(); err != nil {
		return workingDir, outDir, err
	}
	return workingDir, outDir, nil
}
