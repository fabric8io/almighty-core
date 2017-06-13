package controller_test

import (
	"context"
	"testing"

	"github.com/fabric8io/almighty-core/app/test"
	"github.com/fabric8io/almighty-core/application"
	"github.com/fabric8io/almighty-core/codebase"

	"github.com/fabric8io/almighty-core/account"
	"github.com/fabric8io/almighty-core/controller"
	. "github.com/fabric8io/almighty-core/controller"
	"github.com/fabric8io/almighty-core/gormapplication"
	"github.com/fabric8io/almighty-core/gormsupport/cleaner"
	"github.com/fabric8io/almighty-core/gormtestsupport"
	"github.com/fabric8io/almighty-core/resource"
	"github.com/fabric8io/almighty-core/space"
	testsupport "github.com/fabric8io/almighty-core/test"
	almtoken "github.com/fabric8io/almighty-core/token"
	"github.com/goadesign/goa"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// a normal test function that will kick off TestSuiteCodebases
func TestRunCodebasesTest(t *testing.T) {
	resource.Require(t, resource.Database)
	suite.Run(t, &TestCodebaseREST{DBTestSuite: gormtestsupport.NewDBTestSuite("../config.yaml")})
}

// ========== TestCodebaseREST struct that implements SetupSuite, TearDownSuite, SetupTest, TearDownTest ==========
type TestCodebaseREST struct {
	gormtestsupport.DBTestSuite

	db    *gormapplication.GormDB
	clean func()
}

func (s *TestCodebaseREST) SetupTest() {
	s.db = gormapplication.NewGormDB(s.DB)
	s.clean = cleaner.DeleteCreatedEntities(s.DB)
}

func (s *TestCodebaseREST) TearDownTest() {
	s.clean()
}

func (s *TestCodebaseREST) UnsecuredController() (*goa.Service, *CodebaseController) {
	svc := goa.New("Codebases-service")
	return svc, NewCodebaseController(svc, s.db, s.Configuration)
}

func (s *TestCodebaseREST) SecuredControllers(identity account.Identity) (*goa.Service, *CodebaseController) {
	pub, _ := almtoken.ParsePublicKey([]byte(almtoken.RSAPublicKey))

	svc := testsupport.ServiceAsUser("Codebase-Service", almtoken.NewManager(pub), identity)
	return svc, controller.NewCodebaseController(svc, s.db, s.Configuration)
}

func (s *TestCodebaseREST) TestSuccessShowCodebaseWithoutAuth() {
	t := s.T()
	resource.Require(t, resource.Database)

	cb := requireSpaceAndCodebase(t, s.db)

	svc, ctrl := s.UnsecuredController()
	_, cbresp := test.ShowCodebaseOK(t, svc.Context, svc, ctrl, cb.ID)

	assert.NotNil(t, cbresp)
	assert.Equal(t, cb.ID, *cbresp.Data.ID)
	assert.Equal(t, cb.Type, *cbresp.Data.Attributes.Type)
	assert.Equal(t, cb.URL, *cbresp.Data.Attributes.URL)
	assert.Equal(t, cb.LastUsedWorkspace, *cbresp.Data.Attributes.LastUsedWorkspace)

}

func requireSpaceAndCodebase(t *testing.T, db *gormapplication.GormDB) *codebase.Codebase {
	var c *codebase.Codebase
	application.Transactional(db, func(appl application.Application) error {

		s := &space.Space{
			Name: "Test Space 1" + uuid.NewV4().String(),
		}
		p, err := appl.Spaces().Create(context.Background(), s)
		if err != nil {
			t.Error(err)
		}
		c = &codebase.Codebase{
			SpaceID:           p.ID,
			Type:              "git",
			URL:               "https://github.com/fabric8io/almighty-core.git",
			StackID:           "golang-default",
			LastUsedWorkspace: "my-last-used-workspace",
		}
		err = appl.Codebases().Create(context.Background(), c)
		if err != nil {
			t.Error(err)
		}
		return nil
	})
	return c
}
