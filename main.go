package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/manifoldco/promptui"
)

type MangaLocation struct {
	Data string `json:"data"`
	Filesize int `json:"filesize"`
	Ok int `json:"ok"`
}

type Series struct {
	Series []string `json:"series"`
}

func baseUrl() (string) {
	return "https://api.viz.com"
}

func setNecessaryHeaders(req *http.Request) (*http.Request) {
	req.Header.Add("X-Devil-Fruit", "5.5.7 gum-gum fruits")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("User-Agent", "Weekly%20Shonen%20Jump/1 CFNetwork/1490.0.4 Darwin/23.2.0")
	req.Header.Add("Accept-Language", "en-CA,en-US;q=0.9,en;q=0.8")
	req.Header.Add("Referer", "com.viz.wsj")

    return req
}

func formParams(mangaId string, deviceId string) (string) {
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatal("Couldn't load env variables. Exiting")
		os.Exit(1)
	}

	v := url.Values{}
	v.Add("instance_id", os.Getenv("INSTANCE_ID"))
	v.Add("device_id", os.Getenv("DEVICE_ID"))
	v.Add("manga_id", mangaId)
	v.Add("viz_app_id", "1")
	v.Add("trust_user_jwt", os.Getenv("USER_JWT"))
	v.Add("user_id", os.Getenv("USER_ID"))
	v.Add("version", "8")
	v.Add("metadata", "true")
	v.Add("idfa", "00000000-0000-0000-0000-000000000000")

	return v.Encode()
}

func fetchSeriesListing(seriesId string) (map[string]interface{}) {
	var output map[string]interface{}
	client := &http.Client{}
	req, err := http.NewRequest("GET", baseUrl() + "/manga/store/series/" + seriesId + "/1/1/8", nil)

	req.Header.Add("X-Devil-Fruit", "7.4.13 gum-gum fruits")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", "Manga/1 CFNetwork/1335.0.3 Darwin/21.6.0")
	req.Header.Add("Accept-Language", "en-CA,en-US;q=0.9,en;q=0.8")
	req.Header.Add("Referer", "com.viz.iphone-manga")

	if err != nil {
		log.Fatal("Failed to build request. Exiting")
	}

	resp, err := client.Do(req)

	if err != nil {
		log.Fatal("Failed to fetch. Exiting", err)
	}

	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(&output)

	return output
}

func createDirsFromPath(path []string) (string) {
	dirPath := "./data" + strings.Join(path[:len(path) -1], "/")
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

func fetchZipLocation(mangaId string, deviceId string) (MangaLocation){
	var mangaLoc MangaLocation
	client := &http.Client{}
	req, err := http.NewRequest("GET", baseUrl() + "/manga/get_manga_url?" + formParams(mangaId, deviceId), nil)

	req.Header.Add("X-Devil-Fruit", "7.4.13 gum-gum fruits")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", "Manga/1 CFNetwork/1335.0.3 Darwin/21.6.0")
	req.Header.Add("Accept-Language", "en-CA,en-US;q=0.9,en;q=0.8")
	req.Header.Add("Referer", "com.viz.iphone-manga")

	if err != nil {
		log.Fatal("Failed to build request. Exiting")
	}

	resp, err := client.Do(req)

	if err != nil {
		log.Fatal("Failed to fetch. Exiting")
	}

	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(&mangaLoc)

	return mangaLoc
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

func getSeriesInfo() ([]string) {
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

func main() {
	prompt := promptui.Select{
		Label: "Select series to download chapters from",
		Items: getSeriesInfo(),
	}

	_, result, err := prompt.Run()

	if err != nil {
		log.Fatal("Failed to get selection from prompt")
	}

	selectionNumber := strings.TrimSpace(strings.Split(result, "|")[1])

	fmt.Printf("Selection: %q\n", result)

	output := fetchSeriesListing(selectionNumber)

	fetchOk := output["ok"].(float64)


	if fetchOk != 1 {
		log.Fatal("Failed to get data for selected series.")
	}

	data := output["data"].([]interface{})

	var chapters []map[string]interface{}
	for _, v := range data {
		manga := v.(map[string]interface{})["manga"].(map[string]interface{})
		if manga["web_price"] == nil && manga["published"] == true {
			chapters = append(chapters, manga)
		}
	}

	sort.Slice(chapters, func(i, j int) bool {
		d1, _ := time.Parse(time.RFC3339, chapters[i]["publication_date"].(string))
		d2, _ := time.Parse(time.RFC3339, chapters[j]["publication_date"].(string))
		return d1.After(d2)
	})

	var t []string
	for _, v := range chapters {
		chapterNum := v["chapter"].(string)
		id := v["manga_common_id"].(float64)
		deviceId := v["device_id"].(float64)
		t = append(t, chapterNum + " " + "| ("+fmt.Sprintf("%.0f", id)+"~"+fmt.Sprintf("%.0f", deviceId)+")")
	}

	prompt = promptui.Select{
		Label: "Which chapter do you want to download?",
		Items: t,
	}

	_, result, err = prompt.Run()

	if err != nil {
		log.Fatal("Chapter selection failed.")
	}

	fetchData := strings.Split(strings.Split(result, "|")[1], "~")
	chapterId := strings.Trim(strings.TrimSpace(fetchData[0]), "()")
	deviceId := strings.Trim(strings.TrimSpace(fetchData[1]), "()")

	mangaLoc := fetchZipLocation(chapterId, deviceId)
	fetchZip(mangaLoc.Data)

}