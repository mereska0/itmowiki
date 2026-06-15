package autocomplete

import (
	"bufio"
	"log"
	"os"
	"strings"
)

/*reads data  and counts probabilities according to language*/
func ProcessData(lang string) map[string]map[string]int {
	probabilities := make(map[string]map[string]int)

	data, err := os.Open("statistics/" + lang + ".txt")
	if err != nil {
		log.Fatal(err)
	}
	defer data.Close()

	scanner := bufio.NewScanner(data)
	scanner.Split(bufio.ScanWords)

	var prevprevprev string
	var prevprev string
	var prev string
	count := 0
	for scanner.Scan() {
		word := strings.ToLower(scanner.Text())
		word = strings.Trim(word, ".,!?;:()[]{}\"'«»“”‘’—-…")
		if count >= 1 {
			context := prev

			if probabilities[context] == nil {
				probabilities[context] = make(map[string]int)
			}
			probabilities[context][word]++
		}
		if count >= 2 {
			context := prevprev + " " + prev

			if probabilities[context] == nil {
				probabilities[context] = make(map[string]int)
			}

			probabilities[context][word]++
		}
		if count >= 3 {
			context := prevprevprev + " " + prevprev + " " + prev

			if probabilities[context] == nil {
				probabilities[context] = make(map[string]int)
			}
			probabilities[context][word]++
		}
		prevprevprev = prevprev
		prevprev = prev
		prev = word
		count++
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return probabilities
}
