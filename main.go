package main

import (
	"fmt"
	"io"
	"time"

	"github.com/enginy88/PAN-USOM-API2EDL/config"
	"github.com/enginy88/PAN-USOM-API2EDL/job"
	"github.com/enginy88/PAN-USOM-API2EDL/logger"
)

func main() {

	start := time.Now()
	logger.LogAlways.Println("HELLO MSG: Welcome to " + logger.AppName + " v1.0 by EY!")

	_ = config.GetAppFlag()
	_ = config.GetAppEnv()

	if !config.AppEnv.Log.Verbose {
		logger.LogInfo.SetOutput(io.Discard)
	}

	job.RunAllJobs()

	duration := fmt.Sprintf("%.1f", time.Since(start).Seconds())
	logger.LogAlways.Println("BYE MSG: All done in " + duration + "s, bye!")

}
