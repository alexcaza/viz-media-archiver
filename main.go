package main

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	api "viz-media/viz_api"

	"github.com/gosimple/slug"
	_ "github.com/mattn/go-sqlite3"

	goarg "github.com/alexflint/go-arg"
	"github.com/joho/godotenv"
)

type WatchedManga struct {
	Id       int
	SeriesId int
	Title    string
	Slug     string
}

type DownloadedChapters struct {
	Id           int
	WatchingId   int
	SeriesId     int
	ChapterLabel string
}

type SeriesListItem struct {
	Title         string `json:"title"`
	Id            string `json:"id"`
	LatestChapter string `json:"latestChapter"`
	FolderName    string `json:"folderName"`
}

var args struct {
	GenListing  bool  `arg:"--generate-listing" help:"Generates list of series."`
	AddToWatch  []int `arg:"--to-watch, -w" help:"Adds ids to list of series to watch. Must be in series-list.json."`
	ForceUpdate bool  `arg:"--force, -f" help:"Forces watching list to be updated completely."`
	UpdateList  []int `arg:"--update-list, -u" help:"Specific series ids to be updated. Can be paired with -f to force and update"`
}

func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func createDirsFromPath(path []string) string {
	dirPath := "./data" + strings.Join(path, "/")
	err := os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to created path %s", dirPath)
	}
	return dirPath
}

func writeZip(path string, data io.ReadCloser) error {
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

	for _, f := range archive.File {

		if f.FileInfo().IsDir() {
			log.Println("File is a directory... skipping.")
			continue
		}

		fileName := strings.Split(f.Name, ".")[0]
		extension := strings.Split(f.Name, ".")[1]

		if fileName == "0.jpg" {
			// 0.jpg doesn't have any useful information,
			// it's just used in their apps to tell people
			// to swipe the right way. (Left intead of Right)
			continue
		}

		var fileNameWithExtension string
		switch extension {
		case "jpg":
			fileNameWithExtension = fileName + "." + extension
		case "json":
			fileNameWithExtension = "metadata" + "." + extension
		default:
			fileNameWithExtension = f.Name
		}

		archiveFile, _ := f.Open()
		diskFile, _ := os.Create(folderPath + "/" + fileNameWithExtension)
		io.Copy(diskFile, archiveFile)
	}
	return err
}

func fetchZip(zipLocation string, folderName string, chapterId string) bool {
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
	filePath := outputPath + "/" + fileName

	writeErr := writeZip(filePath, zipResp.Body)

	if writeErr != nil {
		log.Fatalf("Failed to write file (%s) to disk.", filePath)
	}

	log.Println("Saved chapter to: ", filePath)

	return true
}

func buildSeriesList(db *sql.DB, api api.Api) {
	log.Println("Starting to fetch.")

	id := 1
	sleepTime := 1 * time.Second
	var series []SeriesListItem

	err := godotenv.Load(".env")
	if err != nil {
		log.Println("Failed to open .env with error: ", err)
	}

	maxId, err := strconv.Atoi(os.Getenv("MAX_ID"))
	if err != nil {
		log.Println("MAX_ID is missing! Please add it to your environment")
	}

	list, err := db.Query("select * from series_list order by id asc")
	if err != nil {
		log.Fatalln("Failed to query series_list with error: ", err)
	}

	defer list.Close()

	for list.Next() {
		var seriesItem SeriesListItem
		err := list.Scan(&seriesItem.Id, &seriesItem.Title, &seriesItem.FolderName)
		if err != nil {
			log.Fatalln("Failed to create watchedManga")
		}
		series = append(series, seriesItem)
	}

	if len(series) > 0 {
		lastSeriesId, err := strconv.Atoi(series[0].Id)
		if err != nil {
			log.Fatalln("Failed to convert series id to int: ", series[0])
		}
		// Set the id to the last series so we don't start over
		// every time
		id = lastSeriesId
	}

	for i := id; i < maxId; i++ {
		// Wait n seconds at the top of each loop.
		// This is to avoid being a bad actor/hitting rate limits.
		// We call it at the top to avoid forgetting to call it in
		// lower branches.
		time.Sleep(sleepTime)

		output, err := api.FetchSeriesChapters(id)
		if err != nil {
			log.Fatalln("Failed to fetch series chapters with error: ", err)
		}

		if output.Data == nil {
			log.Printf("The id %d doesn't exist. Skipping...", id)
			id++

			continue
		}
		data := output.Data

		first := data[0]
		seriesTitle := first.Manga.SeriesTitle
		folderName := slug.Make(first.Manga.SeriesTitle)
		_, err = db.Exec(
			"insert into series_list (id, title, slug) values (?, ?, ?) on conflict do update set id=excluded.id, title=excluded.title, slug=excluded.slug",
			strconv.Itoa(id), seriesTitle, folderName,
		)
		if err != nil {
			log.Println("Failed to add series to database with error: ", err)
			log.Printf("Series info... id: %d; Name: %s\n", id, seriesTitle)
		} else {
			log.Printf("Found series at %d; Name: %s; Slug: %s\n", id, seriesTitle, folderName)
		}
	}

	log.Printf("Finished! Found %d titles.\n", len(series))
}

func upsertWatching(db *sql.DB, toWatch []int) {
	var addedIds []string
	var toWatchStr []string

	for _, id := range toWatch {
		toWatchStr = append(toWatchStr, strconv.Itoa(id))
	}

	// WARNING: Not safe from injections!
	// Since inputs are "known" and the only risk is your own
	// failure to pass valid arguments, I'm not too worried about injection issues.
	// This isn't an exposed service. If you run it as one, fix this before doing so!
	query := fmt.Sprintf("select * from series_list where id in (%s)", strings.Join(toWatchStr, ","))
	items, err := db.Query(query)
	var itemsScanned []SeriesListItem
	if err != nil {
		log.Fatalln("Failed to pull from series_list with error: ", err)
	}

	defer items.Close()

	for items.Next() {
		var item SeriesListItem
		items.Scan(&item.Id, &item.Title, &item.FolderName)
		itemsScanned = append(itemsScanned, item)
	}

	for _, item := range itemsScanned {
		res, err := db.Exec(
			"insert into watching (id, series_id, title, slug) values (?, ?, ?, ?) on conflict do update set series_id=excluded.series_id",
			nil, item.Id, item.Title, item.FolderName,
		)
		if err != nil {
			log.Fatalln("Failed to write to watching with: ", err)
		} else {
			id, _ := res.LastInsertId()
			if id > 0 {
				log.Printf("Adding item to watching: %s (id: %s)\n", item.Title, item.Id)
				addedIds = append(addedIds, item.Id)
			} else {
				log.Printf("Item already created in watching... skipping. (id: %s)\n", item.Id)
				addedIds = append(addedIds, item.Id)
			}
		}
	}

	diff := difference(toWatchStr, addedIds)

	if len(diff) > 0 {
		log.Println("Some ids were invalid: ", diff)
	}
}

func updateWatchList(db *sql.DB, a api.Api, updateList []int, force bool) {
	sleepTime := 5 * time.Second
	var watchList *sql.Rows
	if len(updateList) < 1 {
		list, err := db.Query("select * from watching")
		watchList = list
		if err != nil {
			log.Fatalln("Failed to get watchlist items", err)
		}
	} else {
		var params []string
		for _, id := range updateList {
			params = append(params, strconv.Itoa(id))
		}
		// WARNING: Not safe from injections!
		// Since inputs are "known" and the only risk is your own
		// failure to pass valid arguments, I'm not too worried about injection issues.
		// This isn't an exposed service. If you run it as one, fix this before doing so!
		query := fmt.Sprintf("select * from watching where series_id in (%s)", strings.Join(params, ","))
		list, err := db.Query(query)
		watchList = list
		if err != nil {
			log.Fatalln("Failed to get watchlist items", err)
		}
	}
	var watchedMangas []WatchedManga

	defer watchList.Close()

	for watchList.Next() {
		var watchedManga WatchedManga
		err := watchList.Scan(&watchedManga.Id, &watchedManga.SeriesId, &watchedManga.Title, &watchedManga.Slug)
		if err != nil {
			log.Fatalln("Failed to create watchedManga")
		}
		watchedMangas = append(watchedMangas, watchedManga)
	}

MangaLoop:
	for _, watchedManga := range watchedMangas {
		// Need to check watch list, look at latest chapter in dir
		// then find all missing chapters
		log.Println("Starting downloads...")
		log.Printf("Fetching manga %s\n", watchedManga.Title)
		listings, err := a.FetchSeriesChapters(watchedManga.SeriesId)
		if err != nil {
			log.Fatalln("Failed to fetch series chapter with error: ", err)
		}
		// Sort by oldest first
		sort.Slice(listings.Data, func(i, j int) bool {
			d1, _ := strconv.ParseFloat(listings.Data[i].Manga.PublicationDate, 32)
			d2, _ := strconv.ParseFloat(listings.Data[j].Manga.PublicationDate, 32)
			return d1 < d2
		})

		var chapters []DownloadedChapters
		downloadedChapters, err := db.Query("select * from downloaded where series_id = ? order by cast(chapter_label as real) asc", watchedManga.SeriesId)
		if err != nil {
			log.Fatalf("Failed to get downloaded items for series_id %d with error: %s", watchedManga.SeriesId, err)
		}

		defer downloadedChapters.Close()

		for downloadedChapters.Next() {
			var chapter DownloadedChapters
			downloadedChapters.Scan(&chapter.Id, &chapter.SeriesId, &chapter.WatchingId, &chapter.ChapterLabel)
			chapters = append(chapters, chapter)
		}

		var toDownload []api.MangaData
		for _, listing := range listings.Data {

			if force {
				toDownload = append(toDownload, listing)
				continue
			}

			// Skip chapters we've already downloaded
			if slices.ContainsFunc(chapters, func(dc DownloadedChapters) bool {
				return dc.ChapterLabel == listing.Manga.Chapter
			}) {
				continue
			}

			if listing.Manga.WebPrice != "" {
				log.Println("Chapter/Volume isn't free. Skipping...", listing.Manga.Title, listing.Manga.Chapter)
				continue
			}

			toDownload = append(toDownload, listing)
		}

		for _, chapterToDownload := range toDownload {
			log.Printf("Getting chapter %s (id: %d)\n", chapterToDownload.Manga.Chapter, chapterToDownload.Manga.MangaCommonId)

			if chapterToDownload.Manga.EpochPubDate > int(time.Now().Unix()) {
				log.Println("Chapter not yet published... Skipping!")
				continue
			}

			location, err := a.FetchZipLocation(strconv.Itoa(chapterToDownload.Manga.MangaCommonId))
			if err != nil {
				log.Println("Failed to fetch series zip location with error: ", err)
				break MangaLoop
			}

			if location.Data == "no_auth" {
				log.Println("Run out of daily downloads. Try again tomorrow!")
				break MangaLoop
			}

			didWrite := fetchZip(location.Data, watchedManga.Slug, chapterToDownload.Manga.Chapter)

			if didWrite {
				_, err := db.Exec(
					"insert into downloaded (id, watching_id, series_id, chapter_label) values (?, ?, ?, ?)",
					nil, watchedManga.Id, watchedManga.SeriesId, chapterToDownload.Manga.Chapter,
				)
				if err != nil {
					log.Fatalln("Failed to save downloaded chapter to db", err)
				}

				log.Printf("Updated DB with chapter %s. Moving on.\n", chapterToDownload.Manga.Chapter)
			}

			// Wait 5s before downloading next chapter
			time.Sleep(sleepTime)

		}

	}
}

func main() {
	goarg.MustParse(&args)
	api := api.NewApi()
	db, err := sql.Open("sqlite3", "./viz.db")
	if err != nil {
		log.Println("Filed to open sqlite database with error: ", err)
	}

	createStatements := map[string]string{
		"series_list": "create table if not exists series_list (id integer PRIMARY KEY, title text NOT NULL, slug text NOT NULL)",
		"watching":    "create table if not exists watching (id integer PRIMARY KEY, series_id integer, title text NOT NULL, slug text NOT NULL, UNIQUE(series_id))",
		"downloaded":  "create table if not exists downloaded (id integer PRIMARY KEY, watching_id integer NOT NULL, series_id integer NOT NULL, chapter_label text NOT NULL, FOREIGN KEY(watching_id) REFERENCES watching(id), UNIQUE(watching_id, series_id, chapter_label))",
	}

	defer db.Close()

	for tableName, statement := range createStatements {
		_, err = db.Exec(statement)
		if err != nil {
			log.Printf("Failed to create table: %s\n", tableName)
			log.Println("Error: ", err)
		}
	}

	if args.GenListing {
		buildSeriesList(db, api)
	}

	if len(args.AddToWatch) > 0 {
		log.Println("Adding ids to watching:", args.AddToWatch)
		upsertWatching(db, args.AddToWatch)
	}

	// Check for updates and download files
	// Only if there are no other args present or an update list is passed
	if len(args.UpdateList) > 0 || len(os.Args[1:]) < 1 {
		log.Println("Updating watch list")
		updateWatchList(db, api, args.UpdateList, args.ForceUpdate)
	}
}
