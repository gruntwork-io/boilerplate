package cli

import (
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/templates"
	"github.com/gruntwork-io/boilerplate/variables"
	"github.com/gruntwork-io/go-commons/entrypoint"
	"github.com/gruntwork-io/go-commons/version"
	"github.com/pkg/browser"
	"github.com/urfave/cli"
	"io/fs"
	"io/ioutil"
	"net/http"
	"path/filepath"
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

const RenderedContentDir = "/tmp/rendered"

// When you run the CLI, this is the action function that gets called
func runApp(cliContext *cli.Context) error {
	if !cliContext.Args().Present() && cliContext.NumFlags() == 0 {
		return cli.ShowAppHelp(cliContext)
	}

	opts, err := options.ParseOptions(cliContext)
	if err != nil {
		return err
	}

	schema, err := templates.GetJsonSchema(opts)
	if err != nil {
		return err
	}

	router := gin.Default()

	// create-react-app runs on a different port, so to allow it to make AJAX calls here, add CORS rules
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"http://localhost:3000"}
	router.Use(cors.New(corsConfig))

	router.Static("/rendered", RenderedContentDir)

	router.GET("/form", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, schema)
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

		dirContents, err := GetDirContents(opts.OutputFolder)
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

	//return templates.ProcessTemplate(opts, opts, emptyDep)
}

// TODO: this function only returns the top-level files/folders, but not the ones nested within, which is what we'd
// really need to render a full file selector in the browser
func GetDirContents(path string) ([]FileData, error) {
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(absPath, RenderedContentDir) {
		return nil, fmt.Errorf("Cannot display contents outside of %s", RenderedContentDir)
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	out := []FileData{}
	for _, file := range files {
		out = append(out, fileInfoToFileData(file))
	}

	return out, nil
}

func fileInfoToFileData(file fs.FileInfo) FileData {
	return FileData{
		Name:     file.Name(),
		IsDir:    file.IsDir(),
		Size:     file.Size(),
		Url:      fmt.Sprintf("http://localhost:8080/rendered/%s", file.Name()),
		Language: languageForFile(file),
	}
}

// https://prismjs.com/#supported-languages
func languageForFile(file fs.FileInfo) string {
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
