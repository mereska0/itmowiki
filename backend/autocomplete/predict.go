package autocomplete

import (
	"strings"
)

var models = make(map[string]map[string]map[string]int)

func getModel(lang string) map[string]map[string]int {
	if models[lang] == nil {
		models[lang] = ProcessData(lang)
	}

	return models[lang]
}

/* returns array of 3 the most relevant variants of input */
func Predict(lang string, input string) []string {
	probabilities := getModel(lang)

	tokenizedInput := strings.Fields(input)

	if len(tokenizedInput) < 1 {
		return []string{}
	}

	context1 := tokenizedInput[len(tokenizedInput)-1]

	context2 := ""
	if len(tokenizedInput) >= 2 {
		context2 = tokenizedInput[len(tokenizedInput)-2] + " " + tokenizedInput[len(tokenizedInput)-1]
	}

	context3 := ""
	if len(tokenizedInput) >= 3 {
		context3 = tokenizedInput[len(tokenizedInput)-3] + " " +
			tokenizedInput[len(tokenizedInput)-2] + " " +
			tokenizedInput[len(tokenizedInput)-1]
	}

	var nextWords map[string]int

	if context3 != "" {
		nextWords = probabilities[context3]
	}

	if len(nextWords) == 0 && context2 != "" {
		nextWords = probabilities[context2]
	}

	if len(nextWords) == 0 {
		nextWords = probabilities[context1]
	}

	if len(nextWords) == 0 {
		return []string{}
	}

	bestChoice1 := ""
	bestChoice2 := ""
	bestChoice3 := ""

	bestCount1 := 0
	bestCount2 := 0
	bestCount3 := 0

	for word, count := range nextWords {
		if count > bestCount1 {
			bestCount3 = bestCount2
			bestChoice3 = bestChoice2

			bestCount2 = bestCount1
			bestChoice2 = bestChoice1

			bestCount1 = count
			bestChoice1 = word
		} else if count > bestCount2 {
			bestCount3 = bestCount2
			bestChoice3 = bestChoice2

			bestCount2 = count
			bestChoice2 = word
		} else if count > bestCount3 {
			bestCount3 = count
			bestChoice3 = word
		}
	}

	result := []string{}

	if bestChoice1 != "" {
		result = append(result, bestChoice1)
	}
	if bestChoice2 != "" {
		result = append(result, bestChoice2)
	}
	if bestChoice3 != "" {
		result = append(result, bestChoice3)
	}

	return result
}
