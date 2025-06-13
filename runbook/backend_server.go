package runbook

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/templates"
	"github.com/gruntwork-io/boilerplate/variables"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

// TODO: Should this go somewhere else?
type FileData struct {
	Name     string
	Size     int64
	IsDir    bool
	Url      string
	Language string
}

// TODO: This should be configurable
const RenderedContentDir = "/tmp/rendered"

// Start the backend server that handles the AJAX requests from the frontend
func StartBackendServer(opts *options.BoilerplateOptions) error {
	schema, err := getJsonSchema(opts)
	if err != nil {
		return errors.WithStackTraceAndPrefix(err, "Failed to generate JSON schema")
	}

	e := setupBackendServer()
	setupRoutes(e, schema, opts)

	if err := e.Start(":8080"); err != nil {
		return errors.WithStackTraceAndPrefix(err, "Failed to start backend server")
	}

	return nil
}

// Configure the backend server
func setupBackendServer() *echo.Echo {
	e := echo.New()

	// Our react app is running on a different port, so we need to allow requests from it by adding CORS rules
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000"},
	}))

	// Serve static files
	e.Static("/rendered", RenderedContentDir)

	return e
}

// Setup the routes for the backend server
func setupRoutes(e *echo.Echo, schema interface{}, opts *options.BoilerplateOptions) {
	e.GET("/form", func(c echo.Context) error {
		return c.JSON(200, schema)
	})
	e.GET("/render", func(c echo.Context) error {

		// The root boilerplate.yml is not itself a dependency, so we pass an empty Dependency.Add commentMore actions
		emptyDep := variables.Dependency{}

		// Render
		err := templates.ProcessTemplate(opts, opts, emptyDep)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}

		dirContents, err := GetDirContents(opts.OutputFolder)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}

		return c.JSON(200, map[string]any{"files": dirContents})
	})
}

func getJsonSchema(opts *options.BoilerplateOptions) (any, error) {
	// TODO: Implement actual schema generation based on template variables
	schema := map[string]any{
		"hello": "world",
	}
	return schema, nil
}

// really need to render a full file selector in the browserAdd commentMore actions
func GetDirContents(path string) ([]FileData, error) {
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(absPath, RenderedContentDir) {
		return nil, fmt.Errorf("Cannot display contents outside of %s", RenderedContentDir)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	out := []FileData{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip entries we can't get info for
		}
		out = append(out, fileInfoToFileData(info))
	}

	return out, nil
}

// Convert stdlib fs.FileInfo to our FileData struct
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
