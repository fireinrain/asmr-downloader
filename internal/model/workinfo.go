package model

type WorkInfo struct {
	ID              int           `json:"id"`
	Title           string        `json:"title"`
	CircleID        int           `json:"circle_id"`
	Name            string        `json:"name"`
	Nsfw            bool          `json:"nsfw"`
	Release         string        `json:"release"`
	DlCount         int           `json:"dl_count"`
	Price           int           `json:"price"`
	ReviewCount     int           `json:"review_count"`
	RateCount       int           `json:"rate_count"`
	RateAverage2Dp  float32       `json:"rate_average_2dp"`
	RateCountDetail []interface{} `json:"rate_count_detail"`
	Rank            interface{}   `json:"rank"`
	HasSubtitle     bool          `json:"has_subtitle"`
	CreateDate      string        `json:"create_date"`
	Vas             []struct {
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
				Name    string `json:"name"`
				History []struct {
					Name         string `json:"name"`
					DeprecatedAt int64  `json:"deprecatedAt"`
				} `json:"history"`
			} `json:"zh-cn"`
		} `json:"i18n"`
		Name       string `json:"name"`
		Upvote     int    `json:"upvote"`
		Downvote   int    `json:"downvote"`
		VoteRank   int    `json:"voteRank"`
		VoteStatus int    `json:"voteStatus"`
	} `json:"tags"`
	LanguageEditions []struct {
		Lang         string `json:"lang"`
		Label        string `json:"label"`
		Workno       string `json:"workno"`
		EditionID    int    `json:"edition_id"`
		EditionType  string `json:"edition_type"`
		DisplayOrder int    `json:"display_order"`
	} `json:"language_editions"`
	OriginalWorkno            string `json:"original_workno"`
	OtherLanguageEditionsInDb []struct {
		ID         int    `json:"id"`
		Lang       string `json:"lang"`
		Title      string `json:"title"`
		SourceID   string `json:"source_id"`
		IsOriginal bool   `json:"is_original"`
		SourceType string `json:"source_type"`
	} `json:"other_language_editions_in_db"`
	TranslationInfo struct {
		Lang                    string        `json:"lang"`
		IsChild                 bool          `json:"is_child"`
		IsParent                bool          `json:"is_parent"`
		IsOriginal              bool          `json:"is_original"`
		IsVolunteer             bool          `json:"is_volunteer"`
		ChildWorknos            []interface{} `json:"child_worknos"`
		ParentWorkno            string        `json:"parent_workno"`
		OriginalWorkno          string        `json:"original_workno"`
		IsTranslationAgree      bool          `json:"is_translation_agree"`
		TranslationBonusLangs   interface{}   `json:"translation_bonus_langs"`
		IsTranslationBonusChild bool          `json:"is_translation_bonus_child"`
		//TranslationStatusForTranslator [string]interface{} `json:"translation_status_for_translator"`
	} `json:"translation_info"`
	WorkAttributes    string `json:"work_attributes"`
	AgeCategoryString string `json:"age_category_string"`
	Duration          int    `json:"duration"`
	SourceType        string `json:"source_type"`
	SourceID          string `json:"source_id"`
	SourceURL         string `json:"source_url"`
	Circle            struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		SourceID   string `json:"source_id"`
		SourceType string `json:"source_type"`
	} `json:"circle"`
	SamCoverURL       string `json:"samCoverUrl"`
	ThumbnailCoverURL string `json:"thumbnailCoverUrl"`
	MainCoverURL      string `json:"mainCoverUrl"`
}
