package test

import (
	"github.com/fabric8io/almighty-core/account"
	"github.com/fabric8io/almighty-core/application"
	"github.com/fabric8io/almighty-core/area"
	"github.com/fabric8io/almighty-core/auth"
	"github.com/fabric8io/almighty-core/codebase"
	"github.com/fabric8io/almighty-core/comment"
	"github.com/fabric8io/almighty-core/iteration"
	"github.com/fabric8io/almighty-core/space"
	"github.com/fabric8io/almighty-core/workitem"
	"github.com/fabric8io/almighty-core/workitem/link"
)

func NewMockDB() *MockDB {
	return &MockDB{wir: &WorkItemRepository{}}
}

type MockDB struct {
	wir *WorkItemRepository
}

func (db *MockDB) WorkItems() workitem.WorkItemRepository {
	return db.wir
}
func (db *MockDB) WorkItemTypes() workitem.WorkItemTypeRepository {
	return nil
}

func (db *MockDB) Spaces() space.Repository {
	return nil
}

func (db *MockDB) SpaceResources() space.ResourceRepository {
	return nil
}

func (db *MockDB) Trackers() application.TrackerRepository {
	return nil
}
func (db *MockDB) TrackerQueries() application.TrackerQueryRepository {
	return nil
}

func (db *MockDB) SearchItems() application.SearchRepository {
	return nil
}
func (db *MockDB) Identities() account.IdentityRepository {
	return nil
}
func (db *MockDB) Users() account.UserRepository {
	return nil
}
func (db *MockDB) WorkItemLinkCategories() link.WorkItemLinkCategoryRepository {
	return nil
}
func (db *MockDB) WorkItemLinkTypes() link.WorkItemLinkTypeRepository {
	return nil
}
func (db *MockDB) WorkItemLinks() link.WorkItemLinkRepository {
	return nil
}
func (db *MockDB) Comments() comment.Repository {
	return nil
}

func (db *MockDB) Iterations() iteration.Repository {
	return nil
}

func (db *MockDB) Areas() area.Repository {
	return nil
}

func (g *MockDB) OauthStates() auth.OauthStateReferenceRepository {
	return nil
}

func (db *MockDB) Codebases() codebase.Repository {
	return nil
}

func (db *MockDB) Commit() error {
	return nil
}
func (db *MockDB) Rollback() error {
	return nil
}

func (db *MockDB) BeginTransaction() (application.Transaction, error) {
	return db, nil
}
