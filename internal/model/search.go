package model

// 搜索结果
type SearchResult struct {
	Works []struct {
		ID              int     `json:"id"`
		Title           string  `json:"title"`
		CircleID        int     `json:"circle_id"`
		Name            string  `json:"name"`
		Nsfw            bool    `json:"nsfw"`
		Release         string  `json:"release"`
		DlCount         int     `json:"dl_count"`
		Price           int     `json:"price"`
		ReviewCount     int     `json:"review_count"`
		RateCount       int     `json:"rate_count"`
		RateAverage2Dp  float64 `json:"rate_average_2dp"`
		RateCountDetail []struct {
			ReviewPoint int `json:"review_point"`
			Count       int `json:"count"`
			Ratio       int `json:"ratio"`
		} `json:"rate_count_detail"`
		Rank        interface{} `json:"rank"`
		HasSubtitle bool        `json:"has_subtitle"`
		CreateDate  string      `json:"create_date"`
		Vas         []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"vas"`
		Tags []struct {
			ID   int `json:"id"`
			I18N struct {
				EnUs struct {
					Name string `json:"name"`
				} `json:"en-us"`
				JaJp struct {
					Name string `json:"name"`
				} `json:"ja-jp"`
				ZhCn struct {
					Name    string        `json:"name"`
					History []interface{} `json:"history"`
				} `json:"zh-cn"`
			} `json:"i18n"`
			Name       string `json:"name"`
			Upvote     int    `json:"upvote"`
			Downvote   int    `json:"downvote"`
			VoteRank   int    `json:"voteRank"`
			VoteStatus int    `json:"voteStatus"`
		} `json:"tags"`
		LanguageEditions          []interface{} `json:"language_editions"`
		OriginalWorkno            interface{}   `json:"original_workno"`
		OtherLanguageEditionsInDb []interface{} `json:"other_language_editions_in_db"`
		TranslationInfo           struct {
			Lang                    interface{}   `json:"lang"`
			IsChild                 bool          `json:"is_child"`
			IsParent                bool          `json:"is_parent"`
			IsOriginal              bool          `json:"is_original"`
			IsVolunteer             bool          `json:"is_volunteer"`
			ChildWorknos            []interface{} `json:"child_worknos"`
			ParentWorkno            interface{}   `json:"parent_workno"`
			OriginalWorkno          interface{}   `json:"original_workno"`
			IsTranslationAgree      bool          `json:"is_translation_agree"`
			TranslationBonusLangs   interface{}   `json:"translation_bonus_langs"`
			IsTranslationBonusChild bool          `json:"is_translation_bonus_child"`
		} `json:"translation_info"`
		WorkAttributes    string      `json:"work_attributes"`
		AgeCategoryString string      `json:"age_category_string"`
		Duration          int         `json:"duration"`
		SourceType        string      `json:"source_type"`
		SourceID          string      `json:"source_id"`
		SourceURL         string      `json:"source_url"`
		UserRating        interface{} `json:"userRating"`
		PlaylistStatus    struct {
			E3B3E90636A44Fd4B753A5D1Fc50Cb5D bool `json:"e3b3e906-36a4-4fd4-b753-a5d1fc50cb5d"`
		} `json:"playlistStatus"`
		Circle struct {
			ID         int    `json:"id"`
			Name       string `json:"name"`
			SourceID   string `json:"source_id"`
			SourceType string `json:"source_type"`
		} `json:"circle"`
		SamCoverURL       string `json:"samCoverUrl"`
		ThumbnailCoverURL string `json:"thumbnailCoverUrl"`
		MainCoverURL      string `json:"mainCoverUrl"`
	} `json:"works"`
	Pagination struct {
		CurrentPage int `json:"currentPage"`
		PageSize    int `json:"pageSize"`
		TotalCount  int `json:"totalCount"`
	} `json:"pagination"`
}

type SearchResultView struct {
	SourceID string `json:"source_id"`

	//Name  string `json:"name"`
	//Nsfw           bool    `json:"nsfw"`
	Release string `json:"release"`
	//Price   int    `json:"price"`
	//ReviewCount    int     `json:"review_count"`
	//RateCount      int     `json:"rate_count"`
	RateAverage2Dp float64 `json:"rate_average_2dp"`
	DlCount        int     `json:"dl_count"`

	HasSubtitle bool `json:"has_subtitle"`
	//CreateDate     string  `json:"create_date"`
	//Duration       int     `json:"duration"`
	//SourceType     string  `json:"source_type"`
	Title string `json:"title"`
}
