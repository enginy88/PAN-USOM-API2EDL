package job

import (
	"context"
	"os"
	"sync"

	"github.com/enginy88/PAN-USOM-API2EDL/config"
	"github.com/enginy88/PAN-USOM-API2EDL/db"
	"github.com/enginy88/PAN-USOM-API2EDL/list"
	"github.com/enginy88/PAN-USOM-API2EDL/logger"
	"github.com/enginy88/PAN-USOM-API2EDL/usom"
)

func RunAllJobs() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := db.InitDB(ctx); err != nil {
		logger.LogErr.Println("JOB: Failed to initialize database! (" + err.Error() + ")")
		return
	}
	defer db.CloseDB()

	if config.AppEnv.Global.ReadFromFile {
		if err := db.LoadFromFile(ctx, config.AppEnv.Global.DBPath); err != nil {
			logger.LogErr.Println("JOB: Failed to load from file! (" + err.Error() + ")")
			return
		}
	} else {
		if err := usom.FetchAllPages(ctx); err != nil {
			logger.LogErr.Println("JOB: Failed to fetch pages! (" + err.Error() + ")")
			return
		}

		if err := db.StoreRecords(ctx, usom.AllModels); err != nil {
			logger.LogErr.Println("JOB: Failed to store records! (" + err.Error() + ")")
			return
		}

		// Check if the database file exists and compare if it does
		if _, err := os.Stat(config.AppEnv.Global.DBPath); err == nil {
			logger.LogInfo.Println("JOB: Database file exists. Comparing with in-memory database...")
			equal, err := db.CompareWithFile(config.AppEnv.Global.DBPath)
			if err != nil {
				logger.LogWarn.Println("JOB: Failed to compare databases! (" + err.Error() + ") Continuing anyway...")
			} else {
				if equal {
					logger.LogInfo.Println("JOB: In-memory database and file database are identical. No changes detected.")

					if config.AppEnv.List.SkipIfDBIdentical {
						logger.LogInfo.Println("JOB: Skipping list generation because databases are identical and skip flag is enabled.")
						return
					}
				} else {
					logger.LogInfo.Println("JOB: Database changes detected. Will create new backup.")

					if err := db.HandleBackupFile(config.AppEnv.Global.DBPath); err != nil {
						logger.LogErr.Println("JOB: Failed to handle backup file! (" + err.Error() + ")")
						return
					}

					if err := db.BackupToFile(ctx, config.AppEnv.Global.DBPath); err != nil {
						logger.LogErr.Println("JOB: Failed to backup database! (" + err.Error() + ")")
						return
					}
				}
			}
		} else {
			logger.LogInfo.Println("JOB: No existing database file found. Will create a new one.")
		}
	}

	if config.AppEnv.Global.EnableConcurrency {
		generateListsConcurrently(ctx, getListConfig())
	} else {
		generateListsSequentially(ctx, getListConfig())
	}
}

func generateListsSequentially(ctx context.Context, listConfigs []list.ListConfig) {
	for _, listConfig := range listConfigs {
		if err := list.GenerateList(ctx, listConfig); err != nil {
			logger.LogErr.Println("Failed to generate list: " + logger.Explain(listConfig) + " (" + err.Error() + ")")
		}
	}
}

func generateListsConcurrently(ctx context.Context, listConfigs []list.ListConfig) {
	var wg sync.WaitGroup

	jobs := make(chan list.ListConfig, len(listConfigs))

	for range config.AppEnv.Global.NumOfWorker {
		wg.Add(1)
		go worker(ctx, &wg, jobs)
	}

	for _, listConfig := range listConfigs {
		jobs <- listConfig
	}
	close(jobs)

	wg.Wait()
}

func worker(ctx context.Context, wg *sync.WaitGroup, listConfigs <-chan list.ListConfig) {
	defer wg.Done()

	for listConfig := range listConfigs {
		select {
		case <-ctx.Done():
			return
		default:
			if err := list.GenerateList(ctx, listConfig); err != nil {
				logger.LogErr.Println("Failed to generate list: " + logger.Explain(listConfig) + " (" + err.Error() + ")")
			}
		}
	}
}

func getListConfig() []list.ListConfig {
	var listConfigs []list.ListConfig

	// Add standalone lists if enabled
	if config.AppEnv.List.CreteStandaloneLists {

		// Standalone IP Lists
		listConfigs = append(listConfigs, []list.ListConfig{
			{Type: list.TypeIP, Severity: list.SeverityAny, TimeLimit: list.Time30D},
			{Type: list.TypeIP, Severity: list.SeverityAny, TimeLimit: list.Time90D},
			{Type: list.TypeIP, Severity: list.SeverityAny, TimeLimit: list.Time180D},
			{Type: list.TypeIP, Severity: list.SeverityAny, TimeLimit: list.Time1Y},
			{Type: list.TypeIP, Severity: list.SeverityAny, TimeLimit: list.Time3Y},
			{Type: list.TypeIP, Severity: list.SeverityAny, TimeLimit: list.Time5Y},
			{Type: list.TypeIP, Severity: list.SeverityAny},
			{Type: list.TypeIP, Severity: list.SeverityHigh, TimeLimit: list.Time30D},
			{Type: list.TypeIP, Severity: list.SeverityHigh, TimeLimit: list.Time90D},
			{Type: list.TypeIP, Severity: list.SeverityHigh, TimeLimit: list.Time180D},
			{Type: list.TypeIP, Severity: list.SeverityHigh, TimeLimit: list.Time1Y},
			{Type: list.TypeIP, Severity: list.SeverityHigh, TimeLimit: list.Time3Y},
			{Type: list.TypeIP, Severity: list.SeverityHigh, TimeLimit: list.Time5Y},
			{Type: list.TypeIP, Severity: list.SeverityHigh},

			// Standalone URL Lists
			{Type: list.TypeURL, Severity: list.SeverityAny, TimeLimit: list.Time30D},
			{Type: list.TypeURL, Severity: list.SeverityAny, TimeLimit: list.Time90D},
			{Type: list.TypeURL, Severity: list.SeverityAny, TimeLimit: list.Time180D},
			{Type: list.TypeURL, Severity: list.SeverityAny, TimeLimit: list.Time1Y},
			{Type: list.TypeURL, Severity: list.SeverityAny, TimeLimit: list.Time3Y},
			{Type: list.TypeURL, Severity: list.SeverityAny, TimeLimit: list.Time5Y},
			{Type: list.TypeURL, Severity: list.SeverityAny},
			{Type: list.TypeURL, Severity: list.SeverityHigh, TimeLimit: list.Time30D},
			{Type: list.TypeURL, Severity: list.SeverityHigh, TimeLimit: list.Time90D},
			{Type: list.TypeURL, Severity: list.SeverityHigh, TimeLimit: list.Time180D},
			{Type: list.TypeURL, Severity: list.SeverityHigh, TimeLimit: list.Time1Y},
			{Type: list.TypeURL, Severity: list.SeverityHigh, TimeLimit: list.Time3Y},
			{Type: list.TypeURL, Severity: list.SeverityHigh, TimeLimit: list.Time5Y},
			{Type: list.TypeURL, Severity: list.SeverityHigh},

			// Standalone Domain Lists
			{Type: list.TypeDomain, Severity: list.SeverityAny, TimeLimit: list.Time30D},
			{Type: list.TypeDomain, Severity: list.SeverityAny, TimeLimit: list.Time90D},
			{Type: list.TypeDomain, Severity: list.SeverityAny, TimeLimit: list.Time180D},
			{Type: list.TypeDomain, Severity: list.SeverityAny, TimeLimit: list.Time1Y},
			{Type: list.TypeDomain, Severity: list.SeverityAny, TimeLimit: list.Time3Y},
			{Type: list.TypeDomain, Severity: list.SeverityAny, TimeLimit: list.Time5Y},
			{Type: list.TypeDomain, Severity: list.SeverityAny},
			{Type: list.TypeDomain, Severity: list.SeverityHigh, TimeLimit: list.Time30D},
			{Type: list.TypeDomain, Severity: list.SeverityHigh, TimeLimit: list.Time90D},
			{Type: list.TypeDomain, Severity: list.SeverityHigh, TimeLimit: list.Time180D},
			{Type: list.TypeDomain, Severity: list.SeverityHigh, TimeLimit: list.Time1Y},
			{Type: list.TypeDomain, Severity: list.SeverityHigh, TimeLimit: list.Time3Y},
			{Type: list.TypeDomain, Severity: list.SeverityHigh, TimeLimit: list.Time5Y},
			{Type: list.TypeDomain, Severity: list.SeverityHigh},

			{Type: list.TypeDomain, Severity: list.SeverityAny, CountLimit: list.Count50K},
			{Type: list.TypeDomain, Severity: list.SeverityAny, CountLimit: list.Count100K},
			{Type: list.TypeDomain, Severity: list.SeverityAny, CountLimit: list.Count150K},
			{Type: list.TypeDomain, Severity: list.SeverityAny, CountLimit: list.Count250K},
			{Type: list.TypeDomain, Severity: list.SeverityAny, CountLimit: list.Count500K},
			{Type: list.TypeDomain, Severity: list.SeverityAny, CountLimit: list.Count1M},
			{Type: list.TypeDomain, Severity: list.SeverityHigh, CountLimit: list.Count50K},
			{Type: list.TypeDomain, Severity: list.SeverityHigh, CountLimit: list.Count100K},
			{Type: list.TypeDomain, Severity: list.SeverityHigh, CountLimit: list.Count150K},
			{Type: list.TypeDomain, Severity: list.SeverityHigh, CountLimit: list.Count250K},
			{Type: list.TypeDomain, Severity: list.SeverityHigh, CountLimit: list.Count500K},
			{Type: list.TypeDomain, Severity: list.SeverityHigh, CountLimit: list.Count1M},
		}...)
	}

	// Add mix lists if enabled
	if config.AppEnv.List.CreteMixLists {

		// Mix Lists
		listConfigs = append(listConfigs, []list.ListConfig{
			{Type: list.TypeAll, Severity: list.SeverityAny, TimeLimit: list.Time30D},
			{Type: list.TypeAll, Severity: list.SeverityAny, TimeLimit: list.Time90D},
			{Type: list.TypeAll, Severity: list.SeverityAny, TimeLimit: list.Time180D},
			{Type: list.TypeAll, Severity: list.SeverityAny, TimeLimit: list.Time1Y},
			{Type: list.TypeAll, Severity: list.SeverityAny, TimeLimit: list.Time3Y},
			{Type: list.TypeAll, Severity: list.SeverityAny, TimeLimit: list.Time5Y},
			{Type: list.TypeAll, Severity: list.SeverityAny},
			{Type: list.TypeAll, Severity: list.SeverityHigh, TimeLimit: list.Time30D},
			{Type: list.TypeAll, Severity: list.SeverityHigh, TimeLimit: list.Time90D},
			{Type: list.TypeAll, Severity: list.SeverityHigh, TimeLimit: list.Time180D},
			{Type: list.TypeAll, Severity: list.SeverityHigh, TimeLimit: list.Time1Y},
			{Type: list.TypeAll, Severity: list.SeverityHigh, TimeLimit: list.Time3Y},
			{Type: list.TypeAll, Severity: list.SeverityHigh, TimeLimit: list.Time5Y},
			{Type: list.TypeAll, Severity: list.SeverityHigh},

			{Type: list.TypeAll, Severity: list.SeverityAny, CountLimit: list.Count50K},
			{Type: list.TypeAll, Severity: list.SeverityAny, CountLimit: list.Count100K},
			{Type: list.TypeAll, Severity: list.SeverityAny, CountLimit: list.Count150K},
			{Type: list.TypeAll, Severity: list.SeverityAny, CountLimit: list.Count250K},
			{Type: list.TypeAll, Severity: list.SeverityAny, CountLimit: list.Count500K},
			{Type: list.TypeAll, Severity: list.SeverityAny, CountLimit: list.Count1M},
			{Type: list.TypeAll, Severity: list.SeverityHigh, CountLimit: list.Count50K},
			{Type: list.TypeAll, Severity: list.SeverityHigh, CountLimit: list.Count100K},
			{Type: list.TypeAll, Severity: list.SeverityHigh, CountLimit: list.Count150K},
			{Type: list.TypeAll, Severity: list.SeverityHigh, CountLimit: list.Count250K},
			{Type: list.TypeAll, Severity: list.SeverityHigh, CountLimit: list.Count500K},
			{Type: list.TypeAll, Severity: list.SeverityHigh, CountLimit: list.Count1M},
		}...)
	}

	// Add aggregated lists if enabled
	if config.AppEnv.List.CreteAggregatedLists {

		// Aggregated IP Lists
		listConfigs = append(listConfigs, []list.ListConfig{
			{Type: list.TypeAggregatedIP, Severity: list.SeverityAny, TimeLimit: list.Time30D},
			{Type: list.TypeAggregatedIP, Severity: list.SeverityAny, TimeLimit: list.Time90D},
			{Type: list.TypeAggregatedIP, Severity: list.SeverityAny, TimeLimit: list.Time180D},
			{Type: list.TypeAggregatedIP, Severity: list.SeverityAny, TimeLimit: list.Time1Y},
			{Type: list.TypeAggregatedIP, Severity: list.SeverityAny, TimeLimit: list.Time3Y},
			{Type: list.TypeAggregatedIP, Severity: list.SeverityAny, TimeLimit: list.Time5Y},
			{Type: list.TypeAggregatedIP, Severity: list.SeverityAny},
			{Type: list.TypeAggregatedIP, Severity: list.SeverityHigh, TimeLimit: list.Time30D},
			{Type: list.TypeAggregatedIP, Severity: list.SeverityHigh, TimeLimit: list.Time90D},
			{Type: list.TypeAggregatedIP, Severity: list.SeverityHigh, TimeLimit: list.Time180D},
			{Type: list.TypeAggregatedIP, Severity: list.SeverityHigh, TimeLimit: list.Time1Y},
			{Type: list.TypeAggregatedIP, Severity: list.SeverityHigh, TimeLimit: list.Time3Y},
			{Type: list.TypeAggregatedIP, Severity: list.SeverityHigh, TimeLimit: list.Time5Y},
			{Type: list.TypeAggregatedIP, Severity: list.SeverityHigh},

			// Aggregated URL Lists
			{Type: list.TypeAggregatedURL, Severity: list.SeverityAny, TimeLimit: list.Time30D},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityAny, TimeLimit: list.Time90D},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityAny, TimeLimit: list.Time180D},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityAny, TimeLimit: list.Time1Y},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityAny, TimeLimit: list.Time3Y},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityAny, TimeLimit: list.Time5Y},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityAny},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityHigh, TimeLimit: list.Time30D},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityHigh, TimeLimit: list.Time90D},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityHigh, TimeLimit: list.Time180D},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityHigh, TimeLimit: list.Time1Y},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityHigh, TimeLimit: list.Time3Y},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityHigh, TimeLimit: list.Time5Y},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityHigh},

			{Type: list.TypeAggregatedURL, Severity: list.SeverityAny, CountLimit: list.Count50K},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityAny, CountLimit: list.Count100K},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityAny, CountLimit: list.Count150K},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityAny, CountLimit: list.Count250K},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityAny, CountLimit: list.Count500K},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityAny, CountLimit: list.Count1M},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityHigh, CountLimit: list.Count50K},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityHigh, CountLimit: list.Count100K},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityHigh, CountLimit: list.Count150K},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityHigh, CountLimit: list.Count250K},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityHigh, CountLimit: list.Count500K},
			{Type: list.TypeAggregatedURL, Severity: list.SeverityHigh, CountLimit: list.Count1M},

			// Aggregated Domain Lists
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityAny, TimeLimit: list.Time30D},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityAny, TimeLimit: list.Time90D},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityAny, TimeLimit: list.Time180D},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityAny, TimeLimit: list.Time1Y},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityAny, TimeLimit: list.Time3Y},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityAny, TimeLimit: list.Time5Y},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityAny},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityHigh, TimeLimit: list.Time30D},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityHigh, TimeLimit: list.Time90D},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityHigh, TimeLimit: list.Time180D},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityHigh, TimeLimit: list.Time1Y},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityHigh, TimeLimit: list.Time3Y},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityHigh, TimeLimit: list.Time5Y},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityHigh},

			{Type: list.TypeAggregatedDomain, Severity: list.SeverityAny, CountLimit: list.Count50K},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityAny, CountLimit: list.Count100K},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityAny, CountLimit: list.Count150K},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityAny, CountLimit: list.Count250K},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityAny, CountLimit: list.Count500K},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityAny, CountLimit: list.Count1M},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityHigh, CountLimit: list.Count50K},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityHigh, CountLimit: list.Count100K},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityHigh, CountLimit: list.Count150K},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityHigh, CountLimit: list.Count250K},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityHigh, CountLimit: list.Count500K},
			{Type: list.TypeAggregatedDomain, Severity: list.SeverityHigh, CountLimit: list.Count1M},
		}...)
	}

	return listConfigs
}
