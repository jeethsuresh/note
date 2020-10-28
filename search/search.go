package search

import (
	"database/sql"
	"io/ioutil"
	"log"
	"regexp"
	"strings"

	"github.com/gobuffalo/envy"
	_ "github.com/mattn/go-sqlite3"
)

// SearchForString ...
func SearchForString(tokens string) (map[string][]string, error) {
	tokenizedString := tokenize(tokens)
	documents := map[string][]string{}
	for _, v := range tokenizedString {
		documentEntries := findAllDocumentsThatContain(v)
		for _, doc := range documentEntries {
			documents[doc] = append(documents[doc], v)
		}
	}
	toreturn := map[string][]string{}

	for doc := range documents {
		toreturn[doc] = fetchDocumentExcerpts(doc, documents[doc])
	}

	return toreturn, nil
}

//need db tables from analysis

func tokenize(tokens string) []string {

	tokenArr := strings.Split(tokens, " ")
	cleanTokens := []string{}
	for _, token := range tokenArr {
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
		cleanTokens = append(cleanTokens, token)
	}
	return cleanTokens
}

func findAllDocumentsThatContain(token string) (toreturn []string) {
	sqlStmt := `SELECT document FROM tokens WHERE token = $1 COLLATE NOCASE`

	db, err := sql.Open("sqlite3", envy.Get("HOME", "")+"/notes/note.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(sqlStmt, token)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var document string
		err = rows.Scan(&document)
		if err != nil {
			log.Fatal(err)
		}

		toreturn = append(toreturn, document)
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	return toreturn
}

func fetchDocumentExcerpts(document string, keywords []string) (toreturn []string) {
	content, err := ioutil.ReadFile(envy.Get("HOME", "") + "/notes/" + document + ".txt")
	if err != nil {
		log.Fatal(err)
	}

	text := string(content)

	// Make a Regex to say we only want letters and numbers
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		log.Fatal(err)
	}
	processedString := reg.ReplaceAllString(text, " ")

	splitText := strings.Split(processedString, " ")

	countMap := map[string]int{}

	for _, word := range keywords {
		skipCount := countMap[word]
		for idx, text := range splitText {
			if strings.EqualFold(text, word) {
				if skipCount == 0 {
					finalstring := "..."
					if idx > 3 {
						finalstring = finalstring + splitText[idx-2] + " " + splitText[idx-1] + " "
					}
					finalstring = finalstring + splitText[idx] + " "
					if idx+3 < len(splitText) {
						finalstring = finalstring + splitText[idx+1] + " " + splitText[idx+2]
					}
					finalstring = finalstring + "..."
					toreturn = append(toreturn, finalstring)
				} else {
					skipCount--
					break
				}
			}
		}
		countMap[word]++
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
