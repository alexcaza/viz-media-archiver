package viz_api

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type MangaData struct {
	Manga struct {
		Author              string `json:"author"`
		Chapter             string `json:"chapter"`
		ContentsStartPage   int    `json:"contents_start_page"`
		ContentsYear        string `json:"contents_year"`
		CreatedAt           string `json:"created_at"`
		Description         string `json:"description"`
		DeviceId            int    `json:"device_id"`
		Entitled            string `json:"entitled"`
		EpochExpDate        int    `json:"epoch_exp_date"`
		EpochPubDate        int    `json:"epoch_pub_date"`
		ExpirationDate      int    `json:"expiration_date"`
		Featured            bool   `json:"featured"`
		Free                bool   `json:"free"`
		FreePages           int    `json:"free_pages"`
		Id                  int    `json:"id"`
		ImprintId           int    `json:"imprint_id"`
		ImprintTitle        string `json:"imprint_title"`
		Isbn13              string `json:"isbn13"`
		IssueDate           int    `json:"issue_date"`
		ListPrice           string `json:"list_price"`
		MangaCommonId       int    `json:"manga_common_id"`
		MangaSeriesCommonId int    `json:"manga_series_common_id"`
		MidrollPage         int    `json:"midroll_page"`
		ModTs               int    `json:"mod_ts"`
		New                 bool   `json:"new"`
		NewFollowing        string `json:"new_following"`
		NewPokemon          bool   `json:"new_pokemon"`
		NextIssueDate       int    `json:"next_issue_date"`
		NextMangaCommonId   int    `json:"next_manga_common_id"`
		Numpages            int    `json:"numpages"`
		OverrideRtl         int    `json:"override_rtl"`
		OverrideShowVolume  int    `json:"override_show_volume"`
		ParentMangaCommonId int    `json:"parent_manga_common_id"`
		PrereleasePreview   bool   `json:"prerelease_preview"`
		PreviewPages        int    `json:"preview_pages"`
		Price               string `json:"price"`
		PublicationDate     string `json:"publication_date"`
		Published           bool   `json:"published"`
		Rating              string `json:"rating"`
		RatingOverride      string `json:"rating_override"`
		Rtl                 bool   `json:"rtl"`
		SeriesTitle         string `json:"series_title"`
		SeriesTitleSort     string `json:"series_title_sort"`
		SeriesVanityurl     string `json:"series_vanityurl"`
		ShareImg            string `json:"share_img"`
		ShowChapter         bool   `json:"show_chapter"`
		ShowVolume          bool   `json:"show_volume"`
		SubscriptionIssue   bool   `json:"subscription_issue"`
		SubscriptionType    string `json:"subscription_type"`
		Thumb               string `json:"thumb"`
		Thumburl            string `json:"thumburl"`
		Title               string `json:"title"`
		UpdatedAt           string `json:"updated_at"`
		Url                 string `json:"url"`
		Volume              int    `json:"volume"`
		WebPrice            string `json:"web_price"`
	} `json:"manga"`
}

type Manga struct {
	Data     []MangaData `json:"data"`
	Filesize int         `json:"filesize"`
	Ok       int         `json:"ok"`
}

type MangaZip struct {
	Data string `json:"data"`
	Ok   int    `json:"ok"`
}

type Api struct {
	baseUrl string
}

func NewApi() Api {
	return Api{baseUrl: "https://api.viz.com"}
}

func (a Api) setNecessaryHeaders(req *http.Request) *http.Request {
	req.Header.Add("X-Devil-Fruit", "5.5.7 gum-gum fruits")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("User-Agent", "Weekly%20Shonen%20Jump/1 CFNetwork/1490.0.4 Darwin/23.2.0")
	req.Header.Add("Accept-Language", "en-CA,en-US;q=0.9,en;q=0.8")
	req.Header.Add("Referer", "com.viz.wsj")

	return req
}

func (a Api) buildFormParams(chapterId string) string {
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatal("Couldn't load env variables. Exiting")
		os.Exit(1)
	}

	v := url.Values{}
	v.Add("instance_id", os.Getenv("INSTANCE_ID"))
	v.Add("device_id", os.Getenv("DEVICE_ID"))
	v.Add("manga_id", chapterId)
	v.Add("viz_app_id", "3")
	v.Add("trust_user_jwt", os.Getenv("USER_JWT"))
	v.Add("user_id", os.Getenv("USER_ID"))
	v.Add("version", "9")
	v.Add("metadata", "true")
	v.Add("idfa", "00000000-0000-0000-0000-000000000000")

	return v.Encode()
}

// TODO: This should return an err so we can handle it
// and properly handle closing out the requests sequence
func (a Api) FetchSeriesChapters(seriesId int) (manga Manga, err error) {
	var output Manga
	client := &http.Client{}
	req, err := http.NewRequest("GET", a.baseUrl+"/manga/store/series/"+strconv.Itoa(seriesId)+"/1/1/8", nil)

	a.setNecessaryHeaders(req)

	if err != nil {
		return output, err
	}

	resp, err := client.Do(req)

	if err != nil {
		return output, err
	}

	decoder := json.NewDecoder(resp.Body)
	defer resp.Body.Close()

	decodeErr := decoder.Decode(&output)

	if decodeErr != nil {
		return output, decodeErr
	}

	return output, nil
}

func (a Api) FetchZipLocation(chapterId string) (zip MangaZip, err error) {
	var mangaZip MangaZip
	client := &http.Client{}
	req, err := http.NewRequest("GET", a.baseUrl+"/manga/get_manga_url?"+a.buildFormParams(chapterId), nil)

	a.setNecessaryHeaders(req)

	if err != nil {
		return mangaZip, err
	}

	resp, err := client.Do(req)

	if err != nil {
		return mangaZip, err
	}

	defer resp.Body.Close()

	decodeErr := json.NewDecoder(resp.Body).Decode(&mangaZip)
	if decodeErr != nil {
		return mangaZip, decodeErr
	}

	return mangaZip, nil
}
