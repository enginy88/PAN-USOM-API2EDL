package config

import (
	"strings"

	"github.com/enginy88/PAN-USOM-API2EDL/logger"

	"github.com/JeremyLoy/config"
)

const (
	DEFAULT_GLB_API_PATH           = "https://www.usom.gov.tr/api/address/index"
	DEFAULT_GLB_DB_PATH            = "usom.db"
	DEFAULT_GLB_READ_FROM_FILE     = false
	DEFAULT_GLB_ENABLE_CONCURRENCY = false
	DEFAULT_GLB_NUM_OF_WORKER      = 4

	DEFAULT_LOG_VERBOSE         = false
	DEFAULT_LOG_WRITE_TO_DIR    = ""
	DEFAULT_LOG_FILENAME_SUFFIX = ""

	DEFAULT_REQ_TOTAL_TIMEOUT       = 180
	DEFAULT_REQ_REQUEST_TIMEOUT     = 30
	DEFAULT_REQ_ADD_RETRY_COUNT     = 2 // Retry count is number of additional requests after the first request.
	DEFAULT_REQ_RETRY_WAIT_TIME     = 1000
	DEFAULT_REQ_RETRY_MAX_WAIT_TIME = 5000
	DEFAULT_REQ_ALLOW_REDIRECT      = false
	DEFAULT_REQ_MAX_REDIRECT        = 2
	DEFAULT_REQ_RESPONSE_BODY_LIMIT = 30000000
	DEFAULT_REQ_USER_AGENT          = "Mozilla/5.0 (compatible; Linux x86_64; IDEUS/1.0)"

	DEFAULT_LST_MIN_CRITICALITY         = 5
	DEFAULT_LST_CREATE_STANDALONE_LISTS = true
	DEFAULT_LST_CREATE_AGGREGATED_LISTS = true
	DEFAULT_LST_CREATE_MIX_LISTS        = false
	DEFAULT_LST_SKIP_IF_DB_IDENTICAL    = false
)

type GlobalSubEnvStruct struct {
	APIPath           string `config:"API_PATH"`
	DBPath            string `config:"DB_PATH"`
	ReadFromFile      bool   `config:"READ_FROM_FILE"`
	EnableConcurrency bool   `config:"ENABLE_CONCURRENCY"`
	NumOfWorker       int    `config:"NUM_OF_WORKER"`
}

type LogSubEnvStruct struct {
	Verbose        bool   `config:"VERBOSE"`
	WriteToDir     string `config:"WRITE_TO_DIR"`
	FilenameSuffix string `config:"FILENAME_SUFFIX"`
}

type RequestSubEnvStruct struct {
	TotalTimeout      int    `config:"TOTAL_TIMEOUT"`
	RequestTimeout    int    `config:"REQUEST_TIMEOUT"`
	AddRetryCount     int    `config:"ADD_RETRY_COUNT"`
	RetryWaitTime     int    `config:"RETRY_WAIT_TIME"`
	RetryMaxWaitTime  int    `config:"RETRY_MAX_WAIT_TIME"`
	AllowRedirect     bool   `config:"ALLOW_REDIRECT"`
	MaxRedirect       int    `config:"MAX_REDIRECT"`
	ResponseBodyLimit int    `config:"RESPONSE_BODY_LIMIT"`
	UserAgent         string `config:"USER_AGENT"`
}

type ListSubEnvStruct struct {
	MinCriticality       int  `config:"MIN_CRITICALITY"`
	CreteStandaloneLists bool `config:"CREATE_STANDALONE_LISTS"`
	CreteAggregatedLists bool `config:"CREATE_AGGREGATED_LISTS"`
	CreteMixLists        bool `config:"CREATE_MIX_LISTS"`
	SkipIfDBIdentical    bool `config:"SKIP_IF_DB_IDENTICAL"`
}

type AppEnvStruct struct {
	Global  GlobalSubEnvStruct  `config:"API2EDL_GLOBAL"`
	Log     LogSubEnvStruct     `config:"API2EDL_LOG"`
	Request RequestSubEnvStruct `config:"API2EDL_REQUEST"`
	List    ListSubEnvStruct    `config:"API2EDL_LIST"`
}

var AppEnv *AppEnvStruct

func createDefaultAppEnvStruct() *AppEnvStruct {
	return &AppEnvStruct{
		Global: GlobalSubEnvStruct{
			APIPath:           DEFAULT_GLB_API_PATH,
			DBPath:            DEFAULT_GLB_DB_PATH,
			ReadFromFile:      DEFAULT_GLB_READ_FROM_FILE,
			EnableConcurrency: DEFAULT_GLB_ENABLE_CONCURRENCY,
			NumOfWorker:       DEFAULT_GLB_NUM_OF_WORKER,
		},
		Log: LogSubEnvStruct{
			Verbose:        DEFAULT_LOG_VERBOSE,
			WriteToDir:     DEFAULT_LOG_WRITE_TO_DIR,
			FilenameSuffix: DEFAULT_LOG_FILENAME_SUFFIX,
		},
		Request: RequestSubEnvStruct{
			TotalTimeout:      DEFAULT_REQ_TOTAL_TIMEOUT,
			RequestTimeout:    DEFAULT_REQ_REQUEST_TIMEOUT,
			AddRetryCount:     DEFAULT_REQ_ADD_RETRY_COUNT,
			RetryWaitTime:     DEFAULT_REQ_RETRY_WAIT_TIME,
			RetryMaxWaitTime:  DEFAULT_REQ_RETRY_MAX_WAIT_TIME,
			AllowRedirect:     DEFAULT_REQ_ALLOW_REDIRECT,
			MaxRedirect:       DEFAULT_REQ_MAX_REDIRECT,
			ResponseBodyLimit: DEFAULT_REQ_RESPONSE_BODY_LIMIT,
			UserAgent:         DEFAULT_REQ_USER_AGENT,
		},
		List: ListSubEnvStruct{
			MinCriticality:       DEFAULT_LST_MIN_CRITICALITY,
			CreteStandaloneLists: DEFAULT_LST_CREATE_STANDALONE_LISTS,
			CreteAggregatedLists: DEFAULT_LST_CREATE_AGGREGATED_LISTS,
			CreteMixLists:        DEFAULT_LST_CREATE_MIX_LISTS,
			SkipIfDBIdentical:    DEFAULT_LST_SKIP_IF_DB_IDENTICAL,
		},
	}
}

func GetAppEnv() *AppEnvStruct {

	appEnvObject := createDefaultAppEnvStruct()
	AppEnv = appEnvObject

	loadAppEnv()
	checkAppEnv()

	return AppEnv

}

func loadAppEnv() {

	err := config.FromOptional("./" + logger.AppName + ".env").FromEnv().To(AppEnv)
	if err != nil {
		logger.LogErr.Fatalln("ENV: Cannot find/load '" + logger.AppName + ".env' file! (" + err.Error() + ")")
	}

}

func checkAppEnv() {

	// It may contain quotes if supplied by .env file. All strings should be stripped from quotes.

	AppEnv.Global.APIPath = stripQuotes(AppEnv.Global.APIPath)
	AppEnv.Global.DBPath = stripQuotes(AppEnv.Global.DBPath)

	AppEnv.Log.WriteToDir = stripQuotes(AppEnv.Log.WriteToDir)
	AppEnv.Log.FilenameSuffix = stripQuotes(AppEnv.Log.FilenameSuffix)

	AppEnv.Request.UserAgent = stripQuotes(AppEnv.Request.UserAgent)

	AppEnv.Global.APIPath = toLowerCase(AppEnv.Global.APIPath)

	if containsSpace(AppEnv.Global.APIPath) {
		logger.LogWarn.Println("ENV: 'API2EDL_GLOBAL__API_PATH' value contains whitespace, this will cause a failure!")
	}

}

// stripQuotes removes quotes from the start and end of the string, if they exist
func stripQuotes(s string) string {
	if len(s) > 1 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// containsSpace checks if the string contains a space character
func containsSpace(s string) bool {
	return strings.Contains(s, " ")
}

// toLowerCase converts all letters in a string to lowercase
func toLowerCase(input string) string {
	return strings.ToLower(input)
}
