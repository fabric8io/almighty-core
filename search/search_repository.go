package search

import (
	"fmt"
	"sync"

	"context"

	"strings"

	"regexp"

	"net/url"

	"github.com/fabric8io/almighty-core/errors"
	"github.com/fabric8io/almighty-core/log"
	"github.com/fabric8io/almighty-core/workitem"

	"github.com/asaskevich/govalidator"
	"github.com/jinzhu/gorm"
	errs "github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

// KnownURL registration key constants
const (
	HostRegistrationKeyForListWI  = "work-item-list-details"
	HostRegistrationKeyForBoardWI = "work-item-board-details"
)

// GormSearchRepository provides a Gorm based repository
type GormSearchRepository struct {
	db  *gorm.DB
	wir *workitem.GormWorkItemTypeRepository
}

// NewGormSearchRepository creates a new search repository
func NewGormSearchRepository(db *gorm.DB) *GormSearchRepository {
	return &GormSearchRepository{db, workitem.NewWorkItemTypeRepository(db)}
}

func generateSearchQuery(q string) (string, error) {
	return q, nil
}

//searchKeyword defines how a decomposed raw search query will look like
type searchKeyword struct {
	workItemTypes []uuid.UUID
	id            []string
	words         []string
}

// KnownURL has a regex string format URL and compiled regex for the same
type KnownURL struct {
	URLRegex          string         // regex for URL, Exposed to make the code testable
	compiledRegex     *regexp.Regexp // valid output of regexp.MustCompile()
	groupNamesInRegex []string       // Valid output of SubexpNames called on compliedRegex
}

/*
KnownURLs is set of KnownURLs will be used while searching on a URL
"Known" means that, our system understands the format of URLs
URLs in this slice will be considered while searching to match search string and decouple it into multiple searchable parts
e.g> Following example defines work-item-detail-page URL on client side, with its compiled version
knownURLs["work-item-details"] = KnownURL{
URLRegex:      `^(?P<protocol>http[s]?)://(?P<domain>demo.almighty.io)(?P<path>/work-item/list/detail/)(?P<id>\d*)`,
compiledRegex: regexp.MustCompile(`^(?P<protocol>http[s]?)://(?P<domain>demo.almighty.io)(?P<path>/work-item/list/detail/)(?P<id>\d*)`),
groupNamesInRegex: []string{"protocol", "domain", "path", "id"}
}
above url will be decoupled into two parts "ID:* | domain+path+id:*" while performing search query
*/
var knownURLs = make(map[string]KnownURL)
var knownURLLock sync.RWMutex

// RegisterAsKnownURL appends to KnownURLs
func RegisterAsKnownURL(name, urlRegex string) {
	compiledRegex := regexp.MustCompile(urlRegex)
	groupNames := compiledRegex.SubexpNames()
	knownURLLock.Lock()
	defer knownURLLock.Unlock()
	knownURLs[name] = KnownURL{
		URLRegex:          urlRegex,
		compiledRegex:     regexp.MustCompile(urlRegex),
		groupNamesInRegex: groupNames,
	}
}

// GetAllRegisteredURLs returns all known URLs
func GetAllRegisteredURLs() map[string]KnownURL {
	return knownURLs
}

/*
isKnownURL compares with registered URLs in our system.
Iterates over knownURLs and finds out most relevant matching pattern.
If found, it returns true along with "name" of the KnownURL
*/
func isKnownURL(url string) (bool, string) {
	// should check on all system's known URLs
	var mostReleventMatchCount int
	var mostReleventMatchName string
	for name, known := range knownURLs {
		match := known.compiledRegex.FindStringSubmatch(url)
		if len(match) > mostReleventMatchCount {
			mostReleventMatchCount = len(match)
			mostReleventMatchName = name
		}
	}
	if mostReleventMatchName == "" {
		return false, ""
	}
	return true, mostReleventMatchName
}

func trimProtocolFromURLString(urlString string) string {
	urlString = strings.TrimPrefix(urlString, `http://`)
	urlString = strings.TrimPrefix(urlString, `https://`)
	return urlString
}

func escapeCharFromURLString(urlString string) string {
	// Replacer will escape `:` and `)` `(`.
	var replacer = strings.NewReplacer(":", "\\:", "(", "\\(", ")", "\\)")
	return replacer.Replace(urlString)
}

// sanitizeURL does cleaning of URL
// returns DB friendly string
// Trims protocol and escapes ":"
func sanitizeURL(urlString string) string {
	trimmedURL := trimProtocolFromURLString(urlString)
	return escapeCharFromURLString(trimmedURL)
}

/*
getSearchQueryFromURLPattern takes
patternName - name of the KnownURL
stringToMatch - search string
Finds all string match for given pattern
Iterates over pattern's groupNames and loads respective values into result
*/
func getSearchQueryFromURLPattern(patternName, stringToMatch string) string {
	pattern := knownURLs[patternName]
	// TODO : handle case for 0 matches
	match := pattern.compiledRegex.FindStringSubmatch(stringToMatch)
	result := make(map[string]string)
	// result will hold key-value for groupName to its value
	// e.g> "domain": "demo.almighty.io", "id": 200
	for i, name := range pattern.groupNamesInRegex {
		if i == 0 {
			continue
		}
		if i > len(match)-1 {
			result[name] = ""
		} else {
			result[name] = match[i]
		}
	}
	// first value from FindStringSubmatch is always full input itself, hence ignored
	// Join rest of the tokens to make query like "demo.almighty.io/work-item/list/detail/100"
	if len(match) > 1 {
		searchQueryString := strings.Join(match[1:], "")
		searchQueryString = strings.Replace(searchQueryString, ":", "\\:", -1)
		// need to escape ":" because this string will go as an input to tsquery
		searchQueryString = fmt.Sprintf("%s:*", searchQueryString)
		if result["id"] != "" {
			// Look for pattern's ID field, if exists update searchQueryString
			searchQueryString = fmt.Sprintf("(%v:* | %v)", result["id"], searchQueryString)
			// searchQueryString = "(" + result["id"] + ":*" + " | " + searchQueryString + ")"
		}
		return searchQueryString
	}
	return match[0] + ":*"
}

/*
getSearchQueryFromURLString gets a url string and checks if that matches with any of known urls.
Respectively it will return a string that can be directly used in search query
e.g>
Unknown url : www.google.com then response = "www.google.com:*"
Known url : almighty.io/detail/500 then response = "500:* | almighty.io/detail/500"
*/
func getSearchQueryFromURLString(url string) string {
	known, patternName := isKnownURL(url)
	if known {
		// this url is known to system
		return getSearchQueryFromURLPattern(patternName, url)
	}
	// any URL other than our system's
	// return url without protocol
	return sanitizeURL(url) + ":*"
}

// parseSearchString accepts a raw string and generates a searchKeyword object
func parseSearchString(rawSearchString string) (searchKeyword, error) {
	// TODO remove special characters and exclaimations if any
	rawSearchString = strings.Trim(rawSearchString, "/") // get rid of trailing slashes
	rawSearchString = strings.Trim(rawSearchString, "\"")
	parts := strings.Fields(rawSearchString)
	var res searchKeyword
	for _, part := range parts {
		// QueryUnescape is required in case of encoded url strings.
		// And does not harm regular search strings
		// but this processing is required because at this moment, we do not know if
		// search input is a regular string or a URL

		part, err := url.QueryUnescape(part)
		if err != nil {
			log.Warn(nil, map[string]interface{}{
				"part": part,
			}, "unable to escape url!")
		}
		// IF part is for search with id:1234
		// TODO: need to find out the way to use ID fields.
		if strings.HasPrefix(part, "id:") {
			res.id = append(res.id, strings.TrimPrefix(part, "id:")+":*A")
		} else if strings.HasPrefix(part, "type:") {
			typeIDStr := strings.TrimPrefix(part, "type:")
			if len(typeIDStr) == 0 {
				return res, errors.NewBadParameterError("Type ID must not be empty", part)
			}
			typeID, err := uuid.FromString(typeIDStr)
			if err != nil {
				return res, errors.NewBadParameterError("failed to parse type ID string as UUID", typeIDStr)
			}
			res.workItemTypes = append(res.workItemTypes, typeID)
		} else if govalidator.IsURL(part) {
			part := strings.ToLower(part)
			part = trimProtocolFromURLString(part)
			searchQueryFromURL := getSearchQueryFromURLString(part)
			res.words = append(res.words, searchQueryFromURL)
		} else {
			part := strings.ToLower(part)
			part = sanitizeURL(part)
			res.words = append(res.words, part+":*")
		}
	}
	return res, nil
}

// generateSQLSearchInfo accepts searchKeyword and join them in a way that can be used in sql
func generateSQLSearchInfo(keywords searchKeyword) (sqlParameter string) {
	idStr := strings.Join(keywords.id, " & ")
	wordStr := strings.Join(keywords.words, " & ")

	searchStr := idStr + wordStr
	if len(wordStr) != 0 && len(idStr) != 0 {
		searchStr = idStr + " & " + wordStr
	}
	return searchStr
}

// extracted this function from List() in order to close the rows object with "defer" for more readability
// workaround for https://github.com/lib/pq/issues/81
func (r *GormSearchRepository) search(ctx context.Context, sqlSearchQueryParameter string, workItemTypes []uuid.UUID, start *int, limit *int, spaceID *string) ([]workitem.WorkItemStorage, uint64, error) {
	db := r.db.Model(workitem.WorkItemStorage{}).Where("tsv @@ query")
	if start != nil {
		if *start < 0 {
			return nil, 0, errors.NewBadParameterError("start", *start)
		}
		db = db.Offset(*start)
	}
	if limit != nil {
		if *limit <= 0 {
			return nil, 0, errors.NewBadParameterError("limit", *limit)
		}
		db = db.Limit(*limit)
	}
	if len(workItemTypes) > 0 {
		// restrict to all given types and their subtypes
		query := fmt.Sprintf("%[1]s.type in ("+
			"select distinct subtype.id from %[2]s subtype "+
			"join %[2]s supertype on subtype.path <@ supertype.path "+
			"where supertype.id in (?))", workitem.WorkItemStorage{}.TableName(), workitem.WorkItemType{}.TableName())
		db = db.Where(query, workItemTypes)
	}

	db = db.Select("count(*) over () as cnt2 , *")
	db = db.Joins(", to_tsquery('english', ?) as query, ts_rank(tsv, query) as rank", sqlSearchQueryParameter)
	if spaceID != nil {
		db = db.Where("space_id=?", *spaceID)
	}
	db = db.Order(fmt.Sprintf("rank desc,%s.updated_at desc", workitem.WorkItemStorage{}.TableName()))

	rows, err := db.Rows()
	if err != nil {
		return nil, 0, errs.WithStack(err)
	}
	defer rows.Close()

	result := []workitem.WorkItemStorage{}
	value := workitem.WorkItemStorage{}
	columns, err := rows.Columns()
	if err != nil {
		return nil, 0, errors.NewInternalError(err)
	}

	// need to set up a result for Scan() in order to extract total count.
	var count uint64
	var ignore interface{}
	columnValues := make([]interface{}, len(columns))

	for index := range columnValues {
		columnValues[index] = &ignore
	}
	columnValues[0] = &count
	first := true

	for rows.Next() {
		db.ScanRows(rows, &value)
		if first {
			first = false
			if err = rows.Scan(columnValues...); err != nil {
				return nil, 0, errors.NewInternalError(err)
			}
		}
		result = append(result, value)

	}
	if first {
		// means 0 rows were returned from the first query,
		count = 0
	}
	return result, count, nil
	//*/
}

// SearchFullText Search returns work items for the given query
func (r *GormSearchRepository) SearchFullText(ctx context.Context, rawSearchString string, start *int, limit *int, spaceID *string) ([]workitem.WorkItem, uint64, error) {
	// parse
	// generateSearchQuery
	// ....
	parsedSearchDict, err := parseSearchString(rawSearchString)
	if err != nil {
		return nil, 0, errs.WithStack(err)
	}

	sqlSearchQueryParameter := generateSQLSearchInfo(parsedSearchDict)
	var rows []workitem.WorkItemStorage
	rows, count, err := r.search(ctx, sqlSearchQueryParameter, parsedSearchDict.workItemTypes, start, limit, spaceID)
	if err != nil {
		return nil, 0, errs.WithStack(err)
	}
	result := make([]workitem.WorkItem, len(rows))

	for index, value := range rows {
		var err error
		// FIXME: Against best practice http://go-database-sql.org/retrieving.html
		wiType, err := r.wir.LoadTypeFromDB(ctx, value.Type)
		if err != nil {
			return nil, 0, errors.NewInternalError(err)
		}
		wiModel, err := wiType.ConvertWorkItemStorageToModel(value)
		if err != nil {
			return nil, 0, errors.NewConversionError(err.Error())
		}
		result[index] = *wiModel
	}

	return result, count, nil
}

func init() {
	// While registering URLs do not include protocol because it will be removed before scanning starts
	// Please do not include trailing slashes because it will be removed before scanning starts
	RegisterAsKnownURL("test-work-item-list-details", `(?P<domain>demo.almighty.io)(?P<path>/work-item/list/detail/)(?P<id>\d*)`)
	RegisterAsKnownURL("test-work-item-board-details", `(?P<domain>demo.almighty.io)(?P<path>/work-item/board/detail/)(?P<id>\d*)`)
}
