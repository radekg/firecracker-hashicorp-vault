package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)

func main() {

	goodConfig := flag.String("good-config", "./4.14.174.config", "Last known good config file path")
	newConfig := flag.String("new-config", "", "New config to check against good config file path")

	bringOldNonExisting := flag.Bool("bring-old-non-existing", false, "When true, brings non-existing configs from old config to new config: destructive action")

	flag.Parse()

	config1bytes, err := ioutil.ReadFile(*goodConfig)
	if err != nil {
		panic(err)
	}
	config2bytes, err := ioutil.ReadFile(*newConfig)
	if err != nil {
		panic(err)
	}

	config1lines := strings.Split(string(config1bytes), "\n")
	config2lines := strings.Split(string(config2bytes), "\n")

	config1map := map[string]string{}
	config2map := map[string]string{}

	oldToBring := map[string]string{}

	for _, line := range config1lines {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		key := line[0:strings.Index(line, "=")]
		value := strings.TrimSpace(line[strings.Index(line, "=")+1:])
		config1map[key] = value
	}

	for _, line := range config2lines {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		key := line[0:strings.Index(line, "=")]
		value := strings.TrimSpace(line[strings.Index(line, "=")+1:])
		config2map[key] = value
	}

	for k, v := range config1map {
		_, ok := config2map[k]
		if !ok {
			fmt.Println(fmt.Sprintf("OLD NOT FOUND: '%s' exists in old config but not found in new config", k))
			oldToBring[k] = v
		}
	}

	for k := range config2map {
		_, ok := config1map[k]
		if !ok {
			fmt.Println(fmt.Sprintf("NEW: '%s' exists in new config but not found in old config", k))
		}
	}

	for k, v := range config1map {
		v2, ok := config2map[k]
		if ok && v2 != v {
			fmt.Println(fmt.Sprintf("DIFFERENT old vs new config: '%s': '%s' != '%s'", k, v, v2))
		}
	}

	if *bringOldNonExisting {
		makeBackup(*newConfig, fmt.Sprintf("%s.backup-%d", *newConfig, time.Now().Unix()))
		lines := []string{}
		for k, v := range oldToBring {
			lines = append(lines, fmt.Sprintf("%s=%s", k, v))
		}
		appendToFile(*goodConfig, *newConfig, lines)
	}

}

func makeBackup(src, dest string) {
	input, err := ioutil.ReadFile(src)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(dest, input, 0644)
	if err != nil {
		panic(err)
	}
}

func appendToFile(source, dest string, lines []string) {
	template := `

#
# These properties have been imported from good known config '%s'
# using the compare-configs.go utility.
# The properties have been found in last known good config
# but did not exist in new config.
#
%s`

	contentToAppend := fmt.Sprintf(template, source, strings.Join(lines, "\n"))
	f, err := os.OpenFile(dest, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if _, err := f.WriteString(fmt.Sprintf("%s\n", contentToAppend)); err != nil {
		log.Println(err)
	}
}
