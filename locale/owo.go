package locale

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

func Owoify(input string) string {
	pieces := strings.Split(input, "{{")
	full := owoifyString(pieces[0])
	if len(pieces) > 1 {
		//NOTE will fail for strings with {{ but no matching }}
		sub := strings.Split(pieces[1], "}}")
		full += "{{" + sub[0] + "}}" + Owoify(sub[1])
	}
	return full
}

func owoifyString(input string) string {
	output := strings.ReplaceAll(input, "th", "d")
	output = strings.ReplaceAll(output, "ove", "uv")
	re := regexp.MustCompile(`(?:r|l)`)
	output = string(re.ReplaceAll([]byte(output), []byte("w")))

	re = regexp.MustCompile(`(?:R|L)`)
	output = string(re.ReplaceAll([]byte(output), []byte("W")))

	re = regexp.MustCompile(`n([aeiou])`)
	output = string(re.ReplaceAll([]byte(output), []byte("ny$1")))

	re = regexp.MustCompile(`N([aeiou])`)
	output = string(re.ReplaceAll([]byte(output), []byte("Ny$1")))

	re = regexp.MustCompile(`N([AEIOU])`)
	output = string(re.ReplaceAll([]byte(output), []byte("NY$1")))

	return output
}

func OwoToml(path, output string) {
	f, err := os.Open(path)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()

	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		log.Println(err)
		return
	}

	outputfile, err := os.Create(output)
	if err != nil {
		log.Println(err)
		return
	}
	defer outputfile.Close()

	lines := strings.Split(string(bytes), "\n")
	for _, line := range lines {
		arr := strings.Split(line, " = ")
		if len(arr) > 1 {
			text := arr[1][1 : len(arr[1])-2]
			text = strings.ReplaceAll(text, "\n", "")
			text = strings.ReplaceAll(text, "\r", "")
			outputfile.WriteString(fmt.Sprintf("%s = \"%s\"\n", arr[0], Owoify(text)))
		}
	}

}
