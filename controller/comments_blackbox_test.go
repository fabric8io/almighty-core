package controller_test

import (
	"fmt"
	"html"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fabric8io/almighty-core/account"
	"github.com/fabric8io/almighty-core/app"
	"github.com/fabric8io/almighty-core/app/test"
	"github.com/fabric8io/almighty-core/auth"
	"github.com/fabric8io/almighty-core/comment"
	. "github.com/fabric8io/almighty-core/controller"
	"github.com/fabric8io/almighty-core/gormapplication"
	"github.com/fabric8io/almighty-core/gormsupport"
	"github.com/fabric8io/almighty-core/gormsupport/cleaner"
	"github.com/fabric8io/almighty-core/gormtestsupport"
	"github.com/fabric8io/almighty-core/rendering"
	"github.com/fabric8io/almighty-core/resource"
	"github.com/fabric8io/almighty-core/rest"
	"github.com/fabric8io/almighty-core/space"
	testsupport "github.com/fabric8io/almighty-core/test"
	almtoken "github.com/fabric8io/almighty-core/token"
	"github.com/fabric8io/almighty-core/workitem"
	uuid "github.com/satori/go.uuid"

	"github.com/goadesign/goa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// a normal test function that will kick off TestSuiteComments
func TestSuiteComments(t *testing.T) {
	resource.Require(t, resource.Database)
	suite.Run(t, &CommentsSuite{DBTestSuite: gormtestsupport.NewDBTestSuite("../config.yaml")})
}

// ========== TestSuiteComments struct that implements SetupSuite, TearDownSuite, SetupTest, TearDownTest ==========
type CommentsSuite struct {
	gormtestsupport.DBTestSuite
	db            *gormapplication.GormDB
	clean         func()
	testIdentity  account.Identity
	testIdentity2 account.Identity
}

func (s *CommentsSuite) SetupTest() {
	s.db = gormapplication.NewGormDB(s.DB)
	s.clean = cleaner.DeleteCreatedEntities(s.DB)
	testIdentity, err := testsupport.CreateTestIdentity(s.DB, "CommentsSuite user", "test provider")
	require.Nil(s.T(), err)
	s.testIdentity = testIdentity
	testIdentity2, err := testsupport.CreateTestIdentity(s.DB, "CommentsSuite user2", "test provider")
	require.Nil(s.T(), err)
	s.testIdentity2 = testIdentity2
}

func (s *CommentsSuite) TearDownTest() {
	s.clean()
}

var (
	markdownMarkup  = rendering.SystemMarkupMarkdown
	plaintextMarkup = rendering.SystemMarkupPlainText
	defaultMarkup   = rendering.SystemMarkupDefault
)

func (s *CommentsSuite) unsecuredController() (*goa.Service, *CommentsController) {
	svc := goa.New("Comments-service-test")
	commentsCtrl := NewCommentsController(svc, s.db, s.Configuration)
	return svc, commentsCtrl
}

func (s *CommentsSuite) securedControllers(identity account.Identity) (*goa.Service, *WorkitemController, *WorkItemCommentsController, *CommentsController) {
	priv, _ := almtoken.ParsePrivateKey([]byte(almtoken.RSAPrivateKey))
	svc := testsupport.ServiceAsUser("Comment-Service", almtoken.NewManagerWithPrivateKey(priv), identity)
	workitemCtrl := NewWorkitemController(svc, s.db, s.Configuration)
	workitemCommentsCtrl := NewWorkItemCommentsController(svc, s.db, s.Configuration)
	commentsCtrl := NewCommentsController(svc, s.db, s.Configuration)
	return svc, workitemCtrl, workitemCommentsCtrl, commentsCtrl
}

// createWorkItem creates a workitem that will be used to perform the comment operations during the tests.
func (s *CommentsSuite) createWorkItem(identity account.Identity) string {
	spaceSelfURL := rest.AbsoluteURL(&goa.RequestData{
		Request: &http.Request{Host: "api.service.domain.org"},
	}, app.SpaceHref(space.SystemSpace.String()))
	witSelfURL := rest.AbsoluteURL(&goa.RequestData{
		Request: &http.Request{Host: "api.service.domain.org"},
	}, app.WorkitemtypeHref(space.SystemSpace.String(), workitem.SystemBug.String()))
	createWorkitemPayload := app.CreateWorkitemPayload{
		Data: &app.WorkItem{
			Type: APIStringTypeWorkItem,
			Attributes: map[string]interface{}{
				workitem.SystemTitle: "work item title",
				workitem.SystemState: workitem.SystemStateNew},
			Relationships: &app.WorkItemRelationships{
				BaseType: &app.RelationBaseType{
					Data: &app.BaseTypeData{
						Type: "workitemtypes",
						ID:   workitem.SystemBug,
					},
					Links: &app.GenericLinks{
						Self: &witSelfURL,
					},
				},
				Space: app.NewSpaceRelation(space.SystemSpace, spaceSelfURL),
			},
		},
	}
	userSvc, workitemCtrl, _, _ := s.securedControllers(identity)
	_, wi := test.CreateWorkitemCreated(s.T(), userSvc.Context, userSvc, workitemCtrl, *createWorkitemPayload.Data.Relationships.Space.Data.ID, &createWorkitemPayload)
	wID := *wi.Data.ID
	s.T().Log(fmt.Sprintf("Created workitem with id %v", wID))
	return wID
}

func (s *CommentsSuite) newCreateWorkItemCommentsPayload(body string, markup *string) *app.CreateWorkItemCommentsPayload {
	return &app.CreateWorkItemCommentsPayload{
		Data: &app.CreateComment{
			Type: "comments",
			Attributes: &app.CreateCommentAttributes{
				Body:   body,
				Markup: markup,
			},
		},
	}
}

func (s *CommentsSuite) newUpdateCommentsPayload(body string, markup *string) *app.UpdateCommentsPayload {
	return &app.UpdateCommentsPayload{
		Data: &app.Comment{
			Type: "comments",
			Attributes: &app.CommentAttributes{
				Body:   &body,
				Markup: markup,
			},
		},
	}
}

// createWorkItemComment creates a workitem comment that will be used to perform the comment operations during the tests.
func (s *CommentsSuite) createWorkItemComment(identity account.Identity, wID string, body string, markup *string) app.CommentSingle {
	createWorkItemCommentPayload := s.newCreateWorkItemCommentsPayload(body, markup)
	userSvc, _, workitemCommentsCtrl, _ := s.securedControllers(identity)
	_, c := test.CreateWorkItemCommentsOK(s.T(), userSvc.Context, userSvc, workitemCommentsCtrl, space.SystemSpace, wID, createWorkItemCommentPayload)
	require.NotNil(s.T(), c)
	s.T().Log(fmt.Sprintf("Created comment with id %v", *c.Data.ID))
	return *c
}

func assertComment(t *testing.T, resultData *app.Comment, expectedIdentity account.Identity, expectedBody string, expectedMarkup string) {
	require.NotNil(t, resultData)
	assert.NotNil(t, resultData.ID)
	assert.NotNil(t, resultData.Type)
	require.NotNil(t, resultData.Attributes)
	require.NotNil(t, resultData.Attributes.CreatedAt)
	require.NotNil(t, resultData.Attributes.UpdatedAt)
	require.NotNil(t, resultData.Attributes.Body)
	require.NotNil(t, resultData.Attributes.Body)
	assert.Equal(t, expectedBody, *resultData.Attributes.Body)
	require.NotNil(t, resultData.Attributes.Markup)
	assert.Equal(t, expectedMarkup, *resultData.Attributes.Markup)
	assert.Equal(t, rendering.RenderMarkupToHTML(html.EscapeString(expectedBody), expectedMarkup), *resultData.Attributes.BodyRendered)
	require.NotNil(t, resultData.Relationships)
	require.NotNil(t, resultData.Relationships.CreatedBy)
	require.NotNil(t, resultData.Relationships.CreatedBy.Data)
	require.NotNil(t, resultData.Relationships.CreatedBy.Data.ID)
	assert.Equal(t, expectedIdentity.ID, *resultData.Relationships.CreatedBy.Data.ID)
	assert.True(t, strings.Contains(*resultData.Relationships.CreatedBy.Links.Related, resultData.Relationships.CreatedBy.Data.ID.String()), "Link not found")
}

func convertCommentToModel(c app.CommentSingle) comment.Comment {
	return comment.Comment{
		ID: *c.Data.ID,
		Lifecycle: gormsupport.Lifecycle{
			UpdatedAt: *c.Data.Attributes.UpdatedAt,
		},
	}
}

func (s *CommentsSuite) TestShowCommentWithoutAuthOK() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", &markdownMarkup)
	// when
	userSvc, commentsCtrl := s.unsecuredController()
	_, result := test.ShowCommentsOK(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID, nil, nil)
	// then
	assertComment(s.T(), result.Data, s.testIdentity, "body", rendering.SystemMarkupMarkdown)
}

func (s *CommentsSuite) TestShowCommentWithoutAuthOKUsingExpiredIfModifiedSinceHeader() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", &markdownMarkup)
	// when
	ifModifiedSince := app.ToHTTPTime(c.Data.Attributes.UpdatedAt.Add(-1 * time.Hour))
	userSvc, commentsCtrl := s.unsecuredController()
	res, result := test.ShowCommentsOK(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID, &ifModifiedSince, nil)
	// then
	assertComment(s.T(), result.Data, s.testIdentity, "body", rendering.SystemMarkupMarkdown)
	assertResponseHeaders(s.T(), res)
}

func (s *CommentsSuite) TestShowCommentWithoutAuthOKUsingExpiredIfNoneMatchHeader() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", &markdownMarkup)
	// when
	ifNoneMatch := "foo"
	userSvc, commentsCtrl := s.unsecuredController()
	res, result := test.ShowCommentsOK(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID, nil, &ifNoneMatch)
	// then
	assertComment(s.T(), result.Data, s.testIdentity, "body", rendering.SystemMarkupMarkdown)
	assertResponseHeaders(s.T(), res)
}

func (s *CommentsSuite) TestShowCommentWithoutAuthNotModifiedUsingIfModifiedSinceHeader() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", &markdownMarkup)
	// when
	ifModifiedSince := app.ToHTTPTime(*c.Data.Attributes.UpdatedAt)
	userSvc, commentsCtrl := s.unsecuredController()
	res := test.ShowCommentsNotModified(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID, &ifModifiedSince, nil)
	// then
	assertResponseHeaders(s.T(), res)
}

func (s *CommentsSuite) TestShowCommentWithoutAuthNotModifiedUsingIfNoneMatchHeader() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", &markdownMarkup)
	// when
	commentModel := convertCommentToModel(c)
	ifNoneMatch := app.GenerateEntityTag(commentModel)
	userSvc, commentsCtrl := s.unsecuredController()
	res := test.ShowCommentsNotModified(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID, nil, &ifNoneMatch)
	// then
	assertResponseHeaders(s.T(), res)
}

func (s *CommentsSuite) TestShowCommentWithoutAuthWithMarkup() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", nil)
	// when
	userSvc, commentsCtrl := s.unsecuredController()
	_, result := test.ShowCommentsOK(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID, nil, nil)
	// then
	assertComment(s.T(), result.Data, s.testIdentity, "body", rendering.SystemMarkupPlainText)
}

func (s *CommentsSuite) TestShowCommentWithAuth() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", &plaintextMarkup)
	// when
	userSvc, _, _, commentsCtrl := s.securedControllers(s.testIdentity)
	_, result := test.ShowCommentsOK(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID, nil, nil)
	// then
	assertComment(s.T(), result.Data, s.testIdentity, "body", rendering.SystemMarkupPlainText)
}

func (s *CommentsSuite) TestShowCommentWithEscapedScriptInjection() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "<img src=x onerror=alert('body') />", &plaintextMarkup)
	// when
	userSvc, _, _, commentsCtrl := s.securedControllers(s.testIdentity)
	_, result := test.ShowCommentsOK(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID, nil, nil)
	// then
	assertComment(s.T(), result.Data, s.testIdentity, "<img src=x onerror=alert('body') />", rendering.SystemMarkupPlainText)
}

func (s *CommentsSuite) TestUpdateCommentWithoutAuth() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", &plaintextMarkup)
	// when
	updateCommentPayload := s.newUpdateCommentsPayload("updated body", &markdownMarkup)
	userSvc, commentsCtrl := s.unsecuredController()
	test.UpdateCommentsUnauthorized(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID, updateCommentPayload)
}

func (s *CommentsSuite) TestUpdateCommentWithSameUserWithOtherMarkup() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", &plaintextMarkup)
	// when
	updateCommentPayload := s.newUpdateCommentsPayload("updated body", &markdownMarkup)
	userSvc, _, _, commentsCtrl := s.securedControllers(s.testIdentity)
	_, result := test.UpdateCommentsOK(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID, updateCommentPayload)
	assertComment(s.T(), result.Data, s.testIdentity, "updated body", rendering.SystemMarkupMarkdown)
}

func (s *CommentsSuite) TestUpdateCommentWithSameUserWithNilMarkup() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", &plaintextMarkup)
	// when
	updateCommentPayload := s.newUpdateCommentsPayload("updated body", nil)
	userSvc, _, _, commentsCtrl := s.securedControllers(s.testIdentity)
	_, result := test.UpdateCommentsOK(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID, updateCommentPayload)
	assertComment(s.T(), result.Data, s.testIdentity, "updated body", rendering.SystemMarkupDefault)
}

func (s *CommentsSuite) TestUpdateCommentWithOtherUser() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", &plaintextMarkup)
	// when
	updatedCommentBody := "An updated comment"
	updateCommentPayload := &app.UpdateCommentsPayload{
		Data: &app.Comment{
			Type: "comments",
			Attributes: &app.CommentAttributes{
				Body: &updatedCommentBody,
			},
		},
	}
	// when/then
	userSvc, _, _, commentsCtrl := s.securedControllers(s.testIdentity2)
	test.UpdateCommentsForbidden(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID, updateCommentPayload)
}

func (s *CommentsSuite) TestDeleteCommentWithSameAuthenticatedUser() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", &plaintextMarkup)
	userSvc, _, _, commentsCtrl := s.securedControllers(s.testIdentity)
	test.DeleteCommentsOK(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID)
}

func (s *CommentsSuite) TestDeleteCommentWithoutAuth() {
	// given
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", &plaintextMarkup)
	userSvc, commentsCtrl := s.unsecuredController()
	test.DeleteCommentsUnauthorized(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID)
}

// Following test creates a space and space_owner creates a WI in that space
// Space owner adds a comment on the created WI
// Create another user, which is not space collaborator.
// Test if another user can delete the comment
func (s *CommentsSuite) TestNonCollaboraterCanNotDelete() {
	// create space
	// create user
	// add user to the space collaborator list
	// create workitem in created space
	// create another user - do not add this user into collaborator list
	testIdentity, err := testsupport.CreateTestIdentity(s.DB, testsupport.CreateRandomValidTestName("TestNonCollaboraterCanNotDelete-"), "TestWIComments")
	require.Nil(s.T(), err)
	space := CreateSecuredSpace(s.T(), gormapplication.NewGormDB(s.DB), s.Configuration, testIdentity)

	payload := minimumRequiredCreateWithTypeAndSpace(workitem.SystemFeature, *space.ID)
	payload.Data.Attributes[workitem.SystemTitle] = "Test WI"
	payload.Data.Attributes[workitem.SystemState] = workitem.SystemStateNew

	priv, _ := almtoken.ParsePrivateKey([]byte(almtoken.RSAPrivateKey))
	svc := testsupport.ServiceAsSpaceUser("Collaborators-Service", almtoken.NewManagerWithPrivateKey(priv), testIdentity, &TestSpaceAuthzService{testIdentity})
	ctrl := NewWorkitemController(svc, gormapplication.NewGormDB(s.DB), s.Configuration)

	_, wi := test.CreateWorkitemCreated(s.T(), svc.Context, svc, ctrl, *payload.Data.Relationships.Space.Data.ID, &payload)
	c := s.createWorkItemComment(testIdentity, *wi.Data.ID, "body", &plaintextMarkup)

	testIdentity2, err := testsupport.CreateTestIdentity(s.DB, testsupport.CreateRandomValidTestName("TestNonCollaboraterCanNotDelete-"), "TestWI")
	svcNotAuthrized := testsupport.ServiceAsSpaceUser("Collaborators-Service", almtoken.NewManagerWithPrivateKey(priv), testIdentity2, &TestSpaceAuthzService{testIdentity})
	ctrlNotAuthrize := NewCommentsController(svcNotAuthrized, gormapplication.NewGormDB(s.DB), s.Configuration)

	test.DeleteCommentsForbidden(s.T(), svcNotAuthrized.Context, svcNotAuthrized, ctrlNotAuthrize, *c.Data.ID)
}

func (s *CommentsSuite) TestCollaboratorCanDelete() {
	testIdentity, err := testsupport.CreateTestIdentity(s.DB, testsupport.CreateRandomValidTestName("TestCollaboratorCanDelete-"), "TestWIComments")
	require.Nil(s.T(), err)
	space := CreateSecuredSpace(s.T(), gormapplication.NewGormDB(s.DB), s.Configuration, testIdentity)

	payload := minimumRequiredCreateWithTypeAndSpace(workitem.SystemFeature, *space.ID)
	payload.Data.Attributes[workitem.SystemTitle] = "Test WI"
	payload.Data.Attributes[workitem.SystemState] = workitem.SystemStateNew

	priv, _ := almtoken.ParsePrivateKey([]byte(almtoken.RSAPrivateKey))
	svc := testsupport.ServiceAsSpaceUser("Collaborators-Service", almtoken.NewManagerWithPrivateKey(priv), testIdentity, &TestSpaceAuthzService{testIdentity})
	ctrl := NewWorkitemController(svc, gormapplication.NewGormDB(s.DB), s.Configuration)

	_, wi := test.CreateWorkitemCreated(s.T(), svc.Context, svc, ctrl, *payload.Data.Relationships.Space.Data.ID, &payload)
	c := s.createWorkItemComment(testIdentity, *wi.Data.ID, "body", &plaintextMarkup)
	commentCtrl := NewCommentsController(svc, gormapplication.NewGormDB(s.DB), s.Configuration)
	test.DeleteCommentsOK(s.T(), svc.Context, svc, commentCtrl, *c.Data.ID)
}

func (s *CommentsSuite) TestCreatorCanDelete() {
	wID := s.createWorkItem(s.testIdentity)
	c := s.createWorkItemComment(s.testIdentity, wID, "body", &plaintextMarkup)
	userSvc, _, _, commentsCtrl := s.securedControllers(s.testIdentity)
	test.DeleteCommentsOK(s.T(), userSvc.Context, userSvc, commentsCtrl, *c.Data.ID)
}

func (s *CommentsSuite) TestOtherCollaboratorCanDelete() {
	// create space owner identity
	spaceOwner, err := testsupport.CreateTestIdentity(s.DB, testsupport.CreateRandomValidTestName("TestOtherCollaboratorCanDelete-"), "TestWIComments")
	require.Nil(s.T(), err)

	// create 2 space collaborators' identity
	collaborator1, err := testsupport.CreateTestIdentity(s.DB, testsupport.CreateRandomValidTestName("TestOtherCollaboratorCanDelete-"), "TestWIComments")
	require.Nil(s.T(), err)

	collaborator2, err := testsupport.CreateTestIdentity(s.DB, testsupport.CreateRandomValidTestName("TestOtherCollaboratorCanDelete-"), "TestWIComments")
	require.Nil(s.T(), err)

	// Add 2 identities as Collaborators
	space := CreateSecuredSpace(s.T(), gormapplication.NewGormDB(s.DB), s.Configuration, spaceOwner)
	priv, _ := almtoken.ParsePrivateKey([]byte(almtoken.RSAPrivateKey))
	svcWithSpaceOwner := testsupport.ServiceAsSpaceUser("Comments-Service", almtoken.NewManagerWithPrivateKey(priv), spaceOwner, &TestSpaceAuthzService{spaceOwner})
	collaboratorRESTInstance := &TestCollaboratorsREST{DBTestSuite: gormtestsupport.NewDBTestSuite("../config.yaml")}
	collaboratorRESTInstance.policy = &auth.KeycloakPolicy{
		Name:             "TestCollaborators-" + uuid.NewV4().String(),
		Type:             auth.PolicyTypeUser,
		Logic:            auth.PolicyLogicPossitive,
		DecisionStrategy: auth.PolicyDecisionStrategyUnanimous,
	}
	collaboratorCtrl := NewCollaboratorsController(svcWithSpaceOwner, s.db, s.Configuration, &DummyPolicyManager{rest: collaboratorRESTInstance})
	test.AddCollaboratorsOK(s.T(), svcWithSpaceOwner.Context, svcWithSpaceOwner, collaboratorCtrl, *space.ID, collaborator1.ID.String())
	test.AddCollaboratorsOK(s.T(), svcWithSpaceOwner.Context, svcWithSpaceOwner, collaboratorCtrl, *space.ID, collaborator2.ID.String())

	// Build WI payload and create 1 WI (created_by = space owner)
	payload := minimumRequiredCreateWithTypeAndSpace(workitem.SystemFeature, *space.ID)
	payload.Data.Attributes[workitem.SystemTitle] = "Test WI"
	payload.Data.Attributes[workitem.SystemState] = workitem.SystemStateNew
	workitemCtrl := NewWorkitemController(svcWithSpaceOwner, gormapplication.NewGormDB(s.DB), s.Configuration)

	_, wi := test.CreateWorkitemCreated(s.T(), svcWithSpaceOwner.Context, svcWithSpaceOwner, workitemCtrl, *payload.Data.Relationships.Space.Data.ID, &payload)

	// collaborator1 adds a comment on newly created work item
	c := s.createWorkItemComment(collaborator1, *wi.Data.ID, "Hello woody", &plaintextMarkup)

	// Collaborator2 deletes the comment
	svcWithCollaborator2 := testsupport.ServiceAsSpaceUser("Comments-Service", almtoken.NewManagerWithPrivateKey(priv), collaborator2, &TestSpaceAuthzService{collaborator2})
	commentCtrl := NewCommentsController(svcWithCollaborator2, gormapplication.NewGormDB(s.DB), s.Configuration)
	test.DeleteCommentsOK(s.T(), svcWithCollaborator2.Context, svcWithCollaborator2, commentCtrl, *c.Data.ID)
}
