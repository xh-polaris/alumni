package service

import (
	"context"

	"github.com/google/wire"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/article"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/chapter"
	"go.mongodb.org/mongo-driver/bson"
)

type ArticleService struct {
	ArticleMapper article.IMongoMapper
	ChapterMapper chapter.IMongoMapper
}

var ArticleServiceSet = wire.NewSet(
	wire.Struct(new(ArticleService), "*"),
)

type PublicArticle struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	Cover       string `json:"cover"`
	WechatURL   string `json:"wechatUrl"`
	Source      string `json:"source"`
	Author      string `json:"author"`
	ChapterID   string `json:"chapterId"`
	ChapterName string `json:"chapterName"`
	PublishTime int64  `json:"publishTime"`
}

func (s *ArticleService) ListPublicArticles(ctx context.Context, chapterID string, page, pageSize int64) (*PageResult[PublicArticle], error) {
	page, pageSize = normalizePage(page, pageSize)
	filter := bson.M{
		"deleted":        bson.M{"$ne": true},
		"publish_status": article.StatusPublished,
	}
	if chapterID != "" {
		filter["chapter_id"] = chapterID
	}
	data, total, err := s.ArticleMapper.FindMany(ctx, filter, offset(page, pageSize), pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]PublicArticle, 0, len(data))
	for _, item := range data {
		items = append(items, s.mapPublicArticle(ctx, item))
	}
	return &PageResult[PublicArticle]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *ArticleService) GetPublicArticle(ctx context.Context, id string) (*PublicArticle, error) {
	item, err := s.ArticleMapper.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.Deleted || item.PublishStatus != article.StatusPublished {
		return nil, ErrAdminNotFound
	}
	result := s.mapPublicArticle(ctx, item)
	return &result, nil
}

func (s *ArticleService) mapPublicArticle(ctx context.Context, item *article.Article) PublicArticle {
	chapterName := ""
	if item.ChapterID != "" && s.ChapterMapper != nil {
		if ch, err := s.ChapterMapper.FindByID(ctx, item.ChapterID); err == nil {
			chapterName = ch.Name
		}
	}
	return PublicArticle{
		ID:          item.ID.Hex(),
		Title:       item.Title,
		Summary:     item.Summary,
		Cover:       item.Cover,
		WechatURL:   item.WechatURL,
		Source:      item.Source,
		Author:      item.Author,
		ChapterID:   item.ChapterID,
		ChapterName: chapterName,
		PublishTime: timeToUnix(item.PublishTime),
	}
}
