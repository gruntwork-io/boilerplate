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
	"github.com/gruntwork-io/go-commons/files"
	"github.com/gruntwork-io/go-commons/version"
	"github.com/gruntwork-io/terratest/modules/collections"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/mattn/go-zglob"
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
	"sort"
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
	case TerraformModule:
		return handleTerraformBoilerplate(opts)
	case TerraformCatalog:
		return handleTerraformCatalog(opts)
	case Live:
		return handleLive(opts)
	case AppDemo:
		return handleAppDemo(opts)
	default:
		return fmt.Errorf("Uh oh, how did I get here?")
	}
}

func handleAppDemo(opts *options.BoilerplateOptions) error {
	livePath := filepath.Join(opts.TemplateFolder, "live")
	modulesPath := filepath.Join(opts.TemplateFolder, "modules")
	scaffoldsPath := filepath.Join(opts.TemplateFolder, "scaffolds")

	tgModulePaths, err := getTgModulePaths(livePath)
	if err != nil {
		return err
	}

	liveView, err := formatLiveView(tgModulePaths, livePath)
	if err != nil {
		return err
	}
	liveContent := fmt.Sprintf("# Live\n\n%s", liveView)

	scaffoldPaths, err := getScaffoldPaths(scaffoldsPath)
	if err != nil {
		return err
	}

	for i, scaffoldPath := range scaffoldPaths {
		scaffoldPaths[i] = filepath.Join("scaffolds", scaffoldPath)
	}

	scaffoldsView, err := formatScaffoldsView(scaffoldPaths, opts.TemplateUrl)
	if err != nil {
		return err
	}
	scaffoldsContent := fmt.Sprintf("# Scaffolds\n\n%s", scaffoldsView)

	tfModulePaths, err := getTerraformModulePaths(modulesPath)
	if err != nil {
		return err
	}

	for i, tfModulePath := range tfModulePaths {
		tfModulePaths[i] = filepath.Join("modules", tfModulePath)
	}

	table, err := formatModuleTable(tfModulePaths, opts.TemplateUrl)
	if err != nil {
		return err
	}
	catalogContent := fmt.Sprintf("# Catalog\n\n%s", table)


	responseParts := []ResponsePart{
		{
			Type:        RawMarkdown,
			RawMarkdown: strPtr(liveContent),
		},
		{
			Type:        RawMarkdown,
			RawMarkdown: strPtr(scaffoldsContent),
		},
		{
			Type:        RawMarkdown,
			RawMarkdown: strPtr(catalogContent),
		},
	}
	return runServer(responseParts, opts)
}

func getScaffoldPaths(templateFolder string) ([]string, error) {
	mdFiles, err := zglob.Glob(filepath.Join(templateFolder, "**/boilerplate.md"))
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	yamlFiles, err := zglob.Glob(filepath.Join(templateFolder, "**/boilerplate.yml"))
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	allFiles := append(mdFiles, yamlFiles...)

	sort.Strings(allFiles)

	scaffoldPaths := []string{}

	for _, scaffoldFile := range allFiles {
		relPath, err := files.GetPathRelativeTo(scaffoldFile, templateFolder)
		if err != nil {
			return nil, errors.WithStackTrace(err)
		}

		if !strings.HasPrefix(relPath, ".") && strings.ContainsRune(relPath, filepath.Separator) {
			scaffoldPath := filepath.Dir(relPath)
			if !collections.ListContains(scaffoldPaths, scaffoldPath) {
				scaffoldPaths = append(scaffoldPaths, scaffoldPath)
			}
		}
	}

	return scaffoldPaths, nil
}

func handleLive(opts *options.BoilerplateOptions) error {
	modulePaths, err := getTgModulePaths(opts.TemplateFolder)
	if err != nil {
		return err
	}

	liveView, err := formatLiveView(modulePaths, opts.TemplateUrl)
	if err != nil {
		return err
	}
	content := fmt.Sprintf("# Live\n\n%s", liveView)

	responseParts := []ResponsePart{
		{
			Type:        RawMarkdown,
			RawMarkdown: strPtr(content),
		},
	}
	return runServer(responseParts, opts)
}

func handleTerraformCatalog(opts *options.BoilerplateOptions) error {
	modulePaths, err := getTerraformModulePaths(opts.TemplateFolder)
	if err != nil {
		return err
	}

	table, err := formatModuleTable(modulePaths, opts.TemplateUrl)
	if err != nil {
		return err
	}
	content := fmt.Sprintf("# Catalog\n\n%s", table)

	responseParts := []ResponsePart{
		{
			Type:        RawMarkdown,
			RawMarkdown: strPtr(content),
		},
	}
	return runServer(responseParts, opts)
}

const scaffoldsTableTemplate = `
| Scaffold | Type |
| -------- | ---- |
%s
`

func formatScaffoldsView(scaffoldPaths []string, templateBasePath string) (string, error) {
	tableRows := []string{}

	for _, scaffoldPath := range scaffoldPaths {
		scaffoldType := "yml"
		if files.FileExists(filepath.Join(templateBasePath, scaffoldPath, "boilerplate.md")) {
			scaffoldType = "markdown"
		}

		scaffoldLink := fmt.Sprintf("[%s](/scaffold/%s)", scaffoldPath, scaffoldPath)
		tableRow := formatAsTableRow([]string{scaffoldLink, scaffoldType})
		tableRows = append(tableRows, tableRow)
	}

	return fmt.Sprintf(scaffoldsTableTemplate, strings.Join(tableRows, "\n")), nil
}

const moduleTableTemplate = `
| Module | Type | Description |
| ------ | ---- | ----------- |
%s
`

func formatLiveView(modulePaths []string, templateBasePath string) (string, error) {
	tgModules := []TgModule{}
	moduleUsageMappings := map[string][]TgModule{}
	envs := []string{}
	regions := []string{}

	for _, modulePath := range modulePaths {
		tgModule, err := parseTgModule(modulePath, templateBasePath)
		if err != nil {
			return "", err
		}

		tgModules = append(tgModules, tgModule)

		moduleUsageMapping, ok := moduleUsageMappings[tgModule.Usage]
		if !ok {
			moduleUsageMapping = []TgModule{}
		}
		moduleUsageMapping = append(moduleUsageMapping, tgModule)
		moduleUsageMappings[tgModule.Usage] = moduleUsageMapping

		if !collections.ListContains(envs, tgModule.Env) {
			envs = append(envs, tgModule.Env)
		}
		if !collections.ListContains(regions, tgModule.Region) {
			regions = append(regions, tgModule.Region)
		}
	}

	sort.Strings(envs)

	firstRow := formatAsTableHeaderRow(append([]string{"Module"}, envs...))
	tableRows := []string{firstRow}

	for usagePath, usages := range moduleUsageMappings {
		sort.SliceStable(usages, func(i, j int) bool {
			return usages[i].Env < usages[j].Env
		})

		cols := []string{fmt.Sprintf("[%s](#todo)", usagePath)}
		for _, usage := range usages {
			cols = append(cols, fmt.Sprintf("[%s](#todo)", usage.Version))
		}
		tableRows = append(tableRows, formatAsTableRow(cols))
	}

	return strings.Join(tableRows, "\n"), nil
}

func formatAsTableRow(vals []string) string {
	return fmt.Sprintf("| %s |", strings.Join(vals, " | "))
}

func formatAsTableHeaderRow(vals []string) string {
	placeHolders := []string{}
	for _, val := range vals {
		placeHolder := strings.Repeat("-", len(val))
		placeHolders = append(placeHolders, placeHolder)
	}

	row := formatAsTableRow(vals)
	headerRow := formatAsTableRow(placeHolders)
	return fmt.Sprintf("%s\n%s", row, headerRow)
}

type TgModule struct {
	Path     string
	Env      string
	Region   string
	Usage    string
	Repo     string
	Module   string
	Version  string
	UpToDate bool
}

// Very hacky regex to parse the source URL in a TG module
var sourceRegex = regexp.MustCompile(`github.com/gruntwork-io/(.+?).git//(.+?)\?ref=(.+)"`)

// Very hacky regex to parse infra-live path of format ENV/REGION/MODULE
var tgEnvRegionRegex = regexp.MustCompile(`^(.+?)/(.+?)/(.+?)$`)

func parseTgModule(modulePath string, templateBasePath string) (TgModule, error) {
	tgModule := TgModule{
		Path:     modulePath,
		Env:      "(unknown)",
		Region:   "(unknown)",
		Usage:    "(unknown)",
		Repo:     "(unknown)",
		Module:   "(unknown)",
		Version:  "(unknown)",
		UpToDate: false,
	}

	moduleParts := tgEnvRegionRegex.FindStringSubmatch(modulePath)
	if len(moduleParts) != 4 {
		util.Logger.Printf("[WARN] Couldn't parse module path %s into account, region, module parts", modulePath)
		return tgModule, nil
	}

	tgModule.Env = moduleParts[1]
	tgModule.Region = moduleParts[2]
	tgModule.Usage = moduleParts[3]

	tgConfigPath := filepath.Join(templateBasePath, modulePath, "terragrunt.hcl")
	if !files.FileExists(tgConfigPath) {
		util.Logger.Printf("[WARN] Found TG module at %s, but %s does not exist", modulePath, tgConfigPath)
		return tgModule, nil
	}

	tgBytes, err := ioutil.ReadFile(tgConfigPath)
	if err != nil {
		return tgModule, errors.WithStackTrace(err)
	}

	// TODO: we should be using the 'terragrunt render-json' command here, as we do with Patcher, but in this hackday
	// project, I'm trying to save time, and parsing HCL directly. This won't work with some TG configs that construct
	// source URLs dynamically.
	hclFile, diag := hclwrite.ParseConfig(tgBytes, tgConfigPath, hcl.InitialPos)
	if diag.HasErrors() {
		return tgModule, errors.WithStackTrace(diag)
	}

	tfBlock := hclFile.Body().FirstMatchingBlock("terraform", nil)
	if tfBlock == nil {
		util.Logger.Printf("[WARN] Could not find terraform block in %s", tgConfigPath)
		return tgModule, nil
	}

	source := tfBlock.Body().GetAttribute("source")
	if source == nil {
		util.Logger.Printf("[WARN] Could not find source URL in terraform block in %s", tgConfigPath)
		return tgModule, nil
	}

	sourceUrl := string(source.Expr().BuildTokens(nil).Bytes())

	matches := sourceRegex.FindStringSubmatch(sourceUrl)
	if len(matches) != 4 {
		util.Logger.Printf("[WARN] Could not parse source URL in terraform block in %s: %s", tgConfigPath, sourceUrl)
		return tgModule, nil
	}

	tgModule.Repo = matches[1]
	tgModule.Module = matches[2]
	tgModule.Version = matches[3]
	tgModule.UpToDate = true

	return tgModule, nil
}

func formatModuleTable(modulePaths []string, templateBasePath string) (string, error) {
	tableRows := []string{}

	for _, modulePath := range modulePaths {
		description, err := extractDescription(modulePath, templateBasePath)
		if err != nil {
			return "", err
		}
		row := fmt.Sprintf("| [%s](/auto-scaffold/%s) | %s | %s |", modulePath, modulePath, "terraform", description)
		tableRows = append(tableRows, row)
	}

	rows := strings.Join(tableRows, "\n")

	return fmt.Sprintf(moduleTableTemplate, rows), nil
}

// Super hacky method to loop over the README of the given module (if one exists) and extract the first line from that
// README that looks like a description (i.e., isn't frontmatter, a header, a badge, an image, etc).
func extractDescription(modulePath string, templateBasePath string) (string, error) {
	readmePath := filepath.Join(templateBasePath, modulePath, "README.md")
	if files.FileExists(readmePath) {
		readmeContentsBytes, err := ioutil.ReadFile(readmePath)
		if err != nil {
			return "", errors.WithStackTrace(err)
		}

		lines := strings.Split(string(readmeContentsBytes), "\n")

		inCommentBlock := false
		for _, line := range lines {
			cleanLine := strings.TrimSpace(line)
			if strings.HasPrefix(cleanLine, "<!--") {
				inCommentBlock = true
			} else if strings.Contains(cleanLine, "-->") {
				inCommentBlock = false
			} else if !inCommentBlock && markdownLineLooksLikeDescription(cleanLine) {
				return fmt.Sprintf("%s...", cleanLine), nil
			}
		}
	}

	return fmt.Sprintf("(couldn't extract description for module %s)", cleanModuleName(modulePath)), nil
}

func markdownLineLooksLikeDescription(line string) bool {
	return len(line) > 0 && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "!") && !strings.HasPrefix(line, "-")
}

func getTgModulePaths(templateFolder string) ([]string, error) {
	tgFiles, err := zglob.Glob(filepath.Join(templateFolder, "**/terragrunt.hcl"))
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	sort.Strings(tgFiles)

	modulePaths := []string{}

	for _, tfFile := range tgFiles {
		relPath, err := files.GetPathRelativeTo(tfFile, templateFolder)
		if err != nil {
			return nil, errors.WithStackTrace(err)
		}

		if !strings.HasPrefix(relPath, ".") && strings.ContainsRune(relPath, filepath.Separator) {
			modulePath := filepath.Dir(relPath)
			if !collections.ListContains(modulePaths, modulePath) {
				modulePaths = append(modulePaths, modulePath)
			}
		}
	}

	return modulePaths, nil
}

func getTerraformModulePaths(templateFolder string) ([]string, error) {
	terraformFiles, err := zglob.Glob(filepath.Join(templateFolder, "**/*.tf"))
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	sort.Strings(terraformFiles)

	modulePaths := []string{}

	for _, tfFile := range terraformFiles {
		relPath, err := files.GetPathRelativeTo(tfFile, templateFolder)
		if err != nil {
			return nil, errors.WithStackTrace(err)
		}

		if !strings.HasPrefix(relPath, ".") && strings.ContainsRune(relPath, filepath.Separator) {
			modulePath := filepath.Dir(relPath)
			if !collections.ListContains(modulePaths, modulePath) {
				modulePaths = append(modulePaths, modulePath)
			}
		}
	}

	return modulePaths, nil
}

func doHandleBoilerplateScaffold(opts *options.BoilerplateOptions) ([]ResponsePart, error) {
	yamlPath := filepath.Join(opts.TemplateFolder, "boilerplate.yml")
	if util.PathExists(yamlPath) {
		return doHandleBoilerplateYamlScaffold(opts)
	}

	mdPath := filepath.Join(opts.TemplateFolder, "boilerplate.md")
	if util.PathExists(mdPath) {
		return doHandleBoilerplateMdScaffold(opts)
	}

	return nil, fmt.Errorf("Couldn't find boilerplate.yml or boilerplate.md in %s", opts.TemplateFolder)
}

func doHandleBoilerplateYamlScaffold(opts *options.BoilerplateOptions) ([]ResponsePart, error) {
	schema, err := templates.GetJsonSchema(opts)
	if err != nil {
		return nil, err
	}

	return []ResponsePart{{Type: BoilerplateYaml, BoilerplateYamlFormSchema: schema}}, nil
}

func doHandleBoilerplateMdScaffold(opts *options.BoilerplateOptions) ([]ResponsePart, error) {
	mdPath := filepath.Join(opts.TemplateFolder, "boilerplate.md")
	mdContents, err := ioutil.ReadFile(mdPath)
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	parts, err := processMarkdown(string(mdContents))
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	// New template dir where we will write all the template files
	tmpTemplateFolder, err := ioutil.TempDir("", "boilerplate-temp-template")
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}
	opts.TemplateFolder = tmpTemplateFolder

	// Put dummy boilerplate.yml in that folder
	if err := ioutil.WriteFile(filepath.Join(opts.TemplateFolder, "boilerplate.yml"), []byte("# Auto-generated"), 0644); err != nil {
		return nil, errors.WithStackTrace(err)
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
				return nil, errors.WithStackTrace(err)
			}

			schema, err := templates.BoilerplateConfigToJsonSchema(boilerplateConfig, opts, "Inputs")
			if err != nil {
				return nil, errors.WithStackTrace(err)
			}

			responsePart.Type = BoilerplateYaml
			responsePart.BoilerplateYamlFormSchema = schema
			responsePart.BoilerplateFormOrder = varOrderForForm(boilerplateConfig)
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
				return nil, errors.WithStackTrace(err)
			}

			if err := ioutil.WriteFile(templatePath, []byte(template), 0644); err != nil {
				return nil, errors.WithStackTrace(err)
			}
			responsePart.Type = BoilerplateTemplate
			responsePart.BoilerplateTemplatePath = part.BoilerplateTemplatePath
		case ExecutableSnippet:
			snippet := strings.Join(part.ExecutableSnippet, "\n")
			responsePart.Type = ExecutableSnippet
			responsePart.ExecutableSnippet = &snippet
			responsePart.ExecutableSnippetLang = part.ExecutableSnippetLang
		default:
			return nil, errors.WithStackTrace(fmt.Errorf("Another impossible result"))
		}

		responseParts = append(responseParts, responsePart)
	}

	return responseParts, nil
}

func handleYamlBoilerplate(opts *options.BoilerplateOptions) error {
	responseParts, err := doHandleBoilerplateYamlScaffold(opts)
	if err != nil {
		return err
	}
	return runServer(responseParts, opts)
}

func runServer(responseParts []ResponsePart, opts *options.BoilerplateOptions) error {
	router := gin.Default()

	originalTemplateUrl := opts.TemplateUrl
	originalOutputFolder := opts.OutputFolder

	// create-react-app runs on a different port, so to allow it to make AJAX calls here, add CORS rules
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"http://localhost:3000"}
	router.Use(cors.New(corsConfig))

	// The OutputFolder may change, so we don't use a static server, but create a more dynamic one ourselves
	//router.Static("/rendered", opts.OutputFolder)
	router.GET("/rendered/*filePath", func(ctx *gin.Context) {
		filePath := ctx.Param("filePath")

		// TODO: we should guard against someone looking up paths outside PWD with ../
		fullPath := filepath.Join(opts.OutputFolder, filePath)
		if !files.FileExists(fullPath) {
			util.Logger.Printf("[ERROR] Requested file %s does not exist")
			ctx.Writer.WriteHeader(http.StatusNotFound)
			return
		}

		ctx.File(fullPath)
	})

	router.GET("/form", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, responseParts)
	})

	router.GET("/scaffold/*scaffoldPath", func(ctx *gin.Context) {
		scaffoldPath := ctx.Param("scaffoldPath")

		// TODO: we should guard against someone looking up paths outside PWD with ../
		rawTemplateUrl := filepath.Join(originalTemplateUrl, scaffoldPath)
		templateUrl, templateFolder, err := options.DetermineTemplateConfig(rawTemplateUrl)
		if err != nil {
			util.Logger.Printf("[ERROR] %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		opts.TemplateUrl = templateUrl
		opts.TemplateFolder = templateFolder
		opts.OutputFolder = filepath.Join(originalOutputFolder, scaffoldPath)

		parts, err := doHandleBoilerplateScaffold(opts)

		if err != nil {
			util.Logger.Printf("[ERROR] %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, parts)
	})

	router.GET("/auto-scaffold/*modulePath", func(ctx *gin.Context) {
		modulePath := ctx.Param("modulePath")

		// TODO: we should guard against someone looking up paths outside PWD with ../
		rawTemplateUrl := filepath.Join(originalTemplateUrl, modulePath)
		templateUrl, templateFolder, err := options.DetermineTemplateConfig(rawTemplateUrl)
		if err != nil {
			util.Logger.Printf("[ERROR] %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		opts.TemplateUrl = templateUrl
		opts.TemplateFolder = templateFolder
		opts.OutputFolder = filepath.Join(originalOutputFolder, modulePath)

		parts, err := doHandleTerraformBoilerplate(opts)
		if err != nil {
			util.Logger.Printf("[ERROR] %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, parts)
	})

	router.POST("/render", func(ctx *gin.Context) {
		var vars map[string]interface{}
		if err := ctx.ShouldBindJSON(&vars); err != nil {
			util.Logger.Printf("[ERROR] %v", err)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// The root boilerplate.yml is not itself a dependency, so we pass an empty Dependency.
		emptyDep := variables.Dependency{}

		// Render
		err := templates.ProcessTemplate(opts, opts, emptyDep, vars)
		if err != nil {
			util.Logger.Printf("[ERROR] %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		dirContents, err := GetDirContents(opts)
		if err != nil {
			util.Logger.Printf("[ERROR] %v", err)
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

func handleTerraformBoilerplate(opts *options.BoilerplateOptions) error {
	parts, err := doHandleTerraformBoilerplate(opts)
	if err != nil {
		return err
	}
	return runServer(parts, opts)
}

func doHandleTerraformBoilerplate(opts *options.BoilerplateOptions) ([]ResponsePart, error) {
	// We use terraform-config-inspect to get the basic info.
	tfConfigModule, diags := tfconfig.LoadModule(opts.TemplateFolder)
	if diags.HasErrors() {
		return nil, errors.WithStackTrace(diags)
	}

	// Unfortunately, terraform-config-inspect stores parsed variables in a map, so they are not in the order we find
	// them in variables.tf. Therefore, here, we have a hacky method that implements variable parsing so we can get them
	// in the proper order
	tfVarsRequired, err := parseTerraformVariables(opts, tfConfigModule, Required)
	if err != nil {
		return nil, err
	}
	tfVarsOptional, err := parseTerraformVariables(opts, tfConfigModule, Optional)
	if err != nil {
		return nil, err
	}

	boilerplateConfigRequiredVars, err := terraformModuleToBoilerplateConfig(tfVarsRequired)
	if err != nil {
		return nil, err
	}

	schemaRequiredVars, err := templates.BoilerplateConfigToJsonSchema(boilerplateConfigRequiredVars, opts, "Required variables")
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	boilerplateConfigOptionalVars, err := terraformModuleToBoilerplateConfig(tfVarsOptional)
	if err != nil {
		return nil, err
	}

	schemaOptionalVars, err := templates.BoilerplateConfigToJsonSchema(boilerplateConfigOptionalVars, opts, "Optional variables")
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	usageTemplateParts, err := createUsageTemplate(opts, tfConfigModule, tfVarsRequired, tfVarsOptional)
	if err != nil {
		return nil, err
	}

	moduleReadmeParts, err := getModuleReadmePart(opts)
	if err != nil {
		return nil, err
	}

	// TODO: this is an ugly hack to force some spacing between sections
	extraSpacing := ResponsePart{
		Type:        RawMarkdown,
		RawMarkdown: strPtr("# &nbsp;\n"),
	}

	parts := append(append([]ResponsePart{
		{
			Type:        RawMarkdown,
			RawMarkdown: strPtr(fmt.Sprintf("# Module %s usage\n", cleanModuleName(tfConfigModule.Path))),
		},
		{
			Type:                      BoilerplateYaml,
			BoilerplateYamlFormSchema: schemaRequiredVars,
			BoilerplateFormOrder:      varOrderForForm(boilerplateConfigRequiredVars),
		},
		extraSpacing,
		{
			Type:              ExecutableSnippet,
			ExecutableSnippet: strPtr("terraform init\nterraform apply"),
		},
		extraSpacing,
		{
			Type:                      BoilerplateYaml,
			BoilerplateYamlFormSchema: schemaOptionalVars,
			BoilerplateFormOrder:      varOrderForForm(boilerplateConfigOptionalVars),
		},
	}, usageTemplateParts...), moduleReadmeParts...)

	return parts, nil
}

func getModuleReadmePart(opts *options.BoilerplateOptions) ([]ResponsePart, error) {
	readmeParts := []ResponsePart{}

	readmePath := filepath.Join(opts.TemplateUrl, "README.md")
	if !files.FileExists(readmePath) {
		return readmeParts, nil
	}

	contents, err := ioutil.ReadFile(readmePath)
	if err != nil {
		return readmeParts, errors.WithStackTrace(err)
	}

	return []ResponsePart{
		{
			Type:        RawMarkdown,
			RawMarkdown: strPtr(string(contents)),
		},
	}, nil
}

func varOrderForForm(cfg *config.BoilerplateConfig) []string {
	out := []string{}

	for _, variable := range cfg.Variables {
		out = append(out, variable.Name())
	}

	return out
}

type TfVariable struct {
	Name        string
	Description *string
	Type        *string
	Default     interface{}
	HasDefault  bool
	DefaultHcl  *string
}

func parseTerraformVariables(opts *options.BoilerplateOptions, tfConfigModule *tfconfig.Module, varType VarType) ([]TfVariable, error) {
	tfVars := []TfVariable{}

	terraformFiles, err := zglob.Glob(filepath.Join(opts.TemplateFolder, "*.tf"))
	if err != nil {
		return tfVars, errors.WithStackTrace(err)
	}

	for _, tfFile := range terraformFiles {
		contents, err := ioutil.ReadFile(tfFile)
		if err != nil {
			return tfVars, errors.WithStackTrace(err)
		}
		hclFile, diag := hclwrite.ParseConfig(contents, tfFile, hcl.InitialPos)
		if diag.HasErrors() {
			return tfVars, errors.WithStackTrace(diag)
		}

		tfVarsInFile, err := findVariablesInHclFile(hclFile, tfFile, tfConfigModule, varType)
		if err != nil {
			return tfVars, err
		}

		tfVars = append(tfVars, tfVarsInFile...)
	}

	return tfVars, nil
}

func findVariablesInHclFile(hclFile *hclwrite.File, filePath string, tfConfigModule *tfconfig.Module, varType VarType) ([]TfVariable, error) {
	tfVars := []TfVariable{}

	for _, block := range hclFile.Body().Blocks() {
		if block.Type() == "variable" {
			if len(block.Labels()) != 1 {
				return tfVars, fmt.Errorf("Unexpected number of labels (%d) on an input variable in '%s': %v", len(block.Labels()), filePath, block.Labels())
			}

			tfVar := TfVariable{
				Name: block.Labels()[0],
			}

			description := block.Body().GetAttribute("description")
			if description != nil {
				tfVar.Description = strPtr(strings.TrimSpace(string(description.Expr().BuildTokens(nil).Bytes())))
			}

			typeOfVar := block.Body().GetAttribute("type")
			if typeOfVar != nil {
				tfVar.Type = strPtr(strings.TrimSpace(string(typeOfVar.Expr().BuildTokens(nil).Bytes())))
			}

			defaultValue := block.Body().GetAttribute("default")
			if defaultValue != nil {
				// TODO: There seems to be no sane way with hclwrite to parse the default value into a Go type (without
				// knowing the full structure of that type ahead of time), so as a hack, we get the default from the
				// terraform-config-inspect library, which does some sort of hacky, best-effort cty parsing of the
				// default value
				tfConfigVar, ok := tfConfigModule.Variables[tfVar.Name]
				if !ok {
					return tfVars, fmt.Errorf("Found variable '%s' in '%s', but tfconfig doesn't contain that variable", tfVar.Name, filePath)
				}

				tfVar.Default = tfConfigVar.Default
				tfVar.HasDefault = true
				tfVar.DefaultHcl = strPtr(string(defaultValue.Expr().BuildTokens(nil).Bytes()))
			}

			if varType == Required && !tfVar.HasDefault || varType == Optional && tfVar.HasDefault {
				tfVars = append(tfVars, tfVar)
			}
		}
	}

	return tfVars, nil
}

func cleanModuleName(modulePath string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(filepath.Base(modulePath)), " ", "_"), "-", "_")
}

func formatMainTf(tfConfigModule *tfconfig.Module, tfVarsRequired []TfVariable, tfVarsOptional []TfVariable) string {
	moduleName := cleanModuleName(tfConfigModule.Path)

	// TODO: this is just a hard-coded mock that we will need to fix
	sourceUrl := fmt.Sprintf("git::git@github.com:gruntwork-io/terraform-aws-service-catalog.git//modules/%s?ref=v0.104.2", moduleName)

	return fmt.Sprintf(`
module "%s" {
  source = "%s"

  # Required vars
  %s

  # Optional vars
  %s
}
`, moduleName, sourceUrl, formatVars(tfVarsRequired), formatVars(tfVarsOptional))
}

func formatVars(tfVars []TfVariable) string {
	lines := []string{}

	for _, tfVar := range tfVars {
		lines = append(lines, formatVar(tfVar))
	}

	return strings.Join(lines, "\n")
}

func formatVar(tfVar TfVariable) string {
	return fmt.Sprintf("{{- if index . \"%s\" }}\n%s = {{ toJson .%s }}{{ end }}", tfVar.Name, tfVar.Name, tfVar.Name)
}

//func formatValue(tfVar TfVariable) string {
//	if tfVar.Type == nil {
//		// TODO: if no type is specified, we assume it's a string, but that probably isn't rig
//		return fmt.Sprintf(`"{{ .%s }}"`, tfVar.Name)
//	}
//}

func createUsageTemplate(opts *options.BoilerplateOptions, tfConfigModule *tfconfig.Module, tfVarsRequired []TfVariable, tfVarsOptional []TfVariable) ([]ResponsePart, error) {
	// New template dir where we will write all the template files
	tmpTemplateFolder, err := ioutil.TempDir("", "boilerplate-temp-tf-template")
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}
	opts.TemplateFolder = tmpTemplateFolder

	util.Logger.Printf("Using temporary temp folder: %s", opts.TemplateFolder)

	if err := createBoilerplateYaml(opts); err != nil {
		return nil, err
	}

	mainTf, err := createMainTf(opts, tfConfigModule, tfVarsRequired, tfVarsOptional)
	if err != nil {
		return nil, err
	}

	varsTf, err := createVarsTf(opts, tfConfigModule, tfVarsRequired, tfVarsOptional)
	if err != nil {
		return nil, err
	}

	outTf, err := createOutTf(opts, tfConfigModule, tfVarsRequired, tfVarsOptional)
	if err != nil {
		return nil, err
	}

	return []ResponsePart{mainTf, varsTf, outTf}, nil
}

func createBoilerplateYaml(opts *options.BoilerplateOptions) error {
	// Put dummy boilerplate.yml in that folder
	// All it does is run 'terraform fmt' as an after hook
	boilerplateYamlContents := `
# Auto-generated
hooks:
  after:
    - command: terraform
      args:
        - fmt
        - "{{ outputFolder }}"
`
	if err := ioutil.WriteFile(filepath.Join(opts.TemplateFolder, "boilerplate.yml"), []byte(boilerplateYamlContents), 0644); err != nil {
		return errors.WithStackTrace(err)
	}

	return nil
}

func createTfFile(content string, filePath string, opts *options.BoilerplateOptions) (ResponsePart, error) {
	templatePath := filepath.Join(opts.TemplateFolder, filePath)

	parentDir := filepath.Dir(templatePath)
	if err := os.MkdirAll(parentDir, 0777); err != nil {
		return ResponsePart{}, errors.WithStackTrace(err)
	}

	if err := ioutil.WriteFile(templatePath, []byte(content), 0644); err != nil {
		return ResponsePart{}, errors.WithStackTrace(err)
	}

	return ResponsePart{
		Type:                    BoilerplateTemplate,
		BoilerplateTemplatePath: &filePath,
	}, nil
}

func createMainTf(opts *options.BoilerplateOptions, tfConfigModule *tfconfig.Module, tfVarsRequired []TfVariable, tfVarsOptional []TfVariable) (ResponsePart, error) {
	content := formatMainTf(tfConfigModule, tfVarsRequired, tfVarsOptional)
	return createTfFile(content, "main.tf", opts)
}

func createVarsTf(opts *options.BoilerplateOptions, tfConfigModule *tfconfig.Module, tfVarsRequired []TfVariable, tfVarsOptional []TfVariable) (ResponsePart, error) {
	content := formatVarsTf(tfConfigModule, tfVarsRequired, tfVarsOptional)
	return createTfFile(content, "variables.tf", opts)
}

func formatVarsTf(tfConfigModule *tfconfig.Module, tfVarsRequired []TfVariable, tfVarsOptional []TfVariable) string {
	return "# TODO"
}

func createOutTf(opts *options.BoilerplateOptions, tfConfigModule *tfconfig.Module, tfVarsRequired []TfVariable, tfVarsOptional []TfVariable) (ResponsePart, error) {
	content := formatOutTf(tfConfigModule, tfVarsRequired, tfVarsOptional)
	return createTfFile(content, "outputs.tf", opts)
}

func formatOutTf(tfConfigModule *tfconfig.Module, tfVarsRequired []TfVariable, tfVarsOptional []TfVariable) string {
	outputs := []string{}

	for _, outputVar := range tfConfigModule.Outputs {
		output := fmt.Sprintf("output \"%s\" {\n  value = module.%s.%s\n}\n", outputVar.Name, cleanModuleName(tfConfigModule.Path), outputVar.Name)
		outputs = append(outputs, output)
	}

	return strings.Join(outputs, "\n")
}

func strPtr(str string) *string {
	return &str
}

type VarType int

const (
	Required VarType = iota
	Optional
)

func terraformModuleToBoilerplateConfig(tfVars []TfVariable) (*config.BoilerplateConfig, error) {
	vars := []variables.Variable{}

	for _, tfVar := range tfVars {
		boilerplateVar, err := terraformVarToBoilerplateVar(tfVar)
		if err != nil {
			return nil, err
		}
		vars = append(vars, boilerplateVar)
	}

	return &config.BoilerplateConfig{
		Variables: vars,
	}, nil
}

func terraformVarToBoilerplateVar(tfVar TfVariable) (variables.Variable, error) {
	boilerVar, err := convertTerraformVarToBoilerplateVarType(tfVar)
	if err != nil {
		return nil, err
	}

	if tfVar.HasDefault {
		boilerVar = boilerVar.WithDefault(tfVar.Default)
	}

	if tfVar.Description != nil {
		boilerVar = boilerVar.WithDescription(*tfVar.Description)
	}

	return boilerVar, nil
}

func convertTerraformVarToBoilerplateVarType(tfVar TfVariable) (variables.Variable, error) {
	if tfVar.Type == nil {
		// TODO: we assume it's a string var if no type is specified, but we really should look at the default value for type hints
		return variables.NewStringVariable(tfVar.Name), nil
	}

	switch *tfVar.Type {
	case "string":
		return variables.NewStringVariable(tfVar.Name), nil
	case "number":
		return variables.NewFloatVariable(tfVar.Name), nil
	case "bool":
		return variables.NewBoolVariable(tfVar.Name), nil
	case "any":
		// TODO: we assume it's a map if the 'any' type is specified, but we really should look at the default value for type hints
		return variables.NewMapVariable(tfVar.Name), nil
	case "null", "":
		// TODO: we assume it's a string var if no type is specified, but we really should look at the default value for type hints
		return variables.NewStringVariable(tfVar.Name), nil
	}

	// TODO: boilerplate only supports lists of strings for now, and we shove both lists and tuples into it...
	if strings.HasPrefix(*tfVar.Type, "list") || strings.HasPrefix(*tfVar.Type, "tuple") || strings.HasPrefix(*tfVar.Type, "set") {
		return variables.NewListVariable(tfVar.Name), nil
	}

	// TODO: boilerplate only supports maps of strings for now, and we shove both maps and objects into it...
	if strings.HasPrefix(*tfVar.Type, "map") || strings.HasPrefix(*tfVar.Type, "object") {
		return variables.NewMapVariable(tfVar.Name), nil
	}

	return nil, errors.WithStackTrace(fmt.Errorf("Unsupported input variable type '%s' for variable '%s'", *tfVar.Type, tfVar.Name))
}

func handleMarkdownBoilerplate(opts *options.BoilerplateOptions) error {
	responseParts, err := doHandleBoilerplateMdScaffold(opts)
	if err != nil {
		return err
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
	BoilerplateFormOrder      []string
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
	TerraformModule
	TerraformCatalog
	Live
	AppDemo
)

func boilerplateType(opts *options.BoilerplateOptions) (BoilerplateType, error) {
	if isAppDemo(opts) {
		return AppDemo, nil
	}

	yamlPath := filepath.Join(opts.TemplateFolder, "boilerplate.yml")
	if util.PathExists(yamlPath) {
		return Yaml, nil
	}

	mdPath := filepath.Join(opts.TemplateFolder, "boilerplate.md")
	if util.PathExists(mdPath) {
		return Markdown, nil
	}

	terraformFiles, err := zglob.Glob(filepath.Join(opts.TemplateFolder, "**/*.tf"))
	if err != nil {
		return Yaml, errors.WithStackTrace(err)
	}
	if len(terraformFiles) > 0 {
		return terraformType(terraformFiles, opts)
	}

	tgFiles, err := zglob.Glob(filepath.Join(opts.TemplateFolder, "**/terragrunt.hcl"))
	if err != nil {
		return Yaml, errors.WithStackTrace(err)
	}
	if len(tgFiles) > 0 {
		return Live, nil
	}

	return Yaml, fmt.Errorf("%s doesn't seem to be a valid boilerplate folder", opts.TemplateFolder)
}

func isAppDemo(opts *options.BoilerplateOptions) bool {
	livePath := filepath.Join(opts.TemplateFolder, "live")
	modulesPath := filepath.Join(opts.TemplateFolder, "modules")
	scaffoldsPath := filepath.Join(opts.TemplateFolder, "scaffolds")

	return files.IsDir(livePath) && files.IsDir(modulesPath) && files.IsDir(scaffoldsPath)
}

func terraformType(terraformFiles []string, opts *options.BoilerplateOptions) (BoilerplateType, error) {
	for _, tfFile := range terraformFiles {
		relPath, err := files.GetPathRelativeTo(tfFile, opts.TemplateFolder)
		if err != nil {
			return Yaml, errors.WithStackTrace(err)
		}
		// If we find a Terraform file in a subfolder (but not a hidden folder), then we assume this is a catalog of
		// TF modules
		if !strings.HasPrefix(relPath, ".") && strings.ContainsRune(relPath, filepath.Separator) {
			return TerraformCatalog, nil
		}
	}
	return TerraformModule, nil
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
