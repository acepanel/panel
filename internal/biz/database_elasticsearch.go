package biz

import (
	"context"
	"log/slog"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/db"
)

type DatabaseElasticsearchRepo interface {
	Indices(ctx context.Context, req *request.DatabaseESIndices) ([]db.ESIndex, error)
	IndexCreate(ctx context.Context, req *request.DatabaseESIndexCreate) error
	IndexDelete(ctx context.Context, req *request.DatabaseESIndexDelete) error
	Data(ctx context.Context, req *request.DatabaseESData) ([]db.ESDocument, int64, error)
	DocumentGet(ctx context.Context, req *request.DatabaseESDocumentGet) (*db.ESDocument, error)
	DocumentSet(ctx context.Context, req *request.DatabaseESDocumentSet) error
	DocumentDelete(ctx context.Context, req *request.DatabaseESDocumentDelete) error
}

// DatabaseElasticsearchUsecase Elasticsearch 业务用例
type DatabaseElasticsearchUsecase struct {
	t    *gotext.Locale
	log  *slog.Logger
	repo DatabaseElasticsearchRepo
}

func NewDatabaseElasticsearchUsecase(t *gotext.Locale, log *slog.Logger, repo DatabaseElasticsearchRepo) *DatabaseElasticsearchUsecase {
	return &DatabaseElasticsearchUsecase{t: t, log: log, repo: repo}
}

func (uc *DatabaseElasticsearchUsecase) Indices(ctx context.Context, req *request.DatabaseESIndices) ([]db.ESIndex, error) {
	indices, err := uc.repo.Indices(ctx, req)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("elasticsearch index list viewed"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("server_id", uint64(req.ServerID)))
	return indices, nil
}

func (uc *DatabaseElasticsearchUsecase) IndexCreate(ctx context.Context, req *request.DatabaseESIndexCreate) error {
	if err := uc.repo.IndexCreate(ctx, req); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("elasticsearch index created"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("index", req.Name), slog.Uint64("server_id", uint64(req.ServerID)))
	return nil
}

func (uc *DatabaseElasticsearchUsecase) IndexDelete(ctx context.Context, req *request.DatabaseESIndexDelete) error {
	if err := uc.repo.IndexDelete(ctx, req); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("elasticsearch index deleted"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("index", req.Name), slog.Uint64("server_id", uint64(req.ServerID)))
	return nil
}

func (uc *DatabaseElasticsearchUsecase) Data(ctx context.Context, req *request.DatabaseESData) ([]db.ESDocument, int64, error) {
	items, total, err := uc.repo.Data(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	uc.log.Info(uc.t.Get("elasticsearch document list viewed"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("index", req.Index), slog.String("search", req.Search), slog.Uint64("page", uint64(req.Page)), slog.Uint64("server_id", uint64(req.ServerID)))
	return items, total, nil
}

func (uc *DatabaseElasticsearchUsecase) DocumentGet(ctx context.Context, req *request.DatabaseESDocumentGet) (*db.ESDocument, error) {
	doc, err := uc.repo.DocumentGet(ctx, req)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("elasticsearch document viewed"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("index", req.Index), slog.String("id", req.ID), slog.Uint64("server_id", uint64(req.ServerID)))
	return doc, nil
}

func (uc *DatabaseElasticsearchUsecase) DocumentSet(ctx context.Context, req *request.DatabaseESDocumentSet) error {
	if err := uc.repo.DocumentSet(ctx, req); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("elasticsearch document saved"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("index", req.Index), slog.String("id", req.ID), slog.Uint64("server_id", uint64(req.ServerID)))
	return nil
}

func (uc *DatabaseElasticsearchUsecase) DocumentDelete(ctx context.Context, req *request.DatabaseESDocumentDelete) error {
	if err := uc.repo.DocumentDelete(ctx, req); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("elasticsearch document deleted"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("index", req.Index), slog.String("id", req.ID), slog.Uint64("server_id", uint64(req.ServerID)))
	return nil
}
