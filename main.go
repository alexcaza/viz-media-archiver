package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

/*

	TODOs & Nice to haves:

	-- TODOs
	1. Split functions up into files
	2. Refactor request code to better encapsulate
	3. Main function should have 1–2 fn calls max

	-- NICE TO HAVES
	1. List all user favourites with their names + ids
	2. Select chapters from terminal +multiselect
	3. Download progress bar

*/

type MangaLocation struct {
	Data string `json:"data"`
	Filesize int `json:"filesize"`
	Ok int `json:"ok"`
}

func baseUrl() (string) {
	return "https://api.viz.com"
}

func formParams(mangaId string) (string) {
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

func fetchZipLocation(mangaId string) (MangaLocation){
	var mangaLoc MangaLocation
	client := &http.Client{}
	req, err := http.NewRequest("GET", baseUrl() + "/manga/get_manga_url?" + formParams(mangaId), nil)

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

func main() {

	// TODO: Bring this in from ARGV or selection
	mangaId := "23065"

	mangaLoc := fetchZipLocation(mangaId)
	fetchZip(mangaLoc.Data)

}