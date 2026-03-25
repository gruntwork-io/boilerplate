package getterhelper

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
)

// CloneSubdir is the subdirectory name used inside the temporary working
// directory when downloading remote templates via go-getter.
const CloneSubdir = "wd"

var forcedRegexp = regexp.MustCompile(`^([A-Za-z0-9]+)::(.+)$`)

// ValidateTemplateURL returns an error if the template URL is not following one of the supported detector patterns.
func ValidateTemplateURL(templateURL string) error {
	_, err := ParseGetterURL(templateURL)
	return err
}

func ParseGetterURL(templateURL string) (*url.URL, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	getterURLWithGetter, err := getter.Detect(templateURL, pwd, getter.Detectors)
	if err != nil {
		return nil, err
	}

	return urlParseGetterURL(getterURLWithGetter)
}

// Parse the given source URL into a URL struct. This method can handle source URLs that include go-getter's "forced
// getter" prefixes, such as git::.
// The following routine was obtained from terragrunt.
func urlParseGetterURL(rawGetterURLStr string) (*url.URL, error) {
	forcedGetter, getterURLStr := getForcedGetter(rawGetterURLStr)

	// Parse the URL without the getter prefix
	canonicalGetterURL, err := urlhelper.Parse(getterURLStr)
	if err != nil {
		return nil, err
	}

	// Reattach the "getter" prefix as part of the scheme
	if forcedGetter != "" {
		canonicalGetterURL.Scheme = fmt.Sprintf("%s::%s", forcedGetter, canonicalGetterURL.Scheme)
	}

	return canonicalGetterURL, nil
}

// Terraform source URLs can contain a "getter" prefix that specifies the type of protocol to use to download that URL,
// such as "git::", which means Git should be used to download the URL. This method returns the getter prefix and the
// rest of the URL. This code is copied from the getForcedGetter method of go-getter/get.go, as that method is not
// exported publicly.
func getForcedGetter(sourceURL string) (string, string) {
	const expectedMatchGroups = 2
	if matches := forcedRegexp.FindStringSubmatch(sourceURL); len(matches) > expectedMatchGroups {
		return matches[1], matches[2]
	}

	return "", sourceURL
}

// DetermineTemplateConfig decides what should be passed to TemplateURL and TemplateFolder. This parses the templateURL
// and determines if it is a local path. If so, use that path directly instead of downloading it to a temp working dir.
// We do this by setting the template folder, which will instruct the process routine to skip downloading the template.
//
// Returns TemplateURL, TemplateFolder, error
func DetermineTemplateConfig(templateURL string) (string, string, error) {
	url, err := ParseGetterURL(templateURL)
	if err != nil {
		return "", "", err
	}

	if url.Scheme == "file" {
		// Intentionally return as both TemplateURL and TemplateFolder so that validation passes, but still skip
		// download.
		return templateURL, templateURL, nil
	}

	return templateURL, "", nil
}

// NewGetterClient creates a new getter client that forces go-getter to copy files instead of creating symlinks.
func NewGetterClient(src string, dst string) (*getter.Client, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
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
func DownloadTemplatesToTemporaryFolder(templateURL string) (string, string, error) {
	workingDir, err := getTempFolder()
	if err != nil {
		return workingDir, workingDir, err
	}

	// Always set a subdir path because go-getter can not clone into an existing dir.
	cloneDir := filepath.Join(workingDir, CloneSubdir)

	slog.Default().Info("Downloading templates to " + workingDir)

	// If there is a subdir component, we download everything and combine the path at the end to return the working path
	mainPath, subDir := getter.SourceDirSubdir(templateURL)
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
