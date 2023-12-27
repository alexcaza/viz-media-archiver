package main

import (
	"encoding/json"
	// "fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	// "sort"
	"strconv"
	"strings"
	"time"

	api "viz-media/viz_api"
	// "github.com/manifoldco/promptui"
)

type Series struct {
	Series []string `json:"series"`
}

func createDirsFromPath(path []string) string {
	dirPath := "./data" + strings.Join(path[:len(path)-1], "/")
	err := os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to created path %s", dirPath)
	}
	return dirPath
}

func writeZip(path string, data io.ReadCloser) (int64, error) {
	out, err := os.Create(path)

	if err != nil {
		log.Fatalf("Failed to create zip file: %s", path)
	}

	defer out.Close()

	return io.Copy(out, data)
}

func fetchZip(zipLocation string) {
	zipLoc := zipLocation

	zipResp, _ := http.Get(zipLoc)

	defer zipResp.Body.Close()

	u, _ := url.Parse(zipLoc)

	pathFragments := strings.Split(u.Path, "/")
	fileName := pathFragments[len(pathFragments)-1]
	outputPath := createDirsFromPath(pathFragments)
	filePath := outputPath + "/" + fileName

	_, err := writeZip(filePath, zipResp.Body)

	if err != nil {
		log.Fatalf("Failed to write file (%s) to disk.", filePath)
	}
}

func getSeriesInfo() []string {
	var output Series
	seriesJSON, err := os.Open("series.json")

	if err != nil {
		log.Fatal("Failed to open series.json. Make sure the file is present!")
	}

	defer seriesJSON.Close()

	seriesBytes, _ := io.ReadAll(seriesJSON)

	json.Unmarshal(seriesBytes, &output)

	return output.Series
}

type SeriesListItem struct {
	Title string `json:"title"`
	Id    string `json:"id"`
}

func buildSeriesList(api api.Api) {
	log.Println("Starting to fetch...")
	var series []SeriesListItem

	file, _ := os.OpenFile("series-list.json", os.O_WRONLY, os.ModeAppend)
	defer file.Close()
	encoder := json.NewEncoder(file)

	// TODO: Get the latest id in the json file so that
	// we don't always start from 1
	i := 1
	for i < 10 {
		output := api.FetchSeriesListing(strconv.Itoa(i))
		if output.Data == nil {
			log.Println("No output: ", output)
			i++
			// Wait 5s before trying the next id
			time.Sleep(5 * time.Second)
			continue
		}
		data := output.Data

		first := data[0]
		seriesTitle := first.Manga.SeriesTitle
		series = append(series, SeriesListItem{Title: seriesTitle, Id: strconv.Itoa(i)})
		// Wait 5s before trying the next id
		time.Sleep(5 * time.Second)
		i += 1
	}

	err := encoder.Encode(series)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	api := api.NewApi()
	buildSeriesList(api)
	// prompt := promptui.Select{
	// 	Label: "Select series to download chapters from",
	// 	Items: getSeriesInfo(),
	// }
	//
	// _, result, err := prompt.Run()
	//
	// if err != nil {
	// 	log.Fatal("Failed to get selection from prompt")
	// }
	//
	// selectionNumber := strings.TrimSpace(strings.Split(result, "|")[1])
	//
	// fmt.Printf("Selection: %q\n", result)
	//
	// output := fetchSeriesListing(selectionNumber)
	//
	// fetchOk := output["ok"].(float64)
	//
	//
	// if fetchOk != 1 {
	// 	log.Fatal("Failed to get data for selected series.")
	// }
	//
	// data := output["data"].([]interface{})
	//
	// var chapters []map[string]interface{}
	// for _, v := range data {
	// 	manga := v.(map[string]interface{})["manga"].(map[string]interface{})
	// 	if manga["web_price"] == nil && manga["published"] == true {
	// 		chapters = append(chapters, manga)
	// 	}
	// }
	//
	// sort.Slice(chapters, func(i, j int) bool {
	// 	d1, _ := time.Parse(time.RFC3339, chapters[i]["publication_date"].(string))
	// 	d2, _ := time.Parse(time.RFC3339, chapters[j]["publication_date"].(string))
	// 	return d1.After(d2)
	// })
	//
	// var t []string
	// for _, v := range chapters {
	// 	chapterNum := v["chapter"].(string)
	// 	id := v["manga_common_id"].(float64)
	// 	deviceId := v["device_id"].(float64)
	// 	t = append(t, chapterNum + " " + "| ("+fmt.Sprintf("%.0f", id)+"~"+fmt.Sprintf("%.0f", deviceId)+")")
	// }
	//
	// prompt = promptui.Select{
	// 	Label: "Which chapter do you want to download?",
	// 	Items: t,
	// }
	//
	// _, result, err = prompt.Run()
	//
	// if err != nil {
	// 	log.Fatal("Chapter selection failed.")
	// }
	//
	// fetchData := strings.Split(strings.Split(result, "|")[1], "~")
	// chapterId := strings.Trim(strings.TrimSpace(fetchData[0]), "()")
	// deviceId := strings.Trim(strings.TrimSpace(fetchData[1]), "()")
	//
	// mangaLoc := fetchZipLocation(chapterId, deviceId)
	// fetchZip(mangaLoc.Data)
	//
}
