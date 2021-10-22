package main

import (
	"bufio"
	"log"
	"os"
	"regexp"
	"strconv"
)

const SecondsInMinute = 60
const MinutesInHour = 60
const HoursInDay = 24
const DaysInWeek = 7
const DaysInYear = 365

// Allows overriding settings on a per-metrics path level
type overrideData struct {
	pattern                 *regexp.Regexp
	retention               []retentionItem
	maxDryMessagesThreshold uint64
	allowUnmodified         bool

	retentionActive               bool
	maxDryMessagesThresholdActive bool
	allowUnmodifiedActive         bool
}

type retentionItem struct {
	resolution  int
	persistence int
}

func getStorageSchemaFromFile(filename string) map[string]overrideData {
	text := getFileLineData(filename)
	iniData := getFieldsFromLineData(text)
	storageSchema := extractStorageSchemaFields(iniData)
	return storageSchema
}

func extractStorageSchemaFields(iniData map[string]map[string]string) map[string]overrideData {

	secondsInRetentionTimeSuffix := map[string]int{
		"":  1,
		"s": 1,
		"m": SecondsInMinute,
		"h": SecondsInMinute * MinutesInHour,
		"d": SecondsInMinute * MinutesInHour * HoursInDay,
		"w": SecondsInMinute * MinutesInHour * HoursInDay * DaysInWeek,
		"y": SecondsInMinute * MinutesInHour * HoursInDay * DaysInYear,
	}

	// storage-schemas.conf pattern, meant to be used recursively to resolve all retention chunks
	retentionRatePattern, _ := regexp.Compile(`^(\d+)([smhdwy]|):(\d+)([smhdwy]|)(?:,(\d+(?:[smhdwy]|):\d+(?:[smhdwy]|)(?:,\d+(?:[smhdwy]|):\d+(?:[smhdwy]|))*))?$`)
	iniBooleanPattern, _ := regexp.Compile(`^(?:(1|on|true|yes)|(0|off|false|no|none))$`)
	iniIntegerPattern, _ := regexp.Compile(`^-?\d+$`)

	// var outputThing []overrideData
	// var outputThing map[string]overrideData
	outputThing := make(map[string]overrideData)
	for section, sectionData := range iniData {
		var currentRetentionItem overrideData

		// Verify that pattern exists and compile in struct
		if patternText, ok := sectionData["pattern"]; ok {
			currentRetentionItem.pattern = regexp.MustCompile(patternText)
		} else {
			log.Println(`Missing key "pattern" in section "` + section + `"`)
			os.Exit(1)
		}

		// Verify that retentions exists and store them in struct
		if retentionText, ok := sectionData["retentions"]; ok {
			retentionResult := retentionRatePattern.FindStringSubmatch(retentionText)
			if len(retentionResult) > 0 {
				for retentionResult != nil {
					var item retentionItem
					timeResolution, _ := strconv.Atoi(retentionResult[1])
					timeToKeep, _ := strconv.Atoi(retentionResult[3])
					item.resolution = timeResolution * secondsInRetentionTimeSuffix[retentionResult[2]]
					item.persistence = timeToKeep * secondsInRetentionTimeSuffix[retentionResult[4]]
					currentRetentionItem.retention = append(currentRetentionItem.retention, item)
					retentionResult = retentionRatePattern.FindStringSubmatch(retentionResult[5])
				}
			} else {
				log.Println(`Invalid or missing value for "retentions" in section "` + section + `"`)
				os.Exit(1)
			}
			currentRetentionItem.retentionActive = true
		} else {
			currentRetentionItem.retentionActive = false
		}

		if maxDryMessagesThresholdText, ok := sectionData["maxdrymessages"]; ok {
			currentRetentionItem.maxDryMessagesThresholdActive = true
			maxDryMessagesThresholdTextResult := iniIntegerPattern.FindStringSubmatch(maxDryMessagesThresholdText)
			if len(maxDryMessagesThresholdTextResult) > 0 {
				thresholdValue, _ := strconv.Atoi(maxDryMessagesThresholdTextResult[0])
				currentRetentionItem.maxDryMessagesThreshold = uint64(thresholdValue)
			} else {
				log.Println(`Invalid or missing value for "maxdrymessages" in section "` + section + `"`)
				os.Exit(1)
			}
		} else {
			currentRetentionItem.maxDryMessagesThresholdActive = false
		}

		// Handle if the metrics path is allowUnmodified
		if allowUnmodifiedText, ok := sectionData["allowunmodified"]; ok {
			currentRetentionItem.allowUnmodifiedActive = true
			allowUnmodifiedResult := iniBooleanPattern.FindStringSubmatch(allowUnmodifiedText)
			if len(allowUnmodifiedResult) > 0 {
				currentRetentionItem.allowUnmodified = (allowUnmodifiedResult[2] == "")
			} else {
				log.Println(`Invalid or missing value for "allowUnmodified" in section "` + section + `"`)
				os.Exit(1)
			}
		} else {
			currentRetentionItem.allowUnmodifiedActive = false
		}

		outputThing[section] = currentRetentionItem
	}
	return outputThing
}

func getFieldsFromLineData(text []string) map[string]map[string]string {
	// INI file patterns
	sectionPattern := regexp.MustCompile(`^\s*\[+\s*([^\]\n]+?)\s*\]+\s*(?:[;#].*)?$`)
	keyPattern := regexp.MustCompile(`^\s*(\S+)[^\S\n]*=[^\S\n]*([^;#\s](?:[^;#\n]|[;#])*)[^\S\n]*(?:[;#].*)?$`)
	irrelevantDataPattern := regexp.MustCompile(`^[^\S\n]*[#;].*|^\s*$`)

	var iniData = map[string]map[string]string{}
	var currentSection string

	for lineNumber, line := range text {
		section := sectionPattern.FindStringSubmatch(line)
		key := keyPattern.FindStringSubmatch(line)

		if section != nil {
			currentSection = section[1]
			var emptyMap = map[string]string{}
			iniData[section[1]] = emptyMap
		} else if key != nil {
			iniData[currentSection][key[1]] = key[2]
		} else if irrelevantDataPattern.FindStringSubmatch(line) != nil {
		} else {
			log.Println(`Invalid content on line ` + strconv.Itoa(lineNumber+1) + `: "` + line + `"`)
			os.Exit(1)
		}
	}
	return iniData
}

func getFileLineData(filename string) []string {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("failed to open file \"" + filename + "\"")
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	var text []string
	for scanner.Scan() {
		text = append(text, scanner.Text())
	}
	return text
}
