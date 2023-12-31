package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	api "viz-media/viz_api"

	goarg "github.com/alexflint/go-arg"
)

type WatchListItem struct {
	Title         string `json:"title"`
	Id            string `json:"id"`
	LatestChapter string `json:"latestChapter"`
	FolderName    string `json:"folderName"`
}

var args struct {
	GenListing bool  `arg:"--generate-listing" help:"Generates list of series."`
	AddToWatch []int `arg:"--to-watch" help:"Adds ids to list of series to watch. Must be in series-list.json"`
}

func createDirsFromPath(path []string) string {
	dirPath := "./data" + strings.Join(path, "/")
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

	// Write zip to disk before trying to open it again
	io.Copy(out, data)

	archive, err := zip.OpenReader(path)

	pathFragments := strings.Split(path, "/")
	folderPath := strings.Join(pathFragments[:len(pathFragments)-1], "/")

	defer archive.Close()

	jpgIndex := 0
	for _, f := range archive.File {
		extension := strings.Split(f.Name, ".")[1]
		var fileName string
		switch extension {
		case "jpg":
			fileName = strconv.Itoa(jpgIndex) + "." + extension
			jpgIndex++
		case "json":
			fileName = "metadata" + "." + extension
		default:
			fileName = f.Name
		}

		archiveFile, _ := f.Open()
		diskFile, _ := os.Create(folderPath + "/" + fileName)
		io.Copy(diskFile, archiveFile)
	}
	return 0, err
}

func fetchZip(zipLocation string, folderName string, chapterId string) {
	zipLoc := zipLocation

	zipResp, httpErr := http.Get(zipLoc)

	if httpErr != nil {
		log.Fatal(httpErr)
	}

	defer zipResp.Body.Close()

	u, _ := url.Parse(zipLoc)

	pathFragments := strings.Split(u.Path, "/")
	fileName := pathFragments[len(pathFragments)-1]
	// TODO: Fix this pathing
	path := []string{"", folderName, chapterId}
	outputPath := createDirsFromPath(path)
	log.Println(outputPath)
	filePath := outputPath + "/" + fileName

	_, writeErr := writeZip(filePath, zipResp.Body)

	if writeErr != nil {
		log.Fatalf("Failed to write file (%s) to disk.", filePath)
	}
}

func buildSeriesList(api api.Api) {
	log.Println("Starting to fetch.")
	var seriesList []WatchListItem
	const MAX_ID = 1000
	// TODO: Get the latest id in the json file so that
	// we don't always start from 1
	id := 1
	sleepTime := 1 * time.Second
	var series []WatchListItem

	file, _ := os.OpenFile("series-list.json", os.O_WRONLY, os.ModeAppend)
	defer file.Close()
	encoder := json.NewEncoder(file)

	seriesListFile, _ := os.ReadFile("series-list.json")
	json.Unmarshal(seriesListFile, &seriesList)

	if len(seriesList) > 0 {
		sort.Slice(seriesList, func(i int, j int) bool {
			id1, _ := strconv.ParseInt(seriesList[i].Id, 10, 32)
			id2, _ := strconv.ParseInt(seriesList[j].Id, 10, 32)
			return id1 > id2
		})
		lastSeriesId, _ := strconv.Atoi(seriesList[0].Id)
		id = lastSeriesId
	}

	for id < MAX_ID {
		output, err := api.FetchSeriesChapters(strconv.Itoa(id))
		if err != nil {
			log.Fatalln("Failed to fetch series chapters with error: ", err)
		}

		if output.Data == nil {
			log.Printf("The id %d doesn't exist. Skipping...", id)
			id++

			// Wait n seconds before trying the next id
			time.Sleep(sleepTime)
			continue
		}
		data := output.Data

		r := regexp.MustCompile(`[^a-zA-Z0-9_.-]\s*`)
		first := data[0]
		seriesTitle := first.Manga.SeriesTitle
		folderName := strings.TrimRight(strings.ToLower(r.ReplaceAllString(first.Manga.SeriesTitle, "-")), "-")
		log.Printf("Found series at %d; Name: %s\n; Folder: %s", id, seriesTitle, folderName)
		series = append(series, WatchListItem{Title: seriesTitle, Id: strconv.Itoa(id), FolderName: folderName})
		id++

		// Wait n seconds before trying the next id
		time.Sleep(sleepTime)
	}

	err := encoder.Encode(series)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Finished! Found %d titles.\n", len(series))
}

func getSeriesList() []WatchListItem {
	var seriesList []WatchListItem
	file, fileErr := os.ReadFile("series-list.json")
	if fileErr != nil {
		log.Fatal(fileErr)
	}

	err := json.Unmarshal(file, &seriesList)
	if err != nil {
		log.Fatal(err)
	}
	return seriesList
}

// TODO: Make this more efficient and do a proper field merge.
func upsertWatchListJSON(items []WatchListItem) {
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
			// If slice doesn't contain this id already
			if !slices.ContainsFunc(watchingList, func(a WatchListItem) bool {
				return a.Id == item.Id
			}) {
				watchingList = append(watchingList, item)
			} else {
				index := slices.IndexFunc(watchingList, func(a WatchListItem) bool {
					return a.Id == item.Id
				})

				// If the latest chapters don't match, we're probably upserting it.
				if watchingList[index].LatestChapter != item.LatestChapter && item.LatestChapter != "" {
					watchingList[index] = item
				}
			}
		}
	}

	err := encoder.Encode(watchingList)
	if err != nil {
		log.Fatal("Encoding error:", err)
	}
}

// TODO: Make this more efficient;
// Allow for starting from start or end?
func updateWatchList(api api.Api) {
	var watchList []WatchListItem
	contents, _ := os.Open("to-watch.json")
	bytes, _ := io.ReadAll(contents)
	sleepTime := 5 * time.Second
	json.Unmarshal(bytes, &watchList)

	if len(watchList) < 1 {
		log.Fatalln("Your watchlist is empty! Please add ids from `series-list.json` by using the --to-watch argument")
	}

	// Need to check watch list, look at latest chapter in dir
	// then find all missing chapters
	log.Println("Starting downloads...")
MangaLoop:
	for i := 0; i < len(watchList); i++ {
		// Later in the function, item will be mutated
		// so we need the reference to the object in memory
		item := &watchList[i]
		log.Printf("Fetching manga %s\n", item.Title)
		listings, err := api.FetchSeriesChapters(item.Id)
		if err != nil {
			log.Fatalln("Failed to fetch series chapter with error: ", err)
		}
		// Sort by oldest first
		sort.Slice(listings.Data, func(i, j int) bool {
			d1, _ := time.Parse(time.RFC3339, listings.Data[i].Manga.PublicationDate)
			d2, _ := time.Parse(time.RFC3339, listings.Data[j].Manga.PublicationDate)
			return d1.Before(d2)
		})
		for i, chapter := range listings.Data {
			chapterAsFloat, _ := strconv.ParseFloat(chapter.Manga.Chapter, 32)
			latestChapterAsFloat, _ := strconv.ParseFloat(item.LatestChapter, 32)

			if !chapter.Manga.Published {
				log.Printf("Chapter isn't published (%s)\n", chapter.Manga.Chapter)
				continue
			}

			if chapterAsFloat <= latestChapterAsFloat {
				log.Printf("Skipping because chapter already downloaded. Latest chapter (%.1f) >= current chapter (%.1f)", latestChapterAsFloat, chapterAsFloat)
				continue
			}

			log.Printf("Getting chapter %s (id: %d)\n", chapter.Manga.Chapter, chapter.Manga.MangaCommonId)
			location, err := api.FetchZipLocation(strconv.Itoa(chapter.Manga.MangaCommonId))

			if err != nil {
				if i == len(listings.Data)-1 {
					item.LatestChapter = chapter.Manga.Chapter
				}
				log.Println("Failed to fetch series zip location with error: ", err)
				log.Println("Likely due to running out of weekly downloads. Try again next week.")
				break MangaLoop
			}

			fetchZip(location.Data, item.FolderName, chapter.Manga.Chapter)

			// Update to item with latest chapter data
			// to be written to disk outside of both iterations
			if i == len(listings.Data)-1 {
				item.LatestChapter = chapter.Manga.Chapter
			}

			// Wait 5s before downloading next chapter
			time.Sleep(sleepTime)
		}

	}

	upsertWatchListJSON(watchList)
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
		upsertWatchListJSON(toWatch)
	}

	// Check for updates and download files
	updateWatchList(api)
}
