package data

import (
	"context"
	"errors"
	"fmt"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/db"
)

type databaseElasticsearchRepo struct {
	t   *gotext.Locale
	orm *gorm.DB
}

func NewDatabaseElasticsearchRepo(db *gorm.DB, t *gotext.Locale) biz.DatabaseElasticsearchRepo {
	return &databaseElasticsearchRepo{
		t:   t,
		orm: db,
	}
}

func (r *databaseElasticsearchRepo) Indices(ctx context.Context, req *request.DatabaseESIndices) ([]db.ESIndex, error) {
	client, err := r.getClient(ctx, req.ServerID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	return client.Indices(ctx)
}

func (r *databaseElasticsearchRepo) IndexCreate(ctx context.Context, req *request.DatabaseESIndexCreate) error {
	client, err := r.getClient(ctx, req.ServerID)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.IndexCreate(ctx, req.Name)
}

func (r *databaseElasticsearchRepo) IndexDelete(ctx context.Context, req *request.DatabaseESIndexDelete) error {
	client, err := r.getClient(ctx, req.ServerID)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.IndexDelete(ctx, req.Name)
}

func (r *databaseElasticsearchRepo) Data(
	ctx context.Context,
	req *request.DatabaseESData,
) ([]db.ESDocument, int64, error) {
	client, err := r.getClient(ctx, req.ServerID)
	if err != nil {
		return nil, 0, err
	}
	defer client.Close()

	return client.Search(ctx, req.Index, req.Search, int(req.Page), int(req.Limit))
}

func (r *databaseElasticsearchRepo) DocumentGet(
	ctx context.Context,
	req *request.DatabaseESDocumentGet,
) (*db.ESDocument, error) {
	client, err := r.getClient(ctx, req.ServerID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	return client.DocumentGet(ctx, req.Index, req.ID)
}

func (r *databaseElasticsearchRepo) DocumentSet(ctx context.Context, req *request.DatabaseESDocumentSet) error {
	client, err := r.getClient(ctx, req.ServerID)
	if err != nil {
		return err
	}
	defer client.Close()

	if req.ID == "" {
		return client.DocumentCreate(ctx, req.Index, req.Body)
	}
	return client.DocumentUpdate(ctx, req.Index, req.ID, req.Body)
}

func (r *databaseElasticsearchRepo) DocumentDelete(ctx context.Context, req *request.DatabaseESDocumentDelete) error {
	client, err := r.getClient(ctx, req.ServerID)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.DocumentDelete(ctx, req.Index, req.ID)
}

// getClient 根据服务器 ID 创建 Elasticsearch 客户端
func (r *databaseElasticsearchRepo) getClient(ctx context.Context, serverID uint) (*db.Elasticsearch, error) {
	server := new(biz.DatabaseServer)
	if err := r.orm.Where("id = ?", serverID).First(server).Error; err != nil {
		return nil, errors.New(r.t.Get("server not found"))
	}
	if server.Type != biz.DatabaseTypeElasticsearch {
		return nil, errors.New(r.t.Get("server is not Elasticsearch type"))
	}

	client, err := db.NewElasticsearch(ctx, fmt.Sprintf("%s:%d", server.Host, server.Port), server.Username, server.Password)
	if err != nil {
		return nil, errors.New(r.t.Get("failed to connect to Elasticsearch: %v", err))
	}

	return client, nil
}
