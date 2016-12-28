package db

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	couchbase "github.com/couchbase/gocb"
	"gopkg.in/telegram-bot-api.v4"
)

var (
	cluster    *couchbase.Cluster
	bucket     *couchbase.Bucket
	bucketName string
)

// InitCouchbase function initialize couchbase bucket with parameters
func InitCouchbase(couchbaseCluster, couchbaseBucket, couchbaseSecret string) {
	cluster, err := couchbase.Connect(couchbaseCluster)
	if err != nil {
		log.Fatalf("Cannot connect to cluster: %s", err)
	}
	bucket, err = cluster.OpenBucket(couchbaseBucket, couchbaseSecret)
	if err != nil {
		log.Fatalf("Cannot open bucket: %s", err)
	}
	bucketName = couchbaseBucket

	caches = make(Caches)
	updateDateCaches()
}

// SaveUserCouchbase method save user to database
func SaveUserCouchbase(user *tgbotapi.User) (err error) {
	key := fmt.Sprintf("user:%d", user.ID)

	type couchuser struct {
		tgbotapi.User
		Type string `json:"type"`
	}
	cUser := couchuser{}

	data, err := json.Marshal(user)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &cUser)
	if err != nil {
		return
	}
	cUser.Type = "user"

	_, err = bucket.Upsert(key, &cUser, 0)
	return
}

// SaveFileCouchbase method save user to database
func SaveFileCouchbase(file *tgbotapi.File, chatID int64) (err error) {
	key := fmt.Sprintf("file:%d:%s", chatID, file.FileID)

	type couchfile struct {
		tgbotapi.File
		Type string `json:"type"`
	}
	cFile := couchfile{}

	data, err := json.Marshal(file)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &cFile)
	if err != nil {
		return
	}
	cFile.Type = "file"

	_, err = bucket.Upsert(key, &cFile, 0)
	return
}

// GetCensLevelCouchbase function returns censore level for user
func GetCensLevelCouchbase(user *tgbotapi.User) (currentLevel int, err error) {
	currentLevel = 0
	currentYear := time.Now().Year()
	key := fmt.Sprintf("censlevel:%d:%d", currentYear, user.ID)

	level := CensLevel{}

	_, err = bucket.Get(key, &level)
	if err != nil {
		return
	}
	currentLevel = level.Level
	return
}

// SetCensLevelCouchbase function sets level for user
func SetCensLevelCouchbase(user *tgbotapi.User, setlevel int) (err error) {
	currentYear := time.Now().Year()
	key := fmt.Sprintf("censlevel:%d:%d", currentYear, user.ID)

	level := CensLevel{}

	_, err = bucket.Get(key, &level)
	if err != nil {
		level.ID = user.ID
		level.Level = setlevel
		level.Year = currentYear
	} else {
		level.Level = setlevel
	}

	_, err = bucket.Upsert(key, &level, 0)
	return
}

// ClearCensLevelCouchbase remove document from bucket
func ClearCensLevelCouchbase(user *tgbotapi.User) (err error) {
	currentYear := time.Now().Year()
	key := fmt.Sprintf("censlevel:%d:%d", currentYear, user.ID)

	level := CensLevel{}

	cas, err := bucket.Get(key, &level)
	if err != nil {
		return
	}

	_, err = bucket.Remove(key, cas)
	if err != nil {
		return
	}
	return
}

// AddCensLevelCouchbase added +1 to cens level in year
func AddCensLevelCouchbase(user *tgbotapi.User) (currentLevel int, err error) {
	currentLevel, err = GetCensLevel(user)
	if err != nil {
		currentLevel = 1
		err = SetCensLevel(user, currentLevel)
		return
	}
	currentLevel++
	err = SetCensLevel(user, currentLevel)

	return
}

// GetFileCouchbase returns file json from couchbase
func GetFileCouchbase(fileID string, chatID int64) (f *tgbotapi.File, err error) {
	key := fmt.Sprintf("file:%d:%s", chatID, fileID)
	f = new(tgbotapi.File)
	_, err = bucket.Get(key, f)
	return
}

// SaveChatCouchbase method for save chat to database
func SaveChatCouchbase(chat *tgbotapi.Chat, forward bool) (err error) {
	key := fmt.Sprintf("chat:%d", chat.ID)

	type couchchat struct {
		tgbotapi.Chat
		Type string `json:"type"`
	}
	cChat := couchchat{}

	data, err := json.Marshal(chat)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &cChat)
	if err != nil {
		return
	}
	if forward {
		cChat.Type = typeForwardType
	} else {
		cChat.Type = typeChat
	}

	_, err = bucket.Upsert(key, cChat, 0)
	return
}

// GetChatsCouchbase returns chat list
func GetChatsCouchbase() (chats []*tgbotapi.Chat, err error) {
	type couchchat struct {
		Msg tgbotapi.Chat `json:"bot"`
	}

	query := couchbase.NewN1qlQuery(fmt.Sprintf("SELECT * FROM %s AS bot WHERE type='chat'", bucketName))
	res, err := bucket.ExecuteN1qlQuery(query, nil)
	if err != nil {
		return
	}

	//var data interface{}

	chat := couchchat{}
	for res.Next(&chat) {

		data, err := json.Marshal(chat.Msg)
		if err != nil {
			log.Printf("Error in marshal GetChats: %s", err)
			continue
		}
		oChat := new(tgbotapi.Chat)
		err = json.Unmarshal(data, oChat)
		if err != nil {
			log.Printf("Error in unmarshal GetChats: %s", err)
			continue
		}
		chats = append(chats, oChat)
	}

	return
}

// GetMessagesCouchbase returns chat list
func GetMessagesCouchbase(chatID int64) (messages []*tgbotapi.Message, err error) {
	type couchmsg struct {
		Msg tgbotapi.Message `json:"bot"`
	}

	queryStr := fmt.Sprintf("SELECT * FROM %s AS bot WHERE type='message' AND chat.id=%d ORDER BY date", bucketName, chatID)
	query := couchbase.NewN1qlQuery(queryStr)
	res, err := bucket.ExecuteN1qlQuery(query, nil)
	if err != nil {
		return
	}

	//var data interface{}

	chat := couchmsg{}
	for res.Next(&chat) {
		data, err := json.Marshal(chat.Msg)
		if err != nil {
			log.Printf("Error in marshal GetMessages: %s", err)
			continue
		}
		oMsg := new(tgbotapi.Message)
		err = json.Unmarshal(data, oMsg)
		if err != nil {
			log.Printf("Error in unmarshal GetMessages: %s", err)
			continue
		}
		messages = append(messages, oMsg)
	}

	return
}

// GetMessagesByDateCouchbase returns chat list on date
func GetMessagesByDateCouchbase(chatID int64, beginTime, endTime time.Time) (messages []*tgbotapi.Message, err error) {
	type couchmsg struct {
		Msg tgbotapi.Message `json:"bot"`
	}

	queryStr := fmt.Sprintf("SELECT * FROM %s AS bot WHERE type='message' AND chat.id=%d AND date >= %d AND date <= %d ORDER BY date", bucketName, chatID, beginTime.Unix(), endTime.Unix())
	query := couchbase.NewN1qlQuery(queryStr)
	res, err := bucket.ExecuteN1qlQuery(query, nil)
	if err != nil {
		return
	}

	//var data interface{}

	chat := couchmsg{}
	for res.Next(&chat) {
		data, err := json.Marshal(chat.Msg)
		if err != nil {
			log.Printf("Error in marshal GetMessages: %s", err)
			continue
		}
		oMsg := new(tgbotapi.Message)
		err = json.Unmarshal(data, oMsg)
		if err != nil {
			log.Printf("Error in unmarshal GetMessages: %s", err)
			continue
		}
		messages = append(messages, oMsg)
	}

	return
}

// GetUsersCouchbase returns chat list
func GetUsersCouchbase() (users []*tgbotapi.User, err error) {
	type couchuser struct {
		User tgbotapi.User `json:"bot"`
	}

	query := couchbase.NewN1qlQuery(fmt.Sprintf("SELECT * FROM %s AS bot WHERE type='user'", bucketName))
	res, err := bucket.ExecuteN1qlQuery(query, nil)
	if err != nil {
		return
	}

	//var data interface{}

	user := couchuser{}
	for res.Next(&user) {

		data, err := json.Marshal(user.User)
		if err != nil {
			log.Printf("Error in marshal GetUsers: %s", err)
			continue
		}
		oUser := new(tgbotapi.User)
		err = json.Unmarshal(data, oUser)
		if err != nil {
			log.Printf("Error in unmarshal GetUsers: %s", err)
			continue
		}
		users = append(users, oUser)
	}

	return
}

func getDatesCouchbase(chatID int64, beginDate, endDate int64) (result []time.Time, err error) {
	type couchdate struct {
		Date int64 `json:"date"`
	}

	var dateWhere string
	if beginDate != 0 || endDate != 0 {
		dateWhere = fmt.Sprintf(" AND date >= %d AND date <= %d", beginDate, endDate)
	}

	queryStr := fmt.Sprintf("SELECT date FROM %s WHERE type='message' AND chat.id=%d %s ORDER BY date", bucketName, chatID, dateWhere)
	query := couchbase.NewN1qlQuery(queryStr)
	res, err := bucket.ExecuteN1qlQuery(query, nil)
	if err != nil {
		return
	}

	date := couchdate{}
	for res.Next(&date) {
		tDate := time.Unix(date.Date, 0)
		result = append(result, tDate)
	}
	return
}

// GetYearsCouchbase function returns years msg date from chat messages
func GetYearsCouchbase(chatID int64) (result []string, err error) {
	years := getCache(chatID).Years
	if len(years) != 0 {
		sort.Strings(years)
		return years, nil
	}
	listDates, err := getDates(chatID, 0, 0)
	if err != nil {
		return
	}
	for _, t := range listDates {
		go AddedDateToCaches(chatID, t)
		s := strconv.Itoa(t.Year())
		result = appendIfNotFound(result, s)
	}
	return
}

// GetMonthListCouchbase function returns month list msg date from chat messages and year
func GetMonthListCouchbase(chatID int64, year int) (result []time.Month, err error) {
	cache := getCache(chatID)
	if list, ok := cache.MonthsByYear[year]; ok {
		if len(list) > 0 {
			return sortMonths(list), nil
		}
	}
	beginDate := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local).Unix()
	endDate := time.Date(year, 12, 31, 23, 59, 59, 100, time.Local).Unix()
	listDates, err := getDates(chatID, beginDate, endDate)
	if err != nil {
		return
	}
	for _, t := range listDates {
		if t.Year() != year {
			continue
		}

		result = appendIfNotFoundMonth(result, t.Month())
	}
	return

}

// GetDatesCouchbase function returns month list msg date from chat messages and year
func GetDatesCouchbase(chatID int64, year int, month int) (result []int, err error) {
	result = getDays(chatID, year, time.Month(month))
	if len(result) > 0 {
		return
	}
	beginTime := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	beginDate := beginTime.Unix()
	endDate := time.Date(year, time.Month(month), 32, 23, 59, 59, 100, time.Local).Unix()
	listDates, err := getDates(chatID, beginDate, endDate)
	if err != nil {
		return
	}
	for _, t := range listDates {
		if t.Year() == year && t.Month() == time.Month(month) {
			result = appendIfNotFoundInt(result, t.Day())
		}
	}
	return
}

// GetUserCouchbase get user by username or first and last name
func GetUserCouchbase(username string) (user *tgbotapi.User, err error) {
	if len(username) == 0 {
		return
	}
	type couchuser struct {
		User tgbotapi.User `json:"bot"`
	}

	var queryStr string

	if username[0] == '@' { // username
		queryStr = fmt.Sprintf("SELECT * FROM %s AS bot WHERE type='user' AND username='%s'", bucketName, username[1:])
	} else { // first and last name
		argList := strings.Split(username, " ")
		switch len(argList) {
		case 1:
			queryStr = fmt.Sprintf("SELECT * FROM %s AS bot WHERE type='user' AND first_name='%s'", bucketName, argList[0])
		case 2:
			queryStr = fmt.Sprintf("SELECT * FROM %s AS bot WHERE type='user' AND first_name='%s' AND last_name='%s'", bucketName, argList[0], argList[1])
		default:
			return nil, fmt.Errorf("User not found\n%s", username)
		}
	}

	query := couchbase.NewN1qlQuery(queryStr)
	res, err := bucket.ExecuteN1qlQuery(query, nil)
	if err != nil {
		return nil, err
	}

	var userList []string
	tempuser := couchuser{}
	for res.Next(&tempuser) {
		data, err := json.Marshal(tempuser.User)
		if err != nil {
			log.Printf("Error in marshal GetUser: %s", err)
			continue
		}
		oUser := new(tgbotapi.User)
		err = json.Unmarshal(data, oUser)
		if err != nil {
			log.Printf("Error in unmarshal GetUser: %s", err)
			continue
		}
		user = oUser
		userList = append(userList, user.String())
	}

	if len(userList) > 1 {
		return nil, fmt.Errorf("Many users\n%s", strings.Join(userList, "\n"))
	} else if len(userList) == 0 {
		return nil, fmt.Errorf("User not found\n%s", username)
	}

	return
}
