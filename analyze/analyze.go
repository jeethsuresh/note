package analyze

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/gobuffalo/envy"
)

func AnalyzeFile(currFile string) {
	fmt.Println("******analyzing file")

	tokens := tokenizeFile(currFile)

	fmt.Println("********file tokenized")
	fileMap := topicModelling(tokens)
	fmt.Println("*********file mapped")
	fmt.Printf("%+v\n", fileMap)
	sendToDatabase(currFile, fileMap)
}

func tokenizeFile(file string) (toreturn []string) {
	content, err := ioutil.ReadFile(envy.Get("HOME", "") + "/notes/" + file + ".txt")
	if err != nil {
		log.Fatal(err)
	}

	// Convert []byte to string and print to screen
	text := string(content)

	splitString := strings.Split(text, " ")
	for _, str := range splitString {
		toreturn = append(toreturn, strings.TrimSpace(str))
	}
	return toreturn
}

var mostCommonWords = []string{
	"the",
	"be",
	"to",
	"of",
	"and",
	"a",
	"in",
	"that",
	"have",
	"I",
	"it",
	"for",
	"not",
	"on",
	"with",
	"he",
	"as",
	"you",
	"do",
	"at",
	"this",
	"but",
	"his",
	"by",
	"from",
	"they",
	"we",
	"say",
	"her",
	"she",
	"or",
	"an",
	"will",
	"my",
	"one",
	"all",
	"would",
	"there",
	"their",
	"what",
	"so",
	"up",
	"out",
	"if",
	"about",
	"who",
	"get",
	"which",
	"go",
	"me",
	"when",
	"make",
	"can",
	"like",
	"time",
	"no",
	"just",
	"him",
	"know",
	"take",
	"people",
	"into",
	"year",
	"your",
	"good",
	"some",
	"could",
	"them",
	"see",
	"other",
	"than",
	"then",
	"now",
	"look",
	"only",
	"come",
	"its",
	"over",
	"think",
	"also",
	"back",
	"after",
	"use",
	"two",
	"how",
	"our",
	"work",
	"first",
	"well",
	"way",
	"even",
	"new",
	"want",
	"because",
	"any",
	"these",
	"give",
	"day",
	"most",
	"us",
}

func topicModelling(tokens []string) map[string]int {
	finalMap := map[string]int{}
	for _, token := range tokens {
		wordIsCommon := false
		for _, word := range mostCommonWords {
			if strings.Contains(token, word) && len(word)*2 > len(token) {
				wordIsCommon = true
				break
			}
		}
		if wordIsCommon {
			continue
		}
		finalMap[token]++
	}

	return finalMap
}

func sendToDatabase(document string, token map[string]int) {
	sqlStmt := `INSERT INTO tokens(token, document, count) VALUES ($1, $2, $3) ON CONFLICT(token, document) DO UPDATE SET count=$3`

	db, err := sql.Open("sqlite3", envy.Get("HOME", "")+"/notes/note.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	for key := range token {
		_, err = db.Exec(sqlStmt, key, document, token[key])
		if err != nil {
			log.Fatal(err)
		}
	}

}
