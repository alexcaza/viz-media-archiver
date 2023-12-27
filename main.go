package main

import (
	"encoding/json"
	"slices"
	"sort"

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

	goarg "github.com/alexflint/go-arg"
	// "github.com/manifoldco/promptui"
)

type WatchListItem struct {
	Title string `json:"title"`
	Id    string `json:"id"`
}

var args struct {
	GenListing bool  `arg:"--generate-listing" help:"Generates list of series."`
	AddToWatch []int `arg:"--to-watch" help:"Adds ids to list of series to watch. Must be in series-list.json"`
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

	// TODO: Add unzipping and better file structure
	// so that it could be easily served from a website statically.

	return io.Copy(out, data)
}

func fetchZip(zipLocation string) {
	zipLoc := zipLocation

	zipResp, httpErr := http.Get(zipLoc)

	if httpErr != nil {
		log.Fatal(httpErr)
	}

	defer zipResp.Body.Close()

	u, _ := url.Parse(zipLoc)

	pathFragments := strings.Split(u.Path, "/")
	fileName := pathFragments[len(pathFragments)-1]
	outputPath := createDirsFromPath(pathFragments)
	filePath := outputPath + "/" + fileName

	_, writeErr := writeZip(filePath, zipResp.Body)

	if writeErr != nil {
		log.Fatalf("Failed to write file (%s) to disk.", filePath)
	}
}

func buildSeriesList(api api.Api) {
	log.Println("Starting to fetch.")
	MAX_ID := 1000
	// TODO: Get the latest id in the json file so that
	// we don't always start from 1
	id := 1
	sleepTime := 1 * time.Second
	var series []WatchListItem

	file, _ := os.OpenFile("series-list.json", os.O_WRONLY, os.ModeAppend)
	defer file.Close()
	encoder := json.NewEncoder(file)

	for id < MAX_ID {
		output := api.FetchSeriesChapters(strconv.Itoa(id))
		if output.Data == nil {
			log.Printf("The id %d doesn't exist. Skipping...", id)
			id++

			// Wait n seconds before trying the next id
			time.Sleep(sleepTime)
			continue
		}
		data := output.Data

		first := data[0]
		seriesTitle := first.Manga.SeriesTitle
		log.Printf("Found series at %d; Name: %s\n", id, seriesTitle)
		series = append(series, WatchListItem{Title: seriesTitle, Id: strconv.Itoa(id)})
		id++

		// Wait n seconds before trying the next id
		time.Sleep(sleepTime)
	}

	err := encoder.Encode(series)
	log.Printf("Finished! Found %d titles.\n", len(series))
	if err != nil {
		log.Fatal(err)
	}
}

func getSeriesList() []WatchListItem {
	var seriesList []WatchListItem
	file, fileerr := os.ReadFile("series-list.json")
	if fileerr != nil {
		log.Fatal(fileerr)
	}

	err := json.Unmarshal(file, &seriesList)
	if err != nil {
		log.Fatal(err)
	}
	return seriesList
}

func addToWatch(items []WatchListItem) {
	var watchingList []WatchListItem
	// Get contents
	contents, _ := os.Open("to-watch.json")
	bytes, _ := io.ReadAll(contents)

	// Open file for writing and truncate
	file, _ := os.Create("to-watch.json")
	defer file.Close()
	encoder := json.NewEncoder(file)

	if len(bytes) > 0 {
		jsonErr := json.Unmarshal(bytes, &watchingList)
		if jsonErr != nil {
			log.Fatal("JSON err:", jsonErr)
		}
	}
	for _, item := range items {
		if len(watchingList) < 1 {
			watchingList = append(watchingList, item)
			continue
		}

		for range watchingList {
			if !slices.Contains(watchingList, item) {
				watchingList = append(watchingList, item)
			}
		}
	}

	err := encoder.Encode(watchingList)
	if err != nil {
		log.Fatal("Encoding error:", err)
	}
}

func updateWatchList(api api.Api) {
	var watchList []WatchListItem
	contents, _ := os.Open("to-watch.json")
	bytes, _ := io.ReadAll(contents)
	json.Unmarshal(bytes, &watchList)
	// Need to check watch list, look at latest chapter in dir
	// then find all missing chapters
	log.Println("Starting downloads...")
	for _, item := range watchList {
		log.Printf("Fetching manga %s\n", item.Title)
		listings := api.FetchSeriesChapters(item.Id)
		sort.Slice(listings.Data, func(i, j int) bool {
			d1, _ := time.Parse(time.RFC3339, listings.Data[i].Manga.PublicationDate)
			d2, _ := time.Parse(time.RFC3339, listings.Data[j].Manga.PublicationDate)
			return d1.After(d2)
		})
		for _, chapter := range listings.Data {
			if !chapter.Manga.Published && chapter.Manga.WebPrice != "" {
				continue
			}

			log.Printf("Getting chapter %s (id: %d)\n", chapter.Manga.Chapter, chapter.Manga.MangaCommonId)
			location := api.FetchZipLocation(strconv.Itoa(chapter.Manga.MangaCommonId))
			fetchZip(location.Data)
			// Wait 5s before downloading next chapter
			time.Sleep(5 * time.Second)
		}

	}
}

func main() {
	goarg.MustParse(&args)
	api := api.NewApi()

	if args.GenListing {
		buildSeriesList(api)
	}

	if len(args.AddToWatch) > 0 {
		seriesList := getSeriesList()
		var toWatch []WatchListItem
		for _, id := range args.AddToWatch {
			for _, seriesListItem := range seriesList {
				if seriesListItem.Id == strconv.Itoa(id) {
					toWatch = append(toWatch, seriesListItem)
				}
			}
		}
		addToWatch(toWatch)
	}

	// Check for updates and download files
	updateWatchList(api)

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
