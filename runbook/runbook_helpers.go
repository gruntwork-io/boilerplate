package runbook

import (
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/pkg/browser"
)

func StartRunbookServer(opts *options.BoilerplateOptions) error {
	schema, err := getJsonSchema(opts)
	if err != nil {
		return errors.WithStackTraceAndPrefix(err, "Failed to generate JSON schema")
	}

	e := setupWebServer()
	setupRoutes(e, schema, opts)

	// Try to open browser, but don't fail if it doesn't work
	if err := browser.OpenURL("http://localhost:3000"); err != nil {
		return errors.WithStackTraceAndPrefix(err, "Failed to open browser")
	}

	if err := e.Start(":3000"); err != nil {
		return errors.WithStackTraceAndPrefix(err, "Failed to start backend server")
	}

	return nil
}

func getJsonSchema(opts *options.BoilerplateOptions) (any, error) {
	// TODO: Implement actual schema generation based on template variables
	schema := map[string]any{
		"hello": "world",
	}
	return schema, nil
}

func setupWebServer() *echo.Echo {
	e := echo.New()

	// Add CORS middleware
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000"},
	}))

	// Serve static files
	//e.Static("/rendered", RenderedContentDir)

	return e
}

func setupRoutes(e *echo.Echo, schema interface{}, opts *options.BoilerplateOptions) {
	e.GET("/form", func(c echo.Context) error {
		return c.JSON(200, schema)
	})
	//e.POST("/render", postRenderHandler(opts))
}
