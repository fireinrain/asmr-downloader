package model

// PageResult 每页请求响应结构体
type PageResult struct {
	Works      []Works    `json:"works"`
	Pagination Pagination `json:"pagination"`
}
type Pagination struct {
	CurrentPage int `json:"currentPage"`
	PageSize    int `json:"pageSize"`
	TotalCount  int `json:"totalCount"`
}

type RateCountDetail struct {
	ReviewPoint int `json:"review_point"`
	Count       int `json:"count"`
	Ratio       int `json:"ratio"`
}
type Rank struct {
	Term     string `json:"term"`
	Category string `json:"category"`
	Rank     int    `json:"rank"`
	RankDate string `json:"rank_date"`
}
type Vas struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type EnUs struct {
	Name string `json:"name"`
}
type JaJp struct {
	Name string `json:"name"`
}
type ZhCn struct {
	Name string `json:"name"`
}
type I18N struct {
	EnUs EnUs `json:"en-us"`
	JaJp JaJp `json:"ja-jp"`
	ZhCn ZhCn `json:"zh-cn"`
}
type Tags struct {
	ID   int    `json:"id"`
	I18N I18N   `json:"i18n"`
	Name string `json:"name"`
}
type Circle struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
type Works struct {
	ID                int               `json:"id"`
	Title             string            `json:"title"`
	CircleID          int               `json:"circle_id"`
	Name              string            `json:"name"`
	Nsfw              bool              `json:"nsfw"`
	Release           string            `json:"release"`
	DlCount           int               `json:"dl_count"`
	Price             int               `json:"price"`
	ReviewCount       int               `json:"review_count"`
	RateCount         int               `json:"rate_count"`
	RateAverage2Dp    float64           `json:"rate_average_2dp"`
	RateCountDetail   []RateCountDetail `json:"rate_count_detail"`
	Rank              []Rank            `json:"rank"`
	HasSubtitle       bool              `json:"has_subtitle"`
	CreateDate        string            `json:"create_date"`
	Vas               []Vas             `json:"vas"`
	Tags              []Tags            `json:"tags"`
	UserRating        interface{}       `json:"userRating"`
	Circle            Circle            `json:"circle"`
	SamCoverURL       string            `json:"samCoverUrl"`
	ThumbnailCoverURL string            `json:"thumbnailCoverUrl"`
	MainCoverURL      string            `json:"mainCoverUrl"`
}

////////////////////////////////////////// 每页请求响应END ////////////////////////////////////////
