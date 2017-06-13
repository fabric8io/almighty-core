package account

import (
	"net/http"

	"context"

	"net/url"

	"github.com/fabric8io/almighty-core/account/tenant"
	"github.com/fabric8io/almighty-core/goasupport"
	goaclient "github.com/goadesign/goa/client"
)

type tenantConfig interface {
	GetTenantServiceURL() string
}

// NewInitTenant creates a new tenant service in oso
func NewInitTenant(config tenantConfig) func(context.Context) error {
	return func(ctx context.Context) error {
		return InitTenant(ctx, config)
	}
}

// NewUpdateTenant creates a new tenant service in oso
func NewUpdateTenant(config tenantConfig) func(context.Context) error {
	return func(ctx context.Context) error {
		return UpdateTenant(ctx, config)
	}
}

// InitTenant creates a new tenant service in oso
func InitTenant(ctx context.Context, config tenantConfig) error {

	u, err := url.Parse(config.GetTenantServiceURL())
	if err != nil {
		return err
	}

	c := tenant.New(goaclient.HTTPClientDoer(http.DefaultClient))
	c.Host = u.Host
	c.Scheme = u.Scheme
	c.SetJWTSigner(goasupport.NewForwardSigner(ctx))

	// Ignore response for now
	_, err = c.SetupTenant(ctx, tenant.SetupTenantPath())
	if err != nil {
		return err
	}
	return nil
}

// UpdateTenant creates a new tenant service in oso
func UpdateTenant(ctx context.Context, config tenantConfig) error {

	u, err := url.Parse(config.GetTenantServiceURL())
	if err != nil {
		return err
	}

	c := tenant.New(goaclient.HTTPClientDoer(http.DefaultClient))
	c.Host = u.Host
	c.Scheme = u.Scheme
	c.SetJWTSigner(goasupport.NewForwardSigner(ctx))

	// Ignore response for now
	_, err = c.UpdateTenant(ctx, tenant.SetupTenantPath())
	if err != nil {
		return err
	}
	return nil
}
