package config

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/enginy88/PAN-USOM-API2EDL/logger"
)

type AppFlagStruct struct {
	WorkingDir    string
	OutputDir     string
	workingDirRaw string
	outputDirRaw  string
}

var AppFlag *AppFlagStruct

func GetAppFlag() *AppFlagStruct {

	appFlagObject := new(AppFlagStruct)
	AppFlag = appFlagObject

	parseAppFlag()
	changeWorkingDir()

	return AppFlag

}

func parseAppFlag() {

	workingDir := flag.String("dir", "", "Path of the directory where the '"+logger.AppName+".env' file is located, and where the EDL file(s) will also be created.")
	outputgDir := flag.String("out", "", "Path of the directory where the EDL file(s) will be created. (Overrides '-dir' option.)")
	flag.Parse()

	AppFlag.workingDirRaw = *workingDir
	AppFlag.outputDirRaw = *outputgDir

}

func changeWorkingDir() {

	origDir, err := os.Getwd()
	if err != nil {
		logger.LogErr.Fatalln("FATAL ERROR: Cannot get working directory! (" + err.Error() + ")")
	}

	workingDir := origDir
	outputDir := origDir

	if AppFlag.workingDirRaw != "" {

		err := os.Chdir(AppFlag.workingDirRaw)
		if err != nil {
			logger.LogErr.Fatalln("FATAL ERROR: Cannot change working directory! (" + err.Error() + ")")
		}

		newDir, err := os.Getwd()
		if err != nil {
			logger.LogErr.Fatalln("FATAL ERROR: Cannot get working directory! (" + err.Error() + ")")
		}

		workingDir = newDir
		outputDir = newDir

		logger.LogInfo.Println("CONFIG MSG: Flag 'dir' set, changing working directory from '" + origDir + "' to '" + newDir + "'.")
	}

	if AppFlag.outputDirRaw != "" {

		if filepath.IsAbs(AppFlag.outputDirRaw) {
			outputDir = filepath.Clean(AppFlag.outputDirRaw)
		} else {
			outputDir = filepath.Join(origDir, AppFlag.outputDirRaw)
		}

		logger.LogInfo.Println("CONFIG MSG: Flag 'out' set, going to write the EDL file(s) to '" + outputDir + "' directory.")
	}

	AppFlag.WorkingDir = workingDir
	AppFlag.OutputDir = outputDir

}
