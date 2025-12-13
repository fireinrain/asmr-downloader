package model

import (
	"strings"
	"time"
)

// MetadataWork 作品元数据
type MetadataWork struct {
	ID int `gorm:"primaryKey"`

	Title          string  `json:"title"`
	CircleID       int     `json:"circle_id"`
	Name           string  `json:"name"`
	Nsfw           bool    `json:"nsfw"`
	Release        string  `json:"release"`
	DlCount        int     `json:"dl_count"`
	Price          int     `json:"price"`
	ReviewCount    int     `json:"review_count"`
	RateCount      int     `json:"rate_count"`
	RateAverage2Dp float64 `json:"rate_average_2dp"`
	HasSubtitle    bool    `json:"has_subtitle"`
	CreateDate     string  `json:"create_date"`
	Vas            string  `json:"vas"`
	Tags           string  `json:"tags"`
	Duration       int     `json:"duration"`
	SourceType     string  `json:"source_type"`
	SourceID       string  `gorm:"uniqueIndex:idx_source_id" json:"source_id"`

	UpdatedAt time.Time `json:"updated_at"`
}

// WorkSyncInfo 工作同步信息
type WorkSyncInfo struct {
	ID             int    `gorm:"primaryKey"`
	MetadataWorkId int    `json:"metadata_work_id"`
	SourceId       string `json:"source_id"`
	HasSubtitle    bool   `json:"has_subtitle"`
	//目录大小
	DirSize int64 `json:"dir_size"`
	//下载状态
	Status    string    `json:"status"`    // "PENDING", "COMPLETED", "FAILED"
	FilePath  string    `json:"file_path"` // 本地保存路径
	UpdatedAt time.Time `json:"updated_at"`

	// 新增字段
	FailReason string    // 记录具体的错误信息，例如 "http: timeout" 或 "404 not found"
	RetryCount int       // 记录已经重试了多少次
	FailedAt   time.Time // 最后一次失败的时间
}

type MetadataWorkResponse struct {
	Works []struct {
		ID             int     `json:"id"`
		Title          string  `json:"title"`
		CircleID       int     `json:"circle_id"`
		Name           string  `json:"name"`
		Nsfw           bool    `json:"nsfw"`
		Release        string  `json:"release"`
		DlCount        int     `json:"dl_count"`
		Price          int     `json:"price"`
		ReviewCount    int     `json:"review_count"`
		RateCount      int     `json:"rate_count"`
		RateAverage2Dp float64 `json:"rate_average_2dp"`
		//RateCountDetail []struct {
		//	ReviewPoint int `json:"review_point"`
		//	Count       int `json:"count"`
		//	Ratio       int `json:"ratio"`
		//} `json:"rate_count_detail"`
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
		//LanguageEditions          []interface{} `json:"language_editions"`
		//OriginalWorkno            interface{}   `json:"original_workno"`
		//OtherLanguageEditionsInDb []interface{} `json:"other_language_editions_in_db"`
		//TranslationInfo           struct {
		//	Lang                    interface{}   `json:"lang"`
		//	IsChild                 bool          `json:"is_child"`
		//	IsParent                bool          `json:"is_parent"`
		//	IsOriginal              bool          `json:"is_original"`
		//	IsVolunteer             bool          `json:"is_volunteer"`
		//	ChildWorknos            []interface{} `json:"child_worknos"`
		//	ParentWorkno            interface{}   `json:"parent_workno"`
		//	OriginalWorkno          interface{}   `json:"original_workno"`
		//	IsTranslationAgree      bool          `json:"is_translation_agree"`
		//	TranslationBonusLangs   []interface{} `json:"translation_bonus_langs"`
		//	IsTranslationBonusChild bool          `json:"is_translation_bonus_child"`
		//} `json:"translation_info"`
		//WorkAttributes    string      `json:"work_attributes"`
		//AgeCategoryString string      `json:"age_category_string"`
		Duration   int    `json:"duration"`
		SourceType string `json:"source_type"`
		SourceID   string `json:"source_id"`
		//SourceURL         string      `json:"source_url"`
		//UserRating        interface{} `json:"userRating"`
		//PlaylistStatus    struct {
		//	E3B3E90636A44Fd4B753A5D1Fc50Cb5D bool `json:"e3b3e906-36a4-4fd4-b753-a5d1fc50cb5d"`
		//} `json:"playlistStatus"`
		//Circle struct {
		//	ID         int    `json:"id"`
		//	Name       string `json:"name"`
		//	SourceID   string `json:"source_id"`
		//	SourceType string `json:"source_type"`
		//} `json:"circle"`
		//SamCoverURL       string `json:"samCoverUrl"`
		//ThumbnailCoverURL string `json:"thumbnailCoverUrl"`
		//MainCoverURL      string `json:"mainCoverUrl"`
	} `json:"works"`
	Pagination struct {
		CurrentPage int `json:"currentPage"`
		PageSize    int `json:"pageSize"`
		TotalCount  int `json:"totalCount"`
	} `json:"pagination"`
}

func (m *MetadataWorkResponse) BuildMetadataWork() []MetadataWork {
	if len(m.Works) == 0 {
		return nil
	}
	var metadataWorks []MetadataWork
	for _, work := range m.Works {
		var vas []string
		for _, va := range work.Vas {
			vas = append(vas, va.Name)
		}
		var tags []string
		for _, tag := range work.Tags {
			tags = append(tags, tag.Name)
		}
		metadataWork := MetadataWork{
			Title:          work.Title,
			CircleID:       work.CircleID,
			Name:           work.Name,
			Nsfw:           work.Nsfw,
			Release:        work.Release,
			DlCount:        work.DlCount,
			Price:          work.Price,
			ReviewCount:    work.ReviewCount,
			RateCount:      work.RateCount,
			RateAverage2Dp: work.RateAverage2Dp,
			HasSubtitle:    work.HasSubtitle,
			CreateDate:     work.CreateDate,
			Vas:            strings.Join(vas, ","),
			Tags:           strings.Join(tags, ","),
			Duration:       work.Duration,
			SourceType:     work.SourceType,
			SourceID:       work.SourceID,
		}
		metadataWorks = append(metadataWorks, metadataWork)
	}
	return metadataWorks
}
