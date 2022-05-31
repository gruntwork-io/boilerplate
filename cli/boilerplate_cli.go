package cli

import (
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/templates"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
	"github.com/gruntwork-io/go-commons/entrypoint"
	"github.com/gruntwork-io/go-commons/version"
	"github.com/pkg/browser"
	"github.com/urfave/cli"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const customHelpText = `Usage: {{.UsageText}}

A tool for generating files and folders (\"boilerplate\") from a set of templates. Examples:

Generate a project in ~/output from the templates in ~/templates:

    boilerplate --template-url ~/templates --output-folder ~/output

Generate a project in ~/output from the templates in ~/templates, using variables passed in via the command line:

    boilerplate --template-url ~/templates --output-folder ~/output --var "Title=Boilerplate" --var "ShowLogo=false"

Generate a project in ~/output from the templates in ~/templates, using variables read from a file:

    boilerplate --template-url ~/templates --output-folder ~/output --var-file vars.yml

Generate a project in ~/output from the templates in this repo's include example dir, using variables read from a file:

	boilerplate --template-url "git@github.com:gruntwork-io/boilerplate.git//examples/for-learning-and-testing/include?ref=master" --output-folder ~/output --var-file vars.yml


Options:

   {{range .VisibleFlags}}{{.}}
   {{end}}`

func CreateBoilerplateCli() *cli.App {
	cli.HelpPrinter = entrypoint.WrappedHelpPrinter
	cli.AppHelpTemplate = customHelpText
	app := cli.NewApp()
	entrypoint.HelpTextLineWidth = 120

	app.Name = "boilerplate"
	app.Author = "Gruntwork <www.gruntwork.io>"
	app.UsageText = "boilerplate [OPTIONS]"
	app.Version = version.GetVersion()
	app.Action = runApp

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  options.OptTemplateUrl,
			Usage: "Generate the project from the templates in `URL`. This can be a local path, or a go-getter compatible URL for remote templates (e.g., `git@github.com:gruntwork-io/boilerplate.git//examples/for-learning-and-testing/include?ref=master`).",
		},
		cli.StringFlag{
			Name:  options.OptOutputFolder,
			Usage: "Create the output files and folders in `FOLDER`.",
		},
		cli.BoolFlag{
			Name:  options.OptNonInteractive,
			Usage: fmt.Sprintf("Do not prompt for input variables. All variables must be set via --%s and --%s options instead.", options.OptVar, options.OptVarFile),
		},
		cli.StringSliceFlag{
			Name:  options.OptVar,
			Usage: "Use `NAME=VALUE` to set variable NAME to VALUE. May be specified more than once.",
		},
		cli.StringSliceFlag{
			Name:  options.OptVarFile,
			Usage: "Load variable values from the YAML file `FILE`. May be specified more than once.",
		},
		cli.StringFlag{
			Name:  options.OptMissingKeyAction,
			Usage: fmt.Sprintf("What `ACTION` to take if a template looks up a variable that is not defined. Must be one of: %s. Default: %s.", options.AllMissingKeyActions, options.DefaultMissingKeyAction),
		},
		cli.StringFlag{
			Name:  options.OptMissingConfigAction,
			Usage: fmt.Sprintf("What `ACTION` to take if a the template folder does not contain a boilerplate.yml file. Must be one of: %s. Default: %s.", options.AllMissingConfigActions, options.DefaultMissingConfigAction),
		},
		cli.BoolFlag{
			Name:  options.OptDisableHooks,
			Usage: "If this flag is set, no hooks will execute.",
		},
		cli.BoolFlag{
			Name:  options.OptDisableShell,
			Usage: "If this flag is set, no shell helpers will execute. They will instead return the text 'replace-me'.",
		},
	}

	return app

}

type FileData struct {
	Name     string
	Size     int64
	IsDir    bool
	Url      string
	Language string
}

// When you run the CLI, this is the action function that gets called
func runApp(cliContext *cli.Context) error {
	if !cliContext.Args().Present() && cliContext.NumFlags() == 0 {
		return cli.ShowAppHelp(cliContext)
	}

	opts, err := options.ParseOptions(cliContext)
	if err != nil {
		return err
	}

	templateType, err := boilerplateType(opts)
	if err != nil {
		return err
	}

	switch templateType {
	case Yaml:
		return handleYamlBoilerplate(opts)
	case Markdown:
		return handleMarkdownBoilerplate(opts)
	default:
		return fmt.Errorf("Uh oh, how did I get here?")
	}
}

func handleYamlBoilerplate(opts *options.BoilerplateOptions) error {
	schema, err := templates.GetJsonSchema(opts)
	if err != nil {
		return err
	}

	responseParts := []ResponsePart{{Type: BoilerplateYaml, BoilerplateYamlFormSchema: schema}}
	return runServer(responseParts, opts)
}

func runServer(responseParts []ResponsePart, opts *options.BoilerplateOptions) error {
	router := gin.Default()

	// create-react-app runs on a different port, so to allow it to make AJAX calls here, add CORS rules
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"http://localhost:3000"}
	router.Use(cors.New(corsConfig))

	router.Static("/rendered", opts.OutputFolder)

	router.GET("/form", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, responseParts)
	})

	router.POST("/render", func(ctx *gin.Context) {
		var vars map[string]interface{}
		if err := ctx.ShouldBindJSON(&vars); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// The root boilerplate.yml is not itself a dependency, so we pass an empty Dependency.
		emptyDep := variables.Dependency{}

		// Render
		err := templates.ProcessTemplate(opts, opts, emptyDep, vars)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		dirContents, err := GetDirContents(opts)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"files": dirContents})
	})

	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		router.Run()
		wg.Done()
	}()

	browser.OpenURL("http://localhost:3000")

	wg.Wait()

	return nil
}

func handleMarkdownBoilerplate(opts *options.BoilerplateOptions) error {
	mdPath := filepath.Join(opts.TemplateFolder, "boilerplate.md")
	mdContents, err := ioutil.ReadFile(mdPath)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	parts, err := processMarkdown(string(mdContents))
	if err != nil {
		return errors.WithStackTrace(err)
	}

	// New template dir where we will write all the template files
	tmpTemplateFolder, err := ioutil.TempDir("", "boilerplate-temp-template")
	if err != nil {
		return errors.WithStackTrace(err)
	}
	opts.TemplateFolder = tmpTemplateFolder

	// Put dummy boilerplate.yml in that folder
	if err := ioutil.WriteFile(filepath.Join(opts.TemplateFolder, "boilerplate.yml"), []byte("# Auto-generated"), 0644); err != nil {
		return errors.WithStackTrace(err)
	}

	util.Logger.Printf("Using temporary temp folder: %s", opts.TemplateFolder)

	responseParts := []ResponsePart{}
	for _, part := range parts {
		responsePart := ResponsePart{}
		switch part.Type {
		case BoilerplateYaml:
			configStr := strings.Join(part.BoilerplateYaml, "\n")
			boilerplateConfig, err := config.ParseBoilerplateConfigFromString(configStr)
			if err != nil {
				return errors.WithStackTrace(err)
			}

			schema, err := templates.BoilerplateConfigToJsonSchema(boilerplateConfig, opts)
			if err != nil {
				return errors.WithStackTrace(err)
			}

			responsePart.Type = BoilerplateYaml
			responsePart.BoilerplateYamlFormSchema = schema
		case RawMarkdown:
			responsePart.Type = RawMarkdown
			markdown := strings.Join(part.RawMarkdown, "\n")
			responsePart.RawMarkdown = &markdown
		case BoilerplateTemplate:
			// TODO: it's a bit weird to have this side effect here, and this needs some security analysis, but good enough for this hacky code for now
			template := strings.Join(part.BoilerplateTemplate, "\n")
			templatePath := filepath.Join(opts.TemplateFolder, *part.BoilerplateTemplatePath)

			parentDir := filepath.Dir(templatePath)
			if err := os.MkdirAll(parentDir, 0777); err != nil {
				return errors.WithStackTrace(err)
			}

			if err := ioutil.WriteFile(templatePath, []byte(template), 0644); err != nil {
				return errors.WithStackTrace(err)
			}
			responsePart.Type = BoilerplateTemplate
			responsePart.BoilerplateTemplatePath = part.BoilerplateTemplatePath
		case ExecutableSnippet:
			snippet := strings.Join(part.ExecutableSnippet, "\n")
			responsePart.Type = ExecutableSnippet
			responsePart.ExecutableSnippet = &snippet
			responsePart.ExecutableSnippetLang = part.ExecutableSnippetLang
		default:
			return errors.WithStackTrace(fmt.Errorf("Another impossible result"))
		}

		responseParts = append(responseParts, responsePart)
	}

	return runServer(responseParts, opts)
}

type MarkdownPartType int

const (
	RawMarkdown MarkdownPartType = iota
	BoilerplateYaml
	BoilerplateTemplate
	ExecutableSnippet
)

type MarkdownPart struct {
	Type                    MarkdownPartType
	RawMarkdown             []string
	BoilerplateYaml         []string
	BoilerplateTemplate     []string
	BoilerplateTemplatePath *string
	ExecutableSnippet       []string
	ExecutableSnippetLang   *string
}

type ResponsePart struct {
	Type                      MarkdownPartType
	RawMarkdown               *string
	BoilerplateYamlFormSchema map[string]interface{}
	BoilerplateTemplatePath   *string
	ExecutableSnippet         *string
	ExecutableSnippetLang     *string
}

var executableSnippetRegex = regexp.MustCompile("```(.+?\\s+)?\\(boilerplate::executable\\).*")
var inputRegex = regexp.MustCompile("```.*\\(boilerplate::input\\).*")
var templateRegex = regexp.MustCompile("```.*\\(boilerplate::template:\\s*\"(.+?)\"\\).*")

func processMarkdown(mdContents string) ([]MarkdownPart, error) {
	lines := strings.Split(mdContents, "\n")
	parts := []MarkdownPart{}

	// TODO: this is a hacky, line by line, regex-based parsing... We should use a proper Markdown parser instead.
	part := MarkdownPart{
		Type: RawMarkdown,
	}
	for _, line := range lines {
		lineClean := strings.TrimSpace(line)

		switch part.Type {
		case BoilerplateYaml:
			if lineClean == "```" {
				parts = append(parts, part)
				part = MarkdownPart{
					Type: RawMarkdown,
				}
			} else {
				part.BoilerplateYaml = append(part.BoilerplateYaml, line)
			}
		case ExecutableSnippet:
			if lineClean == "```" {
				parts = append(parts, part)
				part = MarkdownPart{
					Type: RawMarkdown,
				}
			} else {
				part.ExecutableSnippet = append(part.ExecutableSnippet, line)
			}
		case BoilerplateTemplate:
			if lineClean == "```" {
				parts = append(parts, part)
				part = MarkdownPart{
					Type: RawMarkdown,
				}
			} else {
				part.BoilerplateTemplate = append(part.BoilerplateTemplate, line)
			}
		case RawMarkdown:
			if executableSnippetRegex.MatchString(lineClean) {
				parts = append(parts, part)
				match := executableSnippetRegex.FindStringSubmatch(lineClean)
				if len(match) != 2 {
					return nil, fmt.Errorf("Invalid executable snippet: %s", line)
				}
				var lang *string
				if match[1] != "" {
					lang = &match[1]
				}
				part = MarkdownPart{
					Type:                  ExecutableSnippet,
					ExecutableSnippetLang: lang,
				}
			} else if inputRegex.MatchString(lineClean) {
				parts = append(parts, part)
				part = MarkdownPart{
					Type: BoilerplateYaml,
				}
			} else if templateRegex.MatchString(lineClean) {
				parts = append(parts, part)
				match := templateRegex.FindStringSubmatch(lineClean)
				if len(match) != 2 {
					return nil, fmt.Errorf("Invalid template marker: %s", line)
				}
				part = MarkdownPart{
					Type:                    BoilerplateTemplate,
					BoilerplateTemplatePath: &match[1],
				}
			} else {
				part.RawMarkdown = append(part.RawMarkdown, line)
			}
		default:
			return nil, fmt.Errorf("This shouldn't be possible...")
		}
	}

	parts = append(parts, part)

	return parts, nil
}

type BoilerplateType int

const (
	Yaml BoilerplateType = iota
	Markdown
)

func boilerplateType(opts *options.BoilerplateOptions) (BoilerplateType, error) {
	yamlPath := filepath.Join(opts.TemplateFolder, "boilerplate.yml")
	if util.PathExists(yamlPath) {
		return Yaml, nil
	}

	mdPath := filepath.Join(opts.TemplateFolder, "boilerplate.md")
	if util.PathExists(mdPath) {
		return Markdown, nil
	}

	return Yaml, fmt.Errorf("%s doesn't seem to be a valid boilerplate folder", opts.TemplateFolder)
}

func GetDirContents(opts *options.BoilerplateOptions) ([]FileData, error) {
	absPath, err := filepath.Abs(filepath.Clean(opts.OutputFolder))
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	// Only allow listing contents of rendered dir
	if !strings.HasPrefix(absPath, opts.OutputFolder) {
		return nil, errors.WithStackTrace(fmt.Errorf("Cannot display contents outside of %s", opts.OutputFolder))
	}

	out := []FileData{}
	walkErr := filepath.WalkDir(opts.OutputFolder, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == opts.OutputFolder {
			return nil
		}

		fileData, err := dirEntryToFileData(path, entry, opts)
		if err != nil {
			return err
		}
		out = append(out, *fileData)
		return nil
	})

	if walkErr != nil {
		return nil, errors.WithStackTrace(err)
	}

	return out, nil
}

func dirEntryToFileData(filePath string, entry fs.DirEntry, opts *options.BoilerplateOptions) (*FileData, error) {
	absPath, err := filepath.Abs(filepath.Clean(filePath))
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	absBasePath, err := filepath.Abs(filepath.Clean(opts.OutputFolder))
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	util.Logger.Printf("Subtracting absBasePath '%s' from absPath '%s'", absBasePath, absPath)

	relPath := strings.TrimPrefix(strings.TrimPrefix(absPath, absBasePath), string(os.PathSeparator))

	info, err := entry.Info()
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	renderedUrl, err := url.Parse("http://localhost:8080/rendered")
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}
	renderedUrl.Path = path.Join(renderedUrl.Path, relPath)

	return &FileData{
		Name:     relPath,
		IsDir:    entry.IsDir(),
		Size:     info.Size(),
		Url:      renderedUrl.String(),
		Language: languageForFile(entry),
	}, nil
}

// https://prismjs.com/#supported-languages
func languageForFile(file fs.DirEntry) string {
	ext := strings.TrimPrefix(filepath.Ext(file.Name()), ".")
	switch ext {
	case "tf", "hcl":
		return "hcl"
	case "sh", "bash":
		return "bash"
	case "go":
		return "go"
	case "md":
		return "markdown"
	default:
		return ext // Hope for the best
	}
}
