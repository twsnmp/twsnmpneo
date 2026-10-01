package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/diagnose"
)

func registerNodeDiagnoseEndpoints(apiGroup *echo.Group, store datastore.DataStore) {
	apiGroup.POST("/nodes/:id/diagnose", func(c echo.Context) error {
		id := c.Param("id")
		node, err := store.GetNode(c.Request().Context(), id)
		if err != nil || node == nil {
			return c.JSON(http.StatusNotFound, map[string]any{
				"error": "node not found",
			})
		}
		result := diagnose.DiagnoseNode(c.Request().Context(), node)
		return c.JSON(http.StatusOK, result)
	})
}
