package list

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/enginy88/PAN-USOM-API2EDL/config"
	"github.com/enginy88/PAN-USOM-API2EDL/db"
	"github.com/enginy88/PAN-USOM-API2EDL/logger"
)

type ListType string
type FilterSeverity string
type TimeLimit string
type CountLimit string

const (
	TypeIP               ListType = "ip"
	TypeURL              ListType = "url"
	TypeDomain           ListType = "domain"
	TypeAll              ListType = "mix"
	TypeAggregatedIP     ListType = "aggr_ip"
	TypeAggregatedURL    ListType = "aggr_url"
	TypeAggregatedDomain ListType = "aggr_domain"

	SeverityAny  FilterSeverity = "any"
	SeverityHigh FilterSeverity = "high"

	Time30D  TimeLimit = "30d"
	Time90D  TimeLimit = "90d"
	Time180D TimeLimit = "180d"
	Time1Y   TimeLimit = "1y"
	Time3Y   TimeLimit = "3y"
	Time5Y   TimeLimit = "5y"

	Count50K  CountLimit = "50k"
	Count100K CountLimit = "100k"
	Count150K CountLimit = "150k"
	Count250K CountLimit = "250k"
	Count500K CountLimit = "500k"
	Count1M   CountLimit = "1m"
)

type ListConfig struct {
	Type       ListType
	Severity   FilterSeverity
	TimeLimit  TimeLimit  // Optional, used for time-based filtering
	CountLimit CountLimit // Optional, used for count-based filtering
}

func generateListHeader(recordCount int) string {
	dbTime := db.GetDBTime()
	return "# Proudly served by IDEUS! Contact: edl[at]ideus.com.tr \n" +
		"# Last updated: " + dbTime + "\n" +
		"# Record Count: " + strconv.Itoa(recordCount)
}

func getTimeLimitDuration(limit TimeLimit) time.Duration {
	switch limit {
	case Time30D:
		return 30 * 24 * time.Hour
	case Time90D:
		return 90 * 24 * time.Hour
	case Time180D:
		return 180 * 24 * time.Hour
	case Time1Y:
		return 365 * 24 * time.Hour
	case Time3Y:
		return 3 * 365 * 24 * time.Hour
	case Time5Y:
		return 5 * 365 * 24 * time.Hour
	default:
		return 0
	}
}

func getCountLimitValue(limit CountLimit) int {
	switch limit {
	case Count50K:
		return 50000
	case Count100K:
		return 100000
	case Count150K:
		return 150000
	case Count250K:
		return 250000
	case Count500K:
		return 500000
	case Count1M:
		return 1000000
	default:
		return 0
	}
}

func GenerateList(ctx context.Context, listConfig ListConfig) error {
	params := db.QueryParams{
		OrderBy:        "date",
		OrderDirection: "DESC",
	}

	// Apply time limit if specified
	if listConfig.TimeLimit != "" {
		params.DateFrom = time.Now().Add(-getTimeLimitDuration(listConfig.TimeLimit))
		params.DateTo = time.Now()
	}

	countLimit := 0

	// Apply count limit if specified
	if listConfig.CountLimit != "" {
		countLimit = getCountLimitValue(listConfig.CountLimit)
		params.Limit = countLimit
	}

	if listConfig.Severity == SeverityHigh {
		params.MinCriticality = config.AppEnv.List.MinCriticality
	}

	switch listConfig.Type {
	case TypeIP:
		params.Types = []string{"ip"}
	case TypeURL:
		params.Types = []string{"url"}
	case TypeDomain:
		params.Types = []string{"domain"}
	case TypeAll:
		params.Types = []string{"ip", "url", "domain"}
	case TypeAggregatedIP:
		params.Types = []string{"ip", "url"}
	case TypeAggregatedURL:
		params.Types = []string{"ip", "url", "domain"}
	case TypeAggregatedDomain:
		params.Types = []string{"url", "domain"}
	}

	records, err := db.GetRecords(ctx, params)
	if err != nil {
		logger.LogErr.Println("ERROR: Cannot get records from DB! (" + err.Error() + ")")
		return err
	}

	validRecords := []string{}
	seenRecords := make(map[string]struct{}) // Map to track unique records

	var regexType regexType
	force := false

	if listConfig.Type == TypeAggregatedIP {
		regexType = regexTypeIP
		force = true
	} else if listConfig.Type == TypeAggregatedURL {
		regexType = regexTypeURL
		force = true
	} else if listConfig.Type == TypeAggregatedDomain {
		regexType = regexTypeDomain
		force = true
	}

	// Validate Records
	for _, record := range records {

		if !force {
			if record.Type == "ip" {
				regexType = regexTypeIP
			} else if record.Type == "url" {
				regexType = regexTypeURL
			} else if record.Type == "domain" {
				regexType = regexTypeDomain
			} else {
				logger.LogErr.Println("REGEX: Unknown record type: '" + record.Type + "'! (RECORD: " + record.URL + ")")
				continue
			}
		}

		validRecord := validateWithRegex(record.URL, regexType, force)
		if validRecord == "" {
			continue
		}

		// Check if we've already seen this record
		if _, exists := seenRecords[validRecord]; !exists {
			seenRecords[validRecord] = struct{}{}
			validRecords = append(validRecords, validRecord)
		} else {
			logger.LogInfo.Println("UNIQUE: Duplicate record found for the record: '" + validRecord + "'!")
		}
	}

	// Prepare file
	unlimitedStr := ""
	if listConfig.TimeLimit == "" && listConfig.CountLimit == "" {
		unlimitedStr = "all"
	}

	filename := "edl-" + string(listConfig.Type) + "-" + string(listConfig.Severity) + "-" + string(listConfig.TimeLimit) + string(listConfig.CountLimit) + unlimitedStr + ".txt"
	fullPath := filepath.Join(config.AppFlag.OutputDir, filename)

	root, err := os.OpenRoot(config.AppFlag.OutputDir)
	if err != nil {
		logger.LogErr.Println("ERROR: Cannot open output directory: '" + config.AppFlag.OutputDir + "'! (" + err.Error() + ")")
		return err
	}
	defer root.Close()

	f, err := root.Create(filename)
	if err != nil {
		logger.LogErr.Println("ERROR: Cannot create file: '" + fullPath + "'! (" + err.Error() + ")")
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)

	if _, err := fmt.Fprintln(writer, generateListHeader(len(validRecords))); err != nil {
		logger.LogErr.Println("ERROR: Cannot write list header to file: '" + fullPath + "'! (" + err.Error() + ")")
		return err
	}

	for n, record := range validRecords {
		if countLimit > 0 && n >= countLimit {
			break
		}
		if _, err := fmt.Fprintln(writer, record); err != nil {
			logger.LogErr.Println("ERROR: Cannot write record to file: '" + fullPath + "'! (" + err.Error() + ")")
			return err
		}
	}

	logger.LogInfo.Println("LIST: The list is created with filename: '" + fullPath + "'.")

	return writer.Flush()
}
