package list

import (
	"regexp"
	"strings"

	"github.com/enginy88/PAN-USOM-API2EDL/logger"
)

type regexType string

const (
	regexTypeIP     regexType = "ip"
	regexTypeURL    regexType = "url"
	regexTypeDomain regexType = "domain"
)

var regex *regexp.Regexp

func prepareRegex() {

	if regex != nil {
		return
	}

	regexPattern := `^(?P<leading_whitespace>[^\S\r\n]*)?(?P<scheme>http[s]?://)?(?:(?P<domain>[^\s/?#]+\.[^0-9\s\./?#:]+[^\s\./?#:]*)|(?P<ip>(?:[0-9]{1,3}\.){3}[0-9]{1,3}))(?P<port>:[0-9]{1,5})?(?:(?P<root>/+)(?P<path>[^\s?#]*))?(?:(?P<query>\?[^\s#]*)?(?P<fragment>#.*)?)?(?P<trailing_whitespace>[^\S\r\n]*)?$`

	var err error
	regex, err = regexp.Compile(regexPattern)
	if err != nil {
		logger.LogErr.Fatalln("REGEX: Error compiling regex pattern: '" + regexPattern + "'! (" + err.Error() + ")")
	}

}

func validateWithRegex(record string, rtype regexType, force bool) string {

	if regex == nil {
		prepareRegex()
	}

	ipIndex := regex.SubexpIndex("ip")
	domainIndex := regex.SubexpIndex("domain")
	rootIndex := regex.SubexpIndex("root")
	pathIndex := regex.SubexpIndex("path")

	if ipIndex == -1 || domainIndex == -1 || pathIndex == -1 {
		logger.LogErr.Println("REGEX: Error getting regex subexpression index!")
	}

	matches := regex.FindStringSubmatch(record)

	if matches == nil {
		logger.LogWarn.Println("REGEX: Cannot parse the record: '" + record + "'!")
		return ""
	}

	switch rtype {
	case regexTypeIP:
		if matches[ipIndex] == "" {
			if !force {
				logger.LogInfo.Println("REGEX: IP match not found for the record: '" + record + "'!")
			}
			return ""
		}
		return matches[ipIndex]
	case regexTypeURL:
		if matches[rootIndex] == "" && !force {
			logger.LogInfo.Println("REGEX: URL match not found for the record: '" + record + "'!")
			return ""
		}

		var url string
		if matches[ipIndex] != "" {
			url = matches[ipIndex] + "/" + matches[pathIndex]
		} else if matches[domainIndex] != "" {
			url = matches[domainIndex] + "/" + matches[pathIndex]
		} else {
			return ""
		}

		// Ensure the URL ends with "/"
		if !strings.HasSuffix(url, "/") {
			url += "/"
		}

		return url
	case regexTypeDomain:
		if matches[domainIndex] == "" {
			if !force {
				logger.LogInfo.Println("REGEX: Domain match not found for the record: '" + record + "'!")
			}
			return ""
		}
		return matches[domainIndex]
	default:
		logger.LogErr.Println("REGEX: Unknown regex type: '" + string(rtype) + "'! (" + record + ")")
		return ""
	}

}
