package link_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/fabric8io/almighty-core/account"
	"github.com/fabric8io/almighty-core/gormsupport/cleaner"
	"github.com/fabric8io/almighty-core/gormtestsupport"
	"github.com/fabric8io/almighty-core/migration"
	"github.com/fabric8io/almighty-core/resource"
	"github.com/fabric8io/almighty-core/space"
	testsupport "github.com/fabric8io/almighty-core/test"
	"github.com/fabric8io/almighty-core/workitem"
	"github.com/fabric8io/almighty-core/workitem/link"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type linkRepoBlackBoxTest struct {
	gormtestsupport.DBTestSuite
	repo               link.WorkItemLinkRepository
	clean              func()
	ctx                context.Context
	testSpace          uuid.UUID
	testIdentity       account.Identity
	testTreeLinkTypeID uuid.UUID
}

// SetupSuite overrides the DBTestSuite's function but calls it before doing anything else
// The SetupSuite method will run before the tests in the suite are run.
// It sets up a database connection for all the tests in this suite without polluting global space.
func (s *linkRepoBlackBoxTest) SetupSuite() {
	s.DBTestSuite.SetupSuite()
	s.ctx = migration.NewMigrationContext(context.Background())
	s.DBTestSuite.PopulateDBTestSuite(s.ctx)
}

func TestRunLinkRepoBlackBoxTest(t *testing.T) {
	resource.Require(t, resource.Database)
	suite.Run(t, &linkRepoBlackBoxTest{DBTestSuite: gormtestsupport.NewDBTestSuite("../../config.yaml")})
}

func (s *linkRepoBlackBoxTest) SetupTest() {
	s.repo = link.NewWorkItemLinkRepository(s.DB)
	s.clean = cleaner.DeleteCreatedEntities(s.DB)
	testIdentity, err := testsupport.CreateTestIdentity(s.DB, "jdoe1", "test")
	s.testIdentity = testIdentity
	require.Nil(s.T(), err)

	// create a space
	spaceRepository := space.NewRepository(s.DB)
	spaceName := testsupport.CreateRandomValidTestName("test-space")
	testSpace, err := spaceRepository.Create(s.ctx, &space.Space{
		Name: spaceName,
	})
	s.testSpace = testSpace.ID
	require.Nil(s.T(), err)
}

func (s *linkRepoBlackBoxTest) TearDownTest() {
	s.clean()
}

// This creates a parent-child link between two workitems -> Parent1 and Child. It tests that when there is an attempt to create another parent (Parent2) of child, it should throw an error.
func (s *linkRepoBlackBoxTest) TestDisallowMultipleParents() {
	// create 3 workitems for linking
	workitemRepository := workitem.NewWorkItemRepository(s.DB)
	Parent1, err := workitemRepository.Create(
		s.ctx, s.testSpace, workitem.SystemBug,
		map[string]interface{}{
			workitem.SystemTitle: "Parent 1",
			workitem.SystemState: workitem.SystemStateNew,
		}, s.testIdentity.ID)
	require.Nil(s.T(), err)
	Parent1ID, err := strconv.ParseUint(Parent1.ID, 10, 64)

	Parent2, err := workitemRepository.Create(
		s.ctx, s.testSpace, workitem.SystemBug,
		map[string]interface{}{
			workitem.SystemTitle: "Parent 2",
			workitem.SystemState: workitem.SystemStateNew,
		}, s.testIdentity.ID)
	require.Nil(s.T(), err)
	Parent2ID, err := strconv.ParseUint(Parent2.ID, 10, 64)

	Child, err := workitemRepository.Create(
		s.ctx, s.testSpace, workitem.SystemBug,
		map[string]interface{}{
			workitem.SystemTitle: "Child",
			workitem.SystemState: workitem.SystemStateNew,
		}, s.testIdentity.ID)
	require.Nil(s.T(), err)
	ChildID, err := strconv.ParseUint(Child.ID, 10, 64)
	require.Nil(s.T(), err)

	// Create a work item link category
	linkCategoryRepository := link.NewWorkItemLinkCategoryRepository(s.DB)
	categoryName := "test" + uuid.NewV4().String()
	categoryDescription := "Test Link Category"
	linkCategoryModel1 := link.WorkItemLinkCategory{
		Name:        categoryName,
		Description: &categoryDescription,
	}
	linkCategory, err := linkCategoryRepository.Create(s.ctx, &linkCategoryModel1)
	require.Nil(s.T(), err)

	// create tree topology link type
	linkTypeRepository := link.NewWorkItemLinkTypeRepository(s.DB)
	linkTypeModel1 := link.WorkItemLinkType{
		Name:           "TestTreeLinkType",
		SourceTypeID:   workitem.SystemBug,
		TargetTypeID:   workitem.SystemBug,
		ForwardName:    "foo",
		ReverseName:    "foo",
		Topology:       "tree",
		LinkCategoryID: linkCategory.ID,
		SpaceID:        s.testSpace,
	}
	TestTreeLinkType, err := linkTypeRepository.Create(s.ctx, &linkTypeModel1)
	require.Nil(s.T(), err)
	s.testTreeLinkTypeID = TestTreeLinkType.ID

	// create a work item link
	linkRepository := link.NewWorkItemLinkRepository(s.DB)
	_, err = linkRepository.Create(s.ctx, Parent1ID, ChildID, s.testTreeLinkTypeID, s.testIdentity.ID)
	require.Nil(s.T(), err)

	_, err = linkRepository.Create(s.ctx, Parent2ID, ChildID, s.testTreeLinkTypeID, s.testIdentity.ID)
	require.NotNil(s.T(), err)
}

// TestCountChildWorkitems tests total number of workitem children returned by list is equal to the total number of workitem children created
// and total number of workitem children in a page are equal to the "limit" specified
func (s *linkRepoBlackBoxTest) TestCountChildWorkitems() {
	workitemRepository := workitem.NewWorkItemRepository(s.DB)

	// create a parent workitem
	parent, err := workitemRepository.Create(
		s.ctx, s.testSpace, workitem.SystemBug,
		map[string]interface{}{
			workitem.SystemTitle: "Parent",
			workitem.SystemState: workitem.SystemStateNew,
		}, s.testIdentity.ID)
	require.Nil(s.T(), err)
	parentID, err := strconv.ParseUint(parent.ID, 10, 64)

	// create 3 workitems for linking as children to parent workitem
	child1, err := workitemRepository.Create(
		s.ctx, s.testSpace, workitem.SystemBug,
		map[string]interface{}{
			workitem.SystemTitle: "Child 1",
			workitem.SystemState: workitem.SystemStateNew,
		}, s.testIdentity.ID)
	require.Nil(s.T(), err)
	child1ID, err := strconv.ParseUint(child1.ID, 10, 64)

	child2, err := workitemRepository.Create(
		s.ctx, s.testSpace, workitem.SystemBug,
		map[string]interface{}{
			workitem.SystemTitle: "Child 2",
			workitem.SystemState: workitem.SystemStateNew,
		}, s.testIdentity.ID)
	require.Nil(s.T(), err)
	child2ID, err := strconv.ParseUint(child2.ID, 10, 64)
	require.Nil(s.T(), err)

	child3, err := workitemRepository.Create(
		s.ctx, s.testSpace, workitem.SystemBug,
		map[string]interface{}{
			workitem.SystemTitle: "Child 3",
			workitem.SystemState: workitem.SystemStateNew,
		}, s.testIdentity.ID)
	require.Nil(s.T(), err)
	child3ID, err := strconv.ParseUint(child3.ID, 10, 64)
	require.Nil(s.T(), err)

	// Create a work item link category
	linkCategoryRepository := link.NewWorkItemLinkCategoryRepository(s.DB)
	categoryName := "test" + uuid.NewV4().String()
	categoryDescription := "Test Link Category"
	linkCategoryModel1 := link.WorkItemLinkCategory{
		Name:        categoryName,
		Description: &categoryDescription,
	}
	linkCategory, err := linkCategoryRepository.Create(s.ctx, &linkCategoryModel1)
	require.Nil(s.T(), err)

	// create tree topology link type
	linkTypeRepository := link.NewWorkItemLinkTypeRepository(s.DB)
	linkTypeModel1 := link.WorkItemLinkType{
		Name:           "Parent child item",
		SourceTypeID:   workitem.SystemBug,
		TargetTypeID:   workitem.SystemBug,
		ForwardName:    "parent of",
		ReverseName:    "child of",
		Topology:       "tree",
		LinkCategoryID: linkCategory.ID,
		SpaceID:        s.testSpace,
	}
	TestTreeLinkType, err := linkTypeRepository.Create(s.ctx, &linkTypeModel1)
	require.Nil(s.T(), err)
	s.testTreeLinkTypeID = TestTreeLinkType.ID

	// link the children workitems to parent
	linkRepository := link.NewWorkItemLinkRepository(s.DB)
	_, err = linkRepository.Create(s.ctx, parentID, child1ID, s.testTreeLinkTypeID, s.testIdentity.ID)
	require.Nil(s.T(), err)

	_, err = linkRepository.Create(s.ctx, parentID, child2ID, s.testTreeLinkTypeID, s.testIdentity.ID)
	require.Nil(s.T(), err)

	_, err = linkRepository.Create(s.ctx, parentID, child3ID, s.testTreeLinkTypeID, s.testIdentity.ID)
	require.Nil(s.T(), err)

	offset := 0
	limit := 1
	res, count, err := linkRepository.ListWorkItemChildren(s.ctx, parent.ID, &offset, &limit)
	require.Nil(s.T(), err)
	require.Len(s.T(), res, 1)
	require.Equal(s.T(), 3, int(count))
}
