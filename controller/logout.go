package controller

import (
	"github.com/fabric8io/almighty-core/app"
	"github.com/fabric8io/almighty-core/errors"
	"github.com/fabric8io/almighty-core/jsonapi"
	"github.com/fabric8io/almighty-core/log"
	"github.com/fabric8io/almighty-core/login"
	"github.com/goadesign/goa"
	errs "github.com/pkg/errors"
)

type logoutConfiguration interface {
	GetKeycloakEndpointLogout(*goa.RequestData) (string, error)
	GetValidRedirectURLs(*goa.RequestData) (string, error)
}

// LogoutController implements the logout resource.
type LogoutController struct {
	*goa.Controller
	logoutService login.LogoutService
	configuration logoutConfiguration
}

// NewLogoutController creates a logout controller.
func NewLogoutController(service *goa.Service, logoutService *login.KeycloakLogoutService, configuration logoutConfiguration) *LogoutController {
	return &LogoutController{Controller: service.NewController("LogoutController"), logoutService: logoutService, configuration: configuration}
}

// Logout runs the logout action.
func (c *LogoutController) Logout(ctx *app.LogoutLogoutContext) error {
	logoutEndpoint, err := c.configuration.GetKeycloakEndpointLogout(ctx.RequestData)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "Unable to get Keycloak logout endpoint URL")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(errs.Wrap(err, "unable to get Keycloak logout endpoint URL")))
	}
	whitelist, err := c.configuration.GetValidRedirectURLs(ctx.RequestData)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(err))
	}

	ctx.ResponseData.Header().Set("Cache-Control", "no-cache")
	return c.logoutService.Logout(ctx, logoutEndpoint, whitelist)
}
