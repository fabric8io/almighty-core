package controller_test

import (
	"os"
	"testing"

	"github.com/fabric8io/almighty-core/app/test"
	"github.com/fabric8io/almighty-core/controller"
	"github.com/fabric8io/almighty-core/gormapplication"
	"github.com/fabric8io/almighty-core/gormtestsupport"
	"github.com/fabric8io/almighty-core/resource"
	"github.com/goadesign/goa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestFiltersREST struct {
	// composing with the DBTestSuite to get the Configuration out-of-the-box, even though this particular Controller
	// does not need an access to the DB.
	gormtestsupport.DBTestSuite
	db    *gormapplication.GormDB
	clean func()
}

func TestRunFiltersREST(t *testing.T) {
	resource.Require(t, resource.Database)
	pwd, err := os.Getwd()
	require.Nil(t, err)
	suite.Run(t, &TestFiltersREST{DBTestSuite: gormtestsupport.NewDBTestSuite(pwd + "/../config.yaml")})
}

func (rest *TestFiltersREST) TestListFiltersOK() {
	// given
	svc := goa.New("filterService")
	ctrl := controller.NewFilterController(svc, rest.Configuration)
	// when
	res, filters := test.ListFilterOK(rest.T(), svc.Context, svc, ctrl, nil)
	// then
	assert.Equal(rest.T(), 5, len(filters.Data))
	assertResponseHeaders(rest.T(), res)
}
